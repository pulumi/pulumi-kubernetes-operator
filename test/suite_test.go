// Copyright 2021, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"encoding/base32"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	// Used to auth against GKE clusters that use gcloud creds.
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	apis "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis"
	controller "github.com/pulumi/pulumi-kubernetes-operator/pkg/controller/stack"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.SlowSpecThreshold = 20 * time.Second
	RunSpecs(t, "Controller Suite", suiteConfig, reporterConfig)
}

var secretsDir string
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	rand.Seed(time.Now().UnixNano())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "deploy", "crds")},
	}

	var err error
	cfg, err = testEnv.Start()
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

// randString returns a short random string that can be used in names.
func randString() string {
	c := 10
	b := make([]byte, c)
	rand.Read(b)
	length := 6
	return strings.ToLower(base32.StdEncoding.EncodeToString(b)[:length])
}

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

func removeTempDir(tmp string) {
	if strings.HasPrefix(tmp, os.TempDir()) {
		os.RemoveAll(tmp)
	}
}

// These waitForStack* are convenience funcs for what most tests will want to do, that is wait until
// a stack succeeds or fails after setting up the circumstances under test.

var waitForStackSince time.Time

func resetWaitForStack() {
	waitForStackSince = time.Now()
}

// internalWaitForStackState refetches the given stack until its status has been updated more
// recently than waitForStackSince; then asserts that state reached is `state`. Usually, you will
// want to resetWaitForStack(), then update a stack, then use this func. NB it fetches into the
// pointer given, so mutates the struct it's pointing at.
func internalWaitForStackState(stack *pulumiv1.Stack, state shared.StackUpdateStateMessage, optionalTimeout ...string) {
	timeout := "30s"
	if len(optionalTimeout) > 0 {
		timeout = optionalTimeout[0]
	}
	EventuallyWithOffset(2 /* called by other helpers */, func() bool {
		k := client.ObjectKeyFromObject(stack)
		err := k8sClient.Get(context.TODO(), k, stack)
		if err != nil {
			return false
		}
		return stack.Status.LastUpdate != nil &&
			stack.Status.LastUpdate.LastResyncTime.Time.After(waitForStackSince)
	}, timeout, "1s").Should(BeTrue(), fmt.Sprintf("stack %q has run", stack.Name))
	ExpectWithOffset(2, stack.Status.LastUpdate.State).To(Equal(state))
}

func waitForStackSuccess(stack *pulumiv1.Stack, optionalTimeout ...string) {
	internalWaitForStackState(stack, shared.SucceededStackStateMessage, optionalTimeout...)
}

func waitForStackFailure(stack *pulumiv1.Stack, optionalTimeout ...string) {
	internalWaitForStackState(stack, shared.FailedStackStateMessage, optionalTimeout...)
}

// Get the object's latest definition from the Kubernetes API
func refetch(s *pulumiv1.Stack) {
	ExpectWithOffset(1, k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(s), s)).To(Succeed())
}

// deleteAndWaitForFinalization removes the stack object, and waits until requesting it gives a "not
// found" result, indicating that finalizers have been run. The object at *stack is invalidated.
func deleteAndWaitForFinalization(obj client.Object) {
	ExpectWithOffset(1, k8sClient.Delete(context.TODO(), obj)).To(Succeed())
	key := client.ObjectKeyFromObject(obj)
	EventuallyWithOffset(1, func() bool {
		err := k8sClient.Get(context.TODO(), key, obj)
		if err == nil {
			return false
		}
		ExpectWithOffset(2, client.IgnoreNotFound(err)).To(BeNil())
		return true
	}, "2m", "5s").Should(BeTrue())
}

func expectReady(conditions []metav1.Condition) {
	ExpectWithOffset(1, apimeta.IsStatusConditionTrue(conditions, pulumiv1.ReadyCondition)).To(BeTrue(), "Ready condition is true")
	ExpectWithOffset(1, apimeta.FindStatusCondition(conditions, pulumiv1.StalledCondition)).To(BeNil(), "Stalled condition absent")
	ExpectWithOffset(1, apimeta.FindStatusCondition(conditions, pulumiv1.ReconcilingCondition)).To(BeNil(), "Reconciling condition absent")
}

func expectStalled(conditions []metav1.Condition) {
	ExpectWithOffset(1, apimeta.IsStatusConditionTrue(conditions, pulumiv1.StalledCondition)).To(BeTrue(), "Stalled condition true")
	ExpectWithOffset(1, apimeta.IsStatusConditionTrue(conditions, pulumiv1.ReadyCondition)).To(BeFalse(), "Ready condition false")
	ExpectWithOffset(1, apimeta.FindStatusCondition(conditions, pulumiv1.ReconcilingCondition)).To(BeNil(), "Reconciling condition absent")
}

func expectStalledWithReason(conditions []metav1.Condition, reason string) {
	stalledCondition := apimeta.FindStatusCondition(conditions, pulumiv1.StalledCondition)
	ExpectWithOffset(1, stalledCondition).ToNot(BeNil(), "Stalled condition is present")
	ExpectWithOffset(1, stalledCondition.Reason).To(Equal(reason), "Stalled reason is as given")
	// not ready, and not in progress
	ExpectWithOffset(1, apimeta.IsStatusConditionTrue(conditions, pulumiv1.ReadyCondition)).To(BeFalse(), "Ready condition is false")
	ExpectWithOffset(1, apimeta.FindStatusCondition(conditions, pulumiv1.ReconcilingCondition)).To(BeNil(), "Reconciling condition is absent")
}

func expectInProgress(conditions []metav1.Condition) {
	ExpectWithOffset(1, apimeta.IsStatusConditionTrue(conditions, pulumiv1.ReconcilingCondition)).To(BeTrue(), "Reconciling condition is true")
	ExpectWithOffset(1, apimeta.IsStatusConditionTrue(conditions, pulumiv1.ReadyCondition)).To(BeFalse(), "Ready condition is false")
	ExpectWithOffset(1, apimeta.FindStatusCondition(conditions, pulumiv1.StalledCondition)).To(BeNil(), "Stalled condition is absent")
}

// The keys of fixtureIgnore are filenames to ignore when making adding files to a repo from a
// fixture directory.
var fixtureIgnore = map[string]struct{}{
	"node_modules": struct{}{},
}

// makeFixtureIntoRepo creates a git repo in `repoDir` given a path `fixture` to a directory of
// files to be committed to the repo.
func makeFixtureIntoRepo(repoDir, fixture string) error {
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		return err
	}

	if err := filepath.Walk(fixture, func(fullpath string, info os.FileInfo, err error) error {
		if fullpath == fixture {
			return nil
		}
		if _, ok := fixtureIgnore[info.Name()]; ok {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		path := fullpath[len(fixture)+1:] // trim fixture and the slash following
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(repoDir, path), info.Mode())
		}
		// copy symlinks as-is, so I can test what happens with broken symlinks
		if info.Mode()&os.ModeSymlink > 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, filepath.Join(repoDir, path))
		}

		source, err := os.Open(fullpath)
		if err != nil {
			return err
		}
		defer source.Close()

		target, err := os.Create(filepath.Join(repoDir, path))
		if err != nil {
			return err
		}
		defer target.Close()

		_, err = io.Copy(target, source)
		return err
	}); err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	wt.Add(".")
	wt.Commit("Initial revision from fixture "+fixture, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Pulumi Test",
			Email: "pulumi.test@example.com",
		},
	})
	// this makes sure there's a default branch
	if err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("default"),
		Create: true,
	}); err != nil {
		return err
	}

	return nil
}
