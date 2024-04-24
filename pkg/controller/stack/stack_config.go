package stack

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type StructuredConfig map[string]apiextensionsv1.JSON

type ConfigKeyValue struct {
	Key   string
	Value auto.ConfigValue
}

func (c StructuredConfig) Unmarshal() ([]ConfigKeyValue, error) {
	flatten, err := flattenKeys(c)
	if err != nil {
		return nil, err
	}

	configValues := make([]ConfigKeyValue, 0, len(flatten))
	for key, value := range flatten {
		configValues = append(configValues, ConfigKeyValue{
			Key: key,
			Value: auto.ConfigValue{
				Value: fmt.Sprint(value),
			},
		})
	}

	return configValues, nil
}

func flattenKeys(config StructuredConfig) (map[string]any, error) {
	output := make(map[string]any)

	for k, jsonValue := range config {
		var d any
		if err := json.Unmarshal(jsonValue.Raw, &d); err != nil {
			return nil, err
		}

		err := flatten(output, d, k)
		if err != nil {
			return nil, err
		}
	}

	return output, nil
}

func flatten(flatMap map[string]any, nested any, prefix string) error {
	assign := func(newKey string, v any) error {
		switch v.(type) {
		case map[string]any, []any:
			if err := flatten(flatMap, v, newKey); err != nil {
				return err
			}
		default:
			flatMap[newKey] = v
		}

		return nil
	}

	switch nested.(type) {
	case map[string]any:
		for k, v := range nested.(map[string]any) {
			newKey := enkey(prefix, k)
			if err := assign(newKey, v); err != nil {
				return err
			}
		}
	case []any:
		for i, v := range nested.([]any) {
			newKey := indexedKey(prefix, strconv.Itoa(i))
			if err := assign(newKey, v); err != nil {
				return err
			}
		}
	default:
		return assign(prefix, nested)
	}

	return nil
}

func enkey(prefix, subkey string) string {
	return fmt.Sprintf("%s.%s", prefix, subkey)
}

func indexedKey(prefix, subkey string) string {
	return fmt.Sprintf("%s[%s]", prefix, subkey)
}
