// Copyright 2022, Pulumi Corporation.  All rights reserved.

package pulumi

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fluxcd/pkg/http/fetch"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pulumi/pulumi-kubernetes-operator/operator/api/pulumi/shared"
)

const maxArtifactDownloadSize = 50 * 1024 * 1024

func (sess *StackReconcilerSession) SetupWorkdirFromFluxSource(ctx context.Context, source unstructured.Unstructured, fluxSource *shared.FluxSource) (string, error) {
	// this source artifact fetching code is based closely on
	// https://github.com/fluxcd/kustomize-controller/blob/db3c321163522259595894ca6c19ed44a876976d/controllers/kustomization_controller.go#L529
	homeDir := sess.getPulumiHome()
	workspaceDir := sess.getWorkspaceDir()
	sess.logger.Debug("Setting up pulumi workspace for stack", "stack", sess.stack, "workspace", workspaceDir)

	artifactURL, err := getArtifactField(source, "url")
	if err != nil {
		return "", err
	}
	revision, err := getArtifactField(source, "revision")
	if err != nil {
		return "", err
	}

	// Check for either the digest or checksum field. If both are present, prefer digest.
	// Checksum was supported/deprecated by the Fluxv2 Artifact type in v1beta2, but was removed in v1.
	// The format of digest is slightly different to checksum.
	// When github.com/fluxcd/pkg/http/fetch is updated to v0.5.1 or higher, the function from Flux that needs checksum/digest
	// accepts both formats. Until then, we'll need to normalize the digest.
	// https://github.com/fluxcd/source-controller/blob/a0ff0cfa885e1e5f506a593a9de39174cf1dfeb8/api/v1beta2/artifact_types.go#L49-L57
	digest, err := checksumOrDigest(source)
	if err != nil {
		return "", err
	}

	fetcher := fetch.NewArchiveFetcher(1, maxArtifactDownloadSize, maxArtifactDownloadSize*10, "")
	if err = fetcher.Fetch(artifactURL, digest, workspaceDir); err != nil {
		return "", fmt.Errorf("failed to get artifact from source: %w", err)
	}

	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)
	w, err := auto.NewLocalWorkspace(
		ctx,
		auto.PulumiHome(homeDir),
		auto.WorkDir(filepath.Join(workspaceDir, fluxSource.Dir)),
		secretsProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create local workspace: %w", err)
	}

	return revision, sess.setupWorkspace(ctx, w)
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
