// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulumi

import (
	"context"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Program Controller", func() {
	var (
		program               v1.Program
		r                     *ProgramReconciler
		programNamespacedName types.NamespacedName
		advertisedAddress     string
	)

	BeforeEach(func(ctx context.Context) {
		advertisedAddress = "http://fake-svc.fake-namespace"

		r = &ProgramReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: record.NewFakeRecorder(10),
			ProgramHandler: &ProgramHandler{
				k8sClient: k8sClient,
				address:   advertisedAddress,
			},
		}

		programNamespacedName = types.NamespacedName{
			Name:      fmt.Sprintf("test-program-%s", utilrand.String(8)),
			Namespace: "default",
		}

		program = v1.Program{
			ObjectMeta: metav1.ObjectMeta{
				Name:       programNamespacedName.Name,
				Namespace:  programNamespacedName.Namespace,
				Generation: 1,
			},
			Program: v1.ProgramSpec{
				Resources: map[string]v1.Resource{
					"test-resource": {
						Type: "kubernetes:core/v1:Pod",
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &program)).Should(Succeed())
	})

	AfterEach(func(ctx context.Context) {
		Expect(k8sClient.Delete(ctx, &program)).Should(Succeed())
	})

	reconcileFn := func(ctx context.Context) (reconcile.Result, error) {
		return r.Reconcile(ctx, reconcile.Request{NamespacedName: programNamespacedName})
	}

	When("reconciling a resource", func() {
		It("should generate an artifact in the status", func(ctx context.Context) {
			By("not expecting a status to be present before first reconciliation")
			program := v1.Program{}
			Expect(k8sClient.Get(ctx, programNamespacedName, &program)).Should(Succeed())
			Expect(program.Status.Artifact).To(BeNil())
			Expect(program.GetGeneration()).To(Equal(int64(1)))

			By("reconciling a new Program object")
			_, err := reconcileFn(ctx)
			Expect(err).NotTo(HaveOccurred())

			program = v1.Program{}
			Expect(k8sClient.Get(ctx, programNamespacedName, &program)).Should(Succeed())
			Expect(program.Status.Artifact).NotTo(BeNil())
			Expect(program.Status.Artifact.Path).To(Equal(fmt.Sprintf("programs/%s/%s/%d.tar.gz",
				programNamespacedName.Namespace, programNamespacedName.Name, program.Generation)))
			Expect(program.Status.Artifact.URL).To(Equal(fmt.Sprintf("%s/programs/%s/%s/%d", advertisedAddress,
				programNamespacedName.Namespace, programNamespacedName.Name, program.Generation)))
			Expect(program.Status.Artifact.Revision).To(Equal(strconv.FormatInt(program.GetGeneration(), 10)))
			Expect(program.Status.Artifact.Digest).To(MatchRegexp(`^sha256:\w+$`))
			Expect(program.Status.ObservedGeneration).To(Equal(program.GetGeneration()))

			By("reconciling the same Program object but with a new generation")
			program.Program.Resources = map[string]v1.Resource{
				"test-resource": {
					Type: "kubernetes:core/v1:Service",
				},
			}
			Expect(k8sClient.Update(ctx, &program)).Should(Succeed())

			_, err = reconcileFn(ctx)
			Expect(err).NotTo(HaveOccurred())

			program = v1.Program{}
			Expect(k8sClient.Get(ctx, programNamespacedName, &program)).Should(Succeed())
			Expect(program.Status.Artifact).NotTo(BeNil())
			Expect(program.Status.Artifact.Path).To(Equal(fmt.Sprintf("programs/%s/%s/%d.tar.gz",
				programNamespacedName.Namespace, programNamespacedName.Name, program.Generation)))
			Expect(program.Status.Artifact.URL).To(Equal(fmt.Sprintf("%s/programs/%s/%s/%d", advertisedAddress,
				programNamespacedName.Namespace, programNamespacedName.Name, program.Generation)))
			Expect(program.Status.Artifact.Revision).To(Equal(strconv.FormatInt(program.GetGeneration(), 10)))
			Expect(program.Status.Artifact.Digest).To(MatchRegexp(`^sha256:\w+$`))
			Expect(program.Status.ObservedGeneration).To(Equal(program.GetGeneration()))

			By("expecting the status to be updated with the new artifact URL when the host changes")
			advertisedAddress = "https://fake-address"
			r.ProgramHandler.address = advertisedAddress
			_, err = reconcileFn(ctx)
			Expect(err).NotTo(HaveOccurred())

			program = v1.Program{}
			Expect(k8sClient.Get(ctx, programNamespacedName, &program)).Should(Succeed())
			Expect(program.Status.Artifact).NotTo(BeNil())
			Expect(program.Status.Artifact.URL).To(Equal(fmt.Sprintf("https://fake-address/programs/%s/%s/%d",
				programNamespacedName.Namespace, programNamespacedName.Name, program.Generation)))
		})

		It("should generate a URL with the correct 'http://' scheme if the advertised address does not include one", func(ctx context.Context) {
			advertisedAddress = "fake-address"
			r.ProgramHandler.address = advertisedAddress

			By("not expecting a status to be present before first reconciliation")
			program := v1.Program{}
			Expect(k8sClient.Get(ctx, programNamespacedName, &program)).Should(Succeed())
			Expect(program.Status.Artifact).To(BeNil())

			By("reconciling a new Program object")
			_, err := reconcileFn(ctx)
			Expect(err).NotTo(HaveOccurred())

			program = v1.Program{}
			Expect(k8sClient.Get(ctx, programNamespacedName, &program)).Should(Succeed())
			Expect(program.Status.Artifact).NotTo(BeNil())
			Expect(program.Status.Artifact.URL).To(Equal(fmt.Sprintf("http://%s/programs/%s/%s/%d", advertisedAddress,
				programNamespacedName.Namespace, programNamespacedName.Name, program.Generation)))
		})

		It("should include packages in the generated Pulumi.yaml file", func(ctx context.Context) {
			By("creating a Program with packages defined")
			programWithPackages := program.DeepCopy()
			programWithPackages.Name = fmt.Sprintf("test-program-with-packages-%s", utilrand.String(8))
			programWithPackages.ResourceVersion = "" // Clear resource version for creation
			programWithPackages.Program.Packages = map[string]string{
				"talos-go-component":      "https://github.com/dirien/pulumi-talos-go-component@0.0.0-x5ed2a9a45c6d287bd2485f4db1e8c052b164a5ef",
				"custom-component":        "file://./custom",
				"parameterized-component": "https://example.com/pkg@v1.0.0",
			}
			Expect(k8sClient.Create(ctx, programWithPackages)).Should(Succeed())
			defer func() {
				Expect(k8sClient.Delete(ctx, programWithPackages)).Should(Succeed())
			}()

			packagesProgramNamespacedName := types.NamespacedName{
				Name:      programWithPackages.Name,
				Namespace: programWithPackages.Namespace,
			}

			By("reconciling the Program with packages")
			result, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: packagesProgramNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			By("verifying the artifact was generated successfully")
			updatedProgram := v1.Program{}
			Expect(k8sClient.Get(ctx, packagesProgramNamespacedName, &updatedProgram)).Should(Succeed())
			Expect(updatedProgram.Status.Artifact).NotTo(BeNil())
			Expect(updatedProgram.Status.Artifact.Digest).To(MatchRegexp(`^sha256:\w+$`))

			By("verifying the packages field is properly included in the ProjectFile conversion")
			project := programToProject(programWithPackages)
			Expect(project.Packages).To(HaveLen(3))
			Expect(project.Packages["talos-go-component"]).To(Equal("https://github.com/dirien/pulumi-talos-go-component@0.0.0-x5ed2a9a45c6d287bd2485f4db1e8c052b164a5ef"))
			Expect(project.Packages["custom-component"]).To(Equal("file://./custom"))
			Expect(project.Packages["parameterized-component"]).To(Equal("https://example.com/pkg@v1.0.0"))
		})
	})

	Context("programToProject function", func() {
		It("should include packages in the ProjectFile", func() {
			program := &v1.Program{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-program",
				},
				Program: v1.ProgramSpec{
					Packages: map[string]string{
						"talos-go-component": "https://github.com/dirien/pulumi-talos-go-component@0.0.0-x5ed2a9a45c6d287bd2485f4db1e8c052b164a5ef",
						"custom-component":   "file://./custom",
					},
					Resources: map[string]v1.Resource{
						"test-resource": {
							Type: "kubernetes:core/v1:Pod",
						},
					},
				},
			}

			project := programToProject(program)

			Expect(project.Name).To(Equal("test-program"))
			Expect(project.Runtime).To(Equal("yaml"))
			Expect(project.Packages).To(HaveLen(2))
			Expect(project.Packages["talos-go-component"]).To(Equal("https://github.com/dirien/pulumi-talos-go-component@0.0.0-x5ed2a9a45c6d287bd2485f4db1e8c052b164a5ef"))
			Expect(project.Packages["custom-component"]).To(Equal("file://./custom"))
			Expect(project.Resources).To(HaveLen(1))
		})
	})
})
