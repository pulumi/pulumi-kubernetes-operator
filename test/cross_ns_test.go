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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("With disabled cross-namespace refs", func() {
	var (
		tmpDir, gitDir, backendDir string
		stack                      pulumiv1.Stack
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "pulumi-test")
		Expect(err).ToNot(HaveOccurred())

		backendDir = filepath.Join(tmpDir, "state")
		Expect(os.Mkdir(backendDir, 0777)).To(Succeed())

		gitDir = filepath.Join(tmpDir, "repo")
		makeFixtureIntoRepo(gitDir, "testdata/success")
	})

	AfterEach(func() {
		if strings.HasPrefix(tmpDir, os.TempDir()) {
			os.RemoveAll(tmpDir)
		}
		if stack.Name != "" { // assume that if it's been named, it was created in the cluster
			Expect(k8sClient.Delete(context.TODO(), &stack)).To(Succeed())
		}
	})

	It("should stall when a cross-namespace reference is used", func() {
		stack = pulumiv1.Stack{
			Spec: shared.StackSpec{
				Stack:       "cross-ns",
				Backend:     fmt.Sprintf("file://%s", backendDir),
				ProjectRepo: gitDir,
				Branch:      "default",
				EnvRefs: map[string]shared.ResourceRef{
					"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
					// this is the offending reference:
					"CROSS_NAMESPACE": shared.NewSecretResourceRef("other-ns", "secret", "key"),
				},
			},
		}
		stack.Name = "test-cross-ns"
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

		Expect(s.Status.LastUpdate.State).To(Equal(shared.FailedStackStateMessage))
		Expect(apimeta.IsStatusConditionTrue(s.Status.Conditions, pulumiv1.StalledCondition)).To(BeTrue())
		Expect(apimeta.IsStatusConditionTrue(s.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
	})
})
