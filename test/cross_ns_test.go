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

	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
)

var _ = Describe("Cross-namespace refs", func() {
	var (
		otherns                    corev1.Namespace
		tmpDir, gitDir, backendDir string
		kubeconfig                 string
		stack                      pulumiv1.Stack
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "pulumi-test")
		Expect(err).ToNot(HaveOccurred())

		kubeconfig = writeKubeconfig(tmpDir)

		backendDir = filepath.Join(tmpDir, "state")
		Expect(os.Mkdir(backendDir, 0777)).To(Succeed())

		gitDir = filepath.Join(tmpDir, "repo")
		Expect(makeFixtureIntoRepo(gitDir, "testdata/success")).To(Succeed())

		// actually create the referenced secret
		otherns = corev1.Namespace{}
		otherns.Name = randString()
		Expect(k8sClient.Create(context.TODO(), &otherns)).To(Succeed())
		secret := corev1.Secret{
			StringData: map[string]string{
				"key": "value",
			},
		}
		secret.Name = "secret"
		secret.Namespace = otherns.Name
		Expect(k8sClient.Create(context.TODO(), &secret)).To(Succeed())
	})

	AfterEach(func() {
		deleteAndWaitForFinalization(&stack)
		Expect(k8sClient.Delete(context.TODO(), &otherns)).To(Succeed())
		if strings.HasPrefix(tmpDir, os.TempDir()) {
			os.RemoveAll(tmpDir)
		}
	})

	It("should stall when a cross-namespace reference is used", func() {
		stack = pulumiv1.Stack{
			Spec: shared.StackSpec{
				Stack:   "cross-ns-denied",
				Backend: fmt.Sprintf("file://%s", backendDir),
				GitSource: &shared.GitSource{
					ProjectRepo: gitDir,
					Branch:      "default",
				},
				EnvRefs: map[string]shared.ResourceRef{
					"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
					"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
					// this is the offending reference:
					"CROSS_NAMESPACE": shared.NewSecretResourceRef(otherns.Name, "secret", "key"),
				},
			},
		}
		stack.Name = "test-cross-ns"
		stack.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

		// wait until the controller has seen the stack object and completed processing it
		waitForStackFailure(&stack)
		Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.StalledCondition)).To(BeTrue())
		Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
	})

	When("namespace isolation is waived", func() {
		BeforeEach(func() {
			os.Setenv("INSECURE_NO_NAMESPACE_ISOLATION", "true")
		})
		AfterEach(func() {
			os.Setenv("INSECURE_NO_NAMESPACE_ISOLATION", "")
		})

		It("should succeed despite a cross-namespace ref", func() {
			stack = pulumiv1.Stack{
				Spec: shared.StackSpec{
					Stack:   "cross-ns-ok",
					Backend: fmt.Sprintf("file://%s", backendDir),
					GitSource: &shared.GitSource{
						ProjectRepo: gitDir,
						Branch:      "default",
					},
					EnvRefs: map[string]shared.ResourceRef{
						"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
						"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
						// this is the offending reference:
						"CROSS_NAMESPACE": shared.NewSecretResourceRef(otherns.Name, "secret", "key"),
					},
				},
			}
			stack.Name = "test-cross-ns-ok"
			stack.Namespace = "default"

			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

			// wait until the controller has seen the stack object and completed processing it
			waitForStackSuccess(&stack)
			Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeTrue())
		})
	})
})
