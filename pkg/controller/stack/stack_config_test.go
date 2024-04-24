package stack

import (
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestFlattenStackConfigFromJson(t *testing.T) {
	toJson := func(v any) apiextensionsv1.JSON {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
			return apiextensionsv1.JSON{}
		}
		return apiextensionsv1.JSON{Raw: b}
	}

	sourceConfigMap := map[string]apiextensionsv1.JSON{
		"aws:region": toJson("us-east-1"),
		"aws:assumeRole": toJson(map[string]any{
			"roleArn":     "my-role-arn",
			"sessionName": "my-session-name",
		}),
		"aws:defaultTags": toJson(map[string]any{
			"tags": map[string]any{
				"my-tag": "tag-value",
			},
		}),
		"an-object-config": toJson(map[string]any{
			"another-config": map[string]any{
				"config-key": "value",
			},
			"a-nested-list-config": []any{"one", "two", "three"},
		}),
		"a-list-config":     toJson([]any{"a", "b", "c"}),
		"a-simple-config":   toJson("just-a-simple-value"),
		"a-boolean-config":  toJson(true),
		"an-integer-config": toJson(123456),
	}

	configValues, err := StructuredConfig(sourceConfigMap).Unmarshal()

	if assert.Nil(t, err) {
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
			{
				Key: "aws:region",
				Value: auto.ConfigValue{
					Value:  "us-east-1",
					Secret: false,
				},
			},
		}

		assert.Equal(t, expected, configValues)
	}
}
