// Copyright 2021, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	// Used to auth against GKE clusters that use gcloud creds.
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	apis "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis"
	controller "github.com/pulumi/pulumi-kubernetes-operator/pkg/controller/stack"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
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
	RunSpecs(t, "Controller Suite")
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
})

var _ = AfterSuite(func() {
	var stacks pulumiv1.StackList
	Expect(k8sClient.List(context.TODO(), &stacks, client.InNamespace(namespace))).To(Succeed())
	fmt.Fprintln(GinkgoWriter, "=== stacks remaining undeleted ===")
	for i := range stacks.Items {
		fmt.Fprintln(GinkgoWriter, stacks.Items[i].Name)
	}
	fmt.Fprintln(GinkgoWriter, "===           e n d            ===")

	By("tearing down the test environment")
	if shutdownController != nil {
		shutdownController()
	}
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())

	if secretsDir != "" {
		os.RemoveAll(secretsDir)
	}

	if len(stacks.Items) > 0 {
		Fail("stacks remain undeleted")
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

// deleteAndWaitForFinalization removes the stack object, and waits until requesting it gives a "not
// found" result, indicating that finalizers have been run. The object at *stack is invalidated.
func deleteAndWaitForFinalization(stack *pulumiv1.Stack) {
	ExpectWithOffset(1, k8sClient.Delete(context.TODO(), stack)).To(Succeed())
	key := client.ObjectKeyFromObject(stack)
	EventuallyWithOffset(1, func() bool {
		err := k8sClient.Get(context.TODO(), key, stack)
		if err == nil {
			return false
		}
		ExpectWithOffset(2, client.IgnoreNotFound(err)).To(BeNil())
		return true
	}, "2m", "5s").Should(BeTrue())
}
