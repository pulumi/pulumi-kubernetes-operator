/*
Copyright 2024, Pulumi Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package apply

import (
	v1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/apply/auto/v1alpha1"
	internal "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/apply/internal"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// ForKind returns an apply configuration type for the given GroupVersionKind, or nil if no
// apply configuration type exists for the given GroupVersionKind.
func ForKind(kind schema.GroupVersionKind) interface{} {
	switch kind {
	// Group=auto.pulumi.com, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithKind("ConfigItem"):
		return &autov1alpha1.ConfigItemApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("ConfigValueFrom"):
		return &autov1alpha1.ConfigValueFromApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("EmbeddedObjectMeta"):
		return &autov1alpha1.EmbeddedObjectMetaApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("EmbeddedPodTemplateSpec"):
		return &autov1alpha1.EmbeddedPodTemplateSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("FluxSource"):
		return &autov1alpha1.FluxSourceApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("GitAuth"):
		return &autov1alpha1.GitAuthApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("GitSource"):
		return &autov1alpha1.GitSourceApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("Workspace"):
		return &autov1alpha1.WorkspaceApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("WorkspaceSpec"):
		return &autov1alpha1.WorkspaceSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("WorkspaceStack"):
		return &autov1alpha1.WorkspaceStackApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("WorkspaceStatus"):
		return &autov1alpha1.WorkspaceStatusApplyConfiguration{}

	}
	return nil
}

func NewTypeConverter(scheme *runtime.Scheme) *testing.TypeConverter {
	return &testing.TypeConverter{Scheme: scheme, TypeResolver: internal.Parser()}
}
