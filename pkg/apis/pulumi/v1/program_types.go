package v1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Program is the schema for the inline YAML program API.
// +kubebuilder:resource:path=programs,scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Program struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Program ProgramSpec `json:"program,omitempty"`
}

// +kubebuilder:object:generate:=false
type Expression = apiextensionsv1.JSON

// +kubebuilder:object:generate:=false
type Any = apiextensionsv1.JSON

type ProgramSpec struct {
	// +optional
	Configuration map[string]Configuration `json:"configuration,omitempty"`
	// +optional
	Resources map[string]Resource `json:"resources,omitempty"`
	// +optional
	Variables map[string]Expression `json:"variables,omitempty"`
	// +optional
	Outputs map[string]Expression `json:"outputs,omitempty"`
}

// +kubebuilder:validation:Enum={"String", "Number", "List<Number>", "List<String>"}
type ConfigTypes string

type Configuration struct {
	// +optional
	Type ConfigTypes `json:"type,omitempty"`
	// +optional
	Default Any `json:"default,omitempty"`
}

type Resource struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Type string `json:"type"`
	// +optional
	Properties map[string]Expression `json:"properties,omitempty"`
	// +optional
	Options *Options `json:"options,omitempty"`
	// +optional
	Get *Getter `json:"get,omitempty"`
}

type Options struct {
	// +optional
	AdditionalSecretOutputs []string `json:"additionalSecretOutputs,omitempty"`
	// +optional
	Aliases []string `json:"aliases,omitempty"`
	// +optional
	CustomTimeouts *CustomTimeouts `json:"customTimeouts,omitempty"`
	// +optional
	DeleteBeforeReplace bool `json:"deleteBeforeReplace,omitempty"`
	// +optional
	DependsOn []Expression `json:"dependsOn,omitempty"`
	// +optional
	IgnoreChanges []string `json:"ignoreChanges,omitempty"`
	// +optional
	Import string `json:"import,omitempty"`
	// +optional
	Parent *Expression `json:"parent,omitempty"`
	// +optional
	Protect bool `json:"protect,omitempty"`
	// +optional
	Provider *Expression `json:"provider,omitempty"`
	// +optional
	Providers map[string]Expression `json:"providers,omitempty"`
	// +optional
	Version string `json:"version,omitempty"`
}

type CustomTimeouts struct {
	// +optional
	Create string `json:"create,omitempty"`
	// +optional
	Delete string `json:"delete,omitempty"`
	// +optional
	Update string `json:"update,omitempty"`
}

type Getter struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength:=1
	Id string `json:"id"`
	// +optional
	State map[string]Expression `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ProgramList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Program `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Program{}, &ProgramList{})
}
