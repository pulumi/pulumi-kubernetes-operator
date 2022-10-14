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
		var stack pulumiv1.Stack

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "pulumi-test")
			Expect(err).ToNot(HaveOccurred())

			kubeconfig = writeKubeconfig(tmpDir)

			backendDir = filepath.Join(tmpDir, "state")
			Expect(os.Mkdir(backendDir, 0777)).To(Succeed())

			stack = pulumiv1.Stack{
				Spec: shared.StackSpec{
					Stack:   randString(),
					Backend: fmt.Sprintf("file://%s", backendDir),
					EnvRefs: map[string]shared.ResourceRef{
						"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
						"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
					},
				},
			}
			stack.Name = randString()
			stack.Namespace = "default"
		})

		AfterEach(func() {
			if strings.HasPrefix(tmpDir, os.TempDir()) {
				os.RemoveAll(tmpDir)
			}
			Expect(k8sClient.Delete(context.TODO(), &stack)).To(Succeed())
		})

		It("should fail if given a non-existent program.", func() {

			stack.Spec.ProgramRef = &shared.ProgramReference{
				Name: randString(),
			}

			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

			waitForStackFailure(&stack)

			Expect(stack.Status.LastUpdate.State).To(Equal(shared.FailedStackStateMessage))
			stalledCondition := apimeta.FindStatusCondition(stack.Status.Conditions, pulumiv1.StalledCondition)
			Expect(stalledCondition).ToNot(BeNil(), "stalled condition is present")
			Expect(stalledCondition.Reason).To(Equal(pulumiv1.StalledSourceUnavailableReason))
			Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
		})

		It("should fail if given a syntactically correct but invalid program.", func() {
			prog := programFromFile("./testdata/test-program-invalid.yaml")
			Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

			stack.Spec.ProgramRef = &shared.ProgramReference{
				Name: prog.Name,
			}

			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

			waitForStackFailure(&stack)

			Expect(stack.Status.LastUpdate.State).To(Equal(shared.FailedStackStateMessage))
			Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeFalse())
		})

		It("should run a Program that has been added to a Stack", func() {
			prog := programFromFile("./testdata/test-program.yaml")
			Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

			stack.Spec.ProgramRef = &shared.ProgramReference{
				Name: prog.Name,
			}

			// Set DestroyOnFinalize to clean up the configmap for repeat runs.
			stack.Spec.DestroyOnFinalize = true

			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

			waitForStackSuccess(&stack)

			var c corev1.ConfigMap
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: stack.Namespace, Name: "test-configmap"}, &c)).To(Succeed())

			Expect(stack.Status.LastUpdate.State).To(Equal(shared.SucceededStackStateMessage))
			Expect(apimeta.IsStatusConditionTrue(stack.Status.Conditions, pulumiv1.ReadyCondition)).To(BeTrue())
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
		Fail(fmt.Sprintf("couldn't read program file: %v", err))
	}
	err = yaml.Unmarshal(programFile, &prog.Program)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	return prog
}
