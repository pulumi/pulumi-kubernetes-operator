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

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/controller/pulumi"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	autov1alpha1apply "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/apply/auto/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _namespace = "pulumi-kubernetes-operator"

func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Generate a random tag to ensure pods get re-created when re-running the
	// test locally.
	b := make([]byte, 12)
	_, _ = rand.Read(b)
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
				// 1. Test `WorkspaceReclaimPolicy` is unset.
				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/random-yaml-nonroot")
				require.NoError(t, run(cmd))
				dumpLogs(t, "random-yaml-nonroot", "pod/random-yaml-nonroot-workspace-0")

				_, err := waitFor[pulumiv1.Stack](
					"stacks/random-yaml-nonroot",
					"random-yaml-nonroot",
					5*time.Minute,
					"condition=Ready")
				require.NoError(t, err)

				// Ensure that the workspace pod was not deleted after successful Stack reconciliation.
				time.Sleep(10 * time.Second)
				found, err := foundEvent("Pod", "random-yaml-nonroot-workspace-0", "random-yaml-nonroot", "Killing")
				require.NoError(t, err)
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
				require.NoError(t, err)

				// Ensure that the workspace pod is now deleted after successful Stack reconciliation.
				retryUntil(t, 10*time.Second, true, func() bool {
					found, err = foundEvent("Pod", "random-yaml-nonroot-workspace-0", "random-yaml-nonroot", "Killing")
					require.NoError(t, err)
					return found
				})

				if t.Failed() {
					cmd := exec.Command("kubectl", "get", "pods", "-A")
					out, err := cmd.CombinedOutput()
					require.NoError(t, err)
					t.Log(string(out))
				}
			},
		},
		{
			name: "git-auth-nonroot",
			f: func(t *testing.T) {
				if os.Getenv("PULUMI_BOT_TOKEN") == "" {
					t.Skip("missing PULUMI_BOT_TOKEN")
				}
				cmd := exec.Command("bash", "-c", "envsubst < e2e/testdata/git-auth-nonroot/* | kubectl apply -f -")
				require.NoError(t, run(cmd))
				dumpLogs(t, "git-auth-nonroot", "pod/git-auth-nonroot-workspace-0")

				stack, err := waitFor[pulumiv1.Stack](
					"stacks/git-auth-nonroot",
					"git-auth-nonroot",
					5*time.Minute,
					"condition=Ready")
				require.NoError(t, err)

				assert.Equal(t, `"[secret]"`, string(stack.Status.Outputs["secretOutput"].Raw))
				assert.Equal(t, `"foo"`, string(stack.Status.Outputs["simpleOutput"].Raw))
			},
		},
		{
			name: "targets",
			f: func(t *testing.T) {
				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/targets")
				require.NoError(t, run(cmd))
				dumpLogs(t, "targets", "pod/targets-workspace-0")

				stack, err := waitFor[pulumiv1.Stack]("stacks/targets", "targets", 5*time.Minute, "condition=Ready")
				require.NoError(t, err)

				assert.Contains(t, stack.Status.Outputs, "targeted")
				assert.NotContains(t, stack.Status.Outputs, "notTargeted")
			},
		},
		{
			name: "issue-801",
			f: func(t *testing.T) {
				dumpEvents(t, "issue-801")
				dumpLogs(t, "issue-801", "pods/issue-801-workspace-0")

				// deploy a workspace with a non-existent container image (pulumi:nosuchimage)
				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/issue-801")
				require.NoError(t, run(cmd))

				// wait for the pod to be created (knowing that it will never become ready)
				_, err := waitFor[corev1.Pod]("pods/issue-801-workspace-0", "issue-801", 5*time.Minute, "create")
				require.NoError(t, err)

				// update the workspace to a valid image (expecting that a new pod will be rolled out)
				cmd = exec.Command("kubectl", "apply", "-f", "e2e/testdata/issue-801/step2")
				require.NoError(t, run(cmd))

				// wait for the workspace to be fully ready
				_, err = waitFor[autov1alpha1.Workspace]("workspaces/issue-801", "issue-801", 5*time.Minute, "condition=Ready")
				require.NoError(t, err)
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
					require.NoError(t, err)
					return found
				})

				// Ensure the Workspace is in a failed state with Unauthenticated.
				_, err := waitFor[pulumiv1.Stack](
					"workspaces/random-yaml-auth-error",
					"random-yaml-auth-error",
					5*time.Minute,
					`jsonpath={.status.conditions[?(@.type=="Ready")].reason}=Unauthenticated`)
				require.NoError(t, err)

				// Ensure that the workspace pod was not deleted after reconciling the failed stack.
				time.Sleep(10 * time.Second)
				found, err := foundEvent("Pod", "random-yaml-auth-error-workspace-0", "random-yaml-auth-error", "Killing")
				require.NoError(t, err)
				assert.False(t, found)
			},
		},
		{
			name: "issue-937",
			f: func(t *testing.T) {
				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/issue-937")
				require.NoError(t, run(cmd))
				dumpLogs(t, "issue-937", "pod/issue-937-workspace-0")

				_, err := waitFor[pulumiv1.Stack]("stacks/issue-937", "issue-937", 5*time.Minute, "condition=Ready")
				require.NoError(t, err)
			},
		},
		{
			name: "structured-config",
			f: func(t *testing.T) {
				// Test comprehensive structured configuration including:
				// - Inline JSON config (objects, arrays, numbers, booleans)
				// - JSON config from ConfigMap
				// - Mixed string and JSON config
				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/structured-config")
				require.NoError(t, run(cmd))
				dumpLogs(t, "structured-config", "pod/structured-config-workspace-0")
				dumpEvents(t, "structured-config")

				stack, err := waitFor[pulumiv1.Stack](
					"stacks/structured-config",
					"structured-config",
					5*time.Minute,
					"condition=Ready")
				require.NoError(t, err)

				// Verify the stack is ready and outputs are available
				assert.NotNil(t, stack)
				assert.NotEmpty(t, stack.Status.Outputs)

				// Helper to unmarshal JSON output
				getOutput := func(key string) interface{} {
					var val interface{}
					err := json.Unmarshal(stack.Status.Outputs[key].Raw, &val)
					require.NoError(t, err, "failed to unmarshal output %s", key)
					return val
				}

				// Verify inline string value
				assert.Equal(t, "hello-world", getOutput("simpleString"))

				// Verify inline object value (databaseConfig)
				databaseConfig := getOutput("databaseConfig").(map[string]interface{})
				assert.Equal(t, "db.example.com", databaseConfig["host"])
				assert.Equal(t, float64(5432), databaseConfig["port"])
				assert.Equal(t, "myapp", databaseConfig["database"])
				assert.Equal(t, true, databaseConfig["ssl"])
				assert.Equal(t, float64(100), databaseConfig["maxConnections"])

				// Verify inline array value (allowedRegions)
				allowedRegions := getOutput("allowedRegions").([]interface{})
				assert.Len(t, allowedRegions, 3)
				assert.Equal(t, "us-west-2", allowedRegions[0])
				assert.Equal(t, "us-east-1", allowedRegions[1])
				assert.Equal(t, "eu-west-1", allowedRegions[2])

				// Verify inline number values
				assert.Equal(t, float64(3), getOutput("maxRetries"))
				assert.Equal(t, 30.5, getOutput("timeout"))

				// Verify inline boolean values
				assert.Equal(t, true, getOutput("enableCaching"))
				assert.Equal(t, false, getOutput("debugMode"))

				// Verify nested object (oauth)
				oauth := getOutput("oauth").(map[string]interface{})
				assert.Equal(t, "my-client-id", oauth["clientId"])

				scopes := oauth["scopes"].([]interface{})
				assert.Len(t, scopes, 2)
				assert.Equal(t, "read", scopes[0])
				assert.Equal(t, "write", scopes[1])

				endpoints := oauth["endpoints"].(map[string]interface{})
				assert.Equal(t, "https://auth.example.com/token", endpoints["token"])
				assert.Equal(t, "https://auth.example.com/authorize", endpoints["authorize"])

				// Verify JSON config from ConfigMap (appSettings)
				appSettings := getOutput("appSettings").(map[string]interface{})
				assert.Equal(t, "https://api.example.com", appSettings["apiEndpoint"])
				assert.Equal(t, float64(30), appSettings["timeout"])
				assert.Equal(t, true, appSettings["retryEnabled"])

				features := appSettings["features"].(map[string]interface{})
				assert.Equal(t, true, features["authentication"])
				assert.Equal(t, float64(1000), features["rateLimit"])

				regions := appSettings["regions"].([]interface{})
				assert.Len(t, regions, 2)
				assert.Equal(t, "us-west-1", regions[0])
				assert.Equal(t, "us-east-1", regions[1])

				// Verify plain text from ConfigMap (appVersion)
				assert.Equal(t, "v1.2.3", getOutput("appVersion"))
			},
		},
		{
			name: "structured-config-version",
			f: func(t *testing.T) {
				// Test version compatibility:
				// 1. Deploy stack with structured config using old Pulumi image (< v3.202.0)
				// 2. Verify stack enters Stalled state
				// 3. Update to new image (>= v3.202.0)
				// 4. Verify stack transitions to Ready state

				cmd := exec.Command("kubectl", "apply", "-f", "e2e/testdata/structured-config-version")
				require.NoError(t, run(cmd))
				dumpLogs(t, "structured-config-version", "pod/structured-config-version-workspace-0")
				dumpEvents(t, "structured-config-version")

				// Wait for the Stack to enter Stalled state due to old Pulumi version
				_, err := waitFor[pulumiv1.Stack](
					"stacks/structured-config-version",
					"structured-config-version",
					5*time.Minute,
					"condition=Stalled",
					`jsonpath={.status.conditions[?(@.type=="Stalled")].reason}=PulumiVersionTooLow`)
				require.NoError(t, err)

				// Update the Stack to use a newer Pulumi image
				cmd = exec.Command("kubectl", "apply", "-f", "e2e/testdata/structured-config-version/step2")
				require.NoError(t, run(cmd))

				// Wait for the workspace to be ready with the new image
				_, err = waitFor[autov1alpha1.Workspace](
					"workspaces/structured-config-version",
					"structured-config-version",
					5*time.Minute,
					"condition=Ready")
				require.NoError(t, err)

				// Wait for the Stack to transition to Ready state
				stack, err := waitFor[pulumiv1.Stack](
					"stacks/structured-config-version",
					"structured-config-version",
					5*time.Minute,
					"condition=Ready")
				require.NoError(t, err)

				// Verify the stack is now ready
				assert.NotNil(t, stack)
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
		require.NoError(t, err)
		t.Log(string(out))
	})
}

func dumpEvents(t *testing.T, namespace string) {
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		t.Logf("=== EVENTS %s", namespace)
		cmd := exec.Command("kubectl", "get", "events", "-n", namespace)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err)
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

// TestSSAFinalizerOwnership tests that SSA correctly handles finalizer ownership
// when multiple controllers add finalizers to the same object. This verifies that
// removing our finalizer via SSA (by applying with empty finalizers) only removes
// the finalizer owned by our field manager, leaving other controllers' finalizers intact.
func TestSSAFinalizerOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Build config from default kubeconfig location
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	require.NoError(t, err, "failed to load kubeconfig")

	testScheme := scheme.Scheme
	require.NoError(t, autov1alpha1.AddToScheme(testScheme))

	k8sClient, err := client.New(config, client.Options{Scheme: testScheme})
	require.NoError(t, err, "failed to create Kubernetes client")

	ctx := context.Background()

	update := &autov1alpha1.Update{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ssa-finalizer-" + utilrand.String(5),
			Namespace: "default",
		},
		Spec: autov1alpha1.UpdateSpec{
			StackName:     "test-stack",
			WorkspaceName: "test-workspace",
		},
	}

	t.Cleanup(func() {
		cleanupUpdate := &autov1alpha1.Update{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: update.Name, Namespace: update.Namespace}, cleanupUpdate); err == nil {
			cleanupUpdate.Finalizers = nil
			_ = k8sClient.Update(ctx, cleanupUpdate)
			_ = k8sClient.Delete(ctx, cleanupUpdate)
		}
	})

	require.NoError(t, k8sClient.Create(ctx, update))

	// Add our finalizer via SSA
	ourPatch := autov1alpha1apply.Update(update.Name, update.Namespace).
		WithFinalizers(pulumi.PulumiFinalizer)
	require.NoError(t, k8sClient.Patch(ctx, update, applyPatch{ourPatch},
		client.FieldOwner("pulumi-kubernetes-operator/stack-finalizer")))

	// Simulate another controller adding its finalizer via SSA
	otherPatch := autov1alpha1apply.Update(update.Name, update.Namespace).
		WithFinalizers("other.finalizer.com")
	require.NoError(t, k8sClient.Patch(ctx, update, applyPatch{otherPatch},
		client.FieldOwner("other-controller")))

	// Verify setup: both finalizers are present
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: update.Name, Namespace: update.Namespace}, update))
	require.Contains(t, update.Finalizers, pulumi.PulumiFinalizer, "setup: Pulumi finalizer should be present")
	require.Contains(t, update.Finalizers, "other.finalizer.com", "setup: other finalizer should be present")

	t.Logf("Before removal - Finalizers: %v", update.Finalizers)

	// Remove our finalizer by applying with empty finalizers (releasing ownership)
	emptyPatch := autov1alpha1apply.Update(update.Name, update.Namespace)
	require.NoError(t, k8sClient.Patch(ctx, update, applyPatch{emptyPatch},
		client.FieldOwner("pulumi-kubernetes-operator/stack-finalizer")))
	// Verify only our finalizer is removed
	updated := &autov1alpha1.Update{}
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: update.Name, Namespace: update.Namespace}, updated))

	t.Logf("After removal - Finalizers: %v", updated.Finalizers)

	assert.NotContains(t, updated.Finalizers, pulumi.PulumiFinalizer, "Pulumi finalizer should be removed")
	assert.Contains(t, updated.Finalizers, "other.finalizer.com", "other finalizer should be preserved")
}

// applyPatch implements client.Patch for server-side apply.
type applyPatch struct {
	patch interface{}
}

func (p applyPatch) Type() types.PatchType {
	return types.ApplyPatchType
}

func (p applyPatch) Data(obj client.Object) ([]byte, error) {
	return json.Marshal(p.patch)
}
