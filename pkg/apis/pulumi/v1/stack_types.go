package v1

import (
	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// StackStatus defines the observed state of Stack
type StackStatus struct {
	// Outputs contains the exported stack output variables resulting from a deployment.
	Outputs shared.StackOutputs `json:"outputs,omitempty"`
	// LastUpdate contains details of the status of the last update.
	LastUpdate *shared.StackUpdateState `json:"lastUpdate,omitempty"`
	// ObservedGeneration records the value of .meta.generation at the point the controller last processed this object
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// The conditions form part of the API. They work as follows:
//  - while the stack is being processed, the condition `Reconciling` will be present with a value of `True`
//  - if the stack is processed to completion, the condition `Ready` will be present with a value of `True`
//  - if the stack failed, the condition `Ready` will be present with a value of `False`
//  - if the stack failed and has not been requeued to retry, the condition `Stalled` will be present with a value of `True`
//
// Assuming a stack has been seen by the controller, it will either have Ready=True, or Ready=False
// and possibly one of {Stalled,Reconciling}=True.

const (
	ReadyCondition       = "Ready"
	StalledCondition     = "Stalled"
	ReconcilingCondition = "Reconciling"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Stack is the Schema for the stacks API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=stacks,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.lastUpdate.state"
type Stack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   shared.StackSpec `json:"spec,omitempty"`
	Status StackStatus      `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StackList contains a list of Stack
type StackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Stack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Stack{}, &StackList{})
}
