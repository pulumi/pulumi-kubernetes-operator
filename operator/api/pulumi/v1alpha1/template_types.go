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

package v1alpha1

import (
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TemplateSpec defines the desired state of Template
type TemplateSpec struct {
	// CRD specifies the Custom Resource Definition that will be generated from this template.
	// +kubebuilder:validation:Required
	CRD CRDSpec `json:"crd"`

	// Schema defines the API schema for instances of the generated CRD.
	// +kubebuilder:validation:Required
	Schema TemplateSchema `json:"schema"`

	// Resources declares the Pulumi resources that will be deployed for each instance.
	// These use the same format as Program resources with ${...} expressions for dynamic values.
	// Supported expressions:
	//   - ${schema.spec.<field>}: References a field from the instance spec
	//   - ${schema.metadata.name}: References the instance name
	//   - ${schema.metadata.namespace}: References the instance namespace
	//   - ${<resource>.<property>}: References another resource's property (passed through to Pulumi)
	//   - ${schema.spec.X || schema.metadata.Y}: Fallback expression (uses first non-empty value)
	// +kubebuilder:validation:Required
	Resources map[string]pulumiv1.Resource `json:"resources"`

	// Variables specifies intermediate computed values that can be referenced in resources and outputs.
	// +optional
	Variables map[string]pulumiv1.Expression `json:"variables,omitempty"`

	// Outputs maps status fields from Pulumi resources to instance status.
	// Keys are the status field names, values are ${...} expressions referencing resource properties.
	// Example: ${website-bucket.websiteEndpoint} maps the bucket's websiteEndpoint to a status field.
	// +optional
	Outputs map[string]pulumiv1.Expression `json:"outputs,omitempty"`

	// Config defines operator-level configuration values from ConfigMaps/Secrets.
	// These can be referenced in resources using ${config.<key>}.
	// +optional
	Config map[string]shared.ResourceRef `json:"config,omitempty"`

	// StackConfig specifies default configuration for generated Stack CRs.
	// +optional
	StackConfig *StackConfiguration `json:"stackConfig,omitempty"`

	// Lifecycle controls the behavior of created infrastructure.
	// +optional
	Lifecycle *LifecycleConfig `json:"lifecycle,omitempty"`

	// Packages specifies external packages to be installed before running the program.
	// +optional
	Packages map[string]string `json:"packages,omitempty"`
}

// CRDSpec specifies the generated CRD's identity.
type CRDSpec struct {
	// APIVersion is the API group and version for the generated CRD (e.g., "platform.example.com/v1").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/v[a-z0-9]+(alpha[0-9]+|beta[0-9]+)?$`
	APIVersion string `json:"apiVersion"`

	// Kind is the resource kind for the generated CRD (e.g., "Database").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Z][a-zA-Z0-9]*$`
	Kind string `json:"kind"`

	// Plural is the plural name for the resource (e.g., "databases").
	// If not specified, defaults to lowercase(kind) + "s".
	// Use this to specify correct pluralization for words that don't follow standard English rules
	// (e.g., "policies" for Policy, "indices" for Index, "statuses" for Status).
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9]*$`
	// +optional
	Plural string `json:"plural,omitempty"`

	// Scope determines whether instances are cluster-scoped or namespace-scoped.
	// Defaults to Namespaced.
	// +kubebuilder:validation:Enum=Namespaced;Cluster
	// +kubebuilder:default=Namespaced
	// +optional
	Scope CRDScope `json:"scope,omitempty"`

	// Categories are the categories this resource belongs to (e.g., ["pulumi", "infrastructure"]).
	// +optional
	Categories []string `json:"categories,omitempty"`

	// ShortNames are abbreviations for the resource kind (e.g., ["db"] for Database).
	// +optional
	ShortNames []string `json:"shortNames,omitempty"`

	// PrinterColumns defines additional columns to display in kubectl get output.
	// +optional
	PrinterColumns []PrinterColumn `json:"printerColumns,omitempty"`
}

// CRDScope determines the scope of the generated CRD.
// +kubebuilder:validation:Enum=Namespaced;Cluster
type CRDScope string

const (
	// CRDScopeNamespaced indicates the CRD is namespace-scoped.
	CRDScopeNamespaced CRDScope = "Namespaced"
	// CRDScopeCluster indicates the CRD is cluster-scoped.
	CRDScopeCluster CRDScope = "Cluster"
)

// PrinterColumn defines an additional column for kubectl output.
type PrinterColumn struct {
	// Name is the column header.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type is the data type (string, integer, date, boolean).
	// +kubebuilder:validation:Enum=string;integer;date;boolean
	// +kubebuilder:default=string
	Type string `json:"type"`

	// JSONPath is the path to the field to display.
	// +kubebuilder:validation:Required
	JSONPath string `json:"jsonPath"`

	// Priority determines when the column is shown (0 = always, higher = wider output).
	// +kubebuilder:default=0
	// +optional
	Priority int32 `json:"priority,omitempty"`
}

// TemplateSchema defines the schema for instances of the generated CRD.
type TemplateSchema struct {
	// Spec defines the spec fields using SimpleSchema format.
	// Each key is a field name, value is the field definition.
	// +kubebuilder:validation:Required
	Spec map[string]SchemaField `json:"spec"`

	// Status defines the status fields that will be populated from outputs.
	// +optional
	Status map[string]SchemaField `json:"status,omitempty"`
}

// SchemaField defines a single field in the schema using SimpleSchema format.
// +kubebuilder:validation:XPreserveUnknownFields
type SchemaField struct {
	// Type is the data type (string, integer, number, boolean, array, object).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=string;integer;number;boolean;array;object
	Type SchemaFieldType `json:"type"`

	// Description provides documentation for the field.
	// +optional
	Description string `json:"description,omitempty"`

	// Required indicates whether this field must be specified.
	// +kubebuilder:default=false
	// +optional
	Required bool `json:"required,omitempty"`

	// Default is the default value if not specified.
	// +optional
	Default *apiextensionsv1.JSON `json:"default,omitempty"`

	// Enum restricts the value to one of the specified options.
	// +optional
	Enum []string `json:"enum,omitempty"`

	// Minimum is the minimum value for integer/number types.
	// +optional
	Minimum *int64 `json:"minimum,omitempty"`

	// Maximum is the maximum value for integer/number types.
	// +optional
	Maximum *int64 `json:"maximum,omitempty"`

	// MinLength is the minimum length for string types.
	// +optional
	MinLength *int64 `json:"minLength,omitempty"`

	// MaxLength is the maximum length for string types.
	// +optional
	MaxLength *int64 `json:"maxLength,omitempty"`

	// Pattern is a regex pattern for string validation.
	// +optional
	Pattern string `json:"pattern,omitempty"`

	// Items defines the schema for array elements (required if Type is "array").
	// Use apiextensionsv1.JSON to avoid recursive type issues.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Items *apiextensionsv1.JSON `json:"items,omitempty"`

	// Properties defines nested fields for object types (required if Type is "object").
	// Use map[string]apiextensionsv1.JSON to avoid recursive type issues.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Properties map[string]apiextensionsv1.JSON `json:"properties,omitempty"`

	// AdditionalProperties allows arbitrary keys for object types.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	AdditionalProperties *apiextensionsv1.JSON `json:"additionalProperties,omitempty"`

	// Format provides additional validation (e.g., "date-time", "email", "uri").
	// +optional
	Format string `json:"format,omitempty"`

	// Immutable indicates the field cannot be changed after creation.
	// +kubebuilder:default=false
	// +optional
	Immutable bool `json:"immutable,omitempty"`

	// Sensitive indicates the field value should be treated as a secret.
	// +kubebuilder:default=false
	// +optional
	Sensitive bool `json:"sensitive,omitempty"`
}

// SchemaFieldType represents the data type of a schema field.
// +kubebuilder:validation:Enum=string;integer;number;boolean;array;object
type SchemaFieldType string

const (
	SchemaFieldTypeString  SchemaFieldType = "string"
	SchemaFieldTypeInteger SchemaFieldType = "integer"
	SchemaFieldTypeNumber  SchemaFieldType = "number"
	SchemaFieldTypeBoolean SchemaFieldType = "boolean"
	SchemaFieldTypeArray   SchemaFieldType = "array"
	SchemaFieldTypeObject  SchemaFieldType = "object"
)

// StackConfiguration specifies default settings for generated Stack CRs.
type StackConfiguration struct {
	// Organization is the Pulumi Cloud organization name used in stack names.
	// If not specified, a local backend should be used or this defaults to the namespace.
	// +optional
	Organization string `json:"organization,omitempty"`

	// Project is the Pulumi project name. Defaults to the template name.
	// +optional
	Project string `json:"project,omitempty"`

	// ServiceAccountName is the Kubernetes service account for the workspace.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Backend is the Pulumi state backend URL.
	// +optional
	Backend string `json:"backend,omitempty"`

	// SecretsProvider is the secrets encryption provider.
	// +optional
	SecretsProvider string `json:"secretsProvider,omitempty"`

	// EnvRefs defines environment variables for the Pulumi operation.
	// +optional
	EnvRefs map[string]shared.ResourceRef `json:"envRefs,omitempty"`

	// Envs is a list of ConfigMap names containing environment variables.
	// +optional
	Envs []string `json:"envs,omitempty"`

	// Environment specifies the Pulumi ESC environment(s) to use for stacks.
	// +optional
	Environment []string `json:"environment,omitempty"`

	// WorkspaceTemplate customizes the Workspace generated for instances.
	// +optional
	WorkspaceTemplate *shared.WorkspaceApplyConfiguration `json:"workspaceTemplate,omitempty"`

	// ResyncFrequencySeconds controls how often the stack is reconciled.
	// +optional
	ResyncFrequencySeconds *int64 `json:"resyncFrequencySeconds,omitempty"`

	// Prerequisites is a list of Template references that must be deployed first.
	// +optional
	Prerequisites []TemplatePrerequisite `json:"prerequisites,omitempty"`
}

// TemplatePrerequisite specifies a dependency on another Template.
type TemplatePrerequisite struct {
	// Name is the name of the Template that must be deployed first.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the prerequisite template (defaults to same namespace).
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// LifecycleConfig controls infrastructure lifecycle behavior.
type LifecycleConfig struct {
	// DestroyOnDelete determines whether infrastructure is destroyed when the instance is deleted.
	// +kubebuilder:default=true
	// +optional
	DestroyOnDelete bool `json:"destroyOnDelete,omitempty"`

	// RefreshBeforeUpdate performs a refresh before each update.
	// +kubebuilder:default=false
	// +optional
	RefreshBeforeUpdate bool `json:"refreshBeforeUpdate,omitempty"`

	// ContinueOnError allows updates to continue even if some resources fail.
	// +kubebuilder:default=false
	// +optional
	ContinueOnError bool `json:"continueOnError,omitempty"`

	// RetryOnUpdateConflict retries updates on HTTP 409 conflicts.
	// +kubebuilder:default=false
	// +optional
	RetryOnUpdateConflict bool `json:"retryOnUpdateConflict,omitempty"`

	// ProtectResources prevents accidental deletion of managed resources.
	// +kubebuilder:default=false
	// +optional
	ProtectResources bool `json:"protectResources,omitempty"`
}

// TemplateStatus defines the observed state of Template.
type TemplateStatus struct {
	// ObservedGeneration is the last observed generation of the Template.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the template's state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// CRD contains information about the generated CRD.
	// +optional
	CRD *GeneratedCRDStatus `json:"crd,omitempty"`

	// InstanceCount is the number of instances currently using this template.
	// +optional
	InstanceCount int32 `json:"instanceCount,omitempty"`

	// LastReconciled is the timestamp of the last successful reconciliation.
	// +optional
	LastReconciled *metav1.Time `json:"lastReconciled,omitempty"`
}

// GeneratedCRDStatus contains information about the generated CRD.
type GeneratedCRDStatus struct {
	// Name is the full name of the generated CRD (e.g., "databases.platform.example.com").
	// +optional
	Name string `json:"name,omitempty"`

	// Group is the API group of the generated CRD.
	// +optional
	Group string `json:"group,omitempty"`

	// Version is the API version of the generated CRD.
	// +optional
	Version string `json:"version,omitempty"`

	// Kind is the resource kind of the generated CRD.
	// +optional
	Kind string `json:"kind,omitempty"`

	// Plural is the plural name of the resource.
	// +optional
	Plural string `json:"plural,omitempty"`

	// Ready indicates whether the CRD has been successfully registered.
	// +optional
	Ready bool `json:"ready,omitempty"`
}

// Template condition types.
const (
	// TemplateConditionTypeReady indicates the template is ready and the CRD is registered.
	TemplateConditionTypeReady = "Ready"
	// TemplateConditionTypeCRDReady indicates the CRD has been generated and registered.
	TemplateConditionTypeCRDReady = "CRDReady"
	// TemplateConditionTypeSchemaValid indicates the schema has been validated.
	TemplateConditionTypeSchemaValid = "SchemaValid"
	// TemplateConditionTypeResourcesValid indicates all resource definitions are valid.
	TemplateConditionTypeResourcesValid = "ResourcesValid"
	// TemplateConditionTypeReconciling indicates the template is currently being reconciled.
	TemplateConditionTypeReconciling = "Reconciling"
)

// Template condition reasons.
const (
	TemplateReasonCRDRegistered      = "CRDRegistered"
	TemplateReasonCRDRegistering     = "CRDRegistering"
	TemplateReasonCRDFailed          = "CRDFailed"
	TemplateReasonSchemaValid        = "SchemaValid"
	TemplateReasonSchemaInvalid      = "SchemaInvalid"
	TemplateReasonResourcesValid     = "ResourcesValid"
	TemplateReasonResourcesInvalid   = "ResourcesInvalid"
	TemplateReasonReconciling        = "Reconciling"
	TemplateReasonReconcileSucceeded = "ReconcileSucceeded"
	TemplateReasonReconcileFailed    = "ReconcileFailed"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=pulumi,shortName=tpl
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".status.crd.name"
// +kubebuilder:printcolumn:name="Kind",type="string",JSONPath=".spec.crd.kind"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Instances",type="integer",JSONPath=".status.instanceCount"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Template is the schema for defining dynamic CRDs backed by Pulumi infrastructure.
// When a Template is created, the operator generates a new CRD and watches for instances
// of that CRD. Each instance triggers the creation of a Pulumi Stack that provisions
// the defined infrastructure.
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemplateSpec   `json:"spec,omitempty"`
	Status TemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TemplateList contains a list of Template
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Template{}, &TemplateList{})
}

// GetCondition returns the condition with the given type, or nil if not found.
func (t *TemplateStatus) GetCondition(conditionType string) *metav1.Condition {
	for i := range t.Conditions {
		if t.Conditions[i].Type == conditionType {
			return &t.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on the status.
func (t *TemplateStatus) SetCondition(condition metav1.Condition) {
	for i := range t.Conditions {
		if t.Conditions[i].Type == condition.Type {
			t.Conditions[i] = condition
			return
		}
	}
	t.Conditions = append(t.Conditions, condition)
}
