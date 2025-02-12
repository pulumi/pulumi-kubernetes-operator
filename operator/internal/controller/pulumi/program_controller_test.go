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
		ctx                   context.Context
		advertisedAddress     string
	)

	BeforeEach(func() {
		ctx = context.Background()

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

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &program)).Should(Succeed())
	})

	reconcileFn := func(ctx context.Context) (reconcile.Result, error) {
		return r.Reconcile(ctx, reconcile.Request{NamespacedName: programNamespacedName})
	}

	When("reconciling a resource", func() {
		It("should generate an artifact URL in the status", func() {
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
			Expect(program.Status.Artifact.Digest).To(MatchRegexp(`^sha256:\w+$`))
			Expect(program.Status.ObservedGeneration).To(Equal(program.GetGeneration()))
			Expect(program.Status.Artifact.URL).
				To(Equal(fmt.Sprintf("%s/programs/%s/%s", advertisedAddress, programNamespacedName.Namespace, programNamespacedName.Name)))

			By("reconciling the same Program object but with a new spec change")
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
			Expect(program.Status.Artifact.URL).
				To(Equal(fmt.Sprintf("%s/programs/%s/%s", advertisedAddress, programNamespacedName.Namespace, programNamespacedName.Name)))
			Expect(program.Status.Artifact.Digest).To(MatchRegexp(`^sha256:\w+$`))
			Expect(program.GetGeneration()).To(Equal(program.GetGeneration()))
			Expect(program.Status.ObservedGeneration).To(Equal(program.GetGeneration()))

			By("expecting the status to be updated with the new artifact URL when the host changes")
			advertisedAddress = "https://fake-address"
			r.ProgramHandler.address = advertisedAddress
			_, err = reconcileFn(ctx)
			Expect(err).NotTo(HaveOccurred())

			program = v1.Program{}
			Expect(k8sClient.Get(ctx, programNamespacedName, &program)).Should(Succeed())
			Expect(program.Status.Artifact).NotTo(BeNil())
			Expect(program.Status.Artifact.URL).
				To(Equal(fmt.Sprintf("https://fake-address/programs/%s/%s", programNamespacedName.Namespace, programNamespacedName.Name)))
		})

		It("should generate a URL with the correct 'http://' scheme if the advertised address does not include one", func() {
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
			Expect(program.Status.Artifact.URL).
				To(Equal(fmt.Sprintf("http://%s/programs/%s/%s", advertisedAddress, programNamespacedName.Namespace, programNamespacedName.Name)))
		})
	})
})
