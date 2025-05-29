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
	"context"
	"fmt"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func (sess *stackReconcilerSession) SetupWorkspaceFromFluxSource(ctx context.Context, source unstructured.Unstructured,
	artifact pulumiv1.Artifact, dir string) error {
	// this source artifact fetching code is based closely on
	sess.logger.V(1).Info("Setting up pulumi workspace for stack", "stack", sess.stack, "workspace", sess.ws.Name,
		"artifact", artifact, "dir", dir)

	sess.ws.Spec.Flux = &autov1alpha1.FluxSource{
		Url:    artifact.URL,
		Digest: artifact.Digest,
		Dir:    dir,
	}

	return nil
}

func getSourceGVK(src shared.FluxSourceReference) (schema.GroupVersionKind, error) {
	gv, err := schema.ParseGroupVersion(src.APIVersion)
	return gv.WithKind(src.Kind), err
}

func fluxSourceKey(gvk schema.GroupVersionKind, name string) string {
	return fmt.Sprintf("%s:%s", gvk, name)
}

type SourceRevisionChangePredicate struct {
	predicate.Funcs
}

func (SourceRevisionChangePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldSource, ok := e.ObjectOld.(*unstructured.Unstructured)
	if !ok || oldSource == nil {
		return false
	}
	oldArtifact, _, _ := getArtifact(*oldSource)

	newSource, ok := e.ObjectNew.(*unstructured.Unstructured)
	if !ok || newSource == nil {
		return false
	}
	newArtifact, _, _ := getArtifact(*newSource)

	if oldArtifact == nil && newArtifact != nil {
		return true
	}

	if oldArtifact != nil && newArtifact != nil &&
		!oldArtifact.HasRevision(newArtifact.Revision) {
		return true
	}

	return false
}

// getArtifact retrieves the Artifact from the given Source object.
// It returns the Artifact, a boolean indicating if the Artifact was found,
// and an error if there was an issue retrieving or decoding the Artifact.
func getArtifact(source unstructured.Unstructured) (*pulumiv1.Artifact, bool, error) {
	m, hasArtifact, err := unstructured.NestedMap(source.Object, "status", "artifact")
	if err != nil {
		return nil, false, err
	}
	if !hasArtifact {
		return nil, false, nil
	}
	var artifact pulumiv1.Artifact
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &artifact); err != nil {
		return nil, false, fmt.Errorf("failed to decode artifact: %w", err)
	}
	return &artifact, true, nil
}
