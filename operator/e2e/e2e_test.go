/*
Copyright 2024.

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

package e2e

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var _namespace = "pulumi-kubernetes-operator"

func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Generate a random tag to ensure pods get re-created when re-running the
	// test locally.
	bytes := make([]byte, 12)
	_, _ = rand.Read(bytes) //nolint:staticcheck // Don't need crypto here.
	tag := hex.EncodeToString(bytes)
	projectimage := "pulumi/pulumi-kubernetes-operator-v2:" + tag

	cmd := exec.Command("make", "docker-build", "VERSION="+tag)
	require.NoError(t, run(cmd), "failed to build image")

	err := loadImageToKindClusterWithName(projectimage)
	require.NoError(t, err, "failed to load image into kind")

	cmd = exec.Command("make", "install")
	require.NoError(t, run(cmd), "failed to install CRDs")

	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
	require.NoError(t, run(cmd), "failed to deploy the image")

	cmd = exec.Command("kubectl", "wait", "deployments/controller-manager",
		"--for", "condition=Available", "-n", _namespace, "--timeout", "60s")
	require.NoError(t, run(cmd), "controller didn't become ready after 1 minute")

	// Install Flux
	cmd = exec.Command("kubectl", "apply", "-f", "https://github.com/fluxcd/flux2/releases/download/v2.3.0/install.yaml")
	require.NoError(t, run(cmd), "failed to install flux")

	tests := []struct {
		name string
		f    func(t *testing.T)
	}{
		{
			name: "random-yaml-nonroot",
			f: func(t *testing.T) {
				err := loadImageToKindClusterWithName("pulumi/pulumi:3.130.0-nonroot")
				require.NoError(t, err, "failed to load image into kind")

				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/random-yaml-nonroot")
				require.NoError(t, run(cmd))

				// TODO: Wait for stack to become ready. Currently flakes due
				// to GitHub rate limiting and "resource modified" retries.
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.f)
	}
}

// run executes the provided command within this context
func run(cmd *exec.Cmd) error {
	command := strings.Join(cmd.Args, " ")
	fmt.Fprintf(os.Stderr, "running: %s\n", command)

	// Run from project root so "make" targets are available.
	cmd.Dir = "../"
	bytes, err := cmd.CombinedOutput()
	output := string(bytes)
	if err != nil {
		fmt.Fprint(os.Stderr, output)
		return fmt.Errorf("%s failed with error: %w", command, err)
	}
	return nil
}

// loadImageToKindClusterWithName loads a local docker image to the kind cluster
func loadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	return run(cmd)
}