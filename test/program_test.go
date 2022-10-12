// Copyright 2022, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	yaml "sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Creating a YAML program", func() {
	It("is possible to create an empty YAML program", func() {
		prog := pulumiv1.Program{}
		prog.Name = randString()
		prog.Namespace = "default"
		Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())
	})

	It("should fail to create a YAML program lacking required fields", func() {
		prog := pulumiv1.Program{
			Program: pulumiv1.ProgramSpec{
				Resources: map[string]pulumiv1.Resource{
					"abc": {
						// Do not specify required Type field.
						Options: &pulumiv1.Options{
							Import: "abcdef",
						},
					},
				},
			},
		}
		prog.Name = randString()
		prog.Namespace = "default"
		Expect(k8sClient.Create(context.TODO(), &prog)).NotTo(Succeed())
	})

	It("should be possible to add a Program to a Stack", func() {
		prog := pulumiv1.Program{}
		prog.Name = randString()
		prog.Namespace = "default"
		Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

		stack := pulumiv1.Stack{
			Spec: shared.StackSpec{
				ProgramRef: &shared.ProgramReference{
					Name: prog.Name,
				},
			},
		}
		stack.Name = randString()
		stack.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())
	})

	When("Using a stack", func() {

		var tmpDir, backendDir string
		var kubeconfig string

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "pulumi-test")
			Expect(err).ToNot(HaveOccurred())

			kubeconfig = writeKubeconfig(tmpDir)

			backendDir = filepath.Join(tmpDir, "state")
			Expect(os.Mkdir(backendDir, 0777)).To(Succeed())
		})

		AfterEach(func() {
			if strings.HasPrefix(tmpDir, os.TempDir()) {
				os.RemoveAll(tmpDir)
			}
		})

		It("should run a Program that has been added to a Stack", func() {
			prog := programFromFile("./testdata/test-program.yaml")
			Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

			stack := pulumiv1.Stack{
				Spec: shared.StackSpec{
					Stack:   randString(),
					Backend: fmt.Sprintf("file://%s", backendDir),
					ProgramRef: &shared.ProgramReference{
						Name: prog.Name,
					},
					EnvRefs: map[string]shared.ResourceRef{
						"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
						"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
					},
					DestroyOnFinalize: true,
				},
			}
			stack.Name = randString()
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

			var c corev1.ConfigMap
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: stack.Namespace, Name: "test-configmap"}, &c)).To(Succeed())

			Expect(s.Status.LastUpdate.State).To(Equal(shared.SucceededStackStateMessage))
			Expect(apimeta.IsStatusConditionTrue(s.Status.Conditions, pulumiv1.ReadyCondition)).To(BeTrue())

			// Clean up the ConfigMap for future runs.
			Expect(k8sClient.Delete(context.TODO(), &c)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: c.Name, Namespace: namespace}, &c)
				return k8serrors.IsNotFound(err)
			}, k8sOpTimeout, interval).Should(BeTrue())
		})

		It("should fail if given a non-existent program.", func() {
			stack := pulumiv1.Stack{
				Spec: shared.StackSpec{
					Stack:   randString(),
					Backend: fmt.Sprintf("file://%s", backendDir),
					ProgramRef: &shared.ProgramReference{
						Name: randString(),
					},
					EnvRefs: map[string]shared.ResourceRef{
						"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
						"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
					},
				},
			}
			stack.Name = randString()
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
			Expect(apimeta.IsStatusConditionFalse(s.Status.Conditions, pulumiv1.ReadyCondition)).To(BeTrue())
		})

		It("should fail if given a syntactically correct but invalid program.", func() {
			prog := programFromFile("./testdata/test-program-invalid.yaml")
			Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

			stack := pulumiv1.Stack{
				Spec: shared.StackSpec{
					Stack:   randString(),
					Backend: fmt.Sprintf("file://%s", backendDir),
					ProgramRef: &shared.ProgramReference{
						Name: prog.Name,
					},
					EnvRefs: map[string]shared.ResourceRef{
						"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
						"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
					},
				},
			}
			stack.Name = randString()
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
			Expect(apimeta.IsStatusConditionFalse(s.Status.Conditions, pulumiv1.ReadyCondition)).To(BeTrue())
		})
	})

})

func programFromFile(path string) pulumiv1.Program {
	prog := pulumiv1.Program{}
	prog.Name = randString()
	prog.Namespace = "default"

	programFile, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("%+v", err)
		Fail(fmt.Sprintf("Couldn't read program file: %v", err))
	}
	err = yaml.Unmarshal(programFile, &prog.Program)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	return prog
}
