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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// pulumiYamlProgramCMFmt is a template for a YAML program that creates a ConfigMap. Templated
// values are: the name of the ConfigMap, and the value of the `foo` key in the ConfigMap's `data`
const pulumiYamlProgramCMFmt = `
configuration:
    foo:
        type: "String"
        default: "%s"
resources:
    provider:
        type: pulumi:providers:kubernetes
    example:
        type: kubernetes:core/v1:ConfigMap
        properties:
            metadata:
                annotations:
                    pulumi.com/patchForce: "true"
                name:
                    %s
            data:
                foo: ${foo}
        options:
            provider: ${provider}
`

// invalidPulumiYamlProgramCM is a YAML program that creates a ConfigMap, but with an invalid Pulumi YAML format.
const invalidPulumiYamlProgramCM = `
resources:
    example:
        type: kubernetes:core/v1:ConfigMap
        properties:
            metadata:
                annotations:
                    pulumi.com/patchForce: "true"
                name:
                    %s
            data:
                foo: bar
        options:
            provider: ${provider}
`

// generatePulumiYamlCMProgram generates a Pulumi YAML program that creates a ConfigMap. It accepts a base
// template and values to be templated into the base template.
func generatePulumiYamlCMProgram(template string, values ...interface{}) pulumiv1.Program {
	prog := pulumiv1.Program{}
	prog.Name = randString()
	prog.Namespace = "default"

	templated := fmt.Sprintf(template, values...)
	//
	fmt.Fprintf(GinkgoWriter, "====================\nTemplated: %s\n", templated)

	err := yaml.Unmarshal([]byte(templated), &prog.Program)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())

	return prog
}

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
		stack.Name = "stack-with-program-" + randString()
		stack.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())
		deleteAndWaitForFinalization(&stack)
		deleteAndWaitForFinalization(&prog)
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
					ResyncFrequencySeconds: 3600, // make sure it doesn't run again unless there's another reason to
				},
			}
			// stack name left to test cases
			stack.Namespace = "default"
		})

		AfterEach(func() {
			deleteAndWaitForFinalization(&stack)
			if strings.HasPrefix(tmpDir, os.TempDir()) {
				os.RemoveAll(tmpDir)
			}
		})

		It("should fail if given a non-existent program.", func() {

			stack.Spec.ProgramRef = &shared.ProgramReference{
				Name: randString(),
			}
			stack.Name = "missing-program-" + randString()
			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

			waitForStackFailure(&stack)
			expectStalledWithReason(stack.Status.Conditions, pulumiv1.StalledSourceUnavailableReason)
		})

		It("should fail if given a syntactically correct but invalid program.", func() {
			prog := generatePulumiYamlCMProgram(invalidPulumiYamlProgramCM, "cm-"+randString())
			Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

			stack.Spec.ProgramRef = &shared.ProgramReference{
				Name: prog.Name,
			}
			stack.Name = "invalid-program-" + randString()
			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

			waitForStackFailure(&stack)
			expectInProgress(stack.Status.Conditions)
		})

		It("should run a Program that has been added to a Stack", func() {
			cmName := "cm-" + randString()
			prog := generatePulumiYamlCMProgram(pulumiYamlProgramCMFmt, "bar", cmName)
			Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

			stack.Spec.ProgramRef = &shared.ProgramReference{
				Name: prog.Name,
			}

			stack.Name = "ok-program-" + randString()
			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

			waitForStackSuccess(&stack)
			expectReady(stack.Status.Conditions)

			// Check that the ConfigMap was created.
			var c corev1.ConfigMap
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: stack.Namespace, Name: cmName}, &c)).To(Succeed())
		})

		When("the program is changed", func() {
			var revisionOnFirstSuccess string

			BeforeEach(func() {
				cmName := "changing-cm-" + randString()
				prog := generatePulumiYamlCMProgram(pulumiYamlProgramCMFmt, "bar", cmName)
				Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

				stack.Spec.ProgramRef = &shared.ProgramReference{
					Name: prog.Name,
				}

				stack.Name = "changing-program-" + randString()
				Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())
				waitForStackSuccess(&stack)

				Expect(stack.Status.LastUpdate).ToNot(BeNil())
				revisionOnFirstSuccess = stack.Status.LastUpdate.LastSuccessfulCommit
				fmt.Fprintf(GinkgoWriter, ".status.lastUpdate.LastSuccessfulCommit before changing program: %s", revisionOnFirstSuccess)
				prog2 := generatePulumiYamlCMProgram(pulumiYamlProgramCMFmt, "newValue", cmName)
				prog.Program = prog2.Program
				resetWaitForStack()
				Expect(k8sClient.Update(context.TODO(), &prog)).To(Succeed())
			})

			It("reruns the stack", func() {
				waitForStackSuccess(&stack)
				Expect(stack.Status.LastUpdate).ToNot(BeNil())
				fmt.Fprintf(GinkgoWriter, ".status.lastUpdate.LastSuccessfulCommit after changing program: %s", stack.Status.LastUpdate.LastSuccessfulCommit)
				Expect(stack.Status.LastUpdate.LastSuccessfulCommit).NotTo(Equal(revisionOnFirstSuccess))
			})
		})
	})
})
