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
	// SecurityProfileRestricted applies the restricted security profile.
	SecurityProfileRestricted SecurityProfile = "restricted"

	// SecurityProfileBaselineDefaultImage is the default image used when the security profile is 'baseline'.
	SecurityProfileBaselineDefaultImage = "pulumi/pulumi:latest"
	// SecurityProfileRestrictedDefaultImage is the default image used when the security profile is 'restricted'.
	SecurityProfileRestrictedDefaultImage = "pulumi/pulumi:latest-nonroot"
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

	// Image is the container image containing the 'pulumi' executable. If no image is provided,
	// the default image is used based on the securityProfile:
	// for 'baseline', it defaults to 'pulumi/pulumi:latest';
	// for 'restricted', it defaults to 'pulumi/pulumi:latest-nonroot'.
	Image string `json:"image,omitempty"`

	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Git is the git source containing the Pulumi program.
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
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// PodTemplate defines a PodTemplateSpec for Workspace's pods.
	//
	// +optional
	PodTemplate *EmbeddedPodTemplateSpec `json:"podTemplate,omitempty"`

	// List of stacks this workspace manages.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	Stacks []WorkspaceStack `json:"stacks,omitempty"`
}

// GitSource specifies how to fetch from a git repository directly.
type GitSource struct {
	// URL is the git source control repository from which we fetch the project
	// code and configuration.
	URL string `json:"url,omitempty"`
	// Ref is the git ref (tag, branch, or commit SHA) to fetch.
	Ref string `json:"ref,omitempty"`
	// Dir is the directory to work from in the project's source repository
	// where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
	// in the project source root.
	// +optional
	Dir string `json:"dir,omitempty"`
	// Auth contains optional authentication information to use when cloning
	// the repository.
	// +optional
	Auth *GitAuth `json:"auth,omitempty"`
	// Shallow controls whether the workspace uses a shallow clone or whether
	// all history is cloned.
	// +optional
	Shallow bool `json:"shallow,omitempty"`
}

// GitAuth specifies git authentication configuration options.
// There are 3 different authentication options:
//   - Personal access token
//   - SSH private key (and its optional password)
//   - Basic auth username and password
//
// Only 1 authentication mode is valid.
type GitAuth struct {
	// SSHPrivateKey should contain a private key for access to the git repo.
	// When using `SSHPrivateKey`, the URL of the repository must be in the
	// format git@github.com:org/repository.git.
	SSHPrivateKey *corev1.SecretKeySelector `json:"sshPrivateKey,omitempty"`
	// Username is the username to use when authenticating to a git repository.
	Username *corev1.SecretKeySelector `json:"username,omitempty"`
	// The password that pairs with a username or as part of an SSH Private Key.
	Password *corev1.SecretKeySelector `json:"password,omitempty"`
	// Token is a Git personal access token in replacement of
	// your password.
	Token *corev1.SecretKeySelector `json:"token,omitempty"`
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

type WorkspaceStack struct {
	Name string `json:"name"`

	// Create the stack if it does not exist.
	Create *bool `json:"create,omitempty"`

	// SecretsProvider is the name of the secret provider to use for the stack.
	SecretsProvider *string `json:"secretsProvider,omitempty"`

	// Config is a list of confguration values to set on the stack.
	// +optional
	// +patchMergeKey=key
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=key
	Config []ConfigItem `json:"config,omitempty"`
}

// +structType=atomic
type ConfigItem struct {
	// Key is the configuration key or path to set.
	Key string `json:"key"`
	// The key contains a path to a property in a map or list to set
	// +optional
	Path *bool `json:"path,omitempty"`
	// Value is the configuration value to set.
	// +optional
	Value *string `json:"value,omitempty"`
	// ValueFrom is a reference to a value from the environment or from a file.
	// +optional
	ValueFrom *ConfigValueFrom `json:"valueFrom,omitempty"`
	// Secret marks the configuration value as a secret.
	Secret *bool `json:"secret,omitempty"`
}

// +structType=atomic
type ConfigValueFrom struct {
	// Env is an environment variable in the pulumi container to use as the value.
	// +optional
	Env string `json:"env,omitempty"`
	// Path is a path to a file in the pulumi container containing the value.
	Path string `json:"path,omitempty"`
}

// EmbeddedPodTemplateSpec is an embedded version of k8s.io/api/core/v1.PodTemplateSpec.
// It contains a reduced ObjectMeta.
type EmbeddedPodTemplateSpec struct {
	// EmbeddedMetadata contains metadata relevant to an embedded resource.
	// +optional
	Metadata EmbeddedObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec *corev1.PodSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// EmbeddedWorkspaceTemplateSpec is an embedded version of WorkspaceSpec with a
// reduced ObjectMeta.
type EmbeddedWorkspaceTemplateSpec struct {
	Metadata EmbeddedObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec     *WorkspaceSpec     `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// EmbeddedObjectMeta contains a subset of the fields included in k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
// Only fields which are relevant to embedded resources are included.
type EmbeddedObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
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
