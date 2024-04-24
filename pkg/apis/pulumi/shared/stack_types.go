package shared

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ReconcileRequestAnnotation = "pulumi.com/reconciliation-request"

// StackSpec defines the desired state of Pulumi Stack being managed by this operator.
type StackSpec struct {
	// Auth info:

	// (optional) AccessTokenSecret is the name of a Secret containing the PULUMI_ACCESS_TOKEN for Pulumi access.
	// Deprecated: use EnvRefs with a "secret" entry with the key PULUMI_ACCESS_TOKEN instead.
	AccessTokenSecret string `json:"accessTokenSecret,omitempty"`

	// (optional) Envs is an optional array of config maps containing environment variables to set.
	// Deprecated: use EnvRefs instead.
	Envs []string `json:"envs,omitempty"`

	// (optional) EnvRefs is an optional map containing environment variables as keys and stores descriptors to where
	// the variables' values should be loaded from (one of literal, environment variable, file on the
	// filesystem, or Kubernetes Secret) as values.
	EnvRefs map[string]ResourceRef `json:"envRefs,omitempty"`

	// (optional) SecretEnvs is an optional array of Secret names containing environment variables to set.
	// Deprecated: use EnvRefs instead.
	SecretEnvs []string `json:"envSecrets,omitempty"`

	// (optional) Backend is an optional backend URL to use for all Pulumi operations.<br/>
	// Examples:<br/>
	//   - Pulumi Service:              "https://app.pulumi.com" (default)<br/>
	//   - Self-managed Pulumi Service: "https://pulumi.acmecorp.com" <br/>
	//   - Local:                       "file://./einstein" <br/>
	//   - AWS:                         "s3://<my-pulumi-state-bucket>" <br/>
	//   - Azure:                       "azblob://<my-pulumi-state-bucket>" <br/>
	//   - GCP:                         "gs://<my-pulumi-state-bucket>" <br/>
	// See: https://www.pulumi.com/docs/intro/concepts/state/
	Backend string `json:"backend,omitempty"`

	// Stack identity:

	// Stack is the fully qualified name of the stack to deploy (<org>/<stack>).
	Stack string `json:"stack"`
	// (optional) Config is the configuration for this stack, which can be optionally specified inline. If this
	// is omitted, configuration is assumed to be checked in and taken from the source repository.
	Config map[string]string `json:"config,omitempty"`
	// (optional) ConfigRefs is the configuration for this stack, which can be specified through ConfigRef.
	// is omitted, configuration is assumed to be checked in and taken from the source repository.
	ConfigRefs map[string]ConfigRef `json:"configsRef,omitempty"`
	// (optional) Secrets is the secret configuration for this stack, which can be optionally specified inline. If this
	// is omitted, secrets configuration is assumed to be checked in and taken from the source repository.
	// Deprecated: use SecretRefs instead.
	Secrets map[string]string `json:"secrets,omitempty"`
	// (optional) SecretRefs is the secret configuration for this stack which can be specified through ResourceRef.
	// If this is omitted, secrets configuration is assumed to be checked in and taken from the source repository.
	SecretRefs map[string]ResourceRef `json:"secretsRef,omitempty"`
	// (optional) SecretsProvider is used to initialize a Stack with alternative encryption.
	// Examples:
	//   - AWS:   "awskms:///arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34bc-56ef-1234567890ab?region=us-east-1"
	//   - Azure: "azurekeyvault://acmecorpvault.vault.azure.net/keys/mykeyname"
	//   - GCP:   "gcpkms://projects/MYPROJECT/locations/MYLOCATION/keyRings/MYKEYRING/cryptoKeys/MYKEY"
	//   -
	// See: https://www.pulumi.com/docs/intro/concepts/secrets/#initializing-a-stack-with-alternative-encryption
	SecretsProvider string `json:"secretsProvider,omitempty"`

	// Source control:

	// GitSource inlines the fields for specifying git sources; it is not itself optional, so as not
	// to break compatibility.
	*GitSource `json:",inline"`
	// FluxSource specifies how to fetch source code from a Flux source object.
	// +optional
	FluxSource *FluxSource `json:"fluxSource,omitempty"`

	// ProgramRef refers to a Program object, to be used as the source for the stack.
	ProgramRef *ProgramReference `json:"programRef,omitempty"`

	// Lifecycle:

	// (optional) Targets is a list of URNs of resources to update exclusively. If supplied, only
	// resources mentioned will be updated.
	Targets []string `json:"targets,omitempty"`

	// (optional) Prerequisites is a list of references to other stacks, each with a constraint on
	// how long ago it must have succeeded. This can be used to make sure e.g., state is
	// re-evaluated before running a stack that depends on it.
	Prerequisites []PrerequisiteRef `json:"prerequisites,omitempty"`

	// (optional) ContinueResyncOnCommitMatch - when true - informs the operator to continue trying
	// to update stacks even if the revision of the source matches. This might be useful in
	// environments where Pulumi programs have dynamic elements for example, calls to internal APIs
	// where GitOps style commit tracking is not sufficient.  Defaults to false, i.e. when a
	// particular revision is successfully run, the operator will not attempt to rerun the program
	// at that revision again.
	ContinueResyncOnCommitMatch bool `json:"continueResyncOnCommitMatch,omitempty"`

	// (optional) Refresh can be set to true to refresh the stack before it is updated.
	Refresh bool `json:"refresh,omitempty"`
	// (optional) ExpectNoRefreshChanges can be set to true if a stack is not expected to have
	// changes during a refresh before the update is run.
	// This could occur, for example, is a resource's state is changing outside of Pulumi
	// (e.g., metadata, timestamps).
	ExpectNoRefreshChanges bool `json:"expectNoRefreshChanges,omitempty"`
	// (optional) DestroyOnFinalize can be set to true to destroy the stack completely upon deletion of the Stack custom resource.
	DestroyOnFinalize bool `json:"destroyOnFinalize,omitempty"`
	// (optional) RetryOnUpdateConflict issues a stack update retry reconciliation loop
	// in the event that the update hits a HTTP 409 conflict due to
	// another update in progress.
	// This is only recommended if you are sure that the stack updates are
	// idempotent, and if you are willing to accept retry loops until
	// all spawned retries succeed. This will also create a more populated,
	// and randomized activity timeline for the stack in the Pulumi Service.
	RetryOnUpdateConflict bool `json:"retryOnUpdateConflict,omitempty"`

	// (optional) UseLocalStackOnly can be set to true to prevent the operator from
	// creating stacks that do not exist in the tracking git repo.
	// The default behavior is to create a stack if it doesn't exist.
	UseLocalStackOnly bool `json:"useLocalStackOnly,omitempty"`

	// (optional) ResyncFrequencySeconds when set to a non-zero value, triggers a resync of the stack at
	// the specified frequency even if no changes to the custom resource are detected.
	// If branch tracking is enabled (branch is non-empty), commit polling will occur at this frequency.
	// The minimal resync frequency supported is 60 seconds. The default value for this field is 60 seconds.
	ResyncFrequencySeconds int64 `json:"resyncFrequencySeconds,omitempty"`
}

// GitSource specifies how to fetch from a git repository directly.
type GitSource struct {
	// ProjectRepo is the git source control repository from which we fetch the project code and configuration.
	// +optional
	ProjectRepo string `json:"projectRepo,omitempty"`
	// (optional) GitAuthSecret is the the name of a Secret containing an
	// authentication option for the git repository.
	// There are 3 different authentication options:
	//   * Personal access token
	//   * SSH private key (and it's optional password)
	//   * Basic auth username and password
	// Only one authentication mode will be considered if more than one option is specified,
	// with ssh private key/password preferred first, then personal access token, and finally
	// basic auth credentials.
	// Deprecated. Use GitAuth instead.
	GitAuthSecret string `json:"gitAuthSecret,omitempty"`

	// (optional) GitAuth allows configuring git authentication options
	// There are 3 different authentication options:
	//   * SSH private key (and its optional password)
	//   * Personal access token
	//   * Basic auth username and password
	// Only one authentication mode will be considered if more than one option is specified,
	// with ssh private key/password preferred first, then personal access token, and finally
	// basic auth credentials.
	GitAuth *GitAuthConfig `json:"gitAuth,omitempty"`
	// (optional) RepoDir is the directory to work from in the project's source repository
	// where Pulumi.yaml is located. It is used in case Pulumi.yaml is not
	// in the project source root.
	RepoDir string `json:"repoDir,omitempty"`
	// (optional) Commit is the hash of the commit to deploy. If used, HEAD will be in detached mode. This
	// is mutually exclusive with the Branch setting. Either value needs to be specified.
	Commit string `json:"commit,omitempty"`
	// (optional) Branch is the branch name to deploy, either the simple or fully qualified ref name, e.g. refs/heads/master. This
	// is mutually exclusive with the Commit setting. Either value needs to be specified.
	// When specified, the operator will periodically poll to check if the branch has any new commits.
	// The frequency of the polling is configurable through ResyncFrequencySeconds, defaulting to every 60 seconds.
	Branch string `json:"branch,omitempty"`
}

// PrerequisiteRef refers to another stack, and gives requirements for the prerequisite to be
// considered satisfied.
type PrerequisiteRef struct {
	// Name is the name of the Stack resource that is a prerequisite.
	Name string `json:"name"`
	// Requirement gives specific requirements for the prerequisite; the base requirement is that
	// the referenced stack is in a successful state.
	Requirement *RequirementSpec `json:"requirement,omitempty"`
}

// RequirementSpec gives constraints for a prerequisite to be considered satisfied.
type RequirementSpec struct {
	// SucceededWithinDuration gives a duration within which the prerequisite must have reached a
	// succeeded state; e.g., "1h" means "the prerequisite must be successful, and have become so in
	// the last hour". Fields (should there ever be more than one) are not intended to be mutually
	// exclusive.
	SucceededWithinDuration *metav1.Duration `json:"succeededWithinDuration,omitempty"`
}

// GitAuthConfig specifies git authentication configuration options.
// There are 3 different authentication options:
//   - Personal access token
//   - SSH private key (and its optional password)
//   - Basic auth username and password
//
// Only 1 authentication mode is valid.
type GitAuthConfig struct {
	PersonalAccessToken *ResourceRef `json:"accessToken,omitempty"`
	SSHAuth             *SSHAuth     `json:"sshAuth,omitempty"`
	BasicAuth           *BasicAuth   `json:"basicAuth,omitempty"`
}

// SSHAuth configures ssh-based auth for git authentication.
// SSHPrivateKey is required but password is optional.
type SSHAuth struct {
	SSHPrivateKey ResourceRef  `json:"sshPrivateKey"`
	Password      *ResourceRef `json:"password,omitempty"`
}

// BasicAuth configures git authentication through basic auth —
// i.e. username and password. Both UserName and Password are required.
type BasicAuth struct {
	UserName ResourceRef `json:"userName"`
	Password ResourceRef `json:"password"`
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

// ResourceRef identifies a resource from which information can be loaded.
// Environment variables, files on the filesystem, Kubernetes Secrets and literal
// strings are currently supported.
type ResourceRef struct {
	// SelectorType is required and signifies the type of selector. Must be one of:
	// Env, FS, Secret, ConfigMap, Literal
	SelectorType     ResourceSelectorType `json:"type"`
	ResourceSelector `json:",inline"`
}

// ConfigRef identifies a resource from which config information can be loaded.
// Environment variables, files on the filesystem, Kubernetes Secrets, ConfigMap, structured and config literal values
// strings are currently supported.
type ConfigRef struct {
	// SelectorType is required and signifies the type of selector. Must be one of:
	// Env, FS, Secret, ConfigMap, Structured, Literal
	SelectorType           ConfigResourceSelectorType `json:"type"`
	ConfigResourceSelector `json:",inline"`
}

// ConfigResourceSelector is a union over resource config selectors supporting one of
// filesystem, environment variable, Kubernetes Secret, Kubernetes ConfigMaps, a structured value and literal values.
type ConfigResourceSelector struct {
	ResourceSelector `json:",inline"`

	// ConfigMapRef refers to a Kubernetes ConfigMap
	ConfigMapRef *ConfigMapSelector `json:"configmap,omitempty"`
	// StructuredRef refers to a structured value
	StructuredRef *StructuredRef `json:"structured,omitempty"`
}

type ProgramReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// NewEnvResourceRef creates a new environment variable resource ref.
func NewEnvResourceRef(envVarName string) ResourceRef {
	return ResourceRef{
		SelectorType: ResourceSelectorEnv,
		ResourceSelector: ResourceSelector{
			Env: &EnvSelector{
				Name: envVarName,
			},
		},
	}
}

// NewFileSystemResourceRef creates a new file system resource ref.
func NewFileSystemResourceRef(path string) ResourceRef {
	return ResourceRef{
		SelectorType: ResourceSelectorFS,
		ResourceSelector: ResourceSelector{
			FileSystem: &FSSelector{
				Path: path,
			},
		},
	}
}

// NewSecretResourceRef creates a new Secret resource ref.
func NewSecretResourceRef(namespace, name, key string) ResourceRef {
	return ResourceRef{
		SelectorType: ResourceSelectorSecret,
		ResourceSelector: ResourceSelector{
			SecretRef: &SecretSelector{
				Namespace: namespace,
				Name:      name,
				Key:       key,
			},
		},
	}
}

// NewSecretResourceRef creates a new Secret resource ref.
func NewSecretConfigResourceRef(namespace, name, key string) ConfigRef {
	secretResourceRef := NewSecretResourceRef(namespace, name, key)
	return ConfigRef{
		SelectorType: ConfigResourceSelectorType(secretResourceRef.SelectorType),
		ConfigResourceSelector: ConfigResourceSelector{
			ResourceSelector: secretResourceRef.ResourceSelector,
		},
	}
}

// NewLiteralResourceRef creates a new literal resource ref.
func NewLiteralResourceRef(value string) ResourceRef {
	return ResourceRef{
		SelectorType: ResourceSelectorLiteral,
		ResourceSelector: ResourceSelector{
			LiteralRef: &LiteralRef{
				Value: value,
			},
		},
	}
}

// ResourceSelectorType identifies the type of the resource reference in
type ResourceSelectorType string

const (
	// ResourceSelectorEnv indicates the resource is an environment variable
	ResourceSelectorEnv = ResourceSelectorType("Env")
	// ResourceSelectorFS indicates the resource is on the filesystem
	ResourceSelectorFS = ResourceSelectorType("FS")
	// ResourceSelectorSecret indicates the resource is a Kubernetes Secret
	ResourceSelectorSecret = ResourceSelectorType("Secret")
	// ResourceSelectorLiteral indicates the resource is a literal
	ResourceSelectorLiteral = ResourceSelectorType("Literal")
)

// ConfigResourceSelectorType identifies the type of the resource reference in
type ConfigResourceSelectorType string

const (
	// ConfigSelectorStructured indicates the resource is a Kubernetes ConfigMap
	ConfigResourceSelectorConfigMap = ConfigResourceSelectorType("ConfigMap")
	// ConfigResourceSelectorStructured indicates the resource is a structured
	ConfigResourceSelectorStructured = ConfigResourceSelectorType("Structured")
)

// ResourceSelector is a union over resource selectors supporting one of
// filesystem, environment variable, Kubernetes Secret and literal values.
type ResourceSelector struct {
	// FileSystem selects a file on the operator's file system
	FileSystem *FSSelector `json:"filesystem,omitempty"`
	// Env selects an environment variable set on the operator process
	Env *EnvSelector `json:"env,omitempty"`
	// SecretRef refers to a Kubernetes Secret
	SecretRef *SecretSelector `json:"secret,omitempty"`
	// LiteralRef refers to a literal value
	LiteralRef *LiteralRef `json:"literal,omitempty"`
}

// FSSelector identifies the path to load information from.
type FSSelector struct {
	// Path on the filesystem to use to load information from.
	Path string `json:"path"`
}

// EnvSelector identifies the environment variable to load information from.
type EnvSelector struct {
	// Name of the environment variable
	Name string `json:"name"`
}

// SecretSelector identifies the information to load from a Kubernetes Secret.
type SecretSelector struct {
	// Namespace where the Secret is stored. Deprecated; non-empty values will be considered invalid
	// unless namespace isolation is disabled in the controller.
	Namespace string `json:"namespace,omitempty"`
	// Name of the Secret
	Name string `json:"name"`
	// Key within the Secret to use.
	Key string `json:"key"`
}

// ConfigMapSelector identifies the information to load from a Kubernetes ConfigMap.
type ConfigMapSelector struct {
	// Namespace where the ConfigMap is stored. Deprecated; non-empty values will be considered invalid
	// unless namespace isolation is disabled in the controller.
	Namespace string `json:"namespace,omitempty"`
	// Name of the ConfigMap
	Name string `json:"name"`
	// Key within the ConfigMap to use.
	Key string `json:"key"`
}

// StructuredRef identifies a structured value to load.
type StructuredRef struct {
	// Value to load
	Value apiextensionsv1.JSON `json:"value"`
}

// LiteralRef identifies a literal value to load.
type LiteralRef struct {
	// Value to load
	Value string `json:"value"`
}

// StackStatus defines the observed state of Stack
type StackStatus struct {
	// Outputs contains the exported stack output variables resulting from a deployment.
	Outputs StackOutputs `json:"outputs,omitempty"`
	// LastUpdate contains details of the status of the last update.
	LastUpdate *StackUpdateState `json:"lastUpdate,omitempty"`
}

type StackOutputs map[string]apiextensionsv1.JSON

// StackUpdateState is the status of a stack update
type StackUpdateState struct {
	// State is the state of the stack update - one of `succeeded` or `failed`
	State StackUpdateStateMessage `json:"state,omitempty"`
	// Last commit attempted
	LastAttemptedCommit string `json:"lastAttemptedCommit,omitempty"`
	// Last commit successfully applied
	LastSuccessfulCommit string `json:"lastSuccessfulCommit,omitempty"`
	// Permalink is the Pulumi Console URL of the stack operation.
	Permalink Permalink `json:"permalink,omitempty"`
	// LastResyncTime contains a timestamp for the last time a resync of the stack took place.
	LastResyncTime metav1.Time `json:"lastResyncTime,omitempty"`
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

type StackUpdateStateMessage string

const (
	// SucceededStackStateMessage is a const to indicate success in stack status state.
	SucceededStackStateMessage StackUpdateStateMessage = "succeeded"
	// FailedStackStateMessage is a const to indicate stack failure in stack status state.
	FailedStackStateMessage StackUpdateStateMessage = "failed"
)

// Permalink is the Pulumi Service URL of the stack operation.
type Permalink string
