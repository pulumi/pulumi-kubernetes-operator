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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WorkspaceSpec defines the desired state of Workspace
type WorkspaceSpec struct {
	// ServiceAccountName is the Kubernetes service account identity of the workspace.
	// +kubebuilder:default="default"
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Image is the Docker image containing the 'pulumi' and 'pulumi-kubernetes-agent' executables.
	Image string `json:"image,omitempty"`

	// Repo is the git source containing the Pulumi program.
	Git *GitSource `json:"git,omitempty"`

	// Flux is the flux source containing the Pulumi program.
	Flux *FluxSource `json:"flux,omitempty"`

	// List of environment variables to set in the workspace.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// GitSource specifies how to fetch from a git repository directly.
type GitSource struct {
	// ProjectRepo is the git source control repository from which we fetch the project code and configuration.
	ProjectRepo string `json:"projectRepo,omitempty"`
	// (optional) RepoDir is the directory to work from in the project's source repository
	// where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
	// in the project source root.
	// +optional
	RepoDir string `json:"repoDir,omitempty"`
	// (optional) Commit is the hash of the commit to deploy. If used, HEAD will be in detached mode. This
	// is mutually exclusive with the Branch setting. Either value needs to be specified.
	// +optional
	Commit string `json:"commit,omitempty"`
	// (optional) Branch is the branch name to deploy, either the simple or fully qualified ref name, e.g. refs/heads/master. This
	// is mutually exclusive with the Commit setting. Either value needs to be specified.
	// When specified, the operator will periodically poll to check if the branch has any new commits.
	// The frequency of the polling is configurable through ResyncFrequencySeconds, defaulting to every 60 seconds.
	// +optional
	Branch string `json:"branch,omitempty"`
}

// FluxSource specifies how to fetch from a Flux source object
type FluxSource struct {
	SourceRef FluxSourceReference `json:"sourceRef"`
	// Dir gives the subdirectory containing the Pulumi project (i.e., containing Pulumi.yaml) of
	// interest, within the fetched source.
	// +optional
	Dir string `json:"dir,omitempty"`
}

type FluxSourceReference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
}

// WorkspaceStatus defines the observed state of Workspace
type WorkspaceStatus struct {
	// observedGeneration represents the .metadata.generation that the status was set based upon.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Address string `json:"address,omitempty"`

	// Represents the observations of a workspace's current state.
	// Known .status.conditions.type are: "Ready"
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
//+kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
//+kubebuilder:printcolumn:name="Address",type=string,JSONPath=`.status.address`

// Workspace is the Schema for the workspaces API
// A Workspace is an execution context containing a single Pulumi project, a program, and multiple stacks.
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkspaceList contains a list of Workspace
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}
