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

package controller

import (
	"encoding/json"
	"testing"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
)

func toJSON(v any) *apiextensionsv1.JSON {
	if v == nil {
		return &apiextensionsv1.JSON{Raw: nil}
	}
	raw, _ := json.Marshal(v)
	return &apiextensionsv1.JSON{Raw: raw}
}

func TestMarshalConfigItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		item    autov1alpha1.ConfigItem
		want    *agentpb.ConfigItem
		wantErr bool
	}{
		{
			name: "string value",
			item: autov1alpha1.ConfigItem{
				Key:   "foo",
				Value: toJSON("bar"),
			},
			want: &agentpb.ConfigItem{
				Key: "foo",
				V:   &agentpb.ConfigItem_Value{Value: structpb.NewStringValue("bar")},
			},
		},
		{
			name: "number value",
			item: autov1alpha1.ConfigItem{
				Key:   "count",
				Value: toJSON(42),
			},
			want: &agentpb.ConfigItem{
				Key: "count",
				V:   &agentpb.ConfigItem_Value{Value: structpb.NewNumberValue(42)},
			},
		},
		{
			name: "boolean value",
			item: autov1alpha1.ConfigItem{
				Key:   "enabled",
				Value: toJSON(true),
			},
			want: &agentpb.ConfigItem{
				Key: "enabled",
				V:   &agentpb.ConfigItem_Value{Value: structpb.NewBoolValue(true)},
			},
		},
		{
			name: "object value",
			item: autov1alpha1.ConfigItem{
				Key: "database",
				Value: toJSON(map[string]interface{}{
					"host": "localhost",
					"port": 5432,
				}),
			},
			want: func() *agentpb.ConfigItem {
				val, _ := structpb.NewValue(map[string]interface{}{
					"host": "localhost",
					"port": float64(5432),
				})
				return &agentpb.ConfigItem{
					Key: "database",
					V:   &agentpb.ConfigItem_Value{Value: val},
				}
			}(),
		},
		{
			name: "array value",
			item: autov1alpha1.ConfigItem{
				Key:   "regions",
				Value: toJSON([]string{"us-west-2", "us-east-1"}),
			},
			want: func() *agentpb.ConfigItem {
				val, _ := structpb.NewValue([]interface{}{"us-west-2", "us-east-1"})
				return &agentpb.ConfigItem{
					Key: "regions",
					V:   &agentpb.ConfigItem_Value{Value: val},
				}
			}(),
		},
		{
			name: "secret value",
			item: autov1alpha1.ConfigItem{
				Key:    "apiKey",
				Value:  toJSON("secret123"),
				Secret: ptr.To(true),
			},
			want: &agentpb.ConfigItem{
				Key:    "apiKey",
				V:      &agentpb.ConfigItem_Value{Value: structpb.NewStringValue("secret123")},
				Secret: ptr.To(true),
			},
		},
		{
			name: "path=true with string value",
			item: autov1alpha1.ConfigItem{
				Key:   "foo.bar",
				Path:  ptr.To(true),
				Value: toJSON("baz"),
			},
			want: &agentpb.ConfigItem{
				Key:  "foo.bar",
				Path: ptr.To(true),
				V:    &agentpb.ConfigItem_Value{Value: structpb.NewStringValue("baz")},
			},
		},
		{
			name: "path=true with object value should error",
			item: autov1alpha1.ConfigItem{
				Key:  "database",
				Path: ptr.To(true),
				Value: toJSON(map[string]string{
					"host": "localhost",
				}),
			},
			wantErr: true,
		},
		{
			name: "path=true with array value should error",
			item: autov1alpha1.ConfigItem{
				Key:   "regions",
				Path:  ptr.To(true),
				Value: toJSON([]string{"us-west-2"}),
			},
			wantErr: true,
		},
		{
			name: "path=true with number value should error",
			item: autov1alpha1.ConfigItem{
				Key:   "count",
				Path:  ptr.To(true),
				Value: toJSON(42),
			},
			wantErr: true,
		},
		{
			name: "path=true with boolean value should error",
			item: autov1alpha1.ConfigItem{
				Key:   "enabled",
				Path:  ptr.To(true),
				Value: toJSON(true),
			},
			wantErr: true,
		},
		{
			name: "valueFrom env",
			item: autov1alpha1.ConfigItem{
				Key: "foo",
				ValueFrom: &autov1alpha1.ConfigValueFrom{
					Env: "MY_VAR",
				},
			},
			want: &agentpb.ConfigItem{
				Key: "foo",
				V: &agentpb.ConfigItem_ValueFrom{
					ValueFrom: &agentpb.ConfigValueFrom{
						F: &agentpb.ConfigValueFrom_Env{Env: "MY_VAR"},
					},
				},
			},
		},
		{
			name: "valueFrom path",
			item: autov1alpha1.ConfigItem{
				Key: "foo",
				ValueFrom: &autov1alpha1.ConfigValueFrom{
					Path: "/etc/config/value",
				},
			},
			want: &agentpb.ConfigItem{
				Key: "foo",
				V: &agentpb.ConfigItem_ValueFrom{
					ValueFrom: &agentpb.ConfigValueFrom{
						F: &agentpb.ConfigValueFrom_Path{Path: "/etc/config/value"},
					},
				},
			},
		},
		{
			name: "valueFrom with json flag",
			item: autov1alpha1.ConfigItem{
				Key: "settings",
				ValueFrom: &autov1alpha1.ConfigValueFrom{
					Env:  "JSON_CONFIG",
					JSON: ptr.To(true),
				},
			},
			want: &agentpb.ConfigItem{
				Key: "settings",
				V: &agentpb.ConfigItem_ValueFrom{
					ValueFrom: &agentpb.ConfigValueFrom{
						F:    &agentpb.ConfigValueFrom_Env{Env: "JSON_CONFIG"},
						Json: ptr.To(true),
					},
				},
			},
		},
		{
			name: "invalid json",
			item: autov1alpha1.ConfigItem{
				Key:   "foo",
				Value: &apiextensionsv1.JSON{Raw: []byte(`{invalid}`)},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := marshalConfigItem(tt.item)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Compare the config items
			assert.Equal(t, tt.want.Key, got.Key)
			assert.Equal(t, tt.want.Path, got.Path)
			assert.Equal(t, tt.want.Secret, got.Secret)

			// Compare values by marshaling to JSON for easier comparison
			if tt.want.V != nil && got.V != nil {
				wantJSON, err := json.Marshal(tt.want.V)
				require.NoError(t, err)
				gotJSON, err := json.Marshal(got.V)
				require.NoError(t, err)
				assert.JSONEq(t, string(wantJSON), string(gotJSON))
			} else {
				assert.Equal(t, tt.want.V, got.V)
			}
		})
	}
}
