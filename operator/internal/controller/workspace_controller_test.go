/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/v1alpha1"
)

const (
	TestFinalizer = "test.pulumi.com/finalizer"
)

var (
	TestGitSource = &autov1alpha1.GitSource{
		Url:      "https://github.com/pulumi/examples.git",
		Revision: "1e2fc471709448f3c9f7a250f28f1eafcde7017b",
		Dir:      "random-yaml",
	}
	TestFluxSource = &autov1alpha1.FluxSource{
		Url:    "http://source-controller.flux-system.svc.cluster.local./gitrepository/default/pulumi-examples/1e2fc471709448f3c9f7a250f28f1eafcde7017b.tar.gz",
		Digest: "sha256:6560311e95689086aa195a82c0310080adc31bea2457936ce528a014d811407a",
		Dir:    "random-yaml",
	}
)

var _ = Describe("Workspace Controller", func() {
	var r *WorkspaceReconciler
	var objName types.NamespacedName
	var obj *autov1alpha1.Workspace
	var ready *metav1.Condition
	var ss *appsv1.StatefulSet
	var svc *corev1.Service

	BeforeEach(func(ctx context.Context) {
		var err error
		objName = types.NamespacedName{
			Name:      fmt.Sprintf("workspace-%s", utilrand.String(8)),
			Namespace: "default",
		}
		obj = &autov1alpha1.Workspace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      objName.Name,
				Namespace: objName.Namespace,
			},
		}
		ready = nil
		ss = &appsv1.StatefulSet{}
		svc = &corev1.Service{}

		//  setup the 'actual' statefulset to be observed by the reconciliation loop.
		ss, err = newStatefulSet(ctx, obj, &sourceSpec{})
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func(ctx context.Context) {
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func(ctx context.Context) {
			_ = k8sClient.Delete(ctx, obj)
		})
		// Expect(controllerutil.SetControllerReference(obj, ss, k8sClient.Scheme())).To(Succeed())
		// Expect(k8sClient.Patch(ctx, ss, client.Apply, client.FieldOwner(FieldManager))).To(Succeed())

		r = &WorkspaceReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	})

	reconcileF := func(ctx context.Context) (result reconcile.Result, err error) {

		result, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: objName})

		// refresh the object and find its status condition(s)
		obj = &autov1alpha1.Workspace{}
		Expect(k8sClient.Get(ctx, objName, obj)).To(Succeed())
		ready = meta.FindStatusCondition(obj.Status.Conditions, autov1alpha1.WorkspaceReady)

		// refresh the dependent objects
		ss = &appsv1.StatefulSet{}
		_ = k8sClient.Get(ctx, types.NamespacedName{
			Name:      fmt.Sprintf("%s-workspace", objName.Name),
			Namespace: objName.Namespace,
		}, ss)
		svc = &corev1.Service{}
		_ = k8sClient.Get(ctx, types.NamespacedName{
			Name:      fmt.Sprintf("%s-workspace", objName.Name),
			Namespace: objName.Namespace,
		}, svc)

		return
	}

	When("the resource does not exist", func() {
		JustBeforeEach(func(ctx context.Context) {
			Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
		})
		It("should not return an error", func(ctx context.Context) {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: objName})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Spec", func() {
		It("creates a service", func(ctx context.Context) {
			_, err := reconcileF(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.UID).NotTo(BeEmpty())
			Expect(svc.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				APIVersion:         "auto.pulumi.com/v1alpha1",
				Kind:               "Workspace",
				Name:               obj.Name,
				UID:                obj.UID,
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			}))
		})

		It("creates a statefulset", func(ctx context.Context) {
			_, err := reconcileF(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(ss.UID).NotTo(BeEmpty())
			Expect(ss.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
				APIVersion:         "auto.pulumi.com/v1alpha1",
				Kind:               "Workspace",
				Name:               obj.Name,
				UID:                obj.UID,
				BlockOwnerDeletion: ptr.To(true),
				Controller:         ptr.To(true),
			}))
		})

		Describe("spec.image", func() {
			It("uses pulumi/pulumi:latest by default", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				container := findContainer(ss.Spec.Template.Spec.Containers, "pulumi")
				Expect(container).NotTo(BeNil())
				Expect(container.Image).To(Equal("pulumi/pulumi:latest"))
			})
			When("image is set", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.Image = "test/pulumi:v3.42.0"
				})
				It("uses the specified image for the pulumi container", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					container := findContainer(ss.Spec.Template.Spec.Containers, "pulumi")
					Expect(container).NotTo(BeNil())
					Expect(container.Image).To(Equal("test/pulumi:v3.42.0"))
				})
			})
		})

		Describe("spec.serviceAccountName", func() {
			When("serviceAccountName is set", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.ServiceAccountName = "test"
				})
				It("uses the specified service account on the pod", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(ss.Spec.Template.Spec.ServiceAccountName).To(Equal("test"))
				})
			})
		})

		Describe("spec.env", func() {
			When("environment variables are set", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.Env = []corev1.EnvVar{
						{Name: "FOO", Value: "BAR"},
					}
				})
				It("applies the environment variables to the pulumi container", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					container := findContainer(ss.Spec.Template.Spec.Containers, "pulumi")
					Expect(container.Env).To(Equal(obj.Spec.Env))
				})
			})
		})

		Describe("spec.envFrom", func() {
			When("environment variable sources are set", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.EnvFrom = []corev1.EnvFromSource{
						{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "test"}}},
					}
				})
				It("applies the sources to the pulumi container", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					container := findContainer(ss.Spec.Template.Spec.Containers, "pulumi")
					Expect(container.EnvFrom).To(Equal(obj.Spec.EnvFrom))
				})
			})
		})

		Describe("spec.resources", func() {
			When("resources are set", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.Resources = corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					}
				})
				It("configures the resources of the pulumi container", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					container := findContainer(ss.Spec.Template.Spec.Containers, "pulumi")
					Expect(container.Resources).To(Equal(obj.Spec.Resources))
				})
			})
		})

		Describe("spec.securityProfile", func() {
			BeforeEach(func(ctx context.Context) {
				// the security profile affects the fetch container too, so add a git source accordingly.
				obj.Spec.Git = TestGitSource
			})
			Describe("Baseline", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.SecurityProfile = autov1alpha1.SecurityProfileBaseline
				})
				It("applies the baseline security profile", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					sc := ss.Spec.Template.Spec.SecurityContext
					Expect(sc.RunAsNonRoot).To(BeNil())
				})
			})
			Describe("Restricted", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.SecurityProfile = autov1alpha1.SecurityProfileRestricted
				})
				It("applies the restricted security profile", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					sc := ss.Spec.Template.Spec.SecurityContext
					Expect(sc.RunAsNonRoot).To(PointTo(BeTrue()))
					pulumi := findContainer(ss.Spec.Template.Spec.Containers, "pulumi")
					Expect(pulumi.SecurityContext.AllowPrivilegeEscalation).To(PointTo(BeFalse()))
					fetch := findContainer(ss.Spec.Template.Spec.InitContainers, "fetch")
					Expect(fetch.SecurityContext.AllowPrivilegeEscalation).To(PointTo(BeFalse()))
				})
			})
		})

		Describe("spec.git", func() {
			When("a repository is set", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.Git = TestGitSource
				})
				It("clones the git repository to the shared volume", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					fetch := findContainer(ss.Spec.Template.Spec.InitContainers, "fetch")
					Expect(fetch).NotTo(BeNil())
					Expect(fetch.Env).To(ConsistOf(
						corev1.EnvVar{Name: "GIT_URL", Value: TestGitSource.Url},
						corev1.EnvVar{Name: "GIT_REVISION", Value: TestGitSource.Revision},
						corev1.EnvVar{Name: "GIT_DIR", Value: TestGitSource.Dir},
					))
				})
			})
		})

		Describe("spec.flux", func() {
			When("an artifact is set", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.Flux = TestFluxSource
				})
				It("downloads the Flux artifact to the shared volume", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					fetch := findContainer(ss.Spec.Template.Spec.InitContainers, "fetch")
					Expect(fetch).NotTo(BeNil())
					Expect(fetch.Env).To(ConsistOf(
						corev1.EnvVar{Name: "FLUX_URL", Value: TestFluxSource.Url},
						corev1.EnvVar{Name: "FLUX_DIGEST", Value: TestFluxSource.Digest},
						corev1.EnvVar{Name: "FLUX_DIR", Value: TestFluxSource.Dir},
					))
				})
			})
		})

		Describe(".podTemplate", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.PodTemplate = &corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: "test", Image: "test/extra:latest"},
						},
						Containers: []corev1.Container{
							{
								Name:         "pulumi",
								VolumeMounts: []corev1.VolumeMount{{Name: "test", MountPath: "/test"}},
							},
						},
						Volumes: []corev1.Volume{
							{Name: "test", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "test"}}},
						},
					},
				}
			})
			It("merges the pod template into the statefulset", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(ss.Spec.Template.Spec.InitContainers).To(ConsistOf(HaveField("Name", "bootstrap"), HaveField("Name", "test")))
				Expect(ss.Spec.Template.Spec.Volumes).To(ConsistOf(HaveField("Name", "share"), HaveField("Name", "test")))
				Expect(ss.Spec.Template.Spec.Containers).To(ConsistOf(HaveField("Name", "pulumi")))
				pulumi := findContainer(ss.Spec.Template.Spec.Containers, "pulumi")
				Expect(pulumi.VolumeMounts).To(ConsistOf(HaveField("Name", "share"), HaveField("Name", "test")))
			})
		})
	})

	Describe("Agent Injection", func() {
		It("should inject the agent into the pod", func(ctx context.Context) {
			_, err := reconcileF(ctx)
			Expect(err).NotTo(HaveOccurred())
			bootstrap := findContainer(ss.Spec.Template.Spec.InitContainers, "bootstrap")
			Expect(bootstrap).NotTo(BeNil())
		})
	})

	Describe("Status", func() {
		BeforeEach(func(ctx context.Context) {
			//  create a pre-existing statefulset to be observed by the reconciliation loop,
			// to test how the workspace status varies based on the statefulset status.
			var err error
			ss, err = newStatefulSet(ctx, obj, &sourceSpec{})
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func(ctx context.Context) {
			Expect(controllerutil.SetControllerReference(obj, ss, k8sClient.Scheme())).To(Succeed())
			Expect(k8sClient.Patch(ctx, ss, client.Apply, client.FieldOwner(FieldManager))).To(Succeed())
		})

		Describe("Ready Condition", func() {
			When("the workspace is being deleted", func() {
				JustBeforeEach(func(ctx context.Context) {
					controllerutil.AddFinalizer(obj, TestFinalizer)
					Expect(k8sClient.Update(ctx, obj)).To(Succeed())
					Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
				})
				It("is False", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(ready).NotTo(BeNil())
					Expect(ready.Status).To(Equal(metav1.ConditionFalse))
					Expect(ready.Reason).To(Equal("Deleting"))
					Expect(ready.Message).NotTo(BeEmpty())
				})
			})
			When("ss.Status.ObservedGeneration != ss.Generation", func() {
				JustBeforeEach(func(ctx context.Context) {
					ss.Status.ObservedGeneration = 0
					Expect(k8sClient.Status().Update(ctx, ss)).To(Succeed())
				})
				It("is False", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(ready).NotTo(BeNil())
					Expect(ready.Status).To(Equal(metav1.ConditionFalse))
					Expect(ready.Reason).To(Equal("RollingUpdate"))
					Expect(ready.Message).NotTo(BeEmpty())
				})
			})
			When("ss.Status.UpdateRevision != ss.Status.CurrentRevision", func() {
				JustBeforeEach(func(ctx context.Context) {
					ss.Status.ObservedGeneration = 2
					ss.Status.Replicas = 1
					ss.Status.ReadyReplicas = 1
					ss.Status.AvailableReplicas = 1
					ss.Status.CurrentRevision = "a"
					ss.Status.CurrentReplicas = 1
					ss.Status.UpdateRevision = "b"
					ss.Status.UpdatedReplicas = 0
					Expect(k8sClient.Status().Update(ctx, ss)).To(Succeed())
				})
				It("is False", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(ready).NotTo(BeNil())
					Expect(ready.Status).To(Equal(metav1.ConditionFalse))
					Expect(ready.Reason).To(Equal("RollingUpdate"))
					Expect(ready.Message).NotTo(BeEmpty())
				})
			})
			When("ss.Status.AvailableReplicas < 1", func() {
				JustBeforeEach(func(ctx context.Context) {
					ss.Status.ObservedGeneration = 2
					ss.Status.Replicas = 1
					ss.Status.ReadyReplicas = 0
					ss.Status.AvailableReplicas = 0
					ss.Status.CurrentRevision = "b"
					ss.Status.CurrentReplicas = 1
					ss.Status.UpdateRevision = "b"
					ss.Status.UpdatedReplicas = 1
					Expect(k8sClient.Status().Update(ctx, ss)).To(Succeed())
				})
				It("is False", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(ready).NotTo(BeNil())
					Expect(ready.Status).To(Equal(metav1.ConditionFalse))
					Expect(ready.Reason).To(Equal("WaitingForReplicas"))
					Expect(ready.Message).NotTo(BeEmpty())
				})
			})
			When("all conditions met", func() {
				JustBeforeEach(func(ctx context.Context) {
					ss.Status.ObservedGeneration = 2
					ss.Status.Replicas = 1
					ss.Status.ReadyReplicas = 1
					ss.Status.AvailableReplicas = 1
					ss.Status.CurrentRevision = "b"
					ss.Status.CurrentReplicas = 1
					ss.Status.UpdateRevision = "b"
					ss.Status.UpdatedReplicas = 1
					Expect(k8sClient.Status().Update(ctx, ss)).To(Succeed())
				})
				It("is True", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(ready).NotTo(BeNil())
					Expect(ready.Status).To(Equal(metav1.ConditionTrue))
					Expect(ready.Reason).To(Equal("Succeeded"))
					Expect(ready.Message).To(BeEmpty())
				})
			})
		})
	})
})

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for _, c := range containers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}
