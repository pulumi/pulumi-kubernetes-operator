package stack

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type StructuredConfig map[string]any

type ConfigKeyValue struct {
	Key   string
	Value auto.ConfigValue
}

func NewStructuredConfigFromJSON(key string, rawValue apiextensionsv1.JSON) (*StructuredConfig, error) {
	var data any
	if err := json.Unmarshal(rawValue.Raw, &data); err != nil {
		return nil, err
	}
	if dataAsMap, err := data.(map[string]any); err {
		structuredConfig := StructuredConfig(dataAsMap)
		return &structuredConfig, nil
	}
	structuredConfig := StructuredConfig(map[string]any{
		key: data,
	})
	return &structuredConfig, nil
}

func (c StructuredConfig) Flatten() []ConfigKeyValue {
	flatten := flattenKeys(c)

	configValues := make([]ConfigKeyValue, 0, len(flatten))
	for key, value := range flatten {
		configValues = append(configValues, ConfigKeyValue{
			Key: key,
			Value: auto.ConfigValue{
				Value: fmt.Sprint(value),
			},
		})
	}

	return configValues
}

func flattenKeys(config StructuredConfig) map[string]any {
	output := make(map[string]any)

	for k, v := range config {
		flatten(output, v, k)
	}

	return output
}

func flatten(flatMap map[string]any, nested any, prefix string) {
	assign := func(newKey string, v any) {
		switch v.(type) {
		case map[string]any, []any:
			flatten(flatMap, v, newKey)
		default:
			flatMap[newKey] = v
		}
	}

	switch nested.(type) {
	case map[string]any:
		for k, v := range nested.(map[string]any) {
			newKey := enkey(prefix, k)
			assign(newKey, v)
		}
	case []any:
		for i, v := range nested.([]any) {
			newKey := indexedKey(prefix, strconv.Itoa(i))
			assign(newKey, v)
		}
	default:
		assign(prefix, nested)
	}
}

func enkey(prefix, subkey string) string {
	return fmt.Sprintf("%s.%s", prefix, subkey)
}

func indexedKey(prefix, subkey string) string {
	return fmt.Sprintf("%s[%s]", prefix, subkey)
}
