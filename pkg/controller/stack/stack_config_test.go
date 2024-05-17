package stack

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestNewStructuredConfigFromJson(t *testing.T) {
	toJson := func(v any) apiextensionsv1.JSON {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
			return apiextensionsv1.JSON{}
		}
		return apiextensionsv1.JSON{Raw: b}
	}

	check := func(sourceConfigMap map[string]any, expected []ConfigKeyValue) {
		structuredConfig, err := NewStructuredConfigFromJSON("", toJson(sourceConfigMap))
		if assert.NoError(t, err) {
			configValues := structuredConfig.Flatten()
			assert.ElementsMatch(t, expected, configValues)
		}
	}

	t.Run("We should be able to handle simple key-value pairs", func(t *testing.T) {
		sourceConfigMap := map[string]any{
			"aws:region": "us-east-1",
		}
		expected := []ConfigKeyValue{
			{
				Key: "aws:region",
				Value: auto.ConfigValue{
					Value:  "us-east-1",
					Secret: false,
				},
			},
		}
		check(sourceConfigMap, expected)
	})
	t.Run("We should be able to handle a structured, complex (namespaced) config", func(t *testing.T) {
		sourceConfigMap := map[string]any{
			"aws:assumeRole": map[string]any{
				"roleArn":     "my-role-arn",
				"sessionName": "my-session-name",
			},
			"aws:defaultTags": map[string]any{
				"tags": map[string]any{
					"my-tag": "tag-value",
				},
			},
		}
		expected := []ConfigKeyValue{
			{
				Key: "aws:assumeRole.roleArn",
				Value: auto.ConfigValue{
					Value:  "my-role-arn",
					Secret: false,
				},
			},
			{
				Key: "aws:assumeRole.sessionName",
				Value: auto.ConfigValue{
					Value:  "my-session-name",
					Secret: false,
				},
			},
			{
				Key: "aws:defaultTags.tags.my-tag",
				Value: auto.ConfigValue{
					Value:  "tag-value",
					Secret: false,
				},
			},
		}
		check(sourceConfigMap, expected)
	})

	t.Run("We should be able to handle a structured, complex (non-namespaced) config", func(t *testing.T) {
		sourceConfigMap := map[string]any{
			"an-object-config": map[string]any{
				"another-config": map[string]any{
					"config-key": "value",
				},
				"a-nested-list-config": []any{"one", "two", "three"},
			},
		}
		expected := []ConfigKeyValue{
			{
				Key: "an-object-config.a-nested-list-config[0]",
				Value: auto.ConfigValue{
					Value:  "one",
					Secret: false,
				},
			},
			{
				Key: "an-object-config.a-nested-list-config[1]",
				Value: auto.ConfigValue{
					Value:  "two",
					Secret: false,
				},
			},
			{
				Key: "an-object-config.a-nested-list-config[2]",
				Value: auto.ConfigValue{
					Value:  "three",
					Secret: false,
				},
			},
			{
				Key: "an-object-config.another-config.config-key",
				Value: auto.ConfigValue{
					Value:  "value",
					Secret: false,
				},
			},
		}
		check(sourceConfigMap, expected)
	})

	t.Run("We should be able to handle simple, non-string config values", func(t *testing.T) {
		sourceConfigMap := map[string]any{
			"a-list-config":     []any{"a", "b", "c"},
			"a-simple-config":   "just-a-simple-value",
			"a-boolean-config":  true,
			"an-integer-config": 123456,
		}
		expected := []ConfigKeyValue{
			{
				Key: "a-boolean-config",
				Value: auto.ConfigValue{
					Value:  "true",
					Secret: false,
				},
			},
			{
				Key: "a-list-config[0]",
				Value: auto.ConfigValue{
					Value:  "a",
					Secret: false,
				},
			},
			{
				Key: "a-list-config[1]",
				Value: auto.ConfigValue{
					Value:  "b",
					Secret: false,
				},
			},
			{
				Key: "a-list-config[2]",
				Value: auto.ConfigValue{
					Value:  "c",
					Secret: false,
				},
			},
			{
				Key: "a-simple-config",
				Value: auto.ConfigValue{
					Value:  "just-a-simple-value",
					Secret: false,
				},
			},
			{
				Key: "an-integer-config",
				Value: auto.ConfigValue{
					Value:  "123456",
					Secret: false,
				},
			},
		}
		check(sourceConfigMap, expected)
	})
	t.Run("We should be able to handle a structured, complex (non-namespaced) config", func(t *testing.T) {
		sourceConfigMap := map[string]any{
			"an-object-config": map[string]any{
				"another-config": map[string]any{
					"config-key": "value",
				},
				"a-nested-list-config": []any{"one", "two", "three"},
			},
		}
		expected := []ConfigKeyValue{
			{
				Key: "an-object-config.a-nested-list-config[0]",
				Value: auto.ConfigValue{
					Value:  "one",
					Secret: false,
				},
			},
			{
				Key: "an-object-config.a-nested-list-config[1]",
				Value: auto.ConfigValue{
					Value:  "two",
					Secret: false,
				},
			},
			{
				Key: "an-object-config.a-nested-list-config[2]",
				Value: auto.ConfigValue{
					Value:  "three",
					Secret: false,
				},
			},
			{
				Key: "an-object-config.another-config.config-key",
				Value: auto.ConfigValue{
					Value:  "value",
					Secret: false,
				},
			},
		}
		check(sourceConfigMap, expected)
	})
}
