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

package pulumi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1alpha1"
)

func TestParseAPIVersion(t *testing.T) {
	tests := []struct {
		name          string
		apiVersion    string
		expectedGroup string
		expectedVer   string
	}{
		{
			name:          "standard api version",
			apiVersion:    "platform.example.com/v1",
			expectedGroup: "platform.example.com",
			expectedVer:   "v1",
		},
		{
			name:          "alpha version",
			apiVersion:    "secrets.example.com/v1alpha1",
			expectedGroup: "secrets.example.com",
			expectedVer:   "v1alpha1",
		},
		{
			name:          "beta version",
			apiVersion:    "database.io/v1beta2",
			expectedGroup: "database.io",
			expectedVer:   "v1beta2",
		},
		{
			name:          "core api version",
			apiVersion:    "v1",
			expectedGroup: "",
			expectedVer:   "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, version := parseAPIVersion(tt.apiVersion)
			assert.Equal(t, tt.expectedGroup, group)
			assert.Equal(t, tt.expectedVer, version)
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	r := &TemplateReconciler{}

	tests := []struct {
		name        string
		template    *pulumiv1alpha1.Template
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid template",
			template: &pulumiv1alpha1.Template{
				Spec: pulumiv1alpha1.TemplateSpec{
					CRD: pulumiv1alpha1.CRDSpec{
						APIVersion: "platform.example.com/v1",
						Kind:       "Database",
					},
					Schema: pulumiv1alpha1.TemplateSchema{
						Spec: map[string]pulumiv1alpha1.SchemaField{
							"name": {
								Type:     pulumiv1alpha1.SchemaFieldTypeString,
								Required: true,
							},
						},
					},
					Resources: map[string]pulumiv1.Resource{
						"db": {
							Type: "aws:rds:Instance",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing api version",
			template: &pulumiv1alpha1.Template{
				Spec: pulumiv1alpha1.TemplateSpec{
					CRD: pulumiv1alpha1.CRDSpec{
						Kind: "Database",
					},
					Schema: pulumiv1alpha1.TemplateSchema{
						Spec: map[string]pulumiv1alpha1.SchemaField{
							"name": {Type: pulumiv1alpha1.SchemaFieldTypeString},
						},
					},
					Resources: map[string]pulumiv1.Resource{
						"db": {Type: "aws:rds:Instance"},
					},
				},
			},
			expectError: true,
			errorMsg:    "crd.apiVersion is required",
		},
		{
			name: "missing kind",
			template: &pulumiv1alpha1.Template{
				Spec: pulumiv1alpha1.TemplateSpec{
					CRD: pulumiv1alpha1.CRDSpec{
						APIVersion: "platform.example.com/v1",
					},
					Schema: pulumiv1alpha1.TemplateSchema{
						Spec: map[string]pulumiv1alpha1.SchemaField{
							"name": {Type: pulumiv1alpha1.SchemaFieldTypeString},
						},
					},
					Resources: map[string]pulumiv1.Resource{
						"db": {Type: "aws:rds:Instance"},
					},
				},
			},
			expectError: true,
			errorMsg:    "crd.kind is required",
		},
		{
			name: "empty schema",
			template: &pulumiv1alpha1.Template{
				Spec: pulumiv1alpha1.TemplateSpec{
					CRD: pulumiv1alpha1.CRDSpec{
						APIVersion: "platform.example.com/v1",
						Kind:       "Database",
					},
					Schema: pulumiv1alpha1.TemplateSchema{
						Spec: map[string]pulumiv1alpha1.SchemaField{},
					},
					Resources: map[string]pulumiv1.Resource{
						"db": {Type: "aws:rds:Instance"},
					},
				},
			},
			expectError: true,
			errorMsg:    "schema.spec must have at least one field",
		},
		{
			name: "no resources",
			template: &pulumiv1alpha1.Template{
				Spec: pulumiv1alpha1.TemplateSpec{
					CRD: pulumiv1alpha1.CRDSpec{
						APIVersion: "platform.example.com/v1",
						Kind:       "Database",
					},
					Schema: pulumiv1alpha1.TemplateSchema{
						Spec: map[string]pulumiv1alpha1.SchemaField{
							"name": {Type: pulumiv1alpha1.SchemaFieldTypeString},
						},
					},
					Resources: map[string]pulumiv1.Resource{},
				},
			},
			expectError: true,
			errorMsg:    "at least one resource must be defined",
		},
		{
			name: "resource without type",
			template: &pulumiv1alpha1.Template{
				Spec: pulumiv1alpha1.TemplateSpec{
					CRD: pulumiv1alpha1.CRDSpec{
						APIVersion: "platform.example.com/v1",
						Kind:       "Database",
					},
					Schema: pulumiv1alpha1.TemplateSchema{
						Spec: map[string]pulumiv1alpha1.SchemaField{
							"name": {Type: pulumiv1alpha1.SchemaFieldTypeString},
						},
					},
					Resources: map[string]pulumiv1.Resource{
						"db": {},
					},
				},
			},
			expectError: true,
			errorMsg:    "resource \"db\" must have a type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.validateTemplate(tt.template)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerateCRD(t *testing.T) {
	r := &TemplateReconciler{}

	template := &pulumiv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "default",
		},
		Spec: pulumiv1alpha1.TemplateSpec{
			CRD: pulumiv1alpha1.CRDSpec{
				APIVersion: "platform.example.com/v1",
				Kind:       "Database",
				Scope:      pulumiv1alpha1.CRDScopeNamespaced,
				Categories: []string{"databases"},
				ShortNames: []string{"db"},
				PrinterColumns: []pulumiv1alpha1.PrinterColumn{
					{
						Name:     "Engine",
						Type:     "string",
						JSONPath: ".spec.engine",
					},
				},
			},
			Schema: pulumiv1alpha1.TemplateSchema{
				Spec: map[string]pulumiv1alpha1.SchemaField{
					"name": {
						Type:        pulumiv1alpha1.SchemaFieldTypeString,
						Description: "Database name",
						Required:    true,
					},
					"engine": {
						Type: pulumiv1alpha1.SchemaFieldTypeString,
						Enum: []string{"postgres", "mysql"},
					},
					"size": {
						Type:    pulumiv1alpha1.SchemaFieldTypeInteger,
						Minimum: ptrInt64(10),
						Maximum: ptrInt64(1000),
					},
				},
				Status: map[string]pulumiv1alpha1.SchemaField{
					"endpoint": {
						Type:        pulumiv1alpha1.SchemaFieldTypeString,
						Description: "Database endpoint",
					},
				},
			},
			Resources: map[string]pulumiv1.Resource{
				"db": {Type: "aws:rds:Instance"},
			},
		},
	}

	crd, err := r.generateCRD(template)
	require.NoError(t, err)
	require.NotNil(t, crd)

	// Check CRD metadata
	assert.Equal(t, "databases.platform.example.com", crd.Name)
	assert.Equal(t, GeneratedByValue, crd.Labels[GeneratedByLabel])
	assert.Equal(t, "test-template", crd.Labels[TemplateNameLabel])
	assert.Equal(t, "default", crd.Labels[TemplateNamespaceLabel])

	// Check CRD spec
	assert.Equal(t, "platform.example.com", crd.Spec.Group)
	assert.Equal(t, "Database", crd.Spec.Names.Kind)
	assert.Equal(t, "databases", crd.Spec.Names.Plural)
	assert.Equal(t, "database", crd.Spec.Names.Singular)
	assert.Contains(t, crd.Spec.Names.ShortNames, "db")
	assert.Contains(t, crd.Spec.Names.Categories, "pulumi")
	assert.Contains(t, crd.Spec.Names.Categories, "databases")
	assert.Equal(t, apiextensionsv1.NamespaceScoped, crd.Spec.Scope)

	// Check version
	require.Len(t, crd.Spec.Versions, 1)
	version := crd.Spec.Versions[0]
	assert.Equal(t, "v1", version.Name)
	assert.True(t, version.Served)
	assert.True(t, version.Storage)

	// Check schema
	require.NotNil(t, version.Schema)
	require.NotNil(t, version.Schema.OpenAPIV3Schema)

	schema := version.Schema.OpenAPIV3Schema
	require.Contains(t, schema.Properties, "spec")
	require.Contains(t, schema.Properties, "status")

	specSchema := schema.Properties["spec"]
	require.Contains(t, specSchema.Properties, "name")
	require.Contains(t, specSchema.Properties, "engine")
	require.Contains(t, specSchema.Properties, "size")
	assert.Equal(t, "Database name", specSchema.Properties["name"].Description)
	assert.Contains(t, specSchema.Required, "name")

	// Check status has standard fields
	statusSchema := schema.Properties["status"]
	require.Contains(t, statusSchema.Properties, "endpoint")
	require.Contains(t, statusSchema.Properties, "conditions")
	require.Contains(t, statusSchema.Properties, "stackRef")
	require.Contains(t, statusSchema.Properties, "lastUpdate")

	// Check subresources
	require.NotNil(t, version.Subresources)
	require.NotNil(t, version.Subresources.Status)

	// Check printer columns
	require.True(t, len(version.AdditionalPrinterColumns) >= 3)
	// First two are Ready and Age
	assert.Equal(t, "Ready", version.AdditionalPrinterColumns[0].Name)
	assert.Equal(t, "Age", version.AdditionalPrinterColumns[1].Name)
	assert.Equal(t, "Engine", version.AdditionalPrinterColumns[2].Name)
}

func TestSchemaFieldToJSONSchemaProps(t *testing.T) {
	r := &TemplateReconciler{}

	tests := []struct {
		name     string
		field    pulumiv1alpha1.SchemaField
		validate func(t *testing.T, props apiextensionsv1.JSONSchemaProps)
	}{
		{
			name: "string field",
			field: pulumiv1alpha1.SchemaField{
				Type:        pulumiv1alpha1.SchemaFieldTypeString,
				Description: "A string field",
				MinLength:   ptrInt64(1),
				MaxLength:   ptrInt64(100),
				Pattern:     "^[a-z]+$",
			},
			validate: func(t *testing.T, props apiextensionsv1.JSONSchemaProps) {
				assert.Equal(t, "string", props.Type)
				assert.Equal(t, "A string field", props.Description)
				assert.Equal(t, int64(1), *props.MinLength)
				assert.Equal(t, int64(100), *props.MaxLength)
				assert.Equal(t, "^[a-z]+$", props.Pattern)
			},
		},
		{
			name: "integer field with constraints",
			field: pulumiv1alpha1.SchemaField{
				Type:    pulumiv1alpha1.SchemaFieldTypeInteger,
				Minimum: ptrInt64(10),
				Maximum: ptrInt64(100),
			},
			validate: func(t *testing.T, props apiextensionsv1.JSONSchemaProps) {
				assert.Equal(t, "integer", props.Type)
				assert.Equal(t, float64(10), *props.Minimum)
				assert.Equal(t, float64(100), *props.Maximum)
			},
		},
		{
			name: "enum field",
			field: pulumiv1alpha1.SchemaField{
				Type: pulumiv1alpha1.SchemaFieldTypeString,
				Enum: []string{"small", "medium", "large"},
			},
			validate: func(t *testing.T, props apiextensionsv1.JSONSchemaProps) {
				assert.Equal(t, "string", props.Type)
				require.Len(t, props.Enum, 3)
			},
		},
		{
			name: "array field",
			field: pulumiv1alpha1.SchemaField{
				Type: pulumiv1alpha1.SchemaFieldTypeArray,
			},
			validate: func(t *testing.T, props apiextensionsv1.JSONSchemaProps) {
				assert.Equal(t, "array", props.Type)
				require.NotNil(t, props.Items)
				require.NotNil(t, props.Items.Schema)
				// Items use x-preserve-unknown-fields for flexibility
				assert.True(t, *props.Items.Schema.XPreserveUnknownFields)
			},
		},
		{
			name: "object field",
			field: pulumiv1alpha1.SchemaField{
				Type: pulumiv1alpha1.SchemaFieldTypeObject,
			},
			validate: func(t *testing.T, props apiextensionsv1.JSONSchemaProps) {
				assert.Equal(t, "object", props.Type)
				// Object uses x-preserve-unknown-fields for flexibility
				require.NotNil(t, props.XPreserveUnknownFields)
				assert.True(t, *props.XPreserveUnknownFields)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := r.schemaFieldToJSONSchemaProps(tt.field)
			tt.validate(t, props)
		})
	}
}

func TestSetCondition(t *testing.T) {
	r := &TemplateReconciler{}

	template := &pulumiv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
	}

	// Set initial condition
	r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeReady, metav1.ConditionFalse, "Testing", "Initial condition")

	require.Len(t, template.Status.Conditions, 1)
	assert.Equal(t, pulumiv1alpha1.TemplateConditionTypeReady, template.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, template.Status.Conditions[0].Status)
	assert.Equal(t, "Testing", template.Status.Conditions[0].Reason)
	assert.Equal(t, "Initial condition", template.Status.Conditions[0].Message)
	assert.Equal(t, int64(1), template.Status.Conditions[0].ObservedGeneration)

	// Update existing condition
	r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeReady, metav1.ConditionTrue, "Ready", "Template is ready")

	require.Len(t, template.Status.Conditions, 1)
	assert.Equal(t, metav1.ConditionTrue, template.Status.Conditions[0].Status)
	assert.Equal(t, "Ready", template.Status.Conditions[0].Reason)

	// Add a different condition
	r.setCondition(template, pulumiv1alpha1.TemplateConditionTypeCRDReady, metav1.ConditionTrue, "CRDReady", "CRD registered")

	require.Len(t, template.Status.Conditions, 2)
}

func TestRenderExpression(t *testing.T) {
	r := &TemplateReconciler{}

	instance := &unstructured.Unstructured{}
	instance.SetName("test-instance")
	instance.SetNamespace("test-ns")

	instanceSpec := map[string]interface{}{
		"name":    "my-db",
		"engine":  "postgres",
		"length":  16,
		"special": true,
	}

	tests := []struct {
		name     string
		expr     pulumiv1.Expression
		expected string
	}{
		{
			name:     "schema.spec string reference replaced with actual value",
			expr:     pulumiv1.Expression{Raw: []byte(`"${schema.spec.name}"`)},
			expected: `"my-db"`,
		},
		{
			name:     "schema.spec integer reference replaced with actual value",
			expr:     pulumiv1.Expression{Raw: []byte(`"${schema.spec.length}"`)},
			expected: `16`,
		},
		{
			name:     "schema.spec boolean reference replaced with actual value",
			expr:     pulumiv1.Expression{Raw: []byte(`"${schema.spec.special}"`)},
			expected: `true`,
		},
		{
			name:     "schema.metadata.name replaced with instance name",
			expr:     pulumiv1.Expression{Raw: []byte(`"${schema.metadata.name}-suffix"`)},
			expected: `"test-instance-suffix"`,
		},
		{
			name:     "schema.metadata.namespace replaced with instance namespace",
			expr:     pulumiv1.Expression{Raw: []byte(`"ns: ${schema.metadata.namespace}"`)},
			expected: `"ns: test-ns"`,
		},
		{
			name:     "combined expression with string interpolation",
			expr:     pulumiv1.Expression{Raw: []byte(`"${schema.spec.engine}-${schema.metadata.name}"`)},
			expected: `"postgres-test-instance"`,
		},
		{
			name:     "no schema reference unchanged (resource reference)",
			expr:     pulumiv1.Expression{Raw: []byte(`"${database.address}"`)},
			expected: `"${database.address}"`,
		},
		{
			name:     "pulumi intrinsic function unchanged",
			expr:     pulumiv1.Expression{Raw: []byte(`"${password.result}"`)},
			expected: `"${password.result}"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.renderExpression(tt.expr, instance, instanceSpec)
			assert.Equal(t, tt.expected, string(result.Raw))
		})
	}
}

func TestValueToExpressionString(t *testing.T) {
	r := &TemplateReconciler{}

	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{
			name:     "string value",
			value:    "hello",
			expected: `"hello"`,
		},
		{
			name:     "integer value",
			value:    42,
			expected: "42",
		},
		{
			name:     "float value",
			value:    3.14,
			expected: "3.14",
		},
		{
			name:     "boolean true",
			value:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			value:    false,
			expected: "false",
		},
		{
			name:     "string slice",
			value:    []string{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
		{
			name:     "map",
			value:    map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.valueToExpressionString(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
