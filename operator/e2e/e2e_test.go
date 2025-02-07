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

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
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
	projectimage := "pulumi/pulumi-kubernetes-operator:" + tag

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
				// 1. Test `WorkspaceReclaimPolicy` is unset.
				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/random-yaml-nonroot")
				require.NoError(t, run(cmd))
				dumpLogs(t, "random-yaml-nonroot", "pod/random-yaml-nonroot-workspace-0")

				_, err := waitFor[pulumiv1.Stack]("stacks/random-yaml-nonroot", "random-yaml-nonroot", 5*time.Minute, "condition=Ready")
				assert.NoError(t, err)

				// Ensure that the workspace pod was not deleted after successful Stack reconciliation.
				time.Sleep(10 * time.Second)
				found, err := foundEvent("Pod", "random-yaml-nonroot-workspace-0", "random-yaml-nonroot", "Killing")
				assert.NoError(t, err)
				assert.False(t, found)

				// 2. Test `WorkspaceReclaimPolicy` is set to `Delete`.
				// Update the Stack spec to set the `WorkspaceReclaimPolicy` to `Delete`.
				cmd = exec.Command("kubectl", "patch", "stacks", "--namespace", "random-yaml-nonroot", "random-yaml-nonroot", "--type=merge", "-p", `{"spec":{"workspaceReclaimPolicy":"Delete"}}`)
				require.NoError(t, run(cmd))

				// Wait for the Stack to be reconciled, and observedGeneration to be updated.
				_, err = waitFor[pulumiv1.Stack](
					"stacks/random-yaml-nonroot",
					"random-yaml-nonroot",
					5*time.Minute,
					"condition=Ready",
					"jsonpath={.status.observedGeneration}=3")
				assert.NoError(t, err)

				// Ensure that the workspace pod is now deleted after successful Stack reconciliation.
				retryUntil(t, 10*time.Second, true, func() bool {
					found, err = foundEvent("Pod", "random-yaml-nonroot-workspace-0", "random-yaml-nonroot", "Killing")
					assert.NoError(t, err)
					return found
				})

				if t.Failed() {
					cmd := exec.Command("kubectl", "get", "pods", "-A")
					out, err := cmd.CombinedOutput()
					assert.NoError(t, err)
					t.Log(string(out))
				}
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

				stack, err := waitFor[pulumiv1.Stack]("stacks/git-auth-nonroot", "git-auth-nonroot", 5*time.Minute, "condition=Ready")
				assert.NoError(t, err)

				assert.Equal(t, `"[secret]"`, string(stack.Status.Outputs["secretOutput"].Raw))
				assert.Equal(t, `"foo"`, string(stack.Status.Outputs["simpleOutput"].Raw))
			},
		},
		{
			name: "targets",
			f: func(t *testing.T) {
				t.Parallel()

				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/targets")
				require.NoError(t, run(cmd))
				dumpLogs(t, "targets", "pod/targets-workspace-0")

				stack, err := waitFor[pulumiv1.Stack]("stacks/targets", "targets", 5*time.Minute, "condition=Ready")
				assert.NoError(t, err)

				assert.Contains(t, stack.Status.Outputs, "targeted")
				assert.NotContains(t, stack.Status.Outputs, "notTargeted")
			},
		},
		{
			name: "issue-801",
			f: func(t *testing.T) {
				t.Parallel()

				// deploy a workspace with a non-existent container image (pulumi:nosuchimage)
				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/issue-801")
				require.NoError(t, run(cmd))
				dumpLogs(t, "issue-801", "pods/issue-801-workspace-0")

				// wait for the pod to be created (knowing that it will never become ready)
				_, err := waitFor[autov1alpha1.Workspace]("pods/issue-801-workspace-0", "issue-801", 5*time.Minute, "create")
				assert.NoError(t, err)

				// update the workspace to a valid image (expecting that a new pod will be rolled out)
				cmd = exec.Command("kubectl", "apply", "-f", "e2e/testdata/issue-801/step2")
				require.NoError(t, run(cmd))

				// wait for the workspace to be fully ready
				_, err = waitFor[autov1alpha1.Workspace]("workspaces/issue-801", "issue-801", 5*time.Minute, "condition=Ready")
				assert.NoError(t, err)
			},
		},
		{
			name: "random-yaml-auth-error",
			f: func(t *testing.T) {
				t.Parallel()

				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/random-yaml-auth-error")
				require.NoError(t, run(cmd))
				dumpLogs(t, "random-yaml-auth-error", "pod/random-yaml-auth-error-workspace-0")

				// Wait for the Workspace pod to be created, so that we can watch/wait on the Workspace object.
				retryUntil(t, 30*time.Second, true, func() bool {
					found, err := foundEvent("Pod", "random-yaml-auth-error-workspace-0", "random-yaml-auth-error", "Created")
					assert.NoError(t, err)
					return found
				})

				// Ensure the Workspace is in a failed state with Unauthenticated.
				_, err := waitFor[pulumiv1.Stack](
					"workspaces/random-yaml-auth-error",
					"random-yaml-auth-error",
					5*time.Minute,
					`jsonpath={.status.conditions[?(@.type=="Ready")].reason}=Unauthenticated`)
				assert.NoError(t, err)

				// Ensure that the workspace pod was not deleted after reconciling the failed stack.
				time.Sleep(10 * time.Second)
				found, err := foundEvent("Pod", "random-yaml-auth-error-workspace-0", "random-yaml-auth-error", "Killing")
				assert.NoError(t, err)
				assert.False(t, found)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.f)
	}
}

// retryUntil retries the provided function until it returns the required condition or the deadline is reached.
func retryUntil(t *testing.T, d time.Duration, condition bool, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if f() == condition {
			return
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("timed out waiting for condition")
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

func waitFor[T any](name, namespace string, d time.Duration, conditions ...string) (*T, error) {
	if len(conditions) == 0 {
		return nil, fmt.Errorf("no conditions provided")
	}

	cmd := exec.Command("kubectl", "wait", name,
		"-n", namespace,
		"--for", conditions[0],
		fmt.Sprintf("--timeout=%ds", int(d.Seconds())),
		"--output=yaml",
	)
	// Add additional conditions if provided.
	for _, condition := range conditions[1:] {
		cmd.Args = append(cmd.Args, "--for", condition)
	}

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

// foundEvent checks if a Kubernetes event with the given kind, name, namespace, and reason exists.
func foundEvent(kind, name, namespace, reason string) (bool, error) {
	cmd := exec.Command("kubectl", "get", "events",
		"-n", namespace,
		"--field-selector", fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s,reason=%s", kind, name, reason),
		"--output=name",
	)
	err := run(cmd)
	if err != nil {
		return false, err
	}

	buf, _ := cmd.Stdout.(*bytes.Buffer)
	return strings.Contains(buf.String(), "event/"+name), nil
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
