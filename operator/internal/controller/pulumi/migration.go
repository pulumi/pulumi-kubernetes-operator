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

package pulumi

import (
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func ValidateStack(s *pulumiv1.Stack) (admission.Warnings, error) {
	var allErrs field.ErrorList

	if s.Spec.ExpectNoRefreshChanges {
		field.Invalid(field.NewPath("spec", "expectNoRefreshChanges"), s.Spec.ExpectNoRefreshChanges, "expectNoRefreshChanges is ignored")
	}

	// obsolete: EnvRef containing a reference to a file or environment variable
	for key, envRef := range s.Spec.EnvRefs {
		path := field.NewPath("spec", "envRefs").Key(key)
		if envRef.SelectorType != shared.ResourceSelectorLiteral && envRef.SelectorType != shared.ResourceSelectorSecret {
			field.NotSupported(path.Child("selectorType"), envRef.SelectorType,
				[]string{string(shared.ResourceSelectorLiteral), string(shared.ResourceSelectorSecret)})
		}
		if envRef.SelectorType == shared.ResourceSelectorSecret && envRef.SecretRef != nil && envRef.SecretRef.Namespace != "" && envRef.SecretRef.Namespace != s.Namespace {
			field.Invalid(path.Child("secret", "namespace"), envRef.SecretRef.Namespace, "cross-namespace references are not allowed")
		}
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(
		s.GroupVersionKind().GroupKind(),
		s.Name, allErrs)
}
