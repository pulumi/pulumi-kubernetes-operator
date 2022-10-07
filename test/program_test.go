// Copyright 2021, Pulumi Corporation.  All rights reserved.

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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"

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

		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "pulumi-test")
			Expect(err).ToNot(HaveOccurred())

			backendDir = filepath.Join(tmpDir, "state")
			Expect(os.Mkdir(backendDir, 0777)).To(Succeed())
		})

		AfterEach(func() {
			if strings.HasPrefix(tmpDir, os.TempDir()) {
				os.RemoveAll(tmpDir)
			}
		})

		It("should run a Program that has been added to a Stack", func() {
			prog := pulumiv1.Program{}
			prog.Name = randString()
			prog.Namespace = "default"
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

			Expect(s.Status.LastUpdate.State).To(Equal(shared.SucceededStackStateMessage))
			Expect(apimeta.IsStatusConditionTrue(s.Status.Conditions, pulumiv1.ReadyCondition)).To(BeTrue())
		})
	})

})
