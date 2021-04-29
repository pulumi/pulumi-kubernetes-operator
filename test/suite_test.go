// Copyright 2021, Pulumi Corporation.  All rights reserved.

package tests

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

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
	// Needed for kubebuilder to insert imports for api versions.
	// https://book.kubebuilder.io/cronjob-tutorial/empty-main.html
	// https://github.com/kubernetes-sigs/kubebuilder/issues/1487
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment

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
var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	t := true
	testEnv = &envtest.Environment{
		UseExistingCluster: &t,
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

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	By("Creating directory to store secrets")
	secretsDir, err = os.MkdirTemp("", "secrets")
	if err != nil {
		Fail("Failed to create secret temp directory")
	}

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())

	if secretsDir != "" {
		os.RemoveAll(secretsDir)
	}
})
