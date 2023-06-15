// Copyright 2022, Pulumi Corporation.  All rights reserved.

package stack

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// yamlToUnstructured is a helper to convert a YAML string to an unstructured object.
func yamlToUnstructured(t *testing.T, yml string) unstructured.Unstructured {
	t.Helper()
	m := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yml), &m)
	if err != nil {
		t.Fatalf("error parsing yaml: %v", err)
	}
	return unstructured.Unstructured{Object: m}
}

func TestGetArtifactField(t *testing.T) {
	tests := []struct {
		name, source, field, want string
		wantErr                   bool
	}{
		{
			name: "Get valid field - happy path",
			source: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
    name: pulumi-git-repo
spec:
    interval: 1m
status:
    artifact:
        url: http://example.com
        revision: "1234567890"
        checksum: "123abcdef"`,
			field:   "checksum",
			want:    "123abcdef",
			wantErr: false,
		},
		{
			name: "Expect error for non-existent field",
			source: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
    name: pulumi-git-repo
spec:
    interval: 1m
status:
    artifact:
        url: http://example.com
        revision: "1234567890"
        checksum: "123abcdef"`,
			field:   "unknown",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			obj := yamlToUnstructured(t, tc.source)
			got, err := getArtifactField(obj, tc.field)
			if (err != nil) != tc.wantErr {
				t.Fatalf("getField() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("getField() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestChecksumOrDigest(t *testing.T) {
	tests := []struct {
		name, source, want string
		wantErr            bool
	}{
		{
			name: "Get checksum field - happy path",
			source: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
    name: pulumi-git-repo
spec:
    interval: 1m
status:
    artifact:
        url: http://example.com
        revision: "1234567890"
        checksum: "123abcdef"`,
			want:    "123abcdef",
			wantErr: false,
		},
		{
			name: "Get digest field - happy path",
			source: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
    name: pulumi-git-repo
spec:
    interval: 1m
status:
    artifact:
        url: http://example.com
        revision: "1234567890"
        digest: "sha256:123abcdef"`,
			want:    "sha256:123abcdef",
			wantErr: false,
		},
		{
			name: "Get digest field when both digest and checksum present",
			source: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
    name: pulumi-git-repo
spec:
    interval: 1m
status:
    artifact:
        url: http://example.com
        revision: "1234567890"
        digest: "sha256:123abcdef"
        checksum: "1234567890"`,
			want:    "sha256:123abcdef",
			wantErr: false,
		},
		{
			name: "Error if neither digest nor checksum present",
			source: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
    name: pulumi-git-repo
spec:
    interval: 1m
status:
    artifact:
        url: http://example.com
        revision: "1234567890"`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			obj := yamlToUnstructured(t, tc.source)
			got, err := checksumOrDigest(obj)
			if (err != nil) != tc.wantErr {
				t.Errorf("checksumOrDigest() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if got != tc.want {
				t.Errorf("checksumOrDigest() = %v, want %v", got, tc.want)
			}
		})
	}
}
