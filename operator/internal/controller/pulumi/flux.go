// Copyright 2022, Pulumi Corporation.  All rights reserved.

package pulumi

import (
	"context"
	"fmt"
	"strings"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (sess *StackReconcilerSession) SetupWorkspaceFromFluxSource(ctx context.Context, source unstructured.Unstructured, fluxSource *shared.FluxSource) (string, error) {

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

	return digest, sess.setupWorkspace(ctx)
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

// checkFluxSourceReady looks for the conventional "Ready" condition to see if the supplied object
// can be considered _not_ ready. It returns an error if it can determine that the object is not
// ready, and nil if it cannot determine so.
func checkFluxSourceReady(obj unstructured.Unstructured) error {
	conditions, ok, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if ok && err == nil {
		// didn't find a []Condition, so there's nothing to indicate that it's not ready there
		for _, c0 := range conditions {
			var c map[string]interface{}
			if c, ok = c0.(map[string]interface{}); !ok {
				// condition isn't the right shape, try the next one
				continue
			}
			if t, ok, err := unstructured.NestedString(c, "type"); ok && err == nil && t == "Ready" {
				if v, ok, err := unstructured.NestedString(c, "status"); ok && err == nil && v == "True" {
					// found the Ready condition and it is actually ready; proceed to next check
					break
				}
				// found the Ready condition and it's something other than ready
				return fmt.Errorf("source Ready condition does not have status True %#v", c)
			}
		}
		// Ready=true, or no ready condition to tell us either way
	}

	_, ok, err = unstructured.NestedMap(obj.Object, "status", "artifact")
	if !ok || err != nil {
		return fmt.Errorf(".status.artifact does not have an Artifact object")
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
