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

import (
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

// WorkspaceSpecApplyConfiguration represents a declarative configuration of the WorkspaceSpec type for use
// with apply.
type WorkspaceSpecApplyConfiguration struct {
	ServiceAccountName *string                                    `json:"serviceAccountName,omitempty"`
	SecurityProfile    *autov1alpha1.SecurityProfile              `json:"securityProfile,omitempty"`
	Image              *string                                    `json:"image,omitempty"`
	ImagePullPolicy    *v1.PullPolicy                             `json:"imagePullPolicy,omitempty"`
	Git                *GitSourceApplyConfiguration               `json:"git,omitempty"`
	Flux               *FluxSourceApplyConfiguration              `json:"flux,omitempty"`
	Local              *LocalSourceApplyConfiguration             `json:"local,omitempty"`
	EnvFrom            []v1.EnvFromSource                         `json:"envFrom,omitempty"`
	Env                []v1.EnvVar                                `json:"env,omitempty"`
	Resources          *v1.ResourceRequirements                   `json:"resources,omitempty"`
	PodTemplate        *EmbeddedPodTemplateSpecApplyConfiguration `json:"podTemplate,omitempty"`
	Stacks             []WorkspaceStackApplyConfiguration         `json:"stacks,omitempty"`
}

// WorkspaceSpecApplyConfiguration constructs a declarative configuration of the WorkspaceSpec type for use with
// apply.
func WorkspaceSpec() *WorkspaceSpecApplyConfiguration {
	return &WorkspaceSpecApplyConfiguration{}
}

// WithServiceAccountName sets the ServiceAccountName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ServiceAccountName field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithServiceAccountName(value string) *WorkspaceSpecApplyConfiguration {
	b.ServiceAccountName = &value
	return b
}

// WithSecurityProfile sets the SecurityProfile field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SecurityProfile field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithSecurityProfile(value autov1alpha1.SecurityProfile) *WorkspaceSpecApplyConfiguration {
	b.SecurityProfile = &value
	return b
}

// WithImage sets the Image field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Image field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithImage(value string) *WorkspaceSpecApplyConfiguration {
	b.Image = &value
	return b
}

// WithImagePullPolicy sets the ImagePullPolicy field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ImagePullPolicy field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithImagePullPolicy(value v1.PullPolicy) *WorkspaceSpecApplyConfiguration {
	b.ImagePullPolicy = &value
	return b
}

// WithGit sets the Git field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Git field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithGit(value *GitSourceApplyConfiguration) *WorkspaceSpecApplyConfiguration {
	b.Git = value
	return b
}

// WithFlux sets the Flux field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Flux field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithFlux(value *FluxSourceApplyConfiguration) *WorkspaceSpecApplyConfiguration {
	b.Flux = value
	return b
}

// WithLocal sets the Local field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Local field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithLocal(value *LocalSourceApplyConfiguration) *WorkspaceSpecApplyConfiguration {
	b.Local = value
	return b
}

// WithEnvFrom adds the given value to the EnvFrom field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the EnvFrom field.
func (b *WorkspaceSpecApplyConfiguration) WithEnvFrom(values ...v1.EnvFromSource) *WorkspaceSpecApplyConfiguration {
	for i := range values {
		b.EnvFrom = append(b.EnvFrom, values[i])
	}
	return b
}

// WithEnv adds the given value to the Env field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Env field.
func (b *WorkspaceSpecApplyConfiguration) WithEnv(values ...v1.EnvVar) *WorkspaceSpecApplyConfiguration {
	for i := range values {
		b.Env = append(b.Env, values[i])
	}
	return b
}

// WithResources sets the Resources field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Resources field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithResources(value v1.ResourceRequirements) *WorkspaceSpecApplyConfiguration {
	b.Resources = &value
	return b
}

// WithPodTemplate sets the PodTemplate field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PodTemplate field is set to the value of the last call.
func (b *WorkspaceSpecApplyConfiguration) WithPodTemplate(value *EmbeddedPodTemplateSpecApplyConfiguration) *WorkspaceSpecApplyConfiguration {
	b.PodTemplate = value
	return b
}

// WithStacks adds the given value to the Stacks field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Stacks field.
func (b *WorkspaceSpecApplyConfiguration) WithStacks(values ...*WorkspaceStackApplyConfiguration) *WorkspaceSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithStacks")
		}
		b.Stacks = append(b.Stacks, *values[i])
	}
	return b
}
