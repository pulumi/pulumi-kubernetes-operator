// Copyright 2022, Pulumi Corporation.  All rights reserved.

package pulumi

import (
	"context"
	"fmt"
	"strings"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func (sess *stackReconcilerSession) SetupWorkspaceFromFluxSource(ctx context.Context, source unstructured.Unstructured, fluxSource *shared.FluxSource) (string, error) {
	// this source artifact fetching code is based closely on
	// https://github.com/fluxcd/kustomize-controller/blob/db3c321163522259595894ca6c19ed44a876976d/controllers/kustomization_controller.go#L529
	sess.logger.V(1).Info("Setting up pulumi workspace for stack", "stack", sess.stack, "workspace", sess.ws.Name)

	artifactURL, err := getArtifactField(source, "url")
	if err != nil {
		return "", err
	}
	digest, err := getArtifactField(source, "digest")
	if err != nil {
		return "", err
	}

	sess.ws.Spec.Flux = &autov1alpha1.FluxSource{
		Url:    artifactURL,
		Digest: digest,
		Dir:    fluxSource.Dir,
	}

	return digest, nil
}

// getArtifactField is a helper to get a specified nested field from .status.artifact.
func getArtifactField(source unstructured.Unstructured, field string) (string, error) {
	value, ok, err := unstructured.NestedString(source.Object, "status", "artifact", field)
	if !ok || err != nil || value == "" {
		return "", fmt.Errorf("expected a non-empty string in .status.artifact.%s", field)
	}
	return value, nil
}

// checksumOrDigest returns the digest or checksum field from the artifact status. It prefers digest over checksum.
func checksumOrDigest(source unstructured.Unstructured) (string, error) {
	digest, _ := getArtifactField(source, "digest")
	checksum, _ := getArtifactField(source, "checksum")

	if digest == "" && checksum == "" {
		return "", fmt.Errorf("expected at least one of .status.artifact.{digest,checksum} to be a non-empty string")
	}

	// Prefer digest over checksum.
	if digest != "" {
		// TODO: Remove normalization once we upgrade to github.com/fluxcd/pkg/http/fetch v0.5.1 or higher.

		// Extract the hash from the digest (<algorithm>:<hash>). The current fetcher expects the hash only.
		parts := strings.Split(digest, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid digest format: %q, expected <algorithm>:<hash>", digest)
		}

		return parts[1], nil
	}

	return checksum, nil
}

func checkFluxSourceReady(obj *unstructured.Unstructured) bool {
	observedGeneration, ok, err := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")
	if !ok || err != nil || observedGeneration != obj.GetGeneration() {
		return false
	}
	conditions, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !ok || err != nil {
		return false
	}
	for _, c0 := range conditions {
		var c map[string]interface{}
		if c, ok = c0.(map[string]interface{}); !ok {
			// condition isn't the right shape, try the next one
			continue
		}
		if t, ok, err := unstructured.NestedString(c, "type"); ok && err == nil && t == "Ready" {
			if v, ok, err := unstructured.NestedString(c, "status"); ok && err == nil && v == "True" {
				return true
			}
		}
	}
	return false
}

func getSourceGVK(src shared.FluxSourceReference) (schema.GroupVersionKind, error) {
	gv, err := schema.ParseGroupVersion(src.APIVersion)
	return gv.WithKind(src.Kind), err
}

func fluxSourceKey(gvk schema.GroupVersionKind, name string) string {
	return fmt.Sprintf("%s:%s", gvk, name)
}

type fluxSourceReadyPredicate struct{}

var _ predicate.Predicate = &fluxSourceReadyPredicate{}

func (fluxSourceReadyPredicate) Create(e event.CreateEvent) bool {
	return checkFluxSourceReady(e.Object.(*unstructured.Unstructured))
}

func (fluxSourceReadyPredicate) Delete(_ event.DeleteEvent) bool {
	return false
}

func (fluxSourceReadyPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return !checkFluxSourceReady(e.ObjectOld.(*unstructured.Unstructured)) && checkFluxSourceReady(e.ObjectNew.(*unstructured.Unstructured))
}

func (fluxSourceReadyPredicate) Generic(_ event.GenericEvent) bool {
	return false
}
