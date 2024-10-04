/*
Copyright 2024 The Kubernetes authors.

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

package v1alpha1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
)

const (
	// SecurityProfileBaselineDefaultImage is the default image used when the security profile is 'baseline'.
	SecurityProfileBaselineDefaultImage = "pulumi/pulumi:latest"
	// SecurityProfileRestrictedDefaultImage is the default image used when the security profile is 'restricted'.
	SecurityProfileRestrictedDefaultImage = "pulumi/pulumi:latest-nonroot"
)

// nolint:unused

// SetupWorkspaceWebhookWithManager registers the webhook for Workspace in the manager.
func SetupWorkspaceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&autov1alpha1.Workspace{}).
		WithValidator(&WorkspaceCustomValidator{}).
		WithDefaulter(&WorkspaceCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-auto-pulumi-com-v1alpha1-workspace,mutating=true,failurePolicy=fail,sideEffects=None,groups=auto.pulumi.com,resources=workspaces,verbs=create;update,versions=v1alpha1,name=mworkspace-v1alpha1.kb.io,admissionReviewVersions=v1

// WorkspaceCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Workspace when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type WorkspaceCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &WorkspaceCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Workspace.
func (d *WorkspaceCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	w, ok := obj.(*autov1alpha1.Workspace)
	if !ok {
		return fmt.Errorf("expected an Workspace object but got %T", obj)
	}

	if w.Spec.SecurityProfile == "" {
		w.Spec.SecurityProfile = autov1alpha1.SecurityProfileRestricted
	}

	if w.Spec.Image == "" {
		switch w.Spec.SecurityProfile {
		case autov1alpha1.SecurityProfileRestricted:
			w.Spec.Image = SecurityProfileRestrictedDefaultImage
		default:
		case autov1alpha1.SecurityProfileBaseline:
			w.Spec.Image = SecurityProfileBaselineDefaultImage
		}
	}

	// default resource requirements here are designed to provide a  "burst"-style workspace.
	if w.Spec.Resources.Requests == nil {
		w.Spec.Resources.Requests = corev1.ResourceList{}
	}
	if w.Spec.Resources.Requests.Memory().IsZero() {
		w.Spec.Resources.Requests[corev1.ResourceMemory] = resource.MustParse("64Mi")
	}
	if w.Spec.Resources.Requests.Cpu().IsZero() {
		w.Spec.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("100m")
	}

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-auto-pulumi-com-v1alpha1-workspace,mutating=false,failurePolicy=fail,sideEffects=None,groups=auto.pulumi.com,resources=workspaces,verbs=create;update,versions=v1alpha1,name=vworkspace-v1alpha1.kb.io,admissionReviewVersions=v1

// WorkspaceCustomValidator struct is responsible for validating the Workspace resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type WorkspaceCustomValidator struct {
	//TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &WorkspaceCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Workspace.
func (v *WorkspaceCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	_, ok := obj.(*autov1alpha1.Workspace)
	if !ok {
		return nil, fmt.Errorf("expected a Workspace object but got %T", obj)
	}

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Workspace.
func (v *WorkspaceCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	_, ok := newObj.(*autov1alpha1.Workspace)
	if !ok {
		return nil, fmt.Errorf("expected a Workspace object for the newObj but got %T", newObj)
	}

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Workspace.
func (v *WorkspaceCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	_, ok := obj.(*autov1alpha1.Workspace)
	if !ok {
		return nil, fmt.Errorf("expected a Workspace object but got %T", obj)
	}

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
