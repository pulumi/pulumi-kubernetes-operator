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

package v1

import (
	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/shared"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StackStatus defines the observed state of Stack
type StackStatus struct {
	// Outputs contains the exported stack output variables resulting from a deployment.
	Outputs shared.StackOutputs `json:"outputs,omitempty"`
	// LastUpdate contains details of the status of the last update.
	LastUpdate *shared.StackUpdateState `json:"lastUpdate,omitempty"`
	// ObservedGeneration records the value of .meta.generation at the point the controller last processed this object
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ObservedReconcileRequest records the value of the annotation named for
	// `ReconcileRequestAnnotation` when it was last seen.
	ObservedReconcileRequest string `json:"observedReconcileRequest,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// CurrentUpdate contains details of the status of the current update, if any.
	// +optional
	CurrentUpdate *shared.CurrentStackUpdate `json:"currentUpdate,omitempty"`
}

// The conditions form part of the API. They are used to implement a "ready protocol" which works
// with tooling like kstatus
// (https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md), as follows:
//  - while the stack is being processed, the condition `Reconciling` will be present with a value of `True`
//  - if the stack is processed to completion, the condition `Ready` will be present with a value of `True`
//  - if the stack failed, the condition `Ready` will be present with a value of `False`
//  - if the stack failed and has not been requeued to retry, the condition `Stalled` will be present with a value of `True`
//
// Assuming a stack has been seen by the controller, it will either have Ready=True, or Ready=False
// and possibly one of {Stalled,Reconciling}=True.

// Tooling that only understands a Ready condition (the base convention) will see that; kstatus will
// be able to give a more precise answer using the Stalled and Reconciling conditions.

const (
	ReadyCondition       = "Ready"
	StalledCondition     = "Stalled"
	ReconcilingCondition = "Reconciling"

	// These give standard reasons for various status values in the conditions

	// Not ready because it's in progress
	NotReadyInProgressReason = "NotReadyInProgress"
	// Not ready because it's stalled
	NotReadyStalledReason = "NotReadyStalled"

	// Reconciling because the stack is being processed
	ReconcilingProcessingReason           = "StackProcessing"
	ReconcilingProcessingWorkspaceMessage = "waiting for workspace readiness"
	ReconcilingProcessingUpdateMessage    = "stack is being processed"
	// Reconciling because it failed, and has been requeued
	ReconcilingRetryReason = "RetryingAfterFailure"
	// Reconciling because a prerequisite was not satisfied
	ReconcilingPrerequisiteNotSatisfiedReason = "PrerequisiteNotSatisfied"

	// Stalled because the .spec can't be processed as it is
	StalledSpecInvalidReason = "SpecInvalid"
	// Stalled because the source can't be fetched (due to a bad address, or credentials, or ...)
	StalledSourceUnavailableReason = "SourceUnavailable"
	// Stalled because there was a conflict with another update, and retryOnConflict was not set.
	StalledConflictReason = "UpdateConflict"
	// Stalled because a cross-namespace ref is used, and namespace isolation is in effect.
	StalledCrossNamespaceRefForbiddenReason = "CrossNamespaceRefForbidden"

	// Ready because processing has completed
	ReadyCompletedReason = "ProcessingCompleted"
)

// MarkReconcilingCondition arranges the conditions used in the "ready protocol", so to indicate that
// the resource is being processed.
func (s *StackStatus) MarkReconcilingCondition(reason, msg string) {
	conditions := &s.Conditions
	apimeta.RemoveStatusCondition(conditions, StalledCondition)
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:    ReadyCondition,
		Status:  "False",
		Reason:  NotReadyInProgressReason,
		Message: "reconciliation is in progress",
	})
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:    ReconcilingCondition,
		Status:  "True",
		Reason:  reason,
		Message: msg,
	})
}

// MarkStalledCondition arranges the conditions used in the "ready protocol", so to indicate that
// the resource is stalled; that is, it did not run to completion, and will not be retried until the
// definition is changed. This also marks the resource as not ready.
func (s *StackStatus) MarkStalledCondition(reason, msg string) {
	conditions := &s.Conditions
	apimeta.RemoveStatusCondition(conditions, ReconcilingCondition)
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:    ReadyCondition,
		Status:  "False",
		Reason:  NotReadyStalledReason,
		Message: "reconciliation is stalled",
	})
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:    StalledCondition,
		Status:  "True",
		Reason:  reason,
		Message: msg,
	})
}

// MarkReadyCondition arranges the conditions used in the "ready protocol", so to indicate that
// the resource is considered up to date.
func (s *StackStatus) MarkReadyCondition() {
	conditions := &s.Conditions
	apimeta.RemoveStatusCondition(conditions, ReconcilingCondition)
	apimeta.RemoveStatusCondition(conditions, StalledCondition)
	apimeta.SetStatusCondition(conditions, metav1.Condition{
		Type:    ReadyCondition,
		Status:  "True",
		Reason:  ReadyCompletedReason,
		Message: "the stack has been processed and is up to date",
	})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.lastUpdate.state"

// Stack is the Schema for the stacks API
type Stack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   shared.StackSpec `json:"spec,omitempty"`
	Status StackStatus      `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StackList contains a list of Stack
type StackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Stack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Stack{}, &StackList{})
}
