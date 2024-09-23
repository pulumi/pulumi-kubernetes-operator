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

package pulumi

import (
	"context"
	"fmt"
	"time"

	fluxsourcev1 "github.com/fluxcd/source-controller/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gtypes "github.com/onsi/gomega/types"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testFinalizer = "test.finalizer.pulumi.com"
)

func makeFluxGitRepository(name types.NamespacedName) *fluxsourcev1.GitRepository {
	return &fluxsourcev1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Spec: fluxsourcev1.GitRepositorySpec{
			URL:      "https://github.com/pulumi/examples.git",
			Interval: metav1.Duration{Duration: 1 * time.Hour},
		},
		Status: fluxsourcev1.GitRepositoryStatus{
			ObservedGeneration: 1,
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: metav1.Now()},
			},
			Artifact: &fluxsourcev1.Artifact{
				Digest:         "sha256:bcbed45526b241ab3366707b5a58c900e9d60a1d5c385cdfe976b1306584b454",
				LastUpdateTime: metav1.Now(),
				Path:           "gitrepository/default/pulumi-examples/f143bd369afcb5455edb54c2b90ad7aaac719339.tar.gz",
				Revision:       "master@sha1:f143bd369afcb5455edb54c2b90ad7aaac719339",
				Size:           ptr.To(int64(48988266)),
				URL:            "http://source-controller.flux-system.svc.cluster.local./gitrepository/default/pulumi-examples/f143bd369afcb5455edb54c2b90ad7aaac719339.tar.gz",
			},
		},
	}
}

func makeWorkspace(objMeta metav1.ObjectMeta) *autov1alpha1.Workspace {
	labels := labelsForWorkspace(&objMeta)
	return &autov1alpha1.Workspace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: autov1alpha1.GroupVersion.String(),
			Kind:       "Workspace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameForWorkspace(&objMeta),
			Namespace: objMeta.Namespace,
			Labels:    labels,
		},
		Spec: autov1alpha1.WorkspaceSpec{
			PodTemplate: &autov1alpha1.EmbeddedPodTemplateSpec{
				Spec: &corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "pulumi"},
					},
				},
			},
			Stacks: []autov1alpha1.WorkspaceStack{
				{Name: "dev", Create: ptr.To(true)},
			},
		},
	}
}

func makeUpdate(name types.NamespacedName, spec autov1alpha1.UpdateSpec) *autov1alpha1.Update {
	return &autov1alpha1.Update{
		TypeMeta: metav1.TypeMeta{
			APIVersion: autov1alpha1.GroupVersion.String(),
			Kind:       "Update",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name.Name, utilrand.String(8)),
			Namespace: name.Namespace,
		},
		Spec: spec,
	}
}

var _ = Describe("Stack Controller", func() {
	var r *StackReconciler
	var objName types.NamespacedName
	var obj *pulumiv1.Stack
	var ws *autov1alpha1.Workspace
	var currentUpdate, lastUpdate *autov1alpha1.Update
	var fluxRepo *fluxsourcev1.GitRepository

	var ready, stalled, reconciling *metav1.Condition

	BeforeEach(func(ctx context.Context) {
		r = &StackReconciler{
			Client:   k8sClient,
			Scheme:   k8sClient.Scheme(),
			Recorder: record.NewFakeRecorder(10),
			maybeWatchFluxSourceKind: func(fsr shared.FluxSourceReference) error {
				return nil
			},
		}

		objName = types.NamespacedName{
			Name:      fmt.Sprintf("stack-%s", utilrand.String(8)),
			Namespace: "default",
		}
		obj = &pulumiv1.Stack{
			TypeMeta: metav1.TypeMeta{
				APIVersion: pulumiv1.GroupVersion.String(),
				Kind:       "Stack",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       objName.Name,
				Namespace:  objName.Namespace,
				Finalizers: []string{testFinalizer}, // add a 'test' finalizer to prevent the object from being garbage-collected
			},
			Spec: shared.StackSpec{
				Stack: "dev",
			},
		}
		ready = nil
		stalled = nil
		reconciling = nil

		// make a GitRepository and Workspace to potentially be observed by the reconciler.
		fluxRepo = makeFluxGitRepository(objName)
		ws = makeWorkspace(obj.ObjectMeta)

		// create a "current" Update object to potentially be observed,
		// to test how the stack status varies based on current update.
		currentUpdate = makeUpdate(objName, autov1alpha1.UpdateSpec{
			WorkspaceName: nameForWorkspace(&obj.ObjectMeta),
			StackName:     obj.Spec.Stack,
			Type:          autov1alpha1.UpType,
		})
		lastUpdate = &autov1alpha1.Update{}
	})

	useFluxSource := func() {
		BeforeEach(func(ctx context.Context) {
			ws.Spec.Flux = &autov1alpha1.FluxSource{
				Digest: fluxRepo.Status.Artifact.Digest,
				Url:    fluxRepo.Status.Artifact.URL,
				Dir:    "random-yaml",
			}
			obj.Spec.FluxSource = &shared.FluxSource{
				SourceRef: shared.FluxSourceReference{
					APIVersion: "source.toolkit.fluxcd.io/v1",
					Kind:       "GitRepository",
					Name:       fluxRepo.Name,
				},
				Dir: "random-yaml",
			}
		})
	}

	JustBeforeEach(func(ctx context.Context) {
		status := obj.Status
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		obj.Status = status
		Expect(k8sClient.Status().Update(ctx, obj, client.FieldOwner(FieldManager))).To(Succeed())
	})

	JustBeforeEach(func(ctx context.Context) {
		status := fluxRepo.Status
		Expect(k8sClient.Create(ctx, fluxRepo)).To(Succeed())
		fluxRepo.Status = status
		Expect(k8sClient.Status().Update(ctx, fluxRepo)).To(Succeed())
	})

	JustBeforeEach(func(ctx context.Context) {
		Expect(controllerutil.SetControllerReference(obj, ws, k8sClient.Scheme())).To(Succeed())
		status := ws.Status
		Expect(k8sClient.Patch(ctx, ws, client.Apply, client.FieldOwner(FieldManager))).To(Succeed())
		ws.Status = status
		Expect(k8sClient.Status().Update(ctx, ws, client.FieldOwner(FieldManager))).To(Succeed())
	})

	JustBeforeEach(func(ctx context.Context) {
		Expect(controllerutil.SetControllerReference(obj, currentUpdate, k8sClient.Scheme())).To(Succeed())
		status := currentUpdate.Status
		Expect(k8sClient.Patch(ctx, currentUpdate, client.Apply, client.FieldOwner(FieldManager))).To(Succeed())
		currentUpdate.Status = status
		Expect(k8sClient.Status().Update(ctx, currentUpdate, client.FieldOwner(FieldManager))).To(Succeed())
	})

	reconcileF := func(ctx context.Context) (result reconcile.Result, err error) {
		result, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: objName})

		// update the object and find its status condition(s)
		obj = &pulumiv1.Stack{}
		Expect(k8sClient.Get(ctx, objName, obj)).To(Succeed())
		ready = meta.FindStatusCondition(obj.Status.Conditions, pulumiv1.ReadyCondition)
		stalled = meta.FindStatusCondition(obj.Status.Conditions, pulumiv1.StalledCondition)
		reconciling = meta.FindStatusCondition(obj.Status.Conditions, pulumiv1.ReconcilingCondition)

		// refresh the dependent objects
		ws = &autov1alpha1.Workspace{}
		_ = k8sClient.Get(ctx, types.NamespacedName{
			Name:      nameForWorkspace(&obj.ObjectMeta),
			Namespace: objName.Namespace,
		}, ws)
		currentUpdate = &autov1alpha1.Update{}
		if obj.Status.CurrentUpdate != nil {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      obj.Status.CurrentUpdate.Name,
				Namespace: objName.Namespace,
			}, currentUpdate)
		}
		lastUpdate = &autov1alpha1.Update{}
		if obj.Status.LastUpdate != nil {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      obj.Status.LastUpdate.Name,
				Namespace: objName.Namespace,
			}, lastUpdate)
		}

		return
	}

	ByMarkingAsReconciling := func(reason string, msg gtypes.GomegaMatcher) {
		GinkgoHelper()
		By("marking as reconciling")
		Expect(ready).NotTo(BeNil(), "expected Ready to not be nil")
		Expect(ready.Status).To(Equal(metav1.ConditionFalse), "expected Ready to be false")
		Expect(ready.Reason).To(Equal(pulumiv1.NotReadyInProgressReason), "expected Ready reason to match")
		Expect(reconciling).NotTo(BeNil(), "expected Reconciling to not be nil")
		Expect(reconciling.Status).To(Equal(metav1.ConditionTrue), "expected Reconciling to be true")
		Expect(reconciling.Reason).To(Equal(reason), "expected Reconciling reason to match")
		Expect(reconciling.Message).To(msg, "expected Reconciling message to match")
		Expect(stalled).To(BeNil(), "expected Stalled to be nil")
	}

	ByMarkingAsStalled := func(reason string, msg gtypes.GomegaMatcher) {
		GinkgoHelper()
		By("marking as stalled")
		Expect(ready).NotTo(BeNil(), "expected Ready to not be nil")
		Expect(ready.Status).To(Equal(metav1.ConditionFalse), "expected Ready to be false")
		Expect(ready.Reason).To(Equal(pulumiv1.NotReadyStalledReason), "expected Ready reason to match")
		Expect(reconciling).To(BeNil(), "expected Reconciling to be nil")
		Expect(stalled).NotTo(BeNil(), "expected Stalled to not be nil")
		Expect(stalled.Status).To(Equal(metav1.ConditionTrue), "expected Stalled to be true")
		Expect(stalled.Reason).To(Equal(reason), "expected Stalled reason to match")
		Expect(stalled.Message).To(msg, "expected Stalled message to match")
	}

	ByMarkingAsReady := func() {
		GinkgoHelper()
		By("marking as ready")
		Expect(ready).ToNot(BeNil(), "expected Ready to not be nil")
		Expect(ready.Status).To(Equal(metav1.ConditionTrue), "expected Ready to be true")
		Expect(ready.Reason).To(Equal(pulumiv1.ReadyCompletedReason), "expected Ready reason to match")
		Expect(reconciling).To(BeNil(), "expected Reconciling to be nil")
		Expect(stalled).To(BeNil(), "expected Stalled to be nil")
	}

	Describe("Finalization", func() {
		useFluxSource()
		BeforeEach(func(ctx context.Context) {
			controllerutil.AddFinalizer(obj, pulumiFinalizer)
		})

		JustBeforeEach(func(ctx context.Context) {
			// mark the object for deletion (which increments the generation)
			// then get the latest revision since Delete doesn't update the struct
			Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
			obj = &pulumiv1.Stack{}
			_ = k8sClient.Get(ctx, objName, obj)
		})

		When("the resource is not found", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Finalizers = nil
			})
			It("reconciles", func(ctx context.Context) {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: objName})
				Expect(err).NotTo(HaveOccurred(), "complete without NotFound error")
			})
		})

		When("the resource has already been finalized", func() {
			BeforeEach(func(ctx context.Context) {
				controllerutil.RemoveFinalizer(obj, pulumiFinalizer)
			})
			It("reconciles", func(ctx context.Context) {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: objName})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the resource has an ongoing update from the prior generation", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Status.CurrentUpdate = &shared.CurrentStackUpdate{
					Generation: 1,
					Name:       currentUpdate.Name,
					Commit:     "abcdef",
				}
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				By("not finalizing the object")
				Expect(obj.Finalizers).To(ContainElement(pulumiFinalizer), "not remove the finalizer")
				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingUpdateMessage))
			})
		})

		When("there's no ongoing update", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Status.CurrentUpdate = nil
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				By("finalizing the object")
				Expect(obj.Finalizers).ToNot(ContainElement(pulumiFinalizer), "remove the finalizer")
			})
		})

		Describe("DestroyOnFinalize", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.DestroyOnFinalize = true

				// make the workspace be ready for an update
				ws.Status.ObservedGeneration = 1
				ws.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: metav1.Now()},
				}
			})

			When("the destroy op hasn't started", func() {
				DescribeTableSubtree("given a last update",
					func(lastUpdate *shared.StackUpdateState) {
						BeforeEach(func(ctx context.Context) {
							obj.Status.LastUpdate = lastUpdate
						})
						It("reconciles", func(ctx context.Context) {
							_, err := reconcileF(ctx)
							Expect(err).NotTo(HaveOccurred())
							Expect(obj.Finalizers).To(ContainElement(pulumiFinalizer), "not remove the finalizer")
							ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingUpdateMessage))

							By("creating a destroy op")
							Expect(currentUpdate).ToNot(BeNil())
							Expect(currentUpdate.Spec.StackName).To(Equal(obj.Spec.Stack))
							Expect(currentUpdate.Spec.Type).To(Equal(autov1alpha1.DestroyType))

							By("updating the status with current update")
							Expect(obj.Status.CurrentUpdate).ToNot(BeNil())
							Expect(obj.Status.CurrentUpdate.Generation).To(Equal(int64(2)))
							Expect(obj.Status.CurrentUpdate.Name).ToNot(BeEmpty())
							Expect(obj.Status.CurrentUpdate.Commit).ToNot(BeEmpty())
						})
					},
					Entry("no last update", nil),
					Entry("a prior update", &shared.StackUpdateState{
						Generation:           1,
						State:                shared.SucceededStackStateMessage,
						Name:                 "update-abcdef",
						Type:                 autov1alpha1.UpType,
						LastResyncTime:       metav1.Now(),
						LastAttemptedCommit:  "abcdef",
						LastSuccessfulCommit: "abcdef",
					}),
					Entry("an earlier attempt at finalization", &shared.StackUpdateState{
						Generation:           2,
						State:                shared.FailedStackStateMessage,
						Name:                 "update-abcdef",
						Type:                 autov1alpha1.DestroyType,
						LastResyncTime:       metav1.Now(),
						LastAttemptedCommit:  "abcdef",
						LastSuccessfulCommit: "abcdef",
					}),
				)
			})

			When("the destroy op was successful", func() {
				JustBeforeEach(func(ctx context.Context) {
					// update the gen-2 status to reflect a successful destroy operation
					obj.Status.ObservedGeneration = 2
					obj.Status.LastUpdate = &shared.StackUpdateState{
						Generation:           2,
						State:                shared.SucceededStackStateMessage,
						Name:                 "update-abcdef",
						Type:                 autov1alpha1.DestroyType,
						LastResyncTime:       metav1.Now(),
						LastAttemptedCommit:  "abcdef",
						LastSuccessfulCommit: "abcdef",
					}
					Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
				})
				It("reconciles", func(ctx context.Context) {
					result, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(obj.Finalizers).ToNot(ContainElement(pulumiFinalizer), "remove the finalizer")
					ByMarkingAsReady()
				})
			})
		})
	})

	Describe("Current Update Processing", func() {
		useFluxSource()

		BeforeEach(func(ctx context.Context) {
			obj.Status.CurrentUpdate = &shared.CurrentStackUpdate{
				Generation: 1,
				Name:       currentUpdate.Name,
				Commit:     "abcdef",
			}
		})

		When("the update is not found", func() {
			JustBeforeEach(func(ctx context.Context) {
				Expect(k8sClient.Delete(ctx, currentUpdate)).To(Succeed())
			})
			It("reconciles", func(ctx context.Context) {
				result, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())

				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingUpdateMessage))

				By("retaining the current update")
				Expect(obj.Status.CurrentUpdate).NotTo(BeNil())

				By("not requeuing and by watching for Update status changes")
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		When("the update is incomplete (still running)", func() {
			BeforeEach(func(ctx context.Context) {
				currentUpdate.Status.ObservedGeneration = 1
				currentUpdate.Status.Conditions = []metav1.Condition{
					{
						Type:               autov1alpha1.UpdateConditionTypeComplete,
						Status:             metav1.ConditionFalse,
						Reason:             "Progressing",
						LastTransitionTime: metav1.Now(),
					},
				}
			})
			It("reconciles", func(ctx context.Context) {
				result, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())

				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingUpdateMessage))

				By("retaining the current update")
				Expect(obj.Status.CurrentUpdate).NotTo(BeNil())

				By("not requeuing and by watching for Update status changes")
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		When("the update failed", func() {
			BeforeEach(func(ctx context.Context) {
				currentUpdate.Status.ObservedGeneration = 1
				currentUpdate.Status.Conditions = []metav1.Condition{
					{
						Type:               autov1alpha1.UpdateConditionTypeComplete,
						Status:             metav1.ConditionTrue,
						Reason:             "Updated",
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               autov1alpha1.UpdateConditionTypeFailed,
						Status:             metav1.ConditionTrue,
						Reason:             "unknown",
						LastTransitionTime: metav1.Now(),
					},
				}
			})
			It("reconciles", func(ctx context.Context) {
				update := currentUpdate.DeepCopy()
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())

				By("clearing the current update")
				Expect(obj.Status.CurrentUpdate).To(BeNil())

				By("setting the last update")
				Expect(obj.Status.LastUpdate).NotTo(BeNil())
				Expect(obj.Status.LastUpdate.Generation).To(Equal(update.Generation))
				Expect(obj.Status.LastUpdate.Name).To(Equal(update.Name))
				Expect(obj.Status.LastUpdate.Type).To(Equal(update.Spec.Type))
				Expect(obj.Status.LastUpdate.State).To(Equal(shared.FailedStackStateMessage))
				Expect(obj.Status.LastUpdate.LastAttemptedCommit).To(Equal("abcdef"))
				Expect(obj.Status.LastUpdate.LastSuccessfulCommit).To(BeEmpty())

				By("emitting an event")
				Expect(r.Recorder.(*record.FakeRecorder).Events).To(Receive(matchEvent(pulumiv1.StackUpdateFailure)))
			})
		})

		When("the update succeeded", func() {
			BeforeEach(func(ctx context.Context) {
				currentUpdate.Status.ObservedGeneration = 1
				currentUpdate.Status.Conditions = []metav1.Condition{
					{
						Type:               autov1alpha1.UpdateConditionTypeComplete,
						Status:             metav1.ConditionTrue,
						Reason:             "Updated",
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               autov1alpha1.UpdateConditionTypeFailed,
						Status:             metav1.ConditionFalse,
						Reason:             "unknown",
						LastTransitionTime: metav1.Now(),
					},
				}
			})
			It("reconciles", func(ctx context.Context) {
				update := currentUpdate.DeepCopy()
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())

				By("clearing the current update")
				Expect(obj.Status.CurrentUpdate).To(BeNil())

				By("setting the last update")
				Expect(obj.Status.LastUpdate).NotTo(BeNil())
				Expect(obj.Status.LastUpdate.Generation).To(Equal(update.Generation))
				Expect(obj.Status.LastUpdate.Name).To(Equal(update.Name))
				Expect(obj.Status.LastUpdate.Type).To(Equal(update.Spec.Type))
				Expect(obj.Status.LastUpdate.State).To(Equal(shared.SucceededStackStateMessage))
				Expect(obj.Status.LastUpdate.LastAttemptedCommit).To(Equal("abcdef"))
				Expect(obj.Status.LastUpdate.LastSuccessfulCommit).To(Equal("abcdef"))

				By("emitting an event")
				Expect(r.Recorder.(*record.FakeRecorder).Events).To(Receive(matchEvent(pulumiv1.StackUpdateSuccessful)))
			})

			When("the update produced outputs", func() {
				BeforeEach(func(ctx context.Context) {
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "outputs", Namespace: currentUpdate.Namespace,
							Annotations: map[string]string{
								"pulumi.com/mask": `{"plaintext": true}`,
							},
						},
						StringData: map[string]string{
							"plaintext":          `"not-sensitive"`,
							"should-be-scrubbed": `"sensitive"`,
						},
					}
					Expect(k8sClient.Create(ctx, secret)).To(Succeed())
					currentUpdate.Status.Outputs = secret.Name
				})
				It("captures scrubbed outputs", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(obj.Status.Outputs["plaintext"]).To(Equal(`"not-sensitive"`))
					Expect(obj.Status.Outputs["should-be-scrubbed"]).To(Equal(`"[secret]"`))
				})
			})
		})
	})

	Describe("Update Triggers", func() {
		useFluxSource()

		ByResyncing := func() {
			GinkgoHelper()
			ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingWorkspaceMessage))
		}

		When("an update hasn't run yet", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Status.LastUpdate = nil
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByResyncing()
			})
		})

		When("the last update was not successful", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Status.LastUpdate = &shared.StackUpdateState{
					Generation:           1,
					State:                shared.FailedStackStateMessage,
					Name:                 "update-abcdef",
					Type:                 autov1alpha1.UpType,
					LastResyncTime:       metav1.Now(),
					LastAttemptedCommit:  fluxRepo.Status.Artifact.Digest,
					LastSuccessfulCommit: "",
				}
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByResyncing()
			})
		})

		When("the stack is a new generation", func() {
			BeforeEach(func(ctx context.Context) {
				// assume that the previous update was successful
				obj.Status.LastUpdate = &shared.StackUpdateState{
					Generation:           1,
					State:                shared.SucceededStackStateMessage,
					Name:                 "update-abcdef",
					Type:                 autov1alpha1.UpType,
					LastResyncTime:       metav1.Now(),
					LastAttemptedCommit:  fluxRepo.Status.Artifact.Digest,
					LastSuccessfulCommit: fluxRepo.Status.Artifact.Digest,
				}
			})
			JustBeforeEach(func(ctx context.Context) {
				// touch the stack generation
				obj.Spec.Stack = "prod"
				Expect(k8sClient.Update(ctx, obj)).To(Succeed())
				Expect(obj.Generation).To(Equal(int64(2)))
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByResyncing()
			})
		})

		When("the last update was successful", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Status.LastUpdate = &shared.StackUpdateState{
					Generation:     1,
					State:          shared.SucceededStackStateMessage,
					Name:           "update-abcdef",
					Type:           autov1alpha1.UpType,
					LastResyncTime: metav1.Now(),
				}
			})

			When("a newer commit is available", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Status.LastUpdate.LastAttemptedCommit = "old"
					obj.Status.LastUpdate.LastSuccessfulCommit = "old"
				})
				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					ByResyncing()
					By("emitting an event")
					Expect(r.Recorder.(*record.FakeRecorder).Events).To(Receive(matchEvent(pulumiv1.StackUpdateDetected)))
				})
			})

			When("at latest commit", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Status.LastUpdate.LastAttemptedCommit = fluxRepo.Status.Artifact.Digest
					obj.Status.LastUpdate.LastSuccessfulCommit = fluxRepo.Status.Artifact.Digest
				})

				It("reconciles", func(ctx context.Context) {
					result, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					ByMarkingAsReady()
					By("not requeuing")
					Expect(result).To(Equal(reconcile.Result{}))
				})

				Describe("ContinueResyncOnCommitMatch", func() {
					BeforeEach(func(ctx context.Context) {
						obj.Spec.ContinueResyncOnCommitMatch = true
						obj.Spec.ResyncFrequencySeconds = 10 * 60 // 10 minutes
					})

					When("a resync is not due", func() {
						BeforeEach(func(ctx context.Context) {
							// resync happened 5 minutes ago (less than ResyncFrequencySeconds)
							obj.Status.LastUpdate.LastResyncTime = metav1.NewTime(time.Now().Add(-5 * time.Minute))
						})
						It("reconciles", func(ctx context.Context) {
							result, err := reconcileF(ctx)
							Expect(err).NotTo(HaveOccurred())
							ByMarkingAsReady()
							By("requeuing at the next resync time")
							Expect(result).To(MatchFields(IgnoreExtras, Fields{
								"RequeueAfter": And(BeNumerically(">", 0), BeNumerically("<", 10*time.Minute)),
							}))
						})
					})

					When("a resync is due", func() {
						BeforeEach(func(ctx context.Context) {
							// resync happened an hour ago (more than ResyncFrequencySeconds)
							obj.Status.LastUpdate.LastResyncTime = metav1.NewTime(time.Now().Add(-1 * time.Hour))
						})
						It("reconciles", func(ctx context.Context) {
							_, err := reconcileF(ctx)
							Expect(err).NotTo(HaveOccurred())
							ByResyncing()
						})
					})
				})

				PDescribe("branch tracking", func() {})
			})
		})
	})

	Describe("Updates", func() {
		useFluxSource()

		When("no workspace exists", func() {
			JustBeforeEach(func(ctx context.Context) {
				Expect(k8sClient.Delete(ctx, ws)).To(Succeed())
			})
			It("reconciles", func(ctx context.Context) {
				result, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingWorkspaceMessage))

				By("creating the workspace")
				Expect(ws.Generation).To(Equal(int64(1)))

				By("not requeuing")
				Expect(result).To(Equal(reconcile.Result{}))
				By("watching for a workspace status update")
			})
		})

		When("the workspace is a new generation", func() {
			BeforeEach(func(ctx context.Context) {
				// change the stack specification to trigger a workspace update
				obj.Spec.FluxSource.Dir = "updated"
			})
			JustBeforeEach(func(ctx context.Context) {
				// set the status to reflect that the previous generation was ready
				ws.Status.ObservedGeneration = ws.Generation - 1
				ws.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: metav1.Now()},
				}
				Expect(k8sClient.Status().Update(ctx, ws, client.FieldOwner(FieldManager))).To(Succeed())
			})
			It("reconciles", func(ctx context.Context) {
				result, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingWorkspaceMessage))

				By("updating the workspace")
				Expect(ws.Generation).To(Equal(int64(2)))

				By("not requeuing")
				Expect(result).To(Equal(reconcile.Result{}))
				By("watching for a workspace status update")
			})
		})

		When("the workspace is not ready", func() {
			JustBeforeEach(func(ctx context.Context) {
				ws.Status.ObservedGeneration = ws.Generation
				ws.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionFalse, Reason: "RollingUpdate", LastTransitionTime: metav1.Now()},
				}
				Expect(k8sClient.Status().Update(ctx, ws, client.FieldOwner(FieldManager))).To(Succeed())
			})
			It("reconciles", func(ctx context.Context) {
				result, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingWorkspaceMessage))

				By("not requeuing")
				Expect(result).To(Equal(reconcile.Result{}))
				By("watching for a workspace status update")
			})
		})

		When("the workspace is ready", func() {
			JustBeforeEach(func(ctx context.Context) {
				ws.Status.ObservedGeneration = ws.Generation
				ws.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: metav1.Now()},
				}
				Expect(k8sClient.Status().Update(ctx, ws, client.FieldOwner(FieldManager))).To(Succeed())
			})
			It("reconciles", func(ctx context.Context) {
				result, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingUpdateMessage))

				By("creating an update op")
				Expect(currentUpdate).ToNot(BeNil())
				Expect(currentUpdate.Spec.StackName).To(Equal(obj.Spec.Stack))
				Expect(currentUpdate.Spec.Type).To(Equal(autov1alpha1.UpType))

				By("updating the status with current update")
				Expect(obj.Status.CurrentUpdate).ToNot(BeNil())
				Expect(obj.Status.CurrentUpdate.Generation).To(Equal(obj.Generation))
				Expect(obj.Status.CurrentUpdate.Name).ToNot(BeEmpty())
				Expect(obj.Status.CurrentUpdate.Commit).ToNot(BeEmpty())

				By("not requeuing")
				Expect(result).To(Equal(reconcile.Result{}))
				By("watching for a workspace status update")
			})
		})
	})

	Describe("Prerequisites", func() {
		var prereq *pulumiv1.Stack

		useFluxSource()

		BeforeEach(func(ctx context.Context) {
			prereq = &pulumiv1.Stack{
				TypeMeta: metav1.TypeMeta{
					APIVersion: pulumiv1.GroupVersion.String(),
					Kind:       "Stack",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("prereq-%s", utilrand.String(8)),
					Namespace: objName.Namespace,
				},
				Spec: shared.StackSpec{
					Stack: "dev",
				},
			}
			obj.Spec.Prerequisites = []shared.PrerequisiteRef{
				{
					Name:        prereq.Name,
					Requirement: &shared.RequirementSpec{},
				},
			}
		})

		JustBeforeEach(func(ctx context.Context) {
			status := prereq.Status
			Expect(k8sClient.Create(ctx, prereq)).To(Succeed())
			prereq.Status = status
			Expect(k8sClient.Status().Update(ctx, prereq)).To(Succeed())
		})

		beNotSatisfied := func(msg gtypes.GomegaMatcher) {
			GinkgoHelper()
			It("reconciles", func(ctx context.Context) {
				result, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsReconciling(pulumiv1.ReconcilingPrerequisiteNotSatisfiedReason, msg)
				By("not requeuing")
				Expect(result).To(Equal(reconcile.Result{}))
			})
		}

		beSatisfied := func() {
			GinkgoHelper()
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingWorkspaceMessage))
			})
		}

		When("the prerequisite is not found", func() {
			JustBeforeEach(func(ctx context.Context) {
				Expect(k8sClient.Delete(ctx, prereq)).To(Succeed())
			})
			beNotSatisfied(ContainSubstring("unable to fetch prerequisite"))
		})

		When("the prerequisite hasn't run yet", func() {
			BeforeEach(func(ctx context.Context) {
				prereq.Status.LastUpdate = nil
			})
			beNotSatisfied(ContainSubstring(errRequirementNotRun.Error()))
		})

		When("the prerequisite spec has changed since the last update", func() {
			JustBeforeEach(func(ctx context.Context) {
				prereq.Spec.Stack = "prod"
				Expect(k8sClient.Update(ctx, prereq)).To(Succeed())
				Expect(prereq.Generation).To(Equal(int64(2)))
			})
			BeforeEach(func(ctx context.Context) {
				prereq.Status.LastUpdate = &shared.StackUpdateState{
					Generation:          1,
					Name:                "update-abcdef",
					Type:                autov1alpha1.UpType,
					State:               shared.FailedStackStateMessage,
					LastAttemptedCommit: "abcdef",
				}
			})
			beNotSatisfied(ContainSubstring(errRequirementNotRun.Error()))
		})

		When("the prerequisite failed to update", func() {
			BeforeEach(func(ctx context.Context) {
				prereq.Status.LastUpdate = &shared.StackUpdateState{
					Generation:          1,
					Name:                "update-abcdef",
					Type:                autov1alpha1.UpType,
					State:               shared.FailedStackStateMessage,
					LastAttemptedCommit: "abcdef",
				}
			})
			beNotSatisfied(ContainSubstring(errRequirementFailed.Error()))
		})

		When("the prerequisite succeeded", func() {
			BeforeEach(func(ctx context.Context) {
				prereq.Status.LastUpdate = &shared.StackUpdateState{
					Generation:           1,
					Name:                 "update-abcdef",
					Type:                 autov1alpha1.UpType,
					State:                shared.SucceededStackStateMessage,
					LastResyncTime:       metav1.NewTime(time.Now().Add(-1 * time.Hour)),
					LastAttemptedCommit:  "abcdef",
					LastSuccessfulCommit: "abcdef",
				}
			})
			beSatisfied()
		})

		Describe("resync", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.Prerequisites[0].Requirement.SucceededWithinDuration = &metav1.Duration{Duration: 1 * time.Hour}
			})

			When("the prerequisite is overdue for a sync", func() {
				BeforeEach(func(ctx context.Context) {
					prereq.Status.LastUpdate = &shared.StackUpdateState{
						Generation:           1,
						Name:                 "update-abcdef",
						Type:                 autov1alpha1.UpType,
						State:                shared.SucceededStackStateMessage,
						LastResyncTime:       metav1.NewTime(time.Now().Add(-2 * time.Hour)), // 2 hours ago
						LastAttemptedCommit:  "abcdef",
						LastSuccessfulCommit: "abcdef",
					}
				})
				beNotSatisfied(ContainSubstring(errRequirementOutOfDate.Error()))
			})

			When("the prerequisite was synced recently", func() {
				BeforeEach(func(ctx context.Context) {
					prereq.Status.LastUpdate = &shared.StackUpdateState{
						Generation:           1,
						Name:                 "update-abcdef",
						Type:                 autov1alpha1.UpType,
						State:                shared.SucceededStackStateMessage,
						LastResyncTime:       metav1.NewTime(time.Now().Add(-30 * time.Minute)), // 30 minutes ago
						LastAttemptedCommit:  "abcdef",
						LastSuccessfulCommit: "abcdef",
					}
				})
				beSatisfied()
			})
		})
	})

	Describe("Sources", func() {
		When("the stack has no sources", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.GitSource = nil
				obj.Spec.FluxSource = nil
				obj.Spec.ProgramRef = nil
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsStalled(pulumiv1.StalledSpecInvalidReason, ContainSubstring("exactly one source"))
			})
		})

		Describe("flux", func() {
			var watched shared.FluxSourceReference

			BeforeEach(func(ctx context.Context) {
				r.maybeWatchFluxSourceKind = func(fsr shared.FluxSourceReference) error {
					watched = fsr
					return nil
				}
				obj.Spec.FluxSource = &shared.FluxSource{
					SourceRef: shared.FluxSourceReference{
						APIVersion: "source.toolkit.fluxcd.io/v1",
						Kind:       "GitRepository",
						Name:       fluxRepo.Name,
					},
					Dir: "random-yaml",
				}
			})

			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())

				By("registering a watch event")
				Expect(watched).To(Equal(shared.FluxSourceReference{
					APIVersion: "source.toolkit.fluxcd.io/v1",
					Kind:       "GitRepository",
					Name:       fluxRepo.Name,
				}))
			})

			beStalled := func(reason string, msg gtypes.GomegaMatcher) {
				GinkgoHelper()
				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					ByMarkingAsStalled(reason, msg)
				})
			}

			When("the flux source is not found", func() {
				JustBeforeEach(func(ctx context.Context) {
					Expect(k8sClient.Delete(ctx, fluxRepo)).To(Succeed())
				})
				beStalled(pulumiv1.StalledSourceUnavailableReason, ContainSubstring("could not resolve sourceRef"))
			})

			When("the flux source is not ready", func() {
				BeforeEach(func(ctx context.Context) {
					fluxRepo.Status.Conditions = []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionFalse, Reason: "Unknown", LastTransitionTime: metav1.Now()},
					}
				})
				beStalled(pulumiv1.StalledSourceUnavailableReason, ContainSubstring("source Ready condition does not have status True"))
			})

			When("the flux source is ready", func() {
				BeforeEach(func(ctx context.Context) {
					fluxRepo.Status.Conditions = []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: metav1.Now()},
					}
				})

				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					By("configuring the workspace")
					Expect(ws).ToNot(BeNil())
					Expect(ws.Spec.Flux).To(HaveValue(Equal(autov1alpha1.FluxSource{
						Digest: "sha256:bcbed45526b241ab3366707b5a58c900e9d60a1d5c385cdfe976b1306584b454",
						Url:    "http://source-controller.flux-system.svc.cluster.local./gitrepository/default/pulumi-examples/f143bd369afcb5455edb54c2b90ad7aaac719339.tar.gz",
						Dir:    obj.Spec.FluxSource.Dir,
					})))
				})
			})
		})

		Describe("git", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.GitSource = &shared.GitSource{
					ProjectRepo: "https://github.com/pulumi/examples.git",
					Commit:      "2ca775387e522fd5c29668a85bfba2f8fd791848",
					RepoDir:     "random-yaml",
				}
			})

			beStalled := func(reason string, msg gtypes.GomegaMatcher) {
				GinkgoHelper()
				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					ByMarkingAsStalled(reason, msg)
				})
			}

			When("malformed source", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.GitSource.Branch = ""
					obj.Spec.GitSource.Commit = ""
				})
				beStalled(pulumiv1.StalledSpecInvalidReason, ContainSubstring(`missing "commit" or "branch"`))
			})

			When("unavailable source", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.GitSource.Branch = "invalid"
					obj.Spec.GitSource.Commit = ""
				})
				beStalled(pulumiv1.StalledSourceUnavailableReason, ContainSubstring(`no commits found`))
			})

			When("the git source is available", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.GitSource.Commit = "2ca775387e522fd5c29668a85bfba2f8fd791848"
				})

				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					By("configuring the workspace")
					Expect(ws).ToNot(BeNil())
					Expect(ws.Spec.Git).To(HaveValue(Equal(autov1alpha1.GitSource{
						URL: obj.Spec.GitSource.ProjectRepo,
						Ref: obj.Spec.GitSource.Commit,
						Dir: obj.Spec.GitSource.RepoDir,
					})))
				})
			})

			Describe("git auth", func() {
				When("the git auth cannot be resolved", func() {
					BeforeEach(func(ctx context.Context) {
						obj.Spec.GitSource.GitAuth = &shared.GitAuthConfig{
							PersonalAccessToken: &shared.ResourceRef{
								SelectorType: shared.ResourceSelectorSecret,
								ResourceSelector: shared.ResourceSelector{
									SecretRef: &shared.SecretSelector{
										Name: "not-found",
										Key:  "token",
									},
								},
							},
						}
					})
					beStalled(pulumiv1.StalledSpecInvalidReason, ContainSubstring(`unable to get secret`))
				})
			})
		})
	})
})

func matchEvent(reason pulumiv1.StackEventReason) gtypes.GomegaMatcher {
	return ContainSubstring(string(reason))
}
