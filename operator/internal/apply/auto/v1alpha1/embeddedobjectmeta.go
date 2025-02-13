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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

// EmbeddedObjectMetaApplyConfiguration represents a declarative configuration of the EmbeddedObjectMeta type for use
// with apply.
type EmbeddedObjectMetaApplyConfiguration struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// EmbeddedObjectMetaApplyConfiguration constructs a declarative configuration of the EmbeddedObjectMeta type for use with
// apply.
func EmbeddedObjectMeta() *EmbeddedObjectMetaApplyConfiguration {
	return &EmbeddedObjectMetaApplyConfiguration{}
}

// WithLabels puts the entries into the Labels field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the Labels field,
// overwriting an existing map entries in Labels field with the same key.
func (b *EmbeddedObjectMetaApplyConfiguration) WithLabels(entries map[string]string) *EmbeddedObjectMetaApplyConfiguration {
	if b.Labels == nil && len(entries) > 0 {
		b.Labels = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.Labels[k] = v
	}
	return b
}

// WithAnnotations puts the entries into the Annotations field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the Annotations field,
// overwriting an existing map entries in Annotations field with the same key.
func (b *EmbeddedObjectMetaApplyConfiguration) WithAnnotations(entries map[string]string) *EmbeddedObjectMetaApplyConfiguration {
	if b.Annotations == nil && len(entries) > 0 {
		b.Annotations = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.Annotations[k] = v
	}
	return b
}
