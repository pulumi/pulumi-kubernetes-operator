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

	// Project is the source control repository from which we fetch the project code and configuration.
	Project string `json:"project"`
	// TODO: support RepoDir, in case Pulumi.yaml isn't in the root.
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

// StackUpdateStatus is the status code for the result of a Stack Update run.
type StackUpdateStatus int

const (
	// StackUpdateSucceeded indicates that the stack update completed successfully.
	StackUpdateSucceeded StackUpdateStatus = 0
	// StackUpdateFailed indicates that the stack update failed to complete.
	StackUpdateFailed StackUpdateStatus = 1
	// StackUpdateConflict indicates that the stack update failed to complete due
	// to a conflicting stack update run that is in progress.
	StackUpdateConflict StackUpdateStatus = 2
	// StackUpdatePendingOperations indicates that the stack update failed to complete due
	// to pending operations halting the stack update run.
	StackUpdatePendingOperations StackUpdateStatus = 3
)

// StackController contains methods to operate and manage a Pulumi Project and Stack
// in a Stack update execution run.
type StackController interface {
	// Source control:

	// FetchProjectSource clones the stack's source repo at a commit or branch
	// and returns its temporary workdir path.
	FetchProjectSource(repoURL, accessToken string, commit, branch *string) (string, error)

	// Service setup:

	// Log in to the Pulumi Service to manage the stack state.
	Login(apiURL string, accessToken string) error

	// Project setup:

	// SetWorkingDir changes the relative working directory in the project's
	// source directory to run the stack update.
	SetWorkingDir() error
	// InstallProjectDependencies installs the package manager dependencies for the project's language.
	InstallProjectDependencies() error
	// SetEnvs populates the environment from an array of Kubernetes ConfigMaps
	// with values to set.
	SetEnvs(env []string) error
	// SetSecretEnvs populates the environment from an array of Kubernetes Secrets
	// with values to set.
	SetSecretEnvs(env []string) error

	// Lifecycle:

	// CreateStack creates and selects a new stack instance to use in the
	// update run, configured with an optional secrets provider, and returns the stack name.
	// This is mutually exclusive with the SelectStack setting.
	CreateStack(stackName string, secretsProvider *string) (string, error)
	// SelectStack selects an existing stack instance in the project to use
	// in the update run. This is mutually exclusive with the CreateStack
	// setting.
	SelectStack(stackName string) error
	// OverrideConfig updates the stack with configuration values, overriding
	// any stack configuration values checked into the source repository.
	OverrideConfig(config map[string]string) error
	// OverrideSecrets updates the stack with secret configuration values, overriding
	// any stack secret configuration values checked into the source repository.
	OverrideSecrets(secrets map[string]string) error
	// RefreshStack refreshes the stack before the update step is run, and
	// errors the run if changes were not expected but found after the refresh.
	RefreshStack(expectChanges bool) error
	// UpdateStack deploys the stack's resources, computes the new desired
	// state, and returns the update's status.
	UpdateStack() (StackUpdateStatus, error)
	// GetStackOutputs returns all of the the stack's output properties.
	GetStackOutputs() error
	// UpdateStatus updates the status of the stack with the result of the update run.
	UpdateStatus(status StackUpdateStatus) error
	// DestroyStack destroys the stack's resources and state, and the stack itself.
	DestroyStack() error
}

func init() {
	SchemeBuilder.Register(&Stack{}, &StackList{})
}
