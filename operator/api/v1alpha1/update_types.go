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

// UpdateSpec defines the desired state of Update
type UpdateSpec struct {
	// WorkspaceName is the workspace to update.
	WorkspaceName string `json:"workspaceName,omitempty"`
	StackName     string `json:"stackName,omitempty"`
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
