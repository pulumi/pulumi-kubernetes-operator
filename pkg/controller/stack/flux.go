// Copyright 2022, Pulumi Corporation.  All rights reserved.

package stack

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/fluxcd/pkg/http/fetch"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
)

const maxArtifactDownloadSize = 50 * 1024 * 1024

func (sess *reconcileStackSession) SetupWorkdirFromFluxSource(ctx context.Context, source unstructured.Unstructured, fluxSource *shared.FluxSource) (string, error) {
	// this source artifact fetching code is based closely on
	// https://github.com/fluxcd/kustomize-controller/blob/db3c321163522259595894ca6c19ed44a876976d/controllers/kustomization_controller.go#L529

	getField := func(field string) (string, error) {
		value, ok, err := unstructured.NestedString(source.Object, "status", "artifact", field)
		if !ok || err != nil || value == "" {
			return "", fmt.Errorf("expected a non-empty string in .status.artifact.%s", field)
		}
		return value, nil
	}

	artifactURL, err := getField("url")
	if err != nil {
		return "", err
	}
	revision, err := getField("revision")
	if err != nil {
		return "", err
	}
	checksum, err := getField("checksum")
	if err != nil {
		return "", err
	}

	fetcher := fetch.NewArchiveFetcher(1, maxArtifactDownloadSize, maxArtifactDownloadSize*10, "")
	if err = fetcher.Fetch(artifactURL, checksum, sess.rootDir); err != nil {
		return "", fmt.Errorf("failed to get artifact from source: %w", err)
	}

	// woo! now there's a directory with source in `rootdir`. Construct a workspace.

	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)
	w, err := auto.NewLocalWorkspace(ctx, auto.WorkDir(filepath.Join(sess.rootDir, fluxSource.Dir)), secretsProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create local workspace: %w", err)
	}

	return revision, sess.setupWorkspace(ctx, w)
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
