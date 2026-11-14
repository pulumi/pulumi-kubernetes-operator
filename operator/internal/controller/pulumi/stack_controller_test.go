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
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
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
	stackapply "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/apply/pulumi/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	testFinalizer = "test.finalizer.pulumi.com"
)

// applyPatch implements client.Patch for server-side apply in tests.
type applyPatch struct {
	patch interface{}
}

func (p applyPatch) Type() types.PatchType {
	return types.ApplyPatchType
}

func (p applyPatch) Data(obj client.Object) ([]byte, error) {
	return json.Marshal(p.patch)
}

// stackStatusSSAPatch builds an SSA patch for a Stack's status. This ensures
// that the test setup uses the same field manager type ("Apply") as the controller,
// so that SSA can properly clear fields when they are later omitted.
func stackStatusSSAPatch(stack *pulumiv1.Stack) applyPatch {
	statusApply := stackapply.StackStatus().
		WithObservedGeneration(stack.Status.ObservedGeneration).
		WithObservedReconcileRequest(stack.Status.ObservedReconcileRequest)
	if stack.Status.Outputs != nil {
		statusApply = statusApply.WithOutputs(stack.Status.Outputs)
	}
	for i := range stack.Status.Conditions {
		c := &stack.Status.Conditions[i]
		condApply := metav1apply.Condition().
			WithType(c.Type).
			WithStatus(c.Status).
			WithLastTransitionTime(c.LastTransitionTime).
			WithReason(c.Reason).
			WithMessage(c.Message)
		if c.ObservedGeneration != 0 {
			condApply = condApply.WithObservedGeneration(c.ObservedGeneration)
		}
		statusApply = statusApply.WithConditions(condApply)
	}
	if stack.Status.LastUpdate != nil {
		lastUpdate := *stack.Status.LastUpdate
		// Zero-valued metav1.Time serializes as JSON null, which the CRD
		// rejects. The controller always sets LastResyncTime to metav1.Now(),
		// so use a non-zero default for test setup.
		if lastUpdate.LastResyncTime.IsZero() {
			lastUpdate.LastResyncTime = metav1.Now()
		}
		statusApply = statusApply.WithLastUpdate(lastUpdate)
	}
	if stack.Status.CurrentUpdate != nil {
		statusApply = statusApply.WithCurrentUpdate(*stack.Status.CurrentUpdate)
	}
	return applyPatch{stackapply.Stack(stack.Name, stack.Namespace).WithStatus(statusApply)}
}

// jsonValue creates a JSON value from a string for testing
func jsonValue(s string) *apiextensionsv1.JSON {
	return &apiextensionsv1.JSON{Raw: []byte(`"` + s + `"`)}
}

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
			Finalizers: []string{PulumiFinalizer},
		},
		Spec: spec,
	}
}

// updateHidingClient is a cached-client stand-in that reports one named Update as absent,
// reproducing a cold or lagging informer while the object really does exist in the API.
type updateHidingClient struct {
	client.Client
	hidden string
}

func (c updateHidingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*autov1alpha1.Update); ok && key.Name == c.hidden {
		return apierrors.NewNotFound(autov1alpha1.GroupVersion.WithResource("updates").GroupResource(), key.Name)
	}
	return c.Client.Get(ctx, key, obj, opts...)
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
			Client:    k8sClient,
			APIReader: k8sClient,
			Scheme:    k8sClient.Scheme(),
			Recorder:  record.NewFakeRecorder(10),
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
		// Use SSA to set status so the field manager has "Apply" type entries.
		// This ensures the controller's SSA patches can later clear fields by omitting them.
		Expect(k8sClient.Status().Patch(ctx, obj, stackStatusSSAPatch(obj),
			client.FieldOwner(StackStatusFieldManager), client.ForceOwnership)).To(Succeed())
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
		// Remove the finalizer before creating; it will be added via SSA with the correct field manager
		finalizers := currentUpdate.Finalizers
		currentUpdate.Finalizers = nil
		status := currentUpdate.Status
		Expect(k8sClient.Patch(ctx, currentUpdate, client.Apply, client.FieldOwner(FieldManager))).To(Succeed())
		currentUpdate.Status = status
		Expect(k8sClient.Status().Update(ctx, currentUpdate, client.FieldOwner(FieldManager))).To(Succeed())
		// Add the finalizer via SSA with the dedicated field manager, matching real controller behavior
		if len(finalizers) > 0 {
			finalizerPatch := autov1alpha1apply.Update(currentUpdate.Name, currentUpdate.Namespace).
				WithFinalizers(finalizers...)
			Expect(k8sClient.Patch(ctx, currentUpdate, applyPatch{finalizerPatch}, client.FieldOwner(StackFinalizerFieldManager))).To(Succeed())
		}
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
			controllerutil.AddFinalizer(obj, PulumiFinalizer)
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
				controllerutil.RemoveFinalizer(obj, PulumiFinalizer)
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
				Expect(obj.Finalizers).To(ContainElement(PulumiFinalizer), "not remove the finalizer")
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
				Expect(obj.Finalizers).ToNot(ContainElement(PulumiFinalizer), "remove the finalizer")
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
							Expect(obj.Finalizers).To(ContainElement(PulumiFinalizer), "not remove the finalizer")
							ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingUpdateMessage))

							By("creating a destroy op")
							Expect(currentUpdate).ToNot(BeNil())
							Expect(currentUpdate.Finalizers).To(ContainElement(PulumiFinalizer))
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
					Expect(k8sClient.Status().Patch(ctx, obj, stackStatusSSAPatch(obj),
						client.FieldOwner(StackStatusFieldManager), client.ForceOwnership)).To(Succeed())
				})
				It("reconciles", func(ctx context.Context) {
					result, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(obj.Finalizers).ToNot(ContainElement(PulumiFinalizer), "remove the finalizer")
					ByMarkingAsReady()
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
					Failures:             maxUpdateFailures - 1, // one below the limit: still retrying
				}
			})

			When("within cooldown period", func() {
				It("backs off exponentially", func(ctx context.Context) {
					res, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())
					// 10 seconds * 3^2 = 1m30s
					Expect(res.RequeueAfter).To(BeNumerically("~", 90*time.Second, 30*time.Second))
					ByMarkingAsReconciling(pulumiv1.ReconcilingRetryReason, Equal("2 update failure(s)"))
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

		When("the update has failed past the retry limit", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Status.LastUpdate = &shared.StackUpdateState{
					Generation:           1,
					State:                shared.FailedStackStateMessage,
					Name:                 "update-abcdef",
					Type:                 autov1alpha1.UpType,
					LastResyncTime:       metav1.NewTime(time.Now().Add(-1 * time.Hour)), // cooldown long elapsed
					LastAttemptedCommit:  fluxRepo.Status.Artifact.Revision,
					LastSuccessfulCommit: "",
					Failures:             10, // past maxUpdateFailures
				}
			})
			It("stops retrying - no backoff requeue", func(ctx context.Context) {
				res, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.RequeueAfter).To(BeZero())
			})
			It("marks the stack as stalled", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsStalled(pulumiv1.StalledUpdateFailedReason, Equal("up failed 10 times; not retrying until the spec or source changes"))
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
				Expect(currentUpdate.Finalizers).To(ContainElement(PulumiFinalizer))
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

		Describe("Preview mode", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.Preview = true

				// make the workspace be ready for an update
				ws.Status.ObservedGeneration = 1
				ws.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: metav1.Now()},
				}
			})

			It("creates a preview update", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				ByMarkingAsReconciling(pulumiv1.ReconcilingProcessingReason, Equal(pulumiv1.ReconcilingProcessingUpdateMessage))

				By("creating a preview update op")
				Expect(currentUpdate).ToNot(BeNil())
				Expect(currentUpdate.Spec.Type).To(Equal(autov1alpha1.PreviewType))
			})

			When("marked for deletion with destroyOnFinalize", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.DestroyOnFinalize = true
					controllerutil.AddFinalizer(obj, PulumiFinalizer)
				})

				JustBeforeEach(func(ctx context.Context) {
					Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
					obj = &pulumiv1.Stack{}
					_ = k8sClient.Get(ctx, objName, obj)
				})

				It("finalizes without creating a destroy operation", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					By("finalizing the object")
					Expect(obj.Finalizers).ToNot(ContainElement(PulumiFinalizer))

					By("not creating a destroy update")
					Expect(currentUpdate.Spec.Type).ToNot(Equal(autov1alpha1.DestroyType))
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

		When("the stack is being deleted", func() {
			BeforeEach(func(ctx context.Context) {
				obj.Spec.DestroyOnFinalize = true
				controllerutil.AddFinalizer(obj, PulumiFinalizer)

				// make the workspace be ready for an update
				ws.Status.ObservedGeneration = 1
				ws.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Succeeded", LastTransitionTime: metav1.Now()},
				}
			})

			JustBeforeEach(func(ctx context.Context) {
				// mark the object for deletion
				Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
				obj = &pulumiv1.Stack{}
				_ = k8sClient.Get(ctx, objName, obj)
			})

			When("the prerequisite is missing", func() {
				JustBeforeEach(func(ctx context.Context) {
					Expect(k8sClient.Delete(ctx, prereq)).To(Succeed())
				})

				It("proceeds with deletion despite missing prerequisite", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					By("creating a destroy op even though prerequisite is missing")
					Expect(currentUpdate).ToNot(BeNil())
					Expect(currentUpdate.Spec.Type).To(Equal(autov1alpha1.DestroyType))

					By("not marking as failed due to missing prerequisite")
					for _, condition := range obj.Status.Conditions {
						if condition.Type == "Reconciling" {
							Expect(condition.Reason).NotTo(Equal(pulumiv1.ReconcilingPrerequisiteNotSatisfiedReason))
						}
					}
				})
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
					obj.Spec.Branch = ""
					obj.Spec.Commit = ""
				})
				beStalled(pulumiv1.StalledSpecInvalidReason, ContainSubstring(`missing "commit" or "branch"`))
			})

			When("unavailable source", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.Branch = "invalid"
					obj.Spec.Commit = ""
				})
				beStalled(pulumiv1.StalledSourceUnavailableReason, ContainSubstring(`no commits found`))
			})

			When("the git source is available", func() {
				BeforeEach(func(ctx context.Context) {
					obj.Spec.Commit = "2ca775387e522fd5c29668a85bfba2f8fd791848"
				})

				It("reconciles", func(ctx context.Context) {
					_, err := reconcileF(ctx)
					Expect(err).NotTo(HaveOccurred())

					By("configuring the workspace")
					Expect(ws).ToNot(BeNil())
					Expect(ws.Spec.Git).To(HaveValue(Equal(autov1alpha1.GitSource{
						URL: obj.Spec.ProjectRepo,
						Ref: obj.Spec.Commit,
						Dir: obj.Spec.RepoDir,
					})))
				})
			})

			Describe("git auth", func() {
				When("the git auth cannot be resolved", func() {
					BeforeEach(func(ctx context.Context) {
						obj.Spec.GitAuth = &shared.GitAuthConfig{
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
				obj.Spec.Config = map[string]apiextensionsv1.JSON{
					"c": {Raw: []byte(`"c"`)},
					"b": {Raw: []byte(`"b"`)},
				}
			})
			It("reconciles", func(ctx context.Context) {
				_, err := reconcileF(ctx)
				Expect(err).NotTo(HaveOccurred())
				actual := ws.Spec.Stacks[0]
				By("applying workspace config values (in key order)")
				Expect(actual.Config).To(HaveExactElements(
					autov1alpha1.ConfigItem{Key: "b", Secret: ptr.To(false), Value: jsonValue("b")},
					autov1alpha1.ConfigItem{Key: "c", Secret: ptr.To(false), Value: jsonValue("c")},
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
					autov1alpha1.ConfigItem{Key: "b", Secret: ptr.To(true), Value: jsonValue("b")},
					autov1alpha1.ConfigItem{Key: "c", Secret: ptr.To(true), Value: jsonValue("c")},
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
					autov1alpha1.ConfigItem{Key: "c", Secret: ptr.To(true), Value: jsonValue("c")},
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

		want        bool
		wantMessage string
	}{
		{
			name:        "no update yet",
			stack:       pulumiv1.Stack{},
			want:        false,
			wantMessage: "Initial stack update",
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
			want:        false,
			wantMessage: "New stack generation: 2",
		},
		{
			name: "generation mismatch (deletion case)",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{
					Generation:        int64(2),
					DeletionTimestamp: ptr.To(metav1.Now()),
				},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						Generation: int64(1),
					},
				},
			},
			want:        false,
			wantMessage: "Stack marked for deletion",
		},
		{
			name: "marked for deletion after successful up, matching commit (destroy still needs to run)",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(metav1.Now())},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:                shared.SucceededStackStateMessage,
						Type:                 autov1alpha1.UpType,
						LastSuccessfulCommit: "abc",
					},
				},
			},
			currentCommit: "abc",
			want:          false,
			wantMessage:   "Stack marked for deletion",
		},
		{
			name: "marked for deletion after successful up, no source (LastSuccessfulCommit and currentCommit both empty)",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(metav1.Now())},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State: shared.SucceededStackStateMessage,
						Type:  autov1alpha1.UpType,
					},
				},
			},
			want:        false,
			wantMessage: "Stack marked for deletion",
		},
		{
			name: "marked for deletion after successful destroy (safe to finalize)",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(metav1.Now())},
				Status: pulumiv1.StackStatus{
					LastUpdate: &shared.StackUpdateState{
						State:                shared.SucceededStackStateMessage,
						Type:                 autov1alpha1.DestroyType,
						LastSuccessfulCommit: "abc",
					},
				},
			},
			currentCommit: "abc",
			want:          true,
			wantMessage:   "",
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
			wantMessage:   "New commit detected: \"new-sha\"",
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
			wantMessage:   "",
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
			wantMessage:   "",
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
			wantMessage:   "Resync time elapsed: 1m0s",
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
			wantMessage:   "",
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
			wantMessage:   "",
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
			wantMessage:   "Backoff time elapsed: 10s",
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
			wantMessage:   "New commit detected: \"new-sha\"",
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
			wantMessage:   "New commit detected: \"new-sha\"",
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
			want:        true,
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := testr.New(t)
			rec := record.NewFakeRecorder(10)
			synced, message := isSynced(log, rec, &tt.stack, tt.currentCommit)
			if tt.want {
				assert.True(t, synced, "expected to be in sync (not necessitating an update)")
			} else {
				assert.False(t, synced, "expected to NOT be in sync (necessitating an update)")
			}
			assert.Equal(t, tt.wantMessage, message, "expected message to match")
		})
	}
}

// TestShouldUseProjectInfoWorkspaceForDestroy covers the predicate that decides
// whether the reconciler should bypass source resolution and use a projectInfo
// workspace to run `pulumi destroy` against backend state alone (see #1222).
func TestShouldUseProjectInfoWorkspaceForDestroy(t *testing.T) {
	now := metav1.Now()
	withProjectInfo := &shared.ProjectInfo{Name: "myproject", Runtime: "yaml"}

	tests := []struct {
		name  string
		stack pulumiv1.Stack
		want  bool
	}{
		{
			name: "not marked for deletion",
			stack: pulumiv1.Stack{
				Spec: shared.StackSpec{
					DestroyOnFinalize: true,
				},
				Status: pulumiv1.StackStatus{ProjectInfo: withProjectInfo},
			},
			want: false,
		},
		{
			name: "marked for deletion, but destroyOnFinalize=false",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(now)},
				Spec: shared.StackSpec{
					DestroyOnFinalize: false,
				},
				Status: pulumiv1.StackStatus{ProjectInfo: withProjectInfo},
			},
			want: false,
		},
		{
			name: "marked for deletion, destroyOnFinalize=true, but preview=true (no resources to destroy)",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(now)},
				Spec: shared.StackSpec{
					DestroyOnFinalize: true,
					Preview:           true,
				},
				Status: pulumiv1.StackStatus{ProjectInfo: withProjectInfo},
			},
			want: false,
		},
		{
			name: "marked for deletion, destroyOnFinalize=true, runProgram=true (need real source)",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(now)},
				Spec: shared.StackSpec{
					DestroyOnFinalize: true,
					RunProgram:        true,
				},
				Status: pulumiv1.StackStatus{ProjectInfo: withProjectInfo},
			},
			want: false,
		},
		{
			name: "marked for deletion, destroyOnFinalize=true, runProgram=false, no cached ProjectInfo",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(now)},
				Spec: shared.StackSpec{
					DestroyOnFinalize: true,
				},
				Status: pulumiv1.StackStatus{ProjectInfo: nil},
			},
			want: false,
		},
		{
			name: "marked for deletion, destroyOnFinalize=true, runProgram=false, ProjectInfo cached (the projectInfo-destroy path)",
			stack: pulumiv1.Stack{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: ptr.To(now)},
				Spec: shared.StackSpec{
					DestroyOnFinalize: true,
				},
				Status: pulumiv1.StackStatus{ProjectInfo: withProjectInfo},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseProjectInfoWorkspaceForDestroy(&tt.stack)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWithoutSources(t *testing.T) {
	t.Run("nil spec", func(t *testing.T) {
		got := withoutSources(shared.WorkspaceApplyConfiguration{})
		assert.Nil(t, got.Spec)
	})

	t.Run("strips sources without mutating the original", func(t *testing.T) {
		original := shared.WorkspaceApplyConfiguration{
			Spec: &autov1alpha1apply.WorkspaceSpecApplyConfiguration{
				Image:       ptr.To("custom:v1"),
				Local:       &autov1alpha1apply.LocalSourceApplyConfiguration{Dir: ptr.To("/app")},
				Git:         &autov1alpha1apply.GitSourceApplyConfiguration{URL: ptr.To("https://example.com")},
				Flux:        &autov1alpha1apply.FluxSourceApplyConfiguration{Url: ptr.To("https://example.com/artifact.tar.gz")},
				ProjectInfo: &autov1alpha1apply.ProjectInfoSourceApplyConfiguration{Name: ptr.To("myproject")},
			},
		}

		got := withoutSources(original)

		assert.Nil(t, got.Spec.Local)
		assert.Nil(t, got.Spec.Git)
		assert.Nil(t, got.Spec.Flux)
		assert.Nil(t, got.Spec.ProjectInfo)
		assert.Equal(t, "custom:v1", *got.Spec.Image, "non-source fields must survive")

		require.NotNil(t, original.Spec.Local, "the caller's template must not be mutated")
		assert.Equal(t, "/app", *original.Spec.Local.Dir)
		require.NotNil(t, original.Spec.Git)
		require.NotNil(t, original.Spec.Flux)
		require.NotNil(t, original.Spec.ProjectInfo)
	})
}

func TestSetProjectInfoSource(t *testing.T) {
	info := &shared.ProjectInfo{Name: "myproject", Runtime: "yaml"}

	tests := []struct {
		name string
		ws   autov1alpha1.WorkspaceSpec
	}{
		{
			name: "empty spec",
			ws:   autov1alpha1.WorkspaceSpec{},
		},
		{
			name: "displaces Git source",
			ws: autov1alpha1.WorkspaceSpec{
				Git: &autov1alpha1.GitSource{URL: "https://example.com/repo.git", Ref: "main"},
			},
		},
		{
			name: "displaces Flux source",
			ws: autov1alpha1.WorkspaceSpec{
				Flux: &autov1alpha1.FluxSource{Url: "http://flux/artifact.tgz", Digest: "sha256:abc"},
			},
		},
		{
			name: "displaces Local source",
			ws: autov1alpha1.WorkspaceSpec{
				Local: &autov1alpha1.LocalSource{Dir: "/workspace/project"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := tt.ws.DeepCopy()
			setProjectInfoSource(ws, info)

			require.NotNil(t, ws.ProjectInfo, "ProjectInfo must be set")
			assert.Equal(t, "myproject", ws.ProjectInfo.Name)
			assert.Equal(t, "yaml", ws.ProjectInfo.Runtime)
			assert.Nil(t, ws.Git, "Git source must be cleared")
			assert.Nil(t, ws.Flux, "Flux source must be cleared")
			assert.Nil(t, ws.Local, "Local source must be cleared")
		})
	}
}

func TestApplyProjectInfoToStack(t *testing.T) {
	tests := []struct {
		name   string
		stack  pulumiv1.Stack
		update autov1alpha1.Update
		want   *shared.ProjectInfo
	}{
		{
			name:   "update has no ProjectInfo: stack untouched",
			stack:  pulumiv1.Stack{},
			update: autov1alpha1.Update{},
			want:   nil,
		},
		{
			name:  "update has ProjectInfo: stack picks it up",
			stack: pulumiv1.Stack{},
			update: autov1alpha1.Update{
				Status: autov1alpha1.UpdateStatus{
					ProjectInfo: &autov1alpha1.ProjectInfo{Name: "myproject", Runtime: "yaml"},
				},
			},
			want: &shared.ProjectInfo{Name: "myproject", Runtime: "yaml"},
		},
		{
			name: "update has ProjectInfo: overwrites stale value on stack",
			stack: pulumiv1.Stack{
				Status: pulumiv1.StackStatus{
					ProjectInfo: &shared.ProjectInfo{Name: "old", Runtime: "yaml"},
				},
			},
			update: autov1alpha1.Update{
				Status: autov1alpha1.UpdateStatus{
					ProjectInfo: &autov1alpha1.ProjectInfo{Name: "new", Runtime: "python"},
				},
			},
			want: &shared.ProjectInfo{Name: "new", Runtime: "python"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyProjectInfoToStack(&tt.stack, &tt.update)
			assert.Equal(t, tt.want, tt.stack.Status.ProjectInfo)
		})
	}
}

func TestAddUpdateFinalizer(t *testing.T) {
	ctx := context.Background()

	testScheme := scheme.Scheme
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithReturnManagedFields().
		Build()

	r := &StackReconciler{
		Client: fakeClient,
	}

	t.Run("adds the finalizer via SSA", func(t *testing.T) {
		update := &autov1alpha1.Update{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update-add",
				Namespace: "default",
			},
			Spec: autov1alpha1.UpdateSpec{
				StackName:     "test-stack",
				WorkspaceName: "test-workspace",
			},
		}
		require.NoError(t, fakeClient.Create(ctx, update))

		err := r.addUpdateFinalizer(ctx, update)
		assert.NoError(t, err)

		// Verify the finalizer is added
		updated := &autov1alpha1.Update{}
		require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: update.Name, Namespace: update.Namespace}, updated))
		assert.Contains(t, updated.Finalizers, PulumiFinalizer)

		// Verify the finalizer is owned by StackFinalizerFieldManager (v0.22.0+ fake client supports managed fields)
		found := false
		for _, mf := range updated.GetManagedFields() {
			if mf.Manager == StackFinalizerFieldManager {
				found = true
				break
			}
		}
		assert.True(t, found, "expected finalizer to be owned by StackFinalizerFieldManager")
	})
}

func TestRemoveUpdateFinalizer(t *testing.T) {
	ctx := context.Background()

	testScheme := scheme.Scheme
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithReturnManagedFields().
		Build()

	r := &StackReconciler{
		Client: fakeClient,
	}

	t.Run("removes the finalizer when present", func(t *testing.T) {

		update := &autov1alpha1.Update{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update-remove-" + utilrand.String(5),
				Namespace: "default",
			},
			Spec: autov1alpha1.UpdateSpec{
				StackName:     "test-stack",
				WorkspaceName: "test-workspace",
			},
		}
		require.NoError(t, fakeClient.Create(ctx, update))

		// Add finalizer using SSA (the way it would be added in production)
		require.NoError(t, r.addUpdateFinalizer(ctx, update))

		// Re-fetch to get the updated object with finalizers
		require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: update.Name, Namespace: update.Namespace}, update))
		err := r.removeUpdateFinalizer(ctx, update)
		assert.NoError(t, err)

		// Verify the finalizer is removed
		updated := &autov1alpha1.Update{}
		require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: update.Name, Namespace: update.Namespace}, updated))
		assert.NotContains(t, updated.Finalizers, PulumiFinalizer)
	})

	t.Run("succeeds without error when finalizer not present (idempotent)", func(t *testing.T) {

		update := &autov1alpha1.Update{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-update-no-finalizer-" + utilrand.String(5),
				Namespace:  "default",
				Finalizers: []string{"other.finalizer.com"},
			},
			Spec: autov1alpha1.UpdateSpec{
				StackName:     "test-stack",
				WorkspaceName: "test-workspace",
			},
		}
		require.NoError(t, fakeClient.Create(ctx, update))

		err := r.removeUpdateFinalizer(ctx, update)
		assert.NoError(t, err)

		// Verify other finalizers are preserved
		updated := &autov1alpha1.Update{}
		require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: update.Name, Namespace: update.Namespace}, updated))
		assert.Contains(t, updated.Finalizers, "other.finalizer.com")
	})

	t.Run("returns nil when Update does not exist", func(t *testing.T) {

		update := &autov1alpha1.Update{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nonexistent-update",
				Namespace: "default",
			},
		}

		err := r.removeUpdateFinalizer(ctx, update)
		assert.NoError(t, err)
	})
}

func TestStackDeletionReapsTerminatingUpdates(t *testing.T) {
	testScheme := scheme.Scheme
	require.NoError(t, pulumiv1.AddToScheme(testScheme))
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.3-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	k8sclient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	ctx := t.Context()
	stackName := types.NamespacedName{Name: "deleting-stack-with-update", Namespace: "default"}
	stack := &pulumiv1.Stack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pulumiv1.GroupVersion.String(),
			Kind:       "Stack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       stackName.Name,
			Namespace:  stackName.Namespace,
			Finalizers: []string{testFinalizer},
		},
		Spec: shared.StackSpec{Stack: "organization/project/stack"},
	}
	require.NoError(t, k8sclient.Create(ctx, stack))

	updateName := types.NamespacedName{Name: "terminal-stack-update", Namespace: stack.Namespace}
	update := &autov1alpha1.Update{
		ObjectMeta: metav1.ObjectMeta{
			Name:            updateName.Name,
			Namespace:       updateName.Namespace,
			Labels:          labelsForWorkspace(&stack.ObjectMeta),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(stack, pulumiv1.GroupVersion.WithKind("Stack"))},
		},
		Spec: autov1alpha1.UpdateSpec{
			StackName:     stack.Spec.Stack,
			WorkspaceName: nameForWorkspace(&stack.ObjectMeta),
		},
	}
	require.NoError(t, k8sclient.Create(ctx, update))

	r := &StackReconciler{
		Client:   k8sclient,
		Scheme:   testScheme,
		Recorder: record.NewFakeRecorder(10),
	}
	require.NoError(t, r.addUpdateFinalizer(ctx, update))
	require.NoError(t, k8sclient.Get(ctx, updateName, update))
	update.Status.Conditions = []metav1.Condition{{
		Type:               autov1alpha1.UpdateConditionTypeComplete,
		Status:             metav1.ConditionTrue,
		Reason:             "Complete",
		LastTransitionTime: metav1.Now(),
	}}
	require.NoError(t, k8sclient.Status().Update(ctx, update))
	require.NoError(t, k8sclient.Delete(ctx, update))
	require.NoError(t, k8sclient.Delete(ctx, stack))

	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
	require.NoError(t, err)

	err = k8sclient.Get(ctx, updateName, &autov1alpha1.Update{})
	assert.True(t, apierrors.IsNotFound(err), "terminal Stack-owned Update should finish deleting")

	var retained pulumiv1.Stack
	require.NoError(t, k8sclient.Get(ctx, stackName, &retained))
	assert.Equal(t, []string{testFinalizer}, retained.Finalizers)
}

func TestIsFinalizerOwnedByLegacyManager(t *testing.T) {
	t.Run("returns false for an Update without finalizers", func(t *testing.T) {
		update := &autov1alpha1.Update{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update",
				Namespace: "default",
			},
		}
		assert.False(t, isFinalizerOwnedByLegacyManager(update))
	})

	t.Run("returns false for an Update with finalizer owned by StackFinalizerFieldManager", func(t *testing.T) {
		testScheme := scheme.Scheme
		require.NoError(t, autov1alpha1.AddToScheme(testScheme))

		fakeClient := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithReturnManagedFields().
			Build()

		r := &StackReconciler{
			Client: fakeClient,
		}

		ctx := context.Background()
		update := &autov1alpha1.Update{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-update-new",
				Namespace: "default",
			},
			Spec: autov1alpha1.UpdateSpec{
				StackName:     "test-stack",
				WorkspaceName: "test-workspace",
			},
		}
		require.NoError(t, fakeClient.Create(ctx, update))

		// Add finalizer via SSA with new field manager
		require.NoError(t, r.addUpdateFinalizer(ctx, update))

		// Re-fetch to get the updated object with managed fields
		require.NoError(t, fakeClient.Get(ctx, types.NamespacedName{Name: update.Name, Namespace: update.Namespace}, update))

		assert.False(t, isFinalizerOwnedByLegacyManager(update))
	})
}

// TestWorkspaceStalledPropagation verifies that when a Workspace has a Stalled
// condition, the StackReconciler propagates it to the Stack's status with the
// appropriate reason constant.
//
// Uses envtest (a real API server) because the fake client's managed fields
// tracking panics on deeply nested pointer fields in the Stack/Workspace types.
func TestWorkspaceStalledPropagation(t *testing.T) {
	// Start a dedicated envtest environment for this test.
	testScheme := scheme.Scheme
	require.NoError(t, pulumiv1.AddToScheme(testScheme))
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.3-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	k8sclient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	tests := []struct {
		name                string
		objName             string // lowercase k8s object name
		workspaceReason     string
		workspaceMessage    string
		wantStackReason     string
		wantMessageContains string
	}{
		{
			name:                "IncompatibleConfiguration maps to PulumiVersionTooLow",
			objName:             "stalled-incompat",
			workspaceReason:     "IncompatibleConfiguration",
			workspaceMessage:    "pulumi version too low for required features",
			wantStackReason:     pulumiv1.StalledPulumiVersionTooLowReason,
			wantMessageContains: "pulumi version too low",
		},
		{
			name:                "InstallationFailed maps to WorkspaceFailed",
			objName:             "stalled-install",
			workspaceReason:     "InstallationFailed",
			workspaceMessage:    "unable to install dependencies",
			wantStackReason:     pulumiv1.StalledWorkspaceFailedReason,
			wantMessageContains: "unable to install dependencies",
		},
		{
			name:                "InitializationFailed maps to WorkspaceFailed",
			objName:             "stalled-init",
			workspaceReason:     "InitializationFailed",
			workspaceMessage:    "failed to select stack: MissingRegion: could not find region configuration",
			wantStackReason:     pulumiv1.StalledWorkspaceFailedReason,
			wantMessageContains: "MissingRegion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			stackName := types.NamespacedName{
				Name:      tt.objName,
				Namespace: "default",
			}

			// 1. Create a Program with a ready artifact.
			program := &pulumiv1.Program{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stackName.Name + "-program",
					Namespace: stackName.Namespace,
				},
				Program: pulumiv1.ProgramSpec{
					Resources: map[string]pulumiv1.Resource{
						"password": {Type: "random:RandomPassword"},
					},
				},
			}
			require.NoError(t, k8sclient.Create(ctx, program))
			program.Status.Artifact = &pulumiv1.Artifact{
				Revision:       "test-revision",
				LastUpdateTime: metav1.Now(),
			}
			program.Status.ObservedGeneration = program.Generation
			require.NoError(t, k8sclient.Status().Update(ctx, program))
			t.Cleanup(func() { _ = k8sclient.Delete(ctx, program) })

			// 2. Create a Stack with a ProgramRef.
			stack := &pulumiv1.Stack{
				TypeMeta: metav1.TypeMeta{
					APIVersion: pulumiv1.GroupVersion.String(),
					Kind:       "Stack",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       stackName.Name,
					Namespace:  stackName.Namespace,
					Finalizers: []string{testFinalizer},
				},
				Spec: shared.StackSpec{
					Stack: "dev",
					ProgramRef: &shared.ProgramReference{
						Name: program.Name,
					},
				},
			}
			require.NoError(t, k8sclient.Create(ctx, stack))
			t.Cleanup(func() { _ = k8sclient.Delete(ctx, stack) })

			// 3. Create the Workspace with owner ref and stalled status.
			ws := makeWorkspace(stack.ObjectMeta)
			require.NoError(t, controllerutil.SetControllerReference(stack, ws, testScheme))
			require.NoError(t, k8sclient.Create(ctx, ws))
			ws.Status.Conditions = []metav1.Condition{
				{
					Type:               autov1alpha1.WorkspaceStalled,
					Status:             metav1.ConditionTrue,
					Reason:             tt.workspaceReason,
					Message:            tt.workspaceMessage,
					LastTransitionTime: metav1.Now(),
				},
			}
			require.NoError(t, k8sclient.Status().Update(ctx, ws, client.FieldOwner(FieldManager)))
			t.Cleanup(func() { _ = k8sclient.Delete(ctx, ws) })

			// 4. Reconcile.
			r := &StackReconciler{
				Client:    k8sclient,
				APIReader: k8sclient,
				Scheme:    testScheme,
				Recorder:  record.NewFakeRecorder(10),
			}
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
			require.NoError(t, err)

			// 5. Re-fetch the Stack and check conditions.
			var updated pulumiv1.Stack
			require.NoError(t, k8sclient.Get(ctx, stackName, &updated))

			stalledCond := meta.FindStatusCondition(updated.Status.Conditions, pulumiv1.StalledCondition)
			require.NotNil(t, stalledCond, "expected Stalled condition on Stack")
			assert.Equal(t, metav1.ConditionTrue, stalledCond.Status)
			assert.Equal(t, tt.wantStackReason, stalledCond.Reason)
			assert.Contains(t, stalledCond.Message, tt.wantMessageContains)
			assert.Contains(t, stalledCond.Message, tt.workspaceReason, "workspace stall reason should appear in the Stack's stalled message")

			readyCond := meta.FindStatusCondition(updated.Status.Conditions, pulumiv1.ReadyCondition)
			require.NotNil(t, readyCond, "expected Ready condition on Stack to be False")
			assert.Equal(t, metav1.ConditionFalse, readyCond.Status)

			reconcilingCond := meta.FindStatusCondition(updated.Status.Conditions, pulumiv1.ReconcilingCondition)
			assert.Nil(t, reconcilingCond, "expected Reconciling condition to be removed")
		})
	}
}

func TestProjectInfoDestroyBypassesUnavailableSource(t *testing.T) {
	testScheme := scheme.Scheme
	require.NoError(t, pulumiv1.AddToScheme(testScheme))
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.3-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	k8sclient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	ctx := context.Background()
	stackName := types.NamespacedName{Name: "project-info-destroy", Namespace: "default"}

	stack := &pulumiv1.Stack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pulumiv1.GroupVersion.String(),
			Kind:       "Stack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       stackName.Name,
			Namespace:  stackName.Namespace,
			Finalizers: []string{testFinalizer, PulumiFinalizer},
		},
		Spec: shared.StackSpec{
			Stack:             "dev",
			DestroyOnFinalize: true,
			ProgramRef:        &shared.ProgramReference{Name: "deleted-program"},
		},
	}
	require.NoError(t, k8sclient.Create(ctx, stack))
	t.Cleanup(func() { _ = k8sclient.Delete(ctx, stack) })

	stack.Status.ProjectInfo = &shared.ProjectInfo{Name: "myproject", Runtime: "yaml"}
	stack.Status.LastUpdate = &shared.StackUpdateState{
		Generation:           stack.Generation,
		Name:                 "update-prior",
		Type:                 autov1alpha1.UpType,
		State:                shared.SucceededStackStateMessage,
		LastAttemptedCommit:  "abc",
		LastSuccessfulCommit: "abc",
		LastResyncTime:       metav1.Now(),
	}
	require.NoError(t, k8sclient.Status().Update(ctx, stack))

	require.NoError(t, k8sclient.Delete(ctx, stack))

	r := &StackReconciler{
		Client:    k8sclient,
		APIReader: k8sclient,
		Scheme:    testScheme,
		Recorder:  record.NewFakeRecorder(10),
	}

	// First reconcile: takes the projectInfo bypass and applies a projectInfo workspace,
	// then waits for workspace readiness.
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
	require.NoError(t, err)

	wsName := types.NamespacedName{
		Name:      nameForWorkspace(&stack.ObjectMeta),
		Namespace: stackName.Namespace,
	}
	var ws autov1alpha1.Workspace
	require.NoError(t, k8sclient.Get(ctx, wsName, &ws))
	require.NotNil(t, ws.Spec.ProjectInfo, "Workspace.Spec.ProjectInfo must be set for projectInfo-destroy path")
	assert.Equal(t, "myproject", ws.Spec.ProjectInfo.Name)
	assert.Equal(t, "yaml", ws.Spec.ProjectInfo.Runtime)
	assert.Nil(t, ws.Spec.Flux, "Flux source must be cleared on projectInfo path")
	assert.Nil(t, ws.Spec.Git, "Git source must be cleared on projectInfo path")

	// Simulate the workspace controller marking the workspace Ready so the
	// Stack controller can proceed to create the destroy Update.
	ws.Status.ObservedGeneration = ws.Generation
	ws.Status.Conditions = []metav1.Condition{
		{
			Type:               autov1alpha1.WorkspaceReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Succeeded",
			LastTransitionTime: metav1.Now(),
		},
	}
	require.NoError(t, k8sclient.Status().Update(ctx, &ws))

	// Second reconcile: workspace is Ready, destroy Update is created.
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
	require.NoError(t, err)

	var updated pulumiv1.Stack
	require.NoError(t, k8sclient.Get(ctx, stackName, &updated))

	stalledCond := meta.FindStatusCondition(updated.Status.Conditions, pulumiv1.StalledCondition)
	if stalledCond != nil {
		assert.NotEqual(t, pulumiv1.StalledSourceUnavailableReason, stalledCond.Reason,
			"Stack should NOT be stalled on SourceUnavailable; projectInfo path should bypass source resolution")
	}

	require.NotNil(t, updated.Status.CurrentUpdate, "a destroy Update should have been initiated")

	var update autov1alpha1.Update
	require.NoError(t, k8sclient.Get(ctx, types.NamespacedName{
		Name:      updated.Status.CurrentUpdate.Name,
		Namespace: stackName.Namespace,
	}, &update))
	assert.Equal(t, autov1alpha1.DestroyType, update.Spec.Type, "Update CR must be of type destroy")
}

func TestProjectInfoDestroyClearsLocalFromWorkspaceTemplate(t *testing.T) {
	testScheme := scheme.Scheme
	require.NoError(t, pulumiv1.AddToScheme(testScheme))
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.3-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	k8sclient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	ctx := context.Background()
	stackName := types.NamespacedName{Name: "project-info-local-template", Namespace: "default"}

	stack := &pulumiv1.Stack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pulumiv1.GroupVersion.String(),
			Kind:       "Stack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       stackName.Name,
			Namespace:  stackName.Namespace,
			Finalizers: []string{testFinalizer, PulumiFinalizer},
		},
		Spec: shared.StackSpec{
			Stack:             "dev",
			DestroyOnFinalize: true,
			RunProgram:        false,
			WorkspaceTemplate: &shared.WorkspaceApplyConfiguration{
				Spec: &autov1alpha1apply.WorkspaceSpecApplyConfiguration{
					Local: &autov1alpha1apply.LocalSourceApplyConfiguration{Dir: ptr.To("/workspace/prog")},
				},
			},
		},
	}
	require.NoError(t, k8sclient.Create(ctx, stack))
	t.Cleanup(func() { _ = k8sclient.Delete(ctx, stack) })

	stack.Status.ProjectInfo = &shared.ProjectInfo{Name: "myproject", Runtime: "go"}
	stack.Status.LastUpdate = &shared.StackUpdateState{
		Generation:           stack.Generation,
		Name:                 "update-prior",
		Type:                 autov1alpha1.UpType,
		State:                shared.SucceededStackStateMessage,
		LastAttemptedCommit:  "abc",
		LastSuccessfulCommit: "abc",
		LastResyncTime:       metav1.Now(),
	}
	require.NoError(t, k8sclient.Status().Update(ctx, stack))
	require.NoError(t, k8sclient.Delete(ctx, stack))

	r := &StackReconciler{
		Client:   k8sclient,
		Scheme:   testScheme,
		Recorder: record.NewFakeRecorder(10),
	}
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
	require.NoError(t, err)

	var ws autov1alpha1.Workspace
	require.NoError(t, k8sclient.Get(ctx, types.NamespacedName{
		Name:      nameForWorkspace(&stack.ObjectMeta),
		Namespace: stackName.Namespace,
	}, &ws))

	require.NotNil(t, ws.Spec.ProjectInfo)
	assert.Nil(t, ws.Spec.Local, "workspaceTemplate.local must not survive the projectInfo destroy path")
	assert.Nil(t, ws.Spec.Git)
	assert.Nil(t, ws.Spec.Flux)
}

func TestFailedDestroyRetainsFinalizer(t *testing.T) {
	testScheme := scheme.Scheme
	require.NoError(t, pulumiv1.AddToScheme(testScheme))
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.3-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	k8sclient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	ctx := context.Background()
	stackName := types.NamespacedName{Name: "failed-destroy", Namespace: "default"}

	stack := &pulumiv1.Stack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pulumiv1.GroupVersion.String(),
			Kind:       "Stack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       stackName.Name,
			Namespace:  stackName.Namespace,
			Finalizers: []string{testFinalizer, PulumiFinalizer},
		},
		Spec: shared.StackSpec{Stack: "dev", DestroyOnFinalize: true},
	}
	require.NoError(t, k8sclient.Create(ctx, stack))
	t.Cleanup(func() { _ = k8sclient.Delete(ctx, stack) })

	// Mark for deletion, then record a destroy that just FAILED (still within its backoff
	// cooldown). ProjectInfo makes the destroy take the source-less bypass (currentCommit == "");
	// seeding the status after the delete keeps LastUpdate.Generation aligned with the
	// post-delete generation. Together these make isSynced return true on the failed-update
	// path — exactly where the old code removed the finalizer.
	require.NoError(t, k8sclient.Delete(ctx, stack))
	require.NoError(t, k8sclient.Get(ctx, stackName, stack))
	stack.Status.ProjectInfo = &shared.ProjectInfo{Name: "myproject", Runtime: "yaml"}
	stack.Status.LastUpdate = &shared.StackUpdateState{
		Generation:     stack.Generation,
		Name:           "destroy-failed",
		Type:           autov1alpha1.DestroyType,
		State:          shared.FailedStackStateMessage,
		Message:        "destroy failed: BucketNotEmpty",
		LastResyncTime: metav1.Now(),
		Failures:       1,
	}
	require.NoError(t, k8sclient.Status().Update(ctx, stack))

	r := &StackReconciler{Client: k8sclient, APIReader: k8sclient, Scheme: testScheme, Recorder: record.NewFakeRecorder(10)}
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
	require.NoError(t, err)

	// A failed destroy must not delete the Stack and orphan its resources. testFinalizer keeps
	// the object queryable, so the assertion isolates PulumiFinalizer.
	var updated pulumiv1.Stack
	require.NoError(t, k8sclient.Get(ctx, stackName, &updated))
	assert.Contains(t, updated.Finalizers, PulumiFinalizer)
}

func TestStackDestroyStalledWhenNamespaceTerminating(t *testing.T) {
	testScheme := scheme.Scheme
	require.NoError(t, pulumiv1.AddToScheme(testScheme))
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.3-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	k8sclient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	ctx := context.Background()

	nsName := "terminating-ns"
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
	require.NoError(t, k8sclient.Create(ctx, ns))

	stackName := types.NamespacedName{Name: "stalled-destroy", Namespace: nsName}

	// A Program with a ready artifact so the destroy path resolves its source successfully.
	program := &pulumiv1.Program{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stackName.Name + "-program",
			Namespace: nsName,
		},
		Program: pulumiv1.ProgramSpec{
			Resources: map[string]pulumiv1.Resource{
				"password": {Type: "random:RandomPassword"},
			},
		},
	}
	require.NoError(t, k8sclient.Create(ctx, program))
	program.Status.Artifact = &pulumiv1.Artifact{
		Revision:       "test-revision",
		LastUpdateTime: metav1.Now(),
	}
	program.Status.ObservedGeneration = program.Generation
	require.NoError(t, k8sclient.Status().Update(ctx, program))

	// A Stack with destroyOnFinalize, mid-deletion and held by the Pulumi finalizer.
	stack := &pulumiv1.Stack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pulumiv1.GroupVersion.String(),
			Kind:       "Stack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       stackName.Name,
			Namespace:  nsName,
			Finalizers: []string{PulumiFinalizer},
		},
		Spec: shared.StackSpec{
			Stack:             "dev",
			DestroyOnFinalize: true,
			ProgramRef:        &shared.ProgramReference{Name: program.Name},
		},
	}
	require.NoError(t, k8sclient.Create(ctx, stack))
	require.NoError(t, k8sclient.Delete(ctx, stack))

	// envtest has no namespace controller, so a deleted namespace stays Terminating
	// indefinitely — exactly the state that blocks workspace creation.
	require.NoError(t, k8sclient.Delete(ctx, ns))
	require.Eventually(t, func() bool {
		var got corev1.Namespace
		if err := k8sclient.Get(ctx, types.NamespacedName{Name: nsName}, &got); err != nil {
			return false
		}
		return got.Status.Phase == corev1.NamespaceTerminating
	}, 10*time.Second, 100*time.Millisecond, "namespace should be Terminating")

	recorder := record.NewFakeRecorder(10)
	r := &StackReconciler{
		Client:    k8sclient,
		APIReader: k8sclient,
		Scheme:    testScheme,
		Recorder:  recorder,
	}
	_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
	require.NoError(t, err)

	var updated pulumiv1.Stack
	require.NoError(t, k8sclient.Get(ctx, stackName, &updated))

	stalledCond := meta.FindStatusCondition(updated.Status.Conditions, pulumiv1.StalledCondition)
	require.NotNil(t, stalledCond, "expected Stalled condition on Stack")
	assert.Equal(t, metav1.ConditionTrue, stalledCond.Status)
	assert.Equal(t, pulumiv1.StalledNamespaceTerminatingReason, stalledCond.Reason)
	assert.Contains(t, stalledCond.Message, "terminating")
	assert.Contains(t, stalledCond.Message, "kubectl patch", "message should tell the user how to release the Stack")

	assert.Contains(t, updated.Finalizers, PulumiFinalizer, "finalizer must be retained so resources are not silently orphaned")
}

// TestStackStatusConcurrentModification proves that the Stack controller's
// SSA-based status writes succeed even when another client concurrently modifies
// metadata (e.g. sets a label), which bumps the object's resourceVersion. The
// old Status().Update() approach would fail with a 409 Conflict; SSA ignores
// resourceVersion.
func TestStackStatusConcurrentModification(t *testing.T) {
	testScheme := scheme.Scheme
	require.NoError(t, pulumiv1.AddToScheme(testScheme))
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	k8sClient, err := client.New(cfg, client.Options{Scheme: testScheme})
	require.NoError(t, err)

	ctx := t.Context()

	// Step 1: Create a Program with a ready artifact.
	program := &pulumiv1.Program{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "program-ssa-concurrent",
			Namespace: "default",
		},
		Program: pulumiv1.ProgramSpec{
			Resources: map[string]pulumiv1.Resource{
				"bucket": {Type: "random:RandomPassword"},
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, program))
	program.Status.Artifact = &pulumiv1.Artifact{
		Revision:       "test-revision",
		LastUpdateTime: metav1.Now(),
	}
	program.Status.ObservedGeneration = program.Generation
	require.NoError(t, k8sClient.Status().Update(ctx, program))

	// Step 2: Create a Stack with a ProgramRef.
	stackName := types.NamespacedName{Name: "stack-ssa-concurrent", Namespace: "default"}
	stack := &pulumiv1.Stack{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pulumiv1.GroupVersion.String(),
			Kind:       "Stack",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       stackName.Name,
			Namespace:  stackName.Namespace,
			Finalizers: []string{testFinalizer},
		},
		Spec: shared.StackSpec{
			Stack: "dev",
			ProgramRef: &shared.ProgramReference{
				Name: program.Name,
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, stack))

	reconciler := &StackReconciler{
		Client:    k8sClient,
		APIReader: k8sClient,
		Scheme:    testScheme,
		Recorder:  record.NewFakeRecorder(10),
	}

	// Step 3: First reconciliation — writes status via SSA.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
	require.NoError(t, err)

	require.NoError(t, k8sClient.Get(ctx, stackName, stack))
	staleVersion := stack.ResourceVersion

	// Step 4: Simulate a concurrent metadata modification (e.g. another controller sets a label).
	fresh := &pulumiv1.Stack{}
	require.NoError(t, k8sClient.Get(ctx, stackName, fresh))
	if fresh.Labels == nil {
		fresh.Labels = make(map[string]string)
	}
	fresh.Labels["test.example.com/concurrent-modification"] = "true"
	require.NoError(t, k8sClient.Update(ctx, fresh))

	assert.NotEqual(t, staleVersion, fresh.ResourceVersion,
		"server resourceVersion must have advanced past our local copy")

	// Step 5: Second reconciliation with the now-stale stack object.
	// With Status().Update() this would 409; SSA must succeed.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: stackName})
	require.NoError(t, err)

	// Step 6: Verify the concurrent label and the status update are both present.
	var result pulumiv1.Stack
	require.NoError(t, k8sClient.Get(ctx, stackName, &result))

	assert.Equal(t, "true", result.Labels["test.example.com/concurrent-modification"],
		"label from concurrent metadata write must be preserved")

	assert.Equal(t, result.Generation, result.Status.ObservedGeneration,
		"ObservedGeneration must match the stack generation")

	readyCond := meta.FindStatusCondition(result.Status.Conditions, pulumiv1.ReadyCondition)
	require.NotNil(t, readyCond, "expected Ready condition on stack")
}

// currentUpdateFixture builds a Stack whose status.currentUpdate names a live Update, together
// with the Workspace and Flux source the reconciler reads, and reconciles it.
type currentUpdateFixture struct {
	t   *testing.T
	k8s client.Client
	r   *StackReconciler

	objName       types.NamespacedName
	obj           *pulumiv1.Stack
	ws            *autov1alpha1.Workspace
	fluxRepo      *fluxsourcev1.GitRepository
	currentUpdate *autov1alpha1.Update
	lastUpdate    *autov1alpha1.Update

	ready       *metav1.Condition
	stalled     *metav1.Condition
	reconciling *metav1.Condition
}

func startStackEnv(t *testing.T) client.Client {
	t.Helper()
	require.NoError(t, pulumiv1.AddToScheme(scheme.Scheme))
	require.NoError(t, autov1alpha1.AddToScheme(scheme.Scheme))
	require.NoError(t, fluxsourcev1.AddToScheme(scheme.Scheme))

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "..", "test", "crds", "flux2", "v2.3.0"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.3-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	cfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.Stop() })

	k8s, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	return k8s
}

// newCurrentUpdateFixture applies opts before writing to the API server. Stack names are
// randomized so fixtures can share one environment.
func newCurrentUpdateFixture(
	t *testing.T, ctx context.Context, k8s client.Client, opts ...func(*currentUpdateFixture),
) *currentUpdateFixture {
	t.Helper()

	f := &currentUpdateFixture{t: t, k8s: k8s}
	f.r = &StackReconciler{
		Client:                   k8s,
		APIReader:                k8s,
		Scheme:                   k8s.Scheme(),
		Recorder:                 record.NewFakeRecorder(10),
		maybeWatchFluxSourceKind: func(shared.FluxSourceReference) error { return nil },
	}

	f.objName = types.NamespacedName{
		Name:      fmt.Sprintf("stack-%s", utilrand.String(8)),
		Namespace: "default",
	}
	f.obj = &pulumiv1.Stack{
		TypeMeta: metav1.TypeMeta{APIVersion: pulumiv1.GroupVersion.String(), Kind: "Stack"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.objName.Name,
			Namespace: f.objName.Namespace,
			// Keep the object queryable after it is marked for deletion.
			Finalizers: []string{testFinalizer},
		},
		Spec: shared.StackSpec{Stack: "dev"},
	}
	f.fluxRepo = makeFluxGitRepository(f.objName)
	f.ws = makeWorkspace(f.obj.ObjectMeta)
	f.currentUpdate = makeUpdate(f.objName, f.obj, autov1alpha1.UpdateSpec{
		WorkspaceName: nameForWorkspace(&f.obj.ObjectMeta),
		StackName:     f.obj.Spec.Stack,
		Type:          autov1alpha1.UpType,
	})
	f.lastUpdate = &autov1alpha1.Update{}

	f.ws.Spec.Flux = &autov1alpha1.FluxSource{
		Digest: f.fluxRepo.Status.Artifact.Digest,
		Url:    f.fluxRepo.Status.Artifact.URL,
		Dir:    "random-yaml",
	}
	f.obj.Spec.FluxSource = &shared.FluxSource{
		SourceRef: shared.FluxSourceReference{
			APIVersion: "source.toolkit.fluxcd.io/v1",
			Kind:       "GitRepository",
			Name:       f.fluxRepo.Name,
		},
		Dir: "random-yaml",
	}
	f.obj.Status.CurrentUpdate = &shared.CurrentStackUpdate{
		Generation: 1,
		Name:       f.currentUpdate.Name,
		Commit:     "abcdef",
	}

	for _, opt := range opts {
		opt(f)
	}
	f.create(ctx)
	return f
}

func (f *currentUpdateFixture) create(ctx context.Context) {
	f.t.Helper()

	stackStatus := f.obj.Status
	require.NoError(f.t, f.k8s.Create(ctx, f.obj))
	f.obj.Status = stackStatus
	// Apply so the field manager owns the status; the controller clears fields by omission.
	require.NoError(f.t, f.k8s.Status().Patch(ctx, f.obj, stackStatusSSAPatch(f.obj),
		client.FieldOwner(StackStatusFieldManager), client.ForceOwnership))

	repoStatus := f.fluxRepo.Status
	require.NoError(f.t, f.k8s.Create(ctx, f.fluxRepo))
	f.fluxRepo.Status = repoStatus
	require.NoError(f.t, f.k8s.Status().Update(ctx, f.fluxRepo))

	require.NoError(f.t, controllerutil.SetControllerReference(f.obj, f.ws, f.k8s.Scheme()))
	wsStatus := f.ws.Status
	require.NoError(f.t, f.k8s.Patch(ctx, f.ws, client.Apply, client.FieldOwner(FieldManager)))
	f.ws.Status = wsStatus
	require.NoError(f.t, f.k8s.Status().Update(ctx, f.ws, client.FieldOwner(FieldManager)))

	require.NoError(f.t, controllerutil.SetControllerReference(f.obj, f.currentUpdate, f.k8s.Scheme()))
	finalizers := f.currentUpdate.Finalizers
	f.currentUpdate.Finalizers = nil
	updateStatus := f.currentUpdate.Status
	require.NoError(f.t, f.k8s.Patch(ctx, f.currentUpdate, client.Apply, client.FieldOwner(FieldManager)))
	f.currentUpdate.Status = updateStatus
	require.NoError(f.t, f.k8s.Status().Update(ctx, f.currentUpdate, client.FieldOwner(FieldManager)))
	if len(finalizers) > 0 {
		patch := autov1alpha1apply.Update(f.currentUpdate.Name, f.currentUpdate.Namespace).
			WithFinalizers(finalizers...)
		require.NoError(f.t, f.k8s.Patch(ctx, f.currentUpdate, applyPatch{patch},
			client.FieldOwner(StackFinalizerFieldManager)))
	}
}

func (f *currentUpdateFixture) reconcile(ctx context.Context) (reconcile.Result, error) {
	f.t.Helper()
	result, err := f.r.Reconcile(ctx, reconcile.Request{NamespacedName: f.objName})

	f.obj = &pulumiv1.Stack{}
	require.NoError(f.t, f.k8s.Get(ctx, f.objName, f.obj))
	f.ready = meta.FindStatusCondition(f.obj.Status.Conditions, pulumiv1.ReadyCondition)
	f.stalled = meta.FindStatusCondition(f.obj.Status.Conditions, pulumiv1.StalledCondition)
	f.reconciling = meta.FindStatusCondition(f.obj.Status.Conditions, pulumiv1.ReconcilingCondition)

	f.ws = &autov1alpha1.Workspace{}
	_ = f.k8s.Get(ctx, types.NamespacedName{
		Name: nameForWorkspace(&f.obj.ObjectMeta), Namespace: f.objName.Namespace}, f.ws)

	f.currentUpdate = &autov1alpha1.Update{}
	if f.obj.Status.CurrentUpdate != nil {
		_ = f.k8s.Get(ctx, types.NamespacedName{
			Name: f.obj.Status.CurrentUpdate.Name, Namespace: f.objName.Namespace}, f.currentUpdate)
	}
	f.lastUpdate = &autov1alpha1.Update{}
	if f.obj.Status.LastUpdate != nil {
		_ = f.k8s.Get(ctx, types.NamespacedName{
			Name: f.obj.Status.LastUpdate.Name, Namespace: f.objName.Namespace}, f.lastUpdate)
	}
	return result, err
}

func (f *currentUpdateFixture) requireReconciling(reason, message string) {
	f.t.Helper()
	require.NotNil(f.t, f.ready, "expected Ready to not be nil")
	assert.Equal(f.t, metav1.ConditionFalse, f.ready.Status, "expected Ready to be false")
	assert.Equal(f.t, pulumiv1.NotReadyInProgressReason, f.ready.Reason)
	require.NotNil(f.t, f.reconciling, "expected Reconciling to not be nil")
	assert.Equal(f.t, metav1.ConditionTrue, f.reconciling.Status, "expected Reconciling to be true")
	assert.Equal(f.t, reason, f.reconciling.Reason)
	assert.Equal(f.t, message, f.reconciling.Message)
	assert.Nil(f.t, f.stalled, "expected Stalled to be nil")
}

func (f *currentUpdateFixture) requireEvent(substr string) {
	f.t.Helper()
	recorder, ok := f.r.Recorder.(*record.FakeRecorder)
	require.True(f.t, ok, "recorder is not a FakeRecorder")
	select {
	case event := <-recorder.Events:
		assert.Contains(f.t, event, substr)
	default:
		f.t.Fatalf("expected an event containing %q, but none was recorded", substr)
	}
}

func updateCondition(condType string, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

func withUpdateStatus(conditions ...metav1.Condition) func(*currentUpdateFixture) {
	return func(f *currentUpdateFixture) {
		f.currentUpdate.Status.ObservedGeneration = 1
		f.currentUpdate.Status.Conditions = conditions
	}
}

func updateFailedConditions(reason, message string) []metav1.Condition {
	return []metav1.Condition{
		updateCondition(autov1alpha1.UpdateConditionTypeComplete, metav1.ConditionTrue, "Updated", ""),
		updateCondition(autov1alpha1.UpdateConditionTypeFailed, metav1.ConditionTrue, reason, message),
	}
}

func updateSucceededConditions() []metav1.Condition {
	return []metav1.Condition{
		updateCondition(autov1alpha1.UpdateConditionTypeComplete, metav1.ConditionTrue, "Updated", ""),
		updateCondition(autov1alpha1.UpdateConditionTypeFailed, metav1.ConditionFalse, "unknown", ""),
	}
}

func (f *currentUpdateFixture) patchLastUpdate(ctx context.Context, last *shared.StackUpdateState) {
	f.t.Helper()
	f.obj.Status.LastUpdate = last
	require.NoError(f.t, f.k8s.Status().Patch(ctx, f.obj, stackStatusSSAPatch(f.obj),
		client.FieldOwner(StackStatusFieldManager), client.ForceOwnership))
}

func TestCurrentUpdateProcessing(t *testing.T) {
	k8s := startStackEnv(t)

	t.Run("the update is gone", func(t *testing.T) {
		deleteCurrentUpdate := func(ctx context.Context, f *currentUpdateFixture) string {
			t.Helper()
			lostName := f.currentUpdate.Name
			f.currentUpdate.Finalizers = []string{}
			require.NoError(t, f.k8s.Update(ctx, f.currentUpdate))
			require.NoError(t, f.k8s.Delete(ctx, f.currentUpdate))
			return lostName
		}

		t.Run("clears the reference so a new update can start", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s)
			lostName := deleteCurrentUpdate(ctx, f)

			_, err := f.reconcile(ctx)

			// The error puts the Stack on the workqueue's backoff.
			require.Error(t, err)
			assert.Contains(t, err.Error(), lostName)
			assert.Nil(t, f.obj.Status.CurrentUpdate, "expected the current update to be cleared")
			assert.Nil(t, f.obj.Status.LastUpdate, "expected the retry budget to be preserved")
			f.requireEvent(string(pulumiv1.StackUpdateLost))
		})

		t.Run("starts a fresh update on the following reconcile", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s)
			lostName := deleteCurrentUpdate(ctx, f)

			_, err := f.reconcile(ctx)
			require.Error(t, err)
			require.Nil(t, f.obj.Status.CurrentUpdate)

			// The retry needs a ready workspace to get as far as creating an Update.
			f.ws.Status.ObservedGeneration = f.ws.Generation
			f.ws.Status.Conditions = []metav1.Condition{
				updateCondition("Ready", metav1.ConditionTrue, "Succeeded", ""),
			}
			require.NoError(t, f.k8s.Status().Update(ctx, f.ws, client.FieldOwner(FieldManager)))

			_, err = f.reconcile(ctx)
			require.NoError(t, err)

			require.NotNil(t, f.obj.Status.CurrentUpdate)
			assert.NotEqual(t, lostName, f.obj.Status.CurrentUpdate.Name,
				"expected a different update than the one that was lost")
		})
	})

	t.Run("the update is missing from the cache but present in the API", func(t *testing.T) {
		ctx := t.Context()
		f := newCurrentUpdateFixture(t, ctx, k8s)

		// What a cold or lagging informer produces: cache misses, APIReader still sees it.
		f.r.Client = updateHidingClient{Client: k8s, hidden: f.currentUpdate.Name}
		wanted := f.currentUpdate.Name

		result, err := f.reconcile(ctx)
		require.NoError(t, err)

		f.requireReconciling(pulumiv1.ReconcilingProcessingReason, pulumiv1.ReconcilingProcessingUpdateMessage)

		require.NotNil(t, f.obj.Status.CurrentUpdate, "expected the reference to be retained")
		assert.Equal(t, wanted, f.obj.Status.CurrentUpdate.Name)
		assert.Equal(t, updateCacheLagRequeue, result.RequeueAfter,
			"expected a requeue so a missed watch event cannot strand the Stack")
	})

	t.Run("the reference is lost but the update itself is still running", func(t *testing.T) {
		ctx := t.Context()
		f := newCurrentUpdateFixture(t, ctx, k8s)

		// The Update is alive, but status.currentUpdate names something that never existed.
		orphanName := f.currentUpdate.Name
		f.obj.Status.CurrentUpdate.Name = "does-not-exist"
		require.NoError(t, f.k8s.Status().Patch(ctx, f.obj, stackStatusSSAPatch(f.obj),
			client.FieldOwner(StackStatusFieldManager), client.ForceOwnership))

		result, err := f.reconcile(ctx)
		require.NoError(t, err)

		require.NotNil(t, f.obj.Status.CurrentUpdate)
		assert.Equal(t, orphanName, f.obj.Status.CurrentUpdate.Name)
		assert.Equal(t, updateCacheLagRequeue, result.RequeueAfter)

		// Keeping these would credit the adopted update's result to commit abcdef.
		assert.Empty(t, f.obj.Status.CurrentUpdate.Commit)
		assert.Zero(t, f.obj.Status.CurrentUpdate.Generation)
		assert.Empty(t, f.obj.Status.CurrentUpdate.ReconcileRequest)
	})

	t.Run("the update is incomplete (still running)", func(t *testing.T) {
		ctx := t.Context()
		f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(
			updateCondition(autov1alpha1.UpdateConditionTypeComplete, metav1.ConditionFalse, "Progressing", "")))

		result, err := f.reconcile(ctx)
		require.NoError(t, err)

		f.requireReconciling(pulumiv1.ReconcilingProcessingReason, pulumiv1.ReconcilingProcessingUpdateMessage)
		assert.NotNil(t, f.obj.Status.CurrentUpdate, "expected the current update to be retained")
		assert.Equal(t, reconcile.Result{RequeueAfter: updateInFlightRequeue}, result,
			"expected a slow poll so a stuck Update cannot strand the Stack")
	})

	// A blocked deletion bumps generation, and nothing writes an Update's status once it is
	// Complete, so such an Update sits at N+1/N forever.
	t.Run("the update is terminal but its generation was bumped by a blocked deletion", func(t *testing.T) {
		ctx := t.Context()
		f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(updateFailedConditions("unknown", "")...))

		require.NoError(t, f.k8s.Delete(ctx, f.currentUpdate))
		live := &autov1alpha1.Update{}
		require.NoError(t, f.k8s.Get(ctx, client.ObjectKeyFromObject(f.currentUpdate), live))
		require.NotNil(t, live.DeletionTimestamp)
		require.Greater(t, live.Generation, live.Status.ObservedGeneration)

		_, err := f.reconcile(ctx)
		require.NoError(t, err)

		assert.Nil(t, f.obj.Status.CurrentUpdate, "expected the current update to be cleared")
		require.NotNil(t, f.obj.Status.LastUpdate, "expected the terminal result to be recorded")
		assert.Equal(t, shared.FailedStackStateMessage, f.obj.Status.LastUpdate.State)
	})

	// The finalizer is written by a plain Create, not by apply under StackFinalizerFieldManager,
	// which models an Update finalized by an older operator.
	t.Run("a previous update is stuck Terminating with PulumiFinalizer", func(t *testing.T) {
		stickUpdate := func(ctx context.Context, f *currentUpdateFixture) *autov1alpha1.Update {
			t.Helper()
			stuck := makeUpdate(f.objName, f.obj, autov1alpha1.UpdateSpec{
				WorkspaceName: nameForWorkspace(&f.obj.ObjectMeta),
				StackName:     f.obj.Spec.Stack,
				Type:          autov1alpha1.UpType,
			})
			require.NoError(t, controllerutil.SetControllerReference(f.obj, stuck, f.k8s.Scheme()))
			require.NoError(t, f.k8s.Create(ctx, stuck))
			stuck.Status.Conditions = []metav1.Condition{
				updateCondition(autov1alpha1.UpdateConditionTypeComplete, metav1.ConditionTrue, "Updated", ""),
			}
			require.NoError(t, f.k8s.Status().Update(ctx, stuck))
			require.NoError(t, f.k8s.Delete(ctx, stuck))

			live := &autov1alpha1.Update{}
			require.NoError(t, f.k8s.Get(ctx, client.ObjectKeyFromObject(stuck), live))
			require.Contains(t, live.Finalizers, PulumiFinalizer)
			return stuck
		}

		t.Run("releases the finalizer so the object can finish deleting", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s)
			stuck := stickUpdate(ctx, f)

			_, _ = f.reconcile(ctx)

			live := &autov1alpha1.Update{}
			err := f.k8s.Get(ctx, client.ObjectKeyFromObject(stuck), live)
			if err == nil {
				assert.NotContains(t, live.Finalizers, PulumiFinalizer)
				return
			}
			assert.True(t, apierrors.IsNotFound(err), "the update should be gone once released")
		})

		t.Run("does not release the update it is still waiting on", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s)
			_ = stickUpdate(ctx, f)
			waitingOn := client.ObjectKeyFromObject(f.currentUpdate)

			live := &autov1alpha1.Update{}
			require.NoError(t, f.k8s.Get(ctx, waitingOn, live))
			require.Contains(t, live.Finalizers, PulumiFinalizer)

			_, _ = f.reconcile(ctx)

			require.NoError(t, f.k8s.Get(ctx, waitingOn, live))
			assert.Contains(t, live.Finalizers, PulumiFinalizer)
		})
	})

	t.Run("the update failed", func(t *testing.T) {
		t.Run("records the failure", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(updateFailedConditions("unknown", "")...))
			update := f.currentUpdate.DeepCopy()

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			assert.Nil(t, f.obj.Status.CurrentUpdate, "expected the current update to be cleared")
			require.NotNil(t, f.obj.Status.LastUpdate)
			assert.Equal(t, update.Generation, f.obj.Status.LastUpdate.Generation)
			assert.Equal(t, update.Name, f.obj.Status.LastUpdate.Name)
			assert.Equal(t, update.Spec.Type, f.obj.Status.LastUpdate.Type)
			assert.Equal(t, shared.FailedStackStateMessage, f.obj.Status.LastUpdate.State)
			assert.Equal(t, "abcdef", f.obj.Status.LastUpdate.LastAttemptedCommit)
			assert.Empty(t, f.obj.Status.LastUpdate.LastSuccessfulCommit)
			assert.NotContains(t, f.lastUpdate.Finalizers, PulumiFinalizer,
				"expected the stack finalizer to be released so the update can be deleted")
			f.requireEvent(string(pulumiv1.StackUpdateFailure))
		})

		t.Run("reports a locked stack", func(t *testing.T) {
			ctx := t.Context()
			const message = "the stack is locked; another update is in progress"
			f := newCurrentUpdateFixture(t, ctx, k8s, func(f *currentUpdateFixture) {
				f.currentUpdate.Status.ObservedGeneration = 1
				f.currentUpdate.Status.Message = message
				f.currentUpdate.Status.Conditions = updateFailedConditions("UpdateConflict", message)
			})

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			require.NotNil(t, f.obj.Status.LastUpdate)
			assert.Equal(t, message, f.obj.Status.LastUpdate.Message)
		})

		t.Run("reports an unknown pulumi error", func(t *testing.T) {
			ctx := t.Context()
			const message = "an unknown error occurred running up"
			f := newCurrentUpdateFixture(t, ctx, k8s, func(f *currentUpdateFixture) {
				f.currentUpdate.Status.ObservedGeneration = 1
				f.currentUpdate.Status.Message = message
				f.currentUpdate.Status.Conditions = updateFailedConditions("Unknown", message)
			})

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			require.NotNil(t, f.obj.Status.LastUpdate)
			assert.Equal(t, message, f.obj.Status.LastUpdate.Message)
		})

		t.Run("increments failures when retrying", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(updateFailedConditions("unknown", "")...))
			f.patchLastUpdate(ctx, &shared.StackUpdateState{
				Generation:          1,
				State:               shared.FailedStackStateMessage,
				Name:                "update-retried",
				Type:                autov1alpha1.UpType,
				LastAttemptedCommit: f.obj.Status.CurrentUpdate.Commit,
				Failures:            2,
			})

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			require.NotNil(t, f.obj.Status.LastUpdate)
			assert.Equal(t, int64(3), f.obj.Status.LastUpdate.Failures)
		})

		t.Run("resets failures when retrying at a new generation", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(updateFailedConditions("unknown", "")...))
			f.patchLastUpdate(ctx, &shared.StackUpdateState{
				Generation:          2,
				State:               shared.FailedStackStateMessage,
				Name:                "update-retried-with-new-sha",
				Type:                autov1alpha1.UpType,
				LastAttemptedCommit: f.obj.Status.CurrentUpdate.Commit,
				Failures:            2,
			})

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			require.NotNil(t, f.obj.Status.LastUpdate)
			assert.Equal(t, int64(0), f.obj.Status.LastUpdate.Failures)
		})
	})

	t.Run("the update succeeded", func(t *testing.T) {
		t.Run("records the result", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(updateSucceededConditions()...))
			update := f.currentUpdate.DeepCopy()

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			assert.Nil(t, f.obj.Status.CurrentUpdate, "expected the current update to be cleared")
			require.NotNil(t, f.obj.Status.LastUpdate)
			assert.Equal(t, update.Generation, f.obj.Status.LastUpdate.Generation)
			assert.Equal(t, update.Name, f.obj.Status.LastUpdate.Name)
			assert.Equal(t, update.Spec.Type, f.obj.Status.LastUpdate.Type)
			assert.Equal(t, shared.SucceededStackStateMessage, f.obj.Status.LastUpdate.State)
			assert.Equal(t, "abcdef", f.obj.Status.LastUpdate.LastAttemptedCommit)
			assert.Equal(t, "abcdef", f.obj.Status.LastUpdate.LastSuccessfulCommit)
			assert.NotContains(t, f.lastUpdate.Finalizers, PulumiFinalizer,
				"expected the stack finalizer to be released so the update can be deleted")
			f.requireEvent(string(pulumiv1.StackUpdateSuccessful))
		})

		t.Run("resets failures after retrying", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(updateSucceededConditions()...))
			f.patchLastUpdate(ctx, &shared.StackUpdateState{
				Generation:          1,
				State:               shared.FailedStackStateMessage,
				Name:                "update-retried",
				Type:                autov1alpha1.UpType,
				LastAttemptedCommit: f.obj.Status.CurrentUpdate.Commit,
				Failures:            2,
			})

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			require.NotNil(t, f.obj.Status.LastUpdate)
			assert.Equal(t, int64(0), f.obj.Status.LastUpdate.Failures)
		})

		t.Run("resets failures after retrying at a new generation", func(t *testing.T) {
			ctx := t.Context()
			f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(updateSucceededConditions()...))
			f.patchLastUpdate(ctx, &shared.StackUpdateState{
				Generation:          2,
				State:               shared.FailedStackStateMessage,
				Name:                "update-retried-with-new-sha",
				Type:                autov1alpha1.UpType,
				LastAttemptedCommit: f.obj.Status.CurrentUpdate.Commit,
				Failures:            2,
			})

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			require.NotNil(t, f.obj.Status.LastUpdate)
			assert.Equal(t, int64(0), f.obj.Status.LastUpdate.Failures)
		})

		t.Run("captures scrubbed outputs", func(t *testing.T) {
			ctx := t.Context()
			secretName := fmt.Sprintf("secret-output-%s", utilrand.String(8))
			f := newCurrentUpdateFixture(t, ctx, k8s, withUpdateStatus(updateSucceededConditions()...),
				func(f *currentUpdateFixture) {
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:        secretName,
							Namespace:   f.currentUpdate.Namespace,
							Annotations: map[string]string{"pulumi.com/secrets": `["should-be-scrubbed"]`},
						},
						StringData: map[string]string{
							"plaintext":          `"not-sensitive"`,
							"should-be-scrubbed": `"sensitive"`,
						},
					}
					require.NoError(t, f.k8s.Create(ctx, secret))
					f.currentUpdate.Status.Outputs = secret.Name
				})

			_, err := f.reconcile(ctx)
			require.NoError(t, err)

			assert.JSONEq(t, `"not-sensitive"`, string(f.obj.Status.Outputs["plaintext"].Raw))
			assert.JSONEq(t, `"[secret]"`, string(f.obj.Status.Outputs["should-be-scrubbed"].Raw))
		})
	})
}
