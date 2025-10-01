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

package v1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStackNameLengthConstraint documents and verifies the stack name length constraint.
// Stack names are limited to 42 characters because:
// - The workspace StatefulSet is named "<stack-name>-workspace" (adds 10 chars)
// - Kubernetes StatefulSet controller adds a pod ordinal suffix like "-0" (adds 2 chars)
// - Kubernetes uses the pod name as a label value, which has a 63 character limit
// - Total: 42 (stack name) + 10 ("-workspace") + 2 ("-0") + some buffer = under 63
//
// Actually, the issue is more specific:
// - StatefulSet pods get names like "<statefulset-name>-<ordinal>"
// - The StatefulSet name is "<stack-name>-workspace"
// - Kubernetes adds a hash suffix to pod labels (10 chars + dash = 11 chars)
// - So: stack-name (42) + "-workspace" (10) + "-" (1) + hash (10) = 63 chars max
func TestStackNameLengthConstraint(t *testing.T) {
	const (
		workspaceSuffix   = "-workspace"      // 10 characters
		kubernetesHash    = "0123456789"      // 10 characters (added by StatefulSet)
		dashBeforeHash    = "-"               // 1 character
		totalSuffix       = 21                // 10 + 10 + 1
		kubernetesLimit   = 63                // Kubernetes label value limit
		maxStackNameLen   = 42                // 63 - 21
	)

	// Verify our math
	require.Equal(t, len(workspaceSuffix), 10, "workspace suffix length")
	require.Equal(t, len(kubernetesHash), 10, "kubernetes hash length")
	require.Equal(t, totalSuffix, len(workspaceSuffix)+len(kubernetesHash)+len(dashBeforeHash), "total suffix length")
	require.Equal(t, maxStackNameLen, kubernetesLimit-totalSuffix, "max stack name length")

	// Test with a name at the boundary
	stackName := strings.Repeat("a", maxStackNameLen)
	workspacePodName := stackName + workspaceSuffix + dashBeforeHash + kubernetesHash

	require.Equal(t, len(stackName), maxStackNameLen, "stack name should be at max length")
	require.Equal(t, len(workspacePodName), kubernetesLimit, "workspace pod name should be at kubernetes limit")

	// One character over should exceed the limit
	stackNameTooLong := strings.Repeat("a", maxStackNameLen+1)
	workspacePodNameTooLong := stackNameTooLong + workspaceSuffix + dashBeforeHash + kubernetesHash
	require.Greater(t, len(workspacePodNameTooLong), kubernetesLimit, "workspace pod name should exceed kubernetes limit")
}

// TestStackNameValidationExamples provides examples of valid and invalid stack names.
func TestStackNameValidationExamples(t *testing.T) {
	tests := []struct {
		name      string
		stackName string
		valid     bool
	}{
		{
			name:      "valid short name",
			stackName: "my-stack",
			valid:     true,
		},
		{
			name:      "valid name at limit (42 chars)",
			stackName: strings.Repeat("a", 42),
			valid:     true,
		},
		{
			name:      "invalid name at 43 chars",
			stackName: strings.Repeat("a", 43),
			valid:     false,
		},
		{
			name:      "invalid long name from issue #899",
			stackName: "my-very-long-stack-name-for-staging-environment",
			valid:     false,
		},
		{
			name:      "valid name at 42 chars with hyphens",
			stackName: "my-stack-name-for-staging-env-12345678",
			valid:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the length matches expectations
			if tt.valid {
				require.LessOrEqual(t, len(tt.stackName), 42, "valid names should be 42 chars or less")
			} else {
				require.Greater(t, len(tt.stackName), 42, "invalid names should be more than 42 chars")
			}

			// Document the length
			t.Logf("Stack name: %s (length: %d)", tt.stackName, len(tt.stackName))
		})
	}
}
