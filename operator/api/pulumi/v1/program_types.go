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

package v1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:generate:=false
type Expression = apiextensionsv1.JSON

// +kubebuilder:object:generate:=false
type Any = apiextensionsv1.JSON

type ProgramSpec struct {
	// configuration specifies the Pulumi config inputs to the deployment.
	// Either type or default is required.
	// +optional
	Configuration map[string]Configuration `json:"configuration,omitempty"`

	// resources declares the Pulumi resources that will be deployed and managed by the program.
	// +optional
	Resources map[string]Resource `json:"resources,omitempty"`

	// variables specifies intermediate values of the program; the values of variables are
	// expressions that can be re-used.
	// +optional
	Variables map[string]Expression `json:"variables,omitempty"`

	// outputs specifies the Pulumi stack outputs of the program and how they are computed from the resources.
	// +optional
	Outputs map[string]Expression `json:"outputs,omitempty"`
}

// +kubebuilder:validation:Enum={"String", "Number", "List<Number>", "List<String>"}

type ConfigType string

type Configuration struct {
	// type is the (required) data type for the parameter.
	// +optional
	Type ConfigType `json:"type,omitempty"`

	// default is a value of the appropriate type for the template to use if no value is specified.
	// +optional
	Default *Any `json:"default,omitempty"`
}

type Resource struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1

	// type is the Pulumi type token for this resource.
	Type string `json:"type"`

	// properties contains the primary resource-specific keys and values to initialize the resource state.
	// +optional
	Properties map[string]Expression `json:"properties,omitempty"`

	// options contains all resource options supported by Pulumi.
	// +optional
	Options *Options `json:"options,omitempty"`

	// A getter function for the resource. Supplying get is mutually exclusive to properties.
	// +optional
	Get *Getter `json:"get,omitempty"`
}

type Options struct {
	// additionalSecretOutputs specifies properties that must be encrypted as secrets.
	// +optional
	AdditionalSecretOutputs []string `json:"additionalSecretOutputs,omitempty"`

	// aliases specifies names that this resource used to have, so that renaming or refactoring
	// doesn’t replace it.
	// +optional
	Aliases []string `json:"aliases,omitempty"`

	// customTimeouts overrides the default retry/timeout behavior for resource provisioning.
	// +optional
	CustomTimeouts *CustomTimeouts `json:"customTimeouts,omitempty"`

	// deleteBeforeReplace overrides the default create-before-delete behavior when replacing.
	// +optional
	DeleteBeforeReplace bool `json:"deleteBeforeReplace,omitempty"`

	// dependsOn adds explicit dependencies in addition to the ones in the dependency graph.
	// +optional
	DependsOn []Expression `json:"dependsOn,omitempty"`

	// ignoreChanges declares that changes to certain properties should be ignored when diffing.
	// +optional
	IgnoreChanges []string `json:"ignoreChanges,omitempty"`

	// import adopts an existing resource from your cloud account under the control of Pulumi.
	// +optional
	Import string `json:"import,omitempty"`

	// parent resource option specifies a parent for a resource. It is used to associate
	// children with the parents that encapsulate or are responsible for them.
	// +optional
	Parent *Expression `json:"parent,omitempty"`

	// protect prevents accidental deletion of a resource.
	// +optional
	Protect bool `json:"protect,omitempty"`

	// provider resource option sets a provider for the resource.
	// +optional
	Provider *Expression `json:"provider,omitempty"`

	// providers resource option sets a map of providers for the resource and its children.
	// +optional
	Providers map[string]Expression `json:"providers,omitempty"`

	// version specifies a provider plugin version that should be used when operating on a resource.
	// +optional
	Version string `json:"version,omitempty"`
}

type CustomTimeouts struct {
	// create is the custom timeout for create operations.
	// +optional
	Create string `json:"create,omitempty"`

	// delete is the custom timeout for delete operations.
	// +optional
	Delete string `json:"delete,omitempty"`

	// update is the custom timeout for update operations.
	// +optional
	Update string `json:"update,omitempty"`
}

type Getter struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1

	// The ID of the resource to import.
	Id string `json:"id"`

	// state contains the known properties (input & output) of the resource. This assists
	// the provider in figuring out the correct resource.
	// +optional
	State map[string]Expression `json:"state,omitempty"`
}

// ProgramStatus defines the observed state of Program.
type ProgramStatus struct {
	// ObservedGeneration is the last observed generation of the Program
	// object.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Artifact represents the last successful artifact generated by program reconciliation.
	// +optional
	Artifact *Artifact `json:"artifact,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=pulumi
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.artifact.url"

// Program is the schema for the inline YAML program API.
type Program struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Program ProgramSpec   `json:"program,omitempty"`
	Status  ProgramStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ProgramList contains a list of Program
type ProgramList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Program `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Program{}, &ProgramList{})
}
