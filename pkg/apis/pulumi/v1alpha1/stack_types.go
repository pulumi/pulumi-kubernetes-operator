package v1alpha1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StackSpec defines the desired state of Pulumi Stack being managed by this operator.
type StackSpec struct {
	// Auth info:

	// AccessTokenSecret is the name of a secret containing the PULUMI_ACCESS_TOKEN for Pulumi access.
	AccessTokenSecret string `json:"accessTokenSecret"`
	// Envs is an optional array of config maps containing environment variables to set.
	Envs []string `json:"envs,omitempty"`
	// SecretEnvs is an optional array of secret names containing environment variables to set.
	SecretEnvs []string `json:"envSecrets,omitempty"`

	// Stack identity:

	// Stack is the fully qualified name of the stack to deploy (<org>/<stack>).
	Stack string `json:"stack"`
	// Config is the configuration for this stack, which can be optionally specified inline. If this
	// is omitted, configuration is assumed to be checked in and taken from the source repository.
	// TODO: this is complicated because it needs to support secrets.
	Config *map[string]string `json:"config,omitempty"`

	// Source control:

	// ProjectRepo is the git source control repository from which we fetch the project code and configuration.
	ProjectRepo string `json:"projectRepo"`
	// ProjectRepoAccessTokenSecret is the the name of a secret containing a
	// personal access token to use a private git source control repository.
	ProjectRepoAccessTokenSecret string `json:"projectRepoAccessTokenSecret,omitempty"`
	// RepoDir is the directory to work from in the project's source repository
	// where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
	// in the project source root.
	RepoDir string `json:"repoDir,omitempty"`
	// Commit is the hash of the commit to deploy. If used, HEAD will be in detached mode. This
	// is mutually exclusive with the Branch setting. If both are empty, the `master` branch is deployed.
	Commit string `json:"commit,omitempty"`
	// Branch is the branch name to deploy, either the simple or fully qualified ref name. This
	// is mutually exclusive with the Commit setting. If both are empty, the `master` branch is deployed.
	Branch string `json:"branch,omitempty"`

	// Lifecycle:

	// InitOnCreate can be set to true to create the stack from scratch upon creation of the CRD.
	InitOnCreate bool `json:"initOnCreate,omitempty"`
	// DestroyOnFinalize can be set to true to destroy the stack completely upon deletion of the CRD.
	DestroyOnFinalize bool `json:"destroyOnFinalize,omitempty"`
}

// StackStatus defines the observed state of Stack
type StackStatus struct {
	// Outputs contains the exported stack output variables resulting from a deployment.
	Outputs *StackOutputs `json:"outputs,omitempty"`
	// LastUpdate contains details of the status of the last update.
	LastUpdate *StackUpdateState `json:"lastUpdate,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// StackUpdateState is the status of a stack update
type StackUpdateState struct {
	// State is the state of the stack update - one of `succeeded` or `failed`
	State string `json:"state,omitempty"`
	// TODO: Add additional information about errors if state was `failed`
	// TODO: Potentially add the revision number of the last update
}

// StackOutputs is an opaque JSON blob, since Pulumi stack outputs can contain arbitrary JSON maps and objects.
// Due to a limitation in controller-tools code generation, we need to do this to trick it to generate a JSON
// object instead of byte array. See https://github.com/kubernetes-sigs/controller-tools/issues/155.
type StackOutputs struct {
	// Raw JSON representation of the remote status as a byte array.
	Raw json.RawMessage `json:"raw,omitempty"`
}

// MarshalJSON returns the JSON encoding of the StackOutputs.
func (s StackOutputs) MarshalJSON() ([]byte, error) {
	return s.Raw.MarshalJSON()
}

// UnmarshalJSON sets the StackOutputs to a copy of data.
func (s *StackOutputs) UnmarshalJSON(data []byte) error {
	return s.Raw.UnmarshalJSON(data)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Stack is the Schema for the stacks API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=stacks,scope=Namespaced
type Stack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackSpec   `json:"spec,omitempty"`
	Status StackStatus `json:"status,omitempty"`
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
