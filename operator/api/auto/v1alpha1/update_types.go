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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpdateType string

const (
	PreviewType UpdateType = "preview"
	UpType      UpdateType = "up"
	DestroyType UpdateType = "destroy"
	RefreshType UpdateType = "refresh"
)

const (
	UpdateConditionTypeComplete    = "Complete"
	UpdateConditionTypeFailed      = "Failed"
	UpdateConditionTypeProgressing = "Progressing"
)

// UpdateSpec defines the desired state of Update
type UpdateSpec struct {
	// WorkspaceName is the workspace to update.
	WorkspaceName string `json:"workspaceName,omitempty"`
	StackName     string `json:"stackName,omitempty"`

	Type UpdateType `json:"type,omitempty"`

	// Parallel is the number of resource operations to run in parallel at once
	// (1 for no parallelism). Defaults to unbounded.
	// +optional
	Parallel *int32 `json:"parallel,omitempty"`
	// Message (optional) to associate with the preview operation
	// +optional
	Message *string `json:"message,omitempty"`
	// Return an error if any changes occur during this update
	// +optional
	ExpectNoChanges *bool `json:"expectNoChanges,omitempty"`
	// Specify resources to replace
	// +optional
	Replace []string `json:"replace,omitempty"`
	// Specify an exclusive list of resource URNs to update
	// +optional
	Target []string `json:"target,omitempty"`
	// TargetDependents allows updating of dependent targets discovered but not
	// specified in the Target list
	// +optional
	TargetDependents *bool `json:"targetDependents,omitempty"`
	// refresh will run a refresh before the update.
	// +optional
	Refresh *bool `json:"refresh,omitempty"`
	// ContinueOnError will continue to perform the update operation despite the
	// occurrence of errors.
	ContinueOnError *bool `json:"continueOnError,omitempty"`
	// Remove the stack and its configuration after all resources in the stack
	// have been deleted.
	Remove *bool `json:"remove,omitempty"`
}

// UpdateStatus defines the observed state of Update
type UpdateStatus struct {
	// observedGeneration represents the .metadata.generation that the status was set based upon.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// The start time of the operation.
	StartTime metav1.Time `json:"startTime,omitempty"`

	// The end time of the operation.
	EndTime metav1.Time `json:"endTime,omitempty"`

	// Represents the permalink URL in the Pulumi Console for the operation. Not available for DIY backends.
	// +optional
	Permalink string `json:"permalink,omitempty"`

	// +optional
	Message string `json:"message,omitempty"`

	// Outputs names a secret containing the outputs for this update.
	// +optional
	Outputs string `json:"outputs,omitempty"`

	// Represents the observations of an update's current state.
	// Known .status.conditions.type are: "Complete", "Failed", and "Progressing"
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Workspace",type=string,JSONPath=`.spec.workspaceName`
//+kubebuilder:printcolumn:name="Start Time",type=date,priority=10,JSONPath=`.status.startTime`
//+kubebuilder:printcolumn:name="End Time",type=date,priority=10,JSONPath=`.status.endTime`
//+kubebuilder:printcolumn:name="Progressing",type=string,JSONPath=`.status.conditions[?(@.type=="Progressing")].status`
//+kubebuilder:printcolumn:name="Failed",type=string,JSONPath=`.status.conditions[?(@.type=="Failed")].status`
//+kubebuilder:printcolumn:name="Complete",type=string,JSONPath=`.status.conditions[?(@.type=="Complete")].status`
//+kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.permalink`

// Update is the Schema for the updates API
type Update struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UpdateSpec   `json:"spec,omitempty"`
	Status UpdateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UpdateList contains a list of Update
type UpdateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Update `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Update{}, &UpdateList{})
}
