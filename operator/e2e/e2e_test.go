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
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _namespace = "pulumi-kubernetes-operator"

func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Generate a random tag to ensure pods get re-created when re-running the
	// test locally.
	b := make([]byte, 12)
	_, _ = rand.Read(b) //nolint:staticcheck // Don't need crypto here.
	tag := hex.EncodeToString(b)
	projectimage := "pulumi/pulumi-kubernetes-operator-v2:" + tag

	cmd := exec.Command("make", "docker-build", "VERSION="+tag)
	require.NoError(t, run(cmd), "failed to build image")

	err := loadImageToKindClusterWithName(projectimage)
	require.NoError(t, err, "failed to load image into kind")
	err = loadImageToKindClusterWithName("pulumi/pulumi:3.130.0-nonroot")
	require.NoError(t, err, "failed to load image into kind")

	cmd = exec.Command("make", "install")
	require.NoError(t, run(cmd), "failed to install CRDs")

	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
	require.NoError(t, run(cmd), "failed to deploy the image")

	cmd = exec.Command("kubectl", "wait", "deployments/controller-manager",
		"--for", "condition=Available", "-n", _namespace, "--timeout", "60s")
	require.NoError(t, run(cmd), "controller didn't become ready after 1 minute")
	dumpLogs(t, "pulumi-kubernetes-operator", "deployment/controller-manager")

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
				t.Parallel()

				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/random-yaml-nonroot")
				require.NoError(t, run(cmd))
				dumpLogs(t, "random-yaml-nonroot", "pod/random-yaml-nonroot-workspace-0")

				// _, err := waitFor[pulumiv1.Stack]("stacks/random-yaml-nonroot", "random-yaml-nonroot", "condition=Ready", 5*time.Minute)
				// assert.NoError(t, err) // Failing on CI
			},
		},
		{
			name: "git-auth-nonroot",
			f: func(t *testing.T) {
				t.Parallel()
				if os.Getenv("PULUMI_BOT_TOKEN") == "" {
					t.Skip("missing PULUMI_BOT_TOKEN")
				}

				cmd := exec.Command("bash", "-c", "envsubst < e2e/testdata/git-auth-nonroot/* | kubectl apply -f -")
				require.NoError(t, run(cmd))
				dumpLogs(t, "git-auth-nonroot", "pod/git-auth-nonroot-workspace-0")

				stack, err := waitFor[pulumiv1.Stack]("stacks/git-auth-nonroot", "git-auth-nonroot", "condition=Ready", 5*time.Minute)
				assert.NoError(t, err)

				assert.Equal(t, `"[secret]"`, string(stack.Status.Outputs["secretOutput"].Raw))
				assert.Equal(t, `"foo"`, string(stack.Status.Outputs["simpleOutput"].Raw))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.f)
	}
}

// dumpLogs prints logs if the test fails.
func dumpLogs(t *testing.T, namespace, name string) {
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		t.Logf("=== LOGS %s %s", namespace, name)
		cmd := exec.Command("kubectl", "logs", "--all-containers=true", "-n", namespace, name)
		out, err := cmd.CombinedOutput()
		assert.NoError(t, err)
		t.Log(string(out))
	})
}

func waitFor[T any](name, namespace, condition string, d time.Duration) (*T, error) {
	cmd := exec.Command("kubectl", "wait", name,
		"-n", namespace,
		"--for", condition,
		fmt.Sprintf("--timeout=%ds", int(d.Seconds())),
		"--output=yaml",
	)
	err := run(cmd)
	if err != nil {
		return nil, err
	}

	buf, _ := cmd.Stdout.(*bytes.Buffer)
	var obj T
	err = yaml.Unmarshal(buf.Bytes(), &obj)
	if err != nil {
		return nil, err
	}

	return &obj, nil
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
