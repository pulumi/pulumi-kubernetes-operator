// Copyright 2022, Pulumi Corporation.  All rights reserved.

package stack

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
)

func (sess *reconcileStackSession) SetupWorkdirFromFluxSource(ctx context.Context, source unstructured.Unstructured, fluxSource *shared.FluxSource) (_commit string, retErr error) {
	rootdir, err := os.MkdirTemp("", "pulumi_source")
	if err != nil {
		return "", fmt.Errorf("unable to create tmp directory for workspace: %w", err)
	}
	sess.rootDir = rootdir

	defer func() {
		if retErr != nil {
			_ = os.RemoveAll(rootdir)
			sess.rootDir = ""
		}
	}()

	// this source artifact fetching code is based closely on
	// https://github.com/fluxcd/kustomize-controller/blob/db3c321163522259595894ca6c19ed44a876976d/controllers/kustomization_controller.go#L529
	artifactURL, ok, err := unstructured.NestedString(source.Object, "status", "artifact", "url")
	if !ok || err != nil {
		return "", errors.New("expected source to have .status.artifact.url, but it did not")
	}

	revision, ok, err := unstructured.NestedString(source.Object, "status", "artifact", "revision")
	if !ok || err != nil {
		return "", errors.New("did not find revision in .status.artifact")
	}

	checksum, ok, err := unstructured.NestedString(source.Object, "status", "artifact", "checksum")
	if !ok || err != nil {
		return "", errors.New("did not find revision in .status.artifact")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artifactURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create a request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request for artifact failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download artifact from %s, status %q (expected 200 OK)", artifactURL, resp.Status)
	}
	// TODO validate size, if given

	defer resp.Body.Close()

	var buf bytes.Buffer
	hasher := sha256.New()
	if len(checksum) == 40 { // Flux source-controller <= 0.17.2 used SHA1
		hasher = sha1.New()
	}
	out := io.MultiWriter(hasher, &buf)
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("failed to compute checksum from artifact response: %w", err)
	}
	if checksum1 := fmt.Sprintf("%x", hasher.Sum(nil)); checksum1 != checksum {
		return "", fmt.Errorf("computed checksum of artifact %q does not match checksum recorded %q", checksum1, checksum)
	}

	// we downloaded the artifact gzip-tarball into a buffer and it matches the checksum; untar it
	// into our working dir.
	if err = untar(&buf, rootdir); err != nil {
		return "", fmt.Errorf("failed to extract archive tarball: %w", err)
	}

	// woo! now there's a directory with source in `rootdir`. Construct a workspace.

	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)
	w, err := auto.NewLocalWorkspace(ctx, auto.WorkDir(filepath.Join(rootdir, fluxSource.Dir)), secretsProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create local workspace: %w", err)
	}

	return revision, sess.setupWorkspace(ctx, w)
}
