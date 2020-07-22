package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StackSpec defines the desired state of Pulumi Stack being managed by this operator.
type StackSpec struct {
	// Auth info:

	// AccessTokenSecret is the name of a secret containing the PULUMI_ACCESS_TOKEN for Pulumi access.
	AccessTokenSecret string `json:"accessTokenSecret"`
	// (optional) Envs is an optional array of config maps containing environment variables to set.
	Envs []string `json:"envs,omitempty"`
	// (optional) SecretEnvs is an optional array of secret names containing environment variables to set.
	SecretEnvs []string `json:"envSecrets,omitempty"`

	// Stack identity:

	// Stack is the fully qualified name of the stack to deploy (<org>/<stack>).
	Stack string `json:"stack"`
	// (optional) Config is the configuration for this stack, which can be optionally specified inline. If this
	// is omitted, configuration is assumed to be checked in and taken from the source repository.
	Config map[string]string `json:"config,omitempty"`
	// (optional) Secrets is the secret configuration for this stack, which can be optionally specified inline. If this
	// is omitted, secrets configuration is assumed to be checked in and taken from the source repository.
	Secrets map[string]string `json:"secrets,omitempty"`
	// (optional) SecretsProvider is used with InitOnCreate to initialize a Stack with alternative encryption.
	// Examples:
	//   - AWS:   "awskms://arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34bc-56ef-1234567890ab?region=us-east-1"
	//   - Azure: "azurekeyvault://acmecorpvault.vault.azure.net/keys/mykeyname"
	//   - GCP:   "gcpkms://projects/MYPROJECT/locations/MYLOCATION/keyRings/MYKEYRING/cryptoKeys/MYKEY"
	// See: https://www.pulumi.com/docs/intro/concepts/config/#initializing-a-stack-with-alternative-encryption
	SecretsProvider string `json:"secretsProvider,omitempty"`

	// Source control:

	// ProjectRepo is the git source control repository from which we fetch the project code and configuration.
	ProjectRepo string `json:"projectRepo"`
	// (optional) ProjectRepoAccessTokenSecret is the the name of a secret containing a
	// personal access token to use a private git source control repository.
	ProjectRepoAccessTokenSecret string `json:"projectRepoAccessTokenSecret,omitempty"`
	// (optional) RepoDir is the directory to work from in the project's source repository
	// where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
	// in the project source root.
	RepoDir string `json:"repoDir,omitempty"`
	// (optional) Commit is the hash of the commit to deploy. If used, HEAD will be in detached mode. This
	// is mutually exclusive with the Branch setting. If both are empty, the `master` branch is deployed.
	Commit string `json:"commit,omitempty"`
	// (optional) Branch is the branch name to deploy, either the simple or fully qualified ref name. This
	// is mutually exclusive with the Commit setting. If both are empty, the `master` branch is deployed.
	Branch string `json:"branch,omitempty"`

	// Lifecycle:

	// (optional) InitOnCreate can be set to true to create the stack from scratch upon creation of the CRD.
	InitOnCreate bool `json:"initOnCreate,omitempty"`
	// (optional) Refresh can be set to true to refresh the stack before it is updated.
	Refresh bool `json:"refresh,omitempty"`
	// (optional) ExpectNoRefreshChanges can be set to true if a stack is not expected to have
	// changes during a refresh before the update is run.
	// This could occur, for example, is a resource's state is changing outside of Pulumi
	// (e.g., metadata, timestamps).
	ExpectNoRefreshChanges bool `json:"expectNoRefreshChanges,omitempty"`
	// (optional) DestroyOnFinalize can be set to true to destroy the stack completely upon deletion of the CRD.
	DestroyOnFinalize bool `json:"destroyOnFinalize,omitempty"`
	// (optional) RetryOnUpdateConflict issues a stack update retry reconciliation loop
	// in the event that the update hits a HTTP 409 conflict due to
	// another update in progress.
	// This is only recommended if you are sure that the stack updates are
	// idempotent, and if you are willing to accept retry loops until
	// all spawned retries succeed. This will also create a more populated,
	// and randomized activity timeline for the stack in the Pulumi Service.
	RetryOnUpdateConflict bool `json:"retryOnUpdateConflict,omitempty"`
}

// StackStatus defines the observed state of Stack
type StackStatus struct {
	// Outputs contains the exported stack output variables resulting from a deployment.
	Outputs StackOutputs `json:"outputs,omitempty"`
	// LastUpdate contains details of the status of the last update.
	LastUpdate *StackUpdateState `json:"lastUpdate,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

type StackOutputs map[string]apiextensionsv1.JSON

// StackUpdateState is the status of a stack update
type StackUpdateState struct {
	// State is the state of the stack update - one of `succeeded` or `failed`
	State string `json:"state,omitempty"`
	// TODO: Add additional information about errors if state was `failed`
	// TODO: Potentially add the revision number of the last update
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
	// StackNotFound indicates that the stack update failed to complete due
	// to stack not being found (HTTP 404) in the Pulumi Service.
	StackNotFound StackUpdateStatus = 4
)

// ProjectSourceOptions is the settings to work with the project source repo.
type ProjectSourceOptions struct {
	// The access token to access project source repo. This is required for
	// private repos, but is recommended for public repos to help with rate limiting.
	AccessToken string `json:"accessToken,omitempty"`
	// Commit is the hash of the commit to deploy. If used, HEAD will be in detached mode. This
	// is mutually exclusive with the Branch setting. If both are empty, the `master` branch is deployed.
	Commit string `json:"commit,omitempty"`
	// Branch is the branch name to deploy, either the simple or fully qualified ref name. This
	// is mutually exclusive with the Commit setting. If both are empty, the `master` branch is deployed.
	Branch string `json:"branch,omitempty"`
	// RepoDir is the directory to work from in the project's source repository
	// where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
	// in the project source root.
	RepoDir string `json:"repoDir,omitempty"`
}

// StackController contains methods to operate a Pulumi Project and Stack in an update.
//
// Ignoring operator codegen of interface as it is an API contract for implementation,
// not a type that is used in kubernetes.
// +kubebuilder:object:generate=false
type StackController interface {
	// Source control:

	// FetchProjectSource clones the stack's source repo and returns its temporary workdir path.
	FetchProjectSource(repoURL string, opts *ProjectSourceOptions) (string, error)

	// Project setup:

	// InstallProjectDependencies installs the package manager dependencies for the project's language.
	InstallProjectDependencies(runtime string) error
	// SetEnvs populates the environment of the stack run with values
	// from an array of Kubernetes ConfigMaps in a Namespace.
	SetEnvs(configMapNames []string, namespace string) error
	// SetSecretEnvs populates the environment of the stack run with values
	// from an array of Kubernetes Secrets in a Namespace.
	SetSecretEnvs(secretNames []string, namespace string) error

	// Lifecycle:

	// CreateStack creates a new stack instance to use in the update run.
	// It is optionally configured with a secrets provider.
	CreateStack(stack string, secretsProvider *string) error
	// UpdateConfig updates the stack configuration values by combining
	// any configuration values checked into the source repository with
	// the Config values provided in the Stack, overriding values that match and exist.
	UpdateConfig() error
	// UpdateSecretConfig updates the stack secret configuration values by combining
	// any secret configuration values checked into the source repository with
	// the Secrets values in the Stack, overriding values that match and exist.
	UpdateSecretConfig() error
	// RefreshStack refreshes the stack before the update step is run, and
	// errors the run if changes were not expected but found after the refresh.
	RefreshStack(expectNoChanges bool) error
	// UpdateStack deploys the stack's resources, computes the new desired
	// state, and returns the update's status.
	UpdateStack() (StackUpdateStatus, error)
	// GetStackOutputs returns all of the the stack's output properties.
	GetStackOutputs() (StackOutputs, error)
	// DestroyStack destroys the stack's resources and state, and the stack itself.
	DestroyStack() error
}

func init() {
	SchemeBuilder.Register(&Stack{}, &StackList{})
}
