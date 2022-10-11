// Copyright 2021, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	// Used to auth against GKE clusters that use gcloud creds.
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	st "github.com/onsi/ginkgo/reporters/stenographer"
	apis "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis"
	controller "github.com/pulumi/pulumi-kubernetes-operator/pkg/controller/stack"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"

	// Needed for kubebuilder to insert imports for api versions.
	// https://book.kubebuilder.io/cronjob-tutorial/empty-main.html
	// https://github.com/kubernetes-sigs/kubebuilder/issues/1487
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment
var shutdownController func()

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	stenographer := st.NewFakeStenographer()
	reporterConfig := config.DefaultReporterConfigType{
		NoColor:           false,
		SlowSpecThreshold: 0.1,
		NoisyPendings:     false,
		NoisySkippings:    false,
		Verbose:           true,
		FullTrace:         true,
	}

	reporter := reporters.NewDefaultReporter(reporterConfig, stenographer)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}, reporter})
}

var secretsDir string
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "deploy", "crds")},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = apis.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		HealthProbeBindAddress: "0",
		MetricsBindAddress:     "0",
	})
	Expect(err).ToNot(HaveOccurred())

	err = controller.Add(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()
	shutdownController = cancel

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	By("Creating directory to store secrets")
	secretsDir, err = os.MkdirTemp("", "secrets")
	if err != nil {
		Fail("Failed to create secret temp directory")
	}
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if shutdownController != nil {
		shutdownController()
	}
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())

	if secretsDir != "" {
		os.RemoveAll(secretsDir)
	}
})

// writeKubeconfig is a convenience for anything which needs to use a kubeconfig file pointing at
// the envtest API server -- e.g., a Stack that uses the Kubernetes provider, or an exec.Command
// that needs to run against the envtest API server.
func writeKubeconfig(targetDir string) string {
	// Create a user and write its kubeconfig out for stacks to use
	user, err := testEnv.AddUser(envtest.User{Name: "testuser", Groups: []string{"system:masters"}}, nil)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	kc, err := user.KubeConfig()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	kubeconfig := filepath.Join(targetDir, "kube.config")
	ExpectWithOffset(1, os.WriteFile(kubeconfig, kc, 0666)).To(Succeed())
	return kubeconfig
}

// These waitForStack* are convenience funcs for what most tests will want to do, that is wait until
// a stack succeeds or fails after setting up the circumstances under test.

var waitForStackSince time.Time

func resetWaitForStack() {
	waitForStackSince = time.Now()
}

// internalWaitForStackState refetches the given stack, until its .lastUpdated.state field matches
// the one given. NB it fetches into the pointer given, so mutates the struct it's pointing at.
func internalWaitForStackState(stack *pulumiv1.Stack, state shared.StackUpdateStateMessage) {
	EventuallyWithOffset(2 /* called by other helpers */, func() bool {
		k := client.ObjectKeyFromObject(stack)
		err := k8sClient.Get(context.TODO(), k, stack)
		if err != nil {
			return false
		}
		return stack.Status.LastUpdate != nil &&
			stack.Status.LastUpdate.State == state &&
			stack.Status.LastUpdate.LastResyncTime.Time.After(waitForStackSince)
	}, "30s", "1s").Should(BeTrue(), fmt.Sprintf("stack %q reaches state %q", stack.Name, state))
}

func waitForStackSuccess(stack *pulumiv1.Stack) {
	internalWaitForStackState(stack, shared.SucceededStackStateMessage)
}

func waitForStackFailure(stack *pulumiv1.Stack) {
	internalWaitForStackState(stack, shared.FailedStackStateMessage)
}

// Get the object's latest definition from the Kubernetes API
func refetch(s *pulumiv1.Stack) {
	ExpectWithOffset(1, k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(s), s)).To(Succeed())
}
