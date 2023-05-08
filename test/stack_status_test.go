package tests

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Stack controller status", func() {
	var (
		tmpDir             string
		gitDir, backendDir string
		kubeconfig         string
		stack              pulumiv1.Stack
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "pulumi-test")
		Expect(err).ToNot(HaveOccurred())

		// This makes a git repo to clone from, so to avoid relying on something at GitHub that could
		// change or be inaccessible.
		gitDir = filepath.Join(tmpDir, "repo")
		backendDir = filepath.Join(tmpDir, "state")
		Expect(os.Mkdir(backendDir, 0777)).To(Succeed())
		kubeconfig = writeKubeconfig(tmpDir)
	})

	AfterEach(func() {
		if stack.Name != "" { // assume that if it's been named, it was created in the cluster
			deleteAndWaitForFinalization(&stack)
		}
		if strings.HasPrefix(tmpDir, os.TempDir()) {
			os.RemoveAll(tmpDir)
		}
	})

	It("should mark a stack as ready after a successful run", func() {
		Expect(makeFixtureIntoRepo(gitDir, "testdata/success")).To(Succeed())

		stack = pulumiv1.Stack{
			Spec: shared.StackSpec{
				Stack:   "test",
				Backend: fmt.Sprintf("file://%s", backendDir),
				GitSource: &shared.GitSource{
					ProjectRepo: gitDir,
					RepoDir:     "testdata/success",
					Branch:      "default",
				},
				EnvRefs: map[string]shared.ResourceRef{
					"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
					"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
				},
			},
		}
		stack.Name = "testsuccess"
		stack.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

		// wait until the controller has seen the stack object and completed processing it
		waitForStackSuccess(&stack)
		expectReady(stack.Status.Conditions)
	})

	It("should mark a stack as stalled if it will not complete without intervention", func() {
		stack = pulumiv1.Stack{
			Spec: shared.StackSpec{
				Stack:   "test",
				Backend: fmt.Sprintf("file://%s", backendDir),
				GitSource: &shared.GitSource{
					ProjectRepo: gitDir,
					RepoDir:     "testdata/success", // this would work, but ...
					Branch:      "",                 // )
					Commit:      "",                 // ) ... supplying neither of these makes it invalid
				},
				EnvRefs: map[string]shared.ResourceRef{
					"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
				},
			},
		}
		stack.Name = "teststalled"
		stack.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

		waitForStackFailure(&stack)
		expectStalled(stack.Status.Conditions)
	})

	It("should mark a stack as reconciling while it's being processed", func() {
		Expect(makeFixtureIntoRepo(gitDir, "testdata/success")).To(Succeed())

		stack = pulumiv1.Stack{
			Spec: shared.StackSpec{
				Stack:   "test",
				Backend: fmt.Sprintf("file://%s", backendDir),
				GitSource: &shared.GitSource{
					ProjectRepo: gitDir,
					RepoDir:     "testdata/success",
					Branch:      "default",
				},
				EnvRefs: map[string]shared.ResourceRef{
					"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
				},
			},
		}
		stack.Name = "testprocessing"
		stack.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

		// wait until the controller has seen the stack object, and assume it will take at least a
		// few seconds to process it, so we can see the intermediate status.
		var s pulumiv1.Stack
		Eventually(func() bool {
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: stack.Namespace, Name: stack.Name}, &s)
			if err != nil {
				return false
			}
			return apimeta.IsStatusConditionTrue(s.Status.Conditions, pulumiv1.ReconcilingCondition)
		}, "5s", "0.2s").Should(BeTrue())
		expectInProgress(s.Status.Conditions)
	})

	It("should mark a stack as reconciling if it failed but will retry", func() {
		// there's no git repo (nor directory within), so this will fail at the point that it tries
		// to clone the repo and run the stack. In this case, we expect it to be marked as
		// reconciling, but also marked as observed (i.e., observedGeneration is saved).
		stack = pulumiv1.Stack{
			Spec: shared.StackSpec{
				Stack:   "test",
				Backend: fmt.Sprintf("file://%s", backendDir),
				GitSource: &shared.GitSource{
					ProjectRepo: gitDir,
					RepoDir:     "testdata/doesnotexist",
					Branch:      "default",
				},
				EnvRefs: map[string]shared.ResourceRef{
					"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
				},
			},
		}
		stack.Name = "testretry"
		stack.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

		// wait until the controller has seen the stack object and completed processing it
		var s pulumiv1.Stack
		Eventually(func() bool {
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: stack.Namespace, Name: stack.Name}, &s)
			if err != nil {
				return false
			}
			if s.Generation == 0 {
				return false
			}
			return s.Status.ObservedGeneration == s.Generation
		}, "20s", "1s").Should(BeTrue())
		expectInProgress(s.Status.Conditions)
	})
})
