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
	yaml "sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Creating a YAML program", Ordered, func() {
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
	})

	When("Using a stack", Ordered, func() {

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
			prog := programFromFile("./testdata/test-program-invalid.yaml")
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
			prog := programFromFile("./testdata/test-program.yaml")
			Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

			stack.Spec.ProgramRef = &shared.ProgramReference{
				Name: prog.Name,
			}

			// Set DestroyOnFinalize to clean up the configmap for repeat runs.
			stack.Spec.DestroyOnFinalize = true
			stack.Name = "ok-program-" + randString()
			Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())

			waitForStackSuccess(&stack)
			expectReady(stack.Status.Conditions)

			var c corev1.ConfigMap
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Namespace: stack.Namespace, Name: "test-configmap-valid-test"}, &c)).To(Succeed())
		})

		When("the program is changed", Ordered, func() {
			var revisionOnFirstSuccess string

			BeforeEach(func() {
				prog := programFromFile("./testdata/test-program.yaml")
				Expect(k8sClient.Create(context.TODO(), &prog)).To(Succeed())

				stack.Spec.ProgramRef = &shared.ProgramReference{
					Name: prog.Name,
				}

				// Set DestroyOnFinalize to clean up the configmap for repeat runs.
				stack.Spec.DestroyOnFinalize = true
				stack.Name = "changing-program-" + randString()
				Expect(k8sClient.Create(context.TODO(), &stack)).To(Succeed())
				waitForStackSuccess(&stack)

				Expect(stack.Status.LastUpdate).ToNot(BeNil())
				revisionOnFirstSuccess = stack.Status.LastUpdate.LastSuccessfulCommit
				fmt.Fprintf(GinkgoWriter, ".status.lastUpdate.LastSuccessfulCommit before changing program: %s", revisionOnFirstSuccess)
				prog2 := programFromFile("./testdata/test-program-changed.yaml")
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
