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
	"testing"
	"time"

	fluxsourcev1 "github.com/fluxcd/source-controller/api/v1"
	"github.com/go-logr/logr/testr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gtypes "github.com/onsi/gomega/types"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	autov1alpha1apply "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/apply/auto/v1alpha1"
	"github.com/stretchr/testify/assert"
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

func makeUpdate(name types.NamespacedName, stack *pulumiv1.Stack, spec autov1alpha1.UpdateSpec) *autov1alpha1.Update {
	labels := labelsForWorkspace(&stack.ObjectMeta)
	return &autov1alpha1.Update{
		TypeMeta: metav1.TypeMeta{
			APIVersion: autov1alpha1.GroupVersion.String(),
			Kind:       "Update",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       fmt.Sprintf("%s-%s", name.Name, utilrand.String(8)),
			Namespace:  name.Namespace,
			Labels:     labels,
			Finalizers: []string{pulumiFinalizer},
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
		currentUpdate = makeUpdate(objName, obj, autov1alpha1.UpdateSpec{
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

		return result, err
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

	beStalled := func(reason string, msg gtypes.GomegaMatcher) {
		GinkgoHelper()
		It("reconciles", func(ctx context.Context) {
			_, err := reconcileF(ctx)
			Expect(err).NotTo(HaveOccurred())
			ByMarkingAsStalled(reason, msg)
		})
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
							Expect(currentUpdate.Finalizers).To(ContainElement(pulumiFinalizer))
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
						LastResyncTime:       metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastAttemptedCommit:  "abcdef",
						LastSuccessfulCommit: "abcdef",
					}),
				)

				Describe("updateTemplate", func() {
					BeforeEach(func(ctx context.Context) {
						obj.Spec.UpdateTemplate = &shared.UpdateApplyConfiguration{
							Spec: &autov1alpha1apply.UpdateSpecApplyConfiguration{
								TtlAfterCompleted: &metav1.Duration{Duration: 42 * time.Minute},
							},
						}
					})
					It("reconciles", func(ctx context.Context) {
						_, err := reconcileF(ctx)
						Expect(err).NotTo(HaveOccurred())

						By("applying the update template")
						Expect(currentUpdate.Spec.TtlAfterCompleted).To(Equal(&metav1.Duration{Duration: 42 * time.Minute}))
					})
				})
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
				currentUpdate.Finalizers = []string{}
				Expect(k8sClient.Update(ctx, currentUpdate)).To(Succeed())
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

				By("removing the stack finalizer to allow the update to be deleted later")
				Expect(lastUpdate.Finalizers).ToNot(ContainElement(pulumiFinalizer))

				By("emitting an event")
				Expect(r.Recorder.(*record.FakeRecorder).Events).To(Receive(matchEvent(pulumiv1.StackUpdateFailure)))
			})

			When("a stack is locked", func() {
				BeforeEach(func(ctx context.Context) {
					currentUpdate.Status.Message = "the stack is locked; another update is in progress"
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
							Reason:             "UpdateConflict",
							Message:            "the stack is locked; another update is in progress",
							LastTransitionTime: metav1.Now(),
						},
					}
				})

				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					Expect(obj.Status.LastUpdate.Message).To(Equal("the stack is locked; another update is in progress"))
				})
			})

			When("an unknown error occurred running pulumi", func() {
				BeforeEach(func(ctx context.Context) {
					currentUpdate.Status.Message = "an unknown error occurred running up"
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
							Reason:             "Unknown",
							Message:            "an unknown error occurred running up",
							LastTransitionTime: metav1.Now(),
						},
					}
				})

				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					Expect(obj.Status.LastUpdate.Message).To(Equal("an unknown error occurred running up"))
				})
			})

			When("retrying", func() {
				JustBeforeEach(func(ctx context.Context) {
					obj.Status.LastUpdate = &shared.StackUpdateState{
						Generation:           1,
						State:                shared.FailedStackStateMessage,
						Name:                 "update-retried",
						Type:                 autov1alpha1.UpType,
						LastAttemptedCommit:  obj.Status.CurrentUpdate.Commit,
						LastSuccessfulCommit: "",
						Failures:             2,
					}
					Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
				})
				It("increments failures", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					Expect(obj.Status.LastUpdate).To(Not(BeNil()))
					Expect(obj.Status.LastUpdate.Failures).To(Equal(int64(3)))
				})

				When("with a new generation", func() {
					JustBeforeEach(func(ctx context.Context) {
						obj.Status.LastUpdate = &shared.StackUpdateState{
							Generation:           2,
							State:                shared.FailedStackStateMessage,
							Name:                 "update-retried-with-new-sha",
							Type:                 autov1alpha1.UpType,
							LastAttemptedCommit:  obj.Status.CurrentUpdate.Commit,
							LastSuccessfulCommit: "",
							Failures:             2,
						}
						Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
					})
					It("resets failures", func(ctx context.Context) {
						_, err := reconcileF(ctx)
						Expect(err).NotTo(HaveOccurred())

						Expect(obj.Status.LastUpdate).To(Not(BeNil()))
						Expect(obj.Status.LastUpdate.Failures).To(Equal(int64(0)))
					})
				})
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

				By("removing the stack finalizer to allow the update to be deleted later")
				Expect(lastUpdate.Finalizers).ToNot(ContainElement(pulumiFinalizer))

				By("emitting an event")
				Expect(r.Recorder.(*record.FakeRecorder).Events).To(Receive(matchEvent(pulumiv1.StackUpdateSuccessful)))
			})

			When("after retrying", func() {
				JustBeforeEach(func(ctx context.Context) {
					obj.Status.LastUpdate = &shared.StackUpdateState{
						Generation:          1,
						State:               shared.FailedStackStateMessage,
						Name:                "update-retried",
						Type:                autov1alpha1.UpType,
						LastAttemptedCommit: obj.Status.CurrentUpdate.Commit,
						Failures:            2,
					}
					Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
				})
				It("resets failures", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					Expect(obj.Status.LastUpdate).To(Not(BeNil()))
					Expect(obj.Status.LastUpdate.Failures).To(Equal(int64(0)))
				})

				When("with a new generation", func() {
					JustBeforeEach(func(ctx context.Context) {
						obj.Status.LastUpdate = &shared.StackUpdateState{
							Generation:          2,
							State:               shared.FailedStackStateMessage,
							Name:                "update-retried-with-new-sha",
							Type:                autov1alpha1.UpType,
							LastAttemptedCommit: obj.Status.CurrentUpdate.Commit,
							Failures:            2,
						}
						Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
					})
					It("resets failures", func(ctx context.Context) {
						_, err := reconcileF(ctx)
						Expect(err).NotTo(HaveOccurred())

						Expect(obj.Status.LastUpdate).To(Not(BeNil()))
						Expect(obj.Status.LastUpdate.Failures).To(Equal(int64(0)))
					})
				})
			})

			When("the update produced outputs", func() {
				BeforeEach(func(ctx context.Context) {
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "secret-output-test", Namespace: currentUpdate.Namespace,
							Annotations: map[string]string{
								"pulumi.com/secrets": `["should-be-scrubbed"]`,
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
					Expect(string(obj.Status.Outputs["plaintext"].Raw)).To(Equal(`"not-sensitive"`))
					Expect(string(obj.Status.Outputs["should-be-scrubbed"].Raw)).To(Equal(`"[secret]"`))
				})
			})
		})
	})

	Describe("Update Triggers", func() {
		useFluxSource()

		ByResyncing := func() {
			GinkgoHelper()
			ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingWorkspaceMessage))
			By("emitting an event")
			Expect(r.Recorder.(*record.FakeRecorder).Events).To(Receive(matchEvent(pulumiv1.StackUpdateDetected)))
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
					LastAttemptedCommit:  fluxRepo.Status.Artifact.Revision,
					LastSuccessfulCommit: "",
					Failures:             3,
				}
			})

			When("within cooldown period", func() {
				It("backs off exponentially", func(ctx context.Context) {
					res, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					// 10 seconds * 3^3 = 4m30s
					Expect(res.RequeueAfter).To(BeNumerically("~", 4*time.Minute, time.Minute))
					ByMarkingAsReconciling(pulumiv1.ReconcilingRetryReason, Equal("3 update failure(s)"))
				})

				When("the WorkspaceReclaimPolicy is set to Delete", func() {
					BeforeEach(func(ctx context.Context) {
						obj.Spec.WorkspaceReclaimPolicy = shared.WorkspaceReclaimDelete
					})
					It("does not delete the workspace pod", func(ctx context.Context) {
						_, err := reconcileF(ctx)
						Expect(err).NotTo(HaveOccurred())
						By("not deleting the Workspace object")
						Expect(ws.GetName()).NotTo(BeEmpty())
					})
				})
			})
			When("done cooling down", func() {
				BeforeEach(func() {
					obj.Status.LastUpdate.LastResyncTime = metav1.NewTime(time.Now().Add(-1 * time.Hour))
				})
				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					ByResyncing()
				})
			})
		})

		When("the last update was not successful and max retry cooldown is set", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Status.LastUpdate = &shared.StackUpdateState{
					Generation:           1,
					State:                shared.FailedStackStateMessage,
					Name:                 "update-abcdef",
					Type:                 autov1alpha1.UpType,
					LastResyncTime:       metav1.Now(),
					LastAttemptedCommit:  fluxRepo.Status.Artifact.Revision,
					LastSuccessfulCommit: "",
					Failures:             10,
				}
				// Set a custom max backoff duration (e.g., 5 minutes)
				obj.Spec.RetryMaxBackoffDurationSeconds = int64(300)
			})
			When("done cooling down with the custom duration", func() {
				It("backs off according to the max backoff", func(ctx context.Context) {
					res, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(res.RequeueAfter).To(BeNumerically("~", 5*time.Minute, time.Minute))
				})
				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					ByMarkingAsReconciling(pulumiv1.ReconcilingRetryReason, Equal("10 update failure(s)"))
				})
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
					LastAttemptedCommit:  fluxRepo.Status.Artifact.Revision,
					LastSuccessfulCommit: fluxRepo.Status.Artifact.Revision,
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

		When("the stack has a new sync request", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Annotations = map[string]string{
					shared.ReconcileRequestAnnotation: "after-test",
				}
				// assume that the previous update was successful
				obj.Status.LastUpdate = &shared.StackUpdateState{
					Generation:           1,
					ReconcileRequest:     "test",
					State:                shared.SucceededStackStateMessage,
					Name:                 "update-abcdef",
					Type:                 autov1alpha1.UpType,
					LastResyncTime:       metav1.Now(),
					LastAttemptedCommit:  fluxRepo.Status.Artifact.Revision,
					LastSuccessfulCommit: fluxRepo.Status.Artifact.Revision,
				}
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
				})
			})

			When("at latest commit", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Status.LastUpdate.LastAttemptedCommit = fluxRepo.Status.Artifact.Revision
					obj.Status.LastUpdate.LastSuccessfulCommit = fluxRepo.Status.Artifact.Revision
				})

				When("the WorkspaceReclaimPolicy is set to Delete", func() {
					BeforeEach(func(ctx context.Context) {
						obj.Spec.WorkspaceReclaimPolicy = shared.WorkspaceReclaimDelete
					})
					It("reconciles", func(ctx context.Context) {
						result, err := reconcileF(ctx)
						Expect(err).NotTo(HaveOccurred())
						ByMarkingAsReady()
						By("not requeuing")
						Expect(result).To(Equal(reconcile.Result{}))
						By("deleting the Workspace object")
						Expect(ws.GetName()).To(BeEmpty())
						By("emitting an event")
						Expect(r.Recorder.(*record.FakeRecorder).Events).To(Receive(matchEvent(pulumiv1.WorkspaceDeleted)))
					})
				})

				It("reconciles", func(ctx context.Context) {
					result, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					ByMarkingAsReady()
					By("not requeuing")
					Expect(result).To(Equal(reconcile.Result{}))
					By("not deleting the Workspace object")
					Expect(ws.GetName()).To(Equal(nameForWorkspace(&obj.ObjectMeta)))
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
				Expect(currentUpdate.Finalizers).To(ContainElement(pulumiFinalizer))
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

			Describe("updateTemplate", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.UpdateTemplate = &shared.UpdateApplyConfiguration{
						Spec: &autov1alpha1apply.UpdateSpecApplyConfiguration{
							TtlAfterCompleted: &metav1.Duration{Duration: 42 * time.Minute},
						},
					}
				})
				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					By("applying the update template")
					Expect(currentUpdate.Spec.TtlAfterCompleted).To(Equal(&metav1.Duration{Duration: 42 * time.Minute}))
				})
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

		When("the prerequisite has a new generation since the last update", func() {
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

		When("the prerequisite has a new sync request since the last update", func() {
			BeforeEach(func(ctx context.Context) {
				prereq.Annotations = map[string]string{
					shared.ReconcileRequestAnnotation: "after-test",
				}
				prereq.Status.LastUpdate = &shared.StackUpdateState{
					Generation:          1,
					ReconcileRequest:    "test",
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

				beNotSatisfied(ContainSubstring("prerequisite is out of date"))

				It("reconciles", func(ctx context.Context) {
					result, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))

					By("touching the prerequisite")
					prereqName := types.NamespacedName{Namespace: prereq.Namespace, Name: prereq.Name}
					prereq := &pulumiv1.Stack{}
					Expect(k8sClient.Get(ctx, prereqName, prereq)).To(Succeed())
					Expect(prereq.Annotations).To(HaveKeyWithValue(shared.ReconcileRequestAnnotation, "after-update-abcdef"))
				})
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
			// use-case: user wants to fetch the source themselves, e.g. via init container
			BeforeEach(func(ctx context.Context) {
				obj.Spec.GitSource = nil
				obj.Spec.FluxSource = nil
				obj.Spec.ProgramRef = nil
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				By("configuring the workspace")
				Expect(ws).ToNot(BeNil())
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

			When("the flux source is not found", func() {
				JustBeforeEach(func(ctx context.Context) {
					Expect(k8sClient.Delete(ctx, fluxRepo)).To(Succeed())
				})
				beStalled(pulumiv1.StalledSourceUnavailableReason, ContainSubstring("could not resolve sourceRef"))
			})

			When("the flux source has no artifact", func() {
				BeforeEach(func(ctx context.Context) {
					fluxRepo.Status.Artifact = nil
				})
				beStalled(pulumiv1.StalledSourceUnavailableReason, ContainSubstring("Flux source has no artifact"))
			})

			When("the flux source has an artifact", func() {
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

	Describe("Environment Variables", func() {
		useFluxSource()

		var secret *corev1.Secret
		BeforeEach(func(ctx context.Context) {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-", Namespace: obj.Namespace,
				},
				StringData: map[string]string{
					"foo": "bar",
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		})

		var cm *corev1.ConfigMap
		BeforeEach(func(ctx context.Context) {
			cm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-", Namespace: obj.Namespace,
				},
				Data: map[string]string{
					"foo": "bar",
				},
			}
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())
		})

		Describe("envs", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.Envs = []string{cm.Name}
			})

			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				By("applying environment variables to the workspace")
				Expect(ws.Spec.EnvFrom).To(HaveExactElements(
					corev1.EnvFromSource{
						ConfigMapRef: &corev1.ConfigMapEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: cm.Name},
						},
					},
				))
			})
		})

		Describe("secretEnvs", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.SecretEnvs = []string{secret.Name}
			})

			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				By("applying environment variables to the workspace")
				Expect(ws.Spec.EnvFrom).To(HaveExactElements(
					corev1.EnvFromSource{
						SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
						},
					},
				))
			})
		})

		Describe("envRefs", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.EnvRefs = map[string]shared.ResourceRef{
					"c": {
						SelectorType: shared.ResourceSelectorLiteral,
						ResourceSelector: shared.ResourceSelector{
							LiteralRef: &shared.LiteralRef{
								Value: "c",
							},
						},
					},
					"b": {
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Name: secret.Name,
								Key:  "foo",
							},
						},
					},
				}
			})

			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				By("applying environment variable (refs) to the workspace")
				Expect(ws.Spec.Env).To(HaveExactElements(
					corev1.EnvVar{
						Name: "b",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secret.Name},
								Key:                  "foo",
							},
						},
					},
					corev1.EnvVar{
						Name:  "c",
						Value: "c",
					},
				))
			})

			Describe("deprecated ref type: Env", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.EnvRefs = map[string]shared.ResourceRef{
						"foo": {
							SelectorType: shared.ResourceSelectorEnv,
							ResourceSelector: shared.ResourceSelector{
								Env: &shared.EnvSelector{
									Name: "FOO",
								},
							},
						},
					}
				})
				beStalled(pulumiv1.StalledSpecInvalidReason, ContainSubstring(`ref type "Env" is deprecated`))
			})

			Describe("deprecated ref type: FS", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.EnvRefs = map[string]shared.ResourceRef{
						"foo": {
							SelectorType: shared.ResourceSelectorFS,
							ResourceSelector: shared.ResourceSelector{
								FileSystem: &shared.FSSelector{
									Path: "foo",
								},
							},
						},
					}
				})
				beStalled(pulumiv1.StalledSpecInvalidReason, ContainSubstring(`ref type "FS" is deprecated`))
			})
		})
	})

	Describe("Stack Configuration", func() {
		useFluxSource()

		BeforeEach(func(ctx context.Context) {
			obj.Spec.Stack = "dev"
		})

		It("reconciles", func(ctx context.Context) {
			_, err := reconcileF(ctx)
			Expect(err).NotTo(HaveOccurred())
			By("configuring a workspace stack")
			Expect(ws).ToNot(BeNil())
			Expect(ws.Spec.Stacks).To(HaveLen(1))
			actual := ws.Spec.Stacks[0]
			Expect(actual.Name).To(Equal(obj.Spec.Stack))
		})

		Describe("config", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.Config = map[string]string{
					"c": "c",
					"b": "b",
				}
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				actual := ws.Spec.Stacks[0]
				By("applying workspace config values (in key order)")
				Expect(actual.Config).To(HaveExactElements(
					autov1alpha1.ConfigItem{Key: "b", Secret: ptr.To(false), Value: ptr.To("b")},
					autov1alpha1.ConfigItem{Key: "c", Secret: ptr.To(false), Value: ptr.To("c")},
				))
			})
		})

		Describe("environment", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.Environment = []string{"test/test"}
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				actual := ws.Spec.Stacks[0]
				By("applying workspace stack environment")
				Expect(actual.Environment).To(Equal([]string{"test/test"}))
			})
		})

		Describe("secrets", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.Secrets = map[string]string{
					"c": "c",
					"b": "b",
				}
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				actual := ws.Spec.Stacks[0]
				By("applying workspace config values (in key order and marked as secrets)")
				Expect(actual.Config).To(HaveExactElements(
					autov1alpha1.ConfigItem{Key: "b", Secret: ptr.To(true), Value: ptr.To("b")},
					autov1alpha1.ConfigItem{Key: "c", Secret: ptr.To(true), Value: ptr.To("c")},
				))
			})
		})

		Describe("secretRefs", func() {
			var secret *corev1.Secret
			BeforeEach(func(ctx context.Context) {
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-", Namespace: obj.Namespace,
					},
					StringData: map[string]string{
						"foo": "bar",
					},
				}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			})

			BeforeEach(func(ctx context.Context) {
				obj.Spec.SecretRefs = map[string]shared.ResourceRef{
					"c": {
						SelectorType: shared.ResourceSelectorLiteral,
						ResourceSelector: shared.ResourceSelector{
							LiteralRef: &shared.LiteralRef{
								Value: "c",
							},
						},
					},
					"b": {
						SelectorType: shared.ResourceSelectorSecret,
						ResourceSelector: shared.ResourceSelector{
							SecretRef: &shared.SecretSelector{
								Name: secret.Name,
								Key:  "foo",
							},
						},
					},
				}
			})

			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				actual := ws.Spec.Stacks[0]
				By("applying workspace config values (in key order and marked as secrets)")
				Expect(actual.Config).To(HaveExactElements(
					autov1alpha1.ConfigItem{Key: "b", Secret: ptr.To(true), ValueFrom: &autov1alpha1.ConfigValueFrom{
						Path: "/var/run/secrets/stacks.pulumi.com/secrets/" + secret.Name + "/foo",
					}},
					autov1alpha1.ConfigItem{Key: "c", Secret: ptr.To(true), Value: ptr.To("c")},
				))
				By("attaching the secret(s) as a volume")
				Expect(ws.Spec.PodTemplate.Spec.Volumes).To(ContainElement(corev1.Volume{
					Name: "secret-" + secret.Name,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secret.Name,
						},
					},
				}))
				By("mounting the volume to a local path")
				wspc := ws.Spec.PodTemplate.Spec.Containers[0]
				Expect(wspc.Name).To(Equal("pulumi"))
				Expect(wspc.VolumeMounts).To(ContainElement(corev1.VolumeMount{
					Name:      "secret-" + secret.Name,
					MountPath: "/var/run/secrets/stacks.pulumi.com/secrets/" + secret.Name,
				}))
			})

			Describe("deprecated ref type: Env", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.SecretRefs = map[string]shared.ResourceRef{
						"foo": {
							SelectorType: shared.ResourceSelectorEnv,
							ResourceSelector: shared.ResourceSelector{
								Env: &shared.EnvSelector{
									Name: "FOO",
								},
							},
						},
					}
				})
				beStalled(pulumiv1.StalledSpecInvalidReason, ContainSubstring(`ref type "Env" is deprecated`))
			})

			Describe("deprecated ref type: FS", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.SecretRefs = map[string]shared.ResourceRef{
						"foo": {
							SelectorType: shared.ResourceSelectorFS,
							ResourceSelector: shared.ResourceSelector{
								FileSystem: &shared.FSSelector{
									Path: "foo",
								},
							},
						},
					}
				})
				beStalled(pulumiv1.StalledSpecInvalidReason, ContainSubstring(`ref type "FS" is deprecated`))
			})
		})
	})

	Describe("Workspace Customization", func() {
		useFluxSource()

		When("a service account is specified", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.ServiceAccountName = "pulumi"
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				By("configuring the workspace")
				Expect(ws).ToNot(BeNil())
				Expect(ws.Spec.ServiceAccountName).To(Equal("pulumi"))
			})
		})
	})
})

func matchEvent(reason pulumiv1.StackEventReason) gtypes.GomegaMatcher {
	return ContainSubstring(string(reason))
}

func TestExactlyOneOf(t *testing.T) {
	tests := []struct {
		name     string
		input    []bool
		expected bool
	}{
		{
			name:     "No true values",
			input:    []bool{false, false, false},
			expected: false,
		},
		{
			name:     "One true value",
			input:    []bool{false, true, false},
			expected: true,
		},
		{
			name:     "Multiple true values",
			input:    []bool{true, true, false},
			expected: false,
		},
		{
			name:     "All true values",
			input:    []bool{true, true, true},
			expected: false,
		},
		{
			name:     "Empty input",
			input:    []bool{},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := exactlyOneOf(tc.input...)
			if result != tc.expected {
				t.Errorf("exactlyOneOf(%v) = %v; want %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsSynced(t *testing.T) {
	tests := []struct {
		name          string
		stack         pulumiv1.Stack
		currentCommit string

		want bool
	}{
		{
			name:  "no update yet",
			stack: pulumiv1.Stack{},
			want:  false,
		},
		{
			name: "generation mismatch",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{
					Generation: int64(2),
				},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						Generation: int64(1),
					},
				},
			},
			want: false,
		},
		{
			name: "marked for deletion",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(metav1.Now())},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:                shared.SucceededStackStateMessage,
						LastSuccessfulCommit: "something-else",
					},
				},
			},
			want: true,
		},
		{
			name: "last update succeeeded but a new commit is available",
			stack: pulumiv1.Stack{
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:                shared.SucceededStackStateMessage,
						LastSuccessfulCommit: "old-sha",
					},
				},
			},
			currentCommit: "new-sha",
			want:          false,
		},
		{
			name: "last update succeeeded and we don't continue on commit match",
			stack: pulumiv1.Stack{
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:                shared.SucceededStackStateMessage,
						LastSuccessfulCommit: "sha",
					},
				},
			},
			currentCommit: "sha",
			want:          true,
		},
		{
			name: "last update succeeeded and we continue on commit match but we're inside the resync interval",
			stack: pulumiv1.Stack{
				Spec: shared.StackSpec{
					ContinueResyncOnCommitMatch: true,
				},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:                shared.SucceededStackStateMessage,
						LastSuccessfulCommit: "sha",
						LastResyncTime:       metav1.Now(),
					},
				},
			},
			currentCommit: "sha",
			want:          true,
		},
		{
			name: "last update succeeeded and we continue on commit match and we're outside the resync interval",
			stack: pulumiv1.Stack{
				Spec: shared.StackSpec{
					ContinueResyncOnCommitMatch: true,
				},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:                shared.SucceededStackStateMessage,
						LastSuccessfulCommit: "sha",
						LastResyncTime:       metav1.NewTime(time.Now().Add(-1 * time.Hour)),
					},
				},
			},
			currentCommit: "sha",
			want:          false,
		},
		{
			name: "last update failed but we're inside the cooldown interval",
			stack: pulumiv1.Stack{
				Spec: shared.StackSpec{},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:               shared.FailedStackStateMessage,
						LastAttemptedCommit: "sha",
						LastResyncTime:      metav1.Now(),
					},
				},
			},
			currentCommit: "sha",
			want:          true,
		},
		{
			name: "last update failed and we're inside the cooldown interval, marked for deletion",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(metav1.Now())},
				Spec: shared.StackSpec{
					DestroyOnFinalize: true,
				},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:               shared.FailedStackStateMessage,
						LastAttemptedCommit: "sha",
						LastResyncTime:      metav1.Now(),
					},
				},
			},
			currentCommit: "sha",
			want:          true,
		},
		{
			name: "last update failed and we're outside the cooldown interval",
			stack: pulumiv1.Stack{
				Spec: shared.StackSpec{},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:               shared.FailedStackStateMessage,
						LastAttemptedCommit: "sha",
						LastResyncTime:      metav1.NewTime(time.Now().Add(-1 * time.Hour)),
					},
				},
			},
			currentCommit: "sha",
			want:          false,
		},
		{
			name: "last update failed and commit changed, inside cooldown interval",
			stack: pulumiv1.Stack{
				Spec: shared.StackSpec{},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:               shared.FailedStackStateMessage,
						LastResyncTime:      metav1.Now(),
						LastAttemptedCommit: "old-sha",
					},
				},
			},
			currentCommit: "new-sha",
			want:          false,
		},
		{
			name: "last update failed and commit changed, marked for deletion",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(metav1.Now())},
				Spec: shared.StackSpec{
					DestroyOnFinalize: true,
				},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:               shared.FailedStackStateMessage,
						LastResyncTime:      metav1.Now(),
						LastAttemptedCommit: "old-sha",
					},
				},
			},
			currentCommit: "new-sha",
			want:          false,
		},
		{
			name: "unrecognized state",
			stack: pulumiv1.Stack{
				Spec: shared.StackSpec{},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State: "unknown",
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := testr.New(t)
			rec := record.NewFakeRecorder(10)
			if tt.want {
				assert.True(t, isSynced(log, rec, &tt.stack, tt.currentCommit), "expected to be in sync (not necessitating an update)")
			} else {
				assert.False(t, isSynced(log, rec, &tt.stack, tt.currentCommit), "expected to NOT be in sync (necessitating an update)")
			}
		})
	}
}
