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
	"fmt"
	"reflect"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func marshalConfigItem(item autov1alpha1.ConfigItem) (*agentpb.ConfigItem, error) {
	v := &agentpb.ConfigItem{
		Key:    item.Key,
		Path:   item.Path,
		Secret: item.Secret,
	}
	if item.Value != nil {
		// Convert apiextensionsv1.JSON to structpb.Value
		var val interface{}
		if err := json.Unmarshal(item.Value.Raw, &val); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config value: %w", err)
		}
		pbValue, err := structpb.NewValue(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert config value to protobuf: %w", err)
		}
		v.V = &agentpb.ConfigItem_Value{
			Value: pbValue,
		}
	}
	if item.ValueFrom != nil {
		f := &agentpb.ConfigValueFrom{
			Json: item.ValueFrom.JSON,
		}
		if item.ValueFrom.Env != "" {
			f.F = &agentpb.ConfigValueFrom_Env{
				Env: item.ValueFrom.Env,
			}
		} else if item.ValueFrom.Path != "" {
			f.F = &agentpb.ConfigValueFrom_Path{
				Path: item.ValueFrom.Path,
			}
		}
		v.V = &agentpb.ConfigItem_ValueFrom{
			ValueFrom: f,
		}
	}
	return v, nil
}

var l = log.Log.WithName("predicate").WithName("debug")

type DebugPredicate struct {
	Controller string
}

var _ predicate.Predicate = &DebugPredicate{}

func (p *DebugPredicate) Create(e event.CreateEvent) bool {
	l.V(1).Info("Create", "controller", p.Controller, "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName(), "revision", e.Object.GetResourceVersion())
	return true
}

func (p *DebugPredicate) Delete(e event.DeleteEvent) bool {
	l.V(1).Info("Delete", "controller", p.Controller, "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName(), "revision", e.Object.GetResourceVersion())
	return true
}

func (p *DebugPredicate) Update(e event.UpdateEvent) bool {
	l.V(1).Info("Update", "controller", p.Controller, "type", fmt.Sprintf("%T", e.ObjectOld), "name", e.ObjectOld.GetName(), "old-revision", e.ObjectOld.GetResourceVersion(), "new-revision", e.ObjectNew.GetResourceVersion())
	return true
}

func (p *DebugPredicate) Generic(e event.GenericEvent) bool {
	l.V(1).Info("Generic", "controller", p.Controller, "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName(), "revision", e.Object.GetResourceVersion())
	return true
}

type OwnerReferencesChangedPredicate struct{}

var _ predicate.Predicate = &OwnerReferencesChangedPredicate{}

func (OwnerReferencesChangedPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (OwnerReferencesChangedPredicate) Delete(_ event.DeleteEvent) bool {
	return false
}

func (OwnerReferencesChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return !reflect.DeepEqual(e.ObjectOld.GetOwnerReferences(), e.ObjectNew.GetOwnerReferences())
}

func (OwnerReferencesChangedPredicate) Generic(_ event.GenericEvent) bool {
	return false
}

type Event interface {
	EventType() string
	Reason() string
}

func emitEvent(recorder record.EventRecorder, object runtime.Object, event Event, messageFmt string, args ...interface{}) {
	recorder.Eventf(object, event.EventType(), event.Reason(), messageFmt, args...)
}

func marshalJSON(v any) (strategicpatch.JSONMap, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	data := map[string]any{}
	if len(bytes) > 0 {
		if err := json.Unmarshal(bytes, &data); err != nil {
			return nil, mergepatch.ErrBadJSONDoc
		}
	}
	return data, nil
}

func unmarshalJSON(data strategicpatch.JSONMap, v any) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	if len(bytes) > 0 {
		if err := json.Unmarshal(bytes, v); err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
	}
	return nil
}
