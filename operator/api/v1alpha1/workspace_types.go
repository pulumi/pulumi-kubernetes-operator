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

const (
	WorkspaceReady = "Ready"
)

// SecurityProfile declares the security profile of the workspace, either baseline or restricted.
// +enum
type SecurityProfile string

const (
	// SecurityProfileBaseline applies the baseline security profile.
	SecurityProfileBaseline SecurityProfile = "baseline"
	// SecurityProfileBaseline applies the restricted security profile.
	SecurityProfileRestricted SecurityProfile = "restricted"
)

// WorkspaceSpec defines the desired state of Workspace
type WorkspaceSpec struct {
	// ServiceAccountName is the Kubernetes service account identity of the workspace.
	// +kubebuilder:default="default"
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// SecurityProfile applies a security profile to the workspace, 'restricted' by default.
	// +kubebuilder:default="restricted"
	// +optional
	SecurityProfile SecurityProfile `json:"securityProfile,omitempty"`

	// Image is the Docker image containing the 'pulumi' executable.
	// +kubebuilder:default="pulumi/pulumi:latest"
	Image string `json:"image,omitempty"`

	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Repo is the git source containing the Pulumi program.
	// +optional
	Git *GitSource `json:"git,omitempty"`

	// Flux is the flux source containing the Pulumi program.
	// +optional
	Flux *FluxSource `json:"flux,omitempty"`

	// List of sources to populate environment variables in the workspace.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// +optional
	// +listType=atomic
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// List of environment variables to set in the container.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Compute Resources required by this workspace.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,8,opt,name=resources"`

	// PodTemplate defines a PodTemplateSpec for Workspace's pods.
	//
	// +optional
	PodTemplate *corev1.PodTemplateSpec `json:"podTemplate,omitempty"`
}

// GitSource specifies how to fetch from a git repository directly.
type GitSource struct {
	// Url is the git source control repository from which we fetch the project code and configuration.
	Url string `json:"url,omitempty"`
	// Revision is the git revision (tag, or commit SHA) to fetch.
	Revision string `json:"revision,omitempty"`
	// (optional) Dir is the directory to work from in the project's source repository
	// where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
	// in the project source root.
	// +optional
	Dir string `json:"dir,omitempty"`
}

// FluxSource specifies how to fetch a Fllux source artifact.
type FluxSource struct {
	// URL is the URL of the artifact to fetch.
	Url string `json:"url,omitempty"`
	// Digest is the digest of the artifact to fetch.
	Digest string `json:"digest,omitempty"`
	// Dir gives the subdirectory containing the Pulumi project (i.e., containing Pulumi.yaml) of
	// interest, within the fetched artifact.
	// +optional
	Dir string `json:"dir,omitempty"`
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
