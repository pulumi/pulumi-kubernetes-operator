// Copyright 2021, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"encoding/base32"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	"gopkg.in/src-d/go-git.v4"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	pulumiAccessToken  = ""
	awsAccessKeyID     = ""
	awsSecretAccessKey = ""
	awsSessionToken    = ""

	baseDir = ""
)

const namespace = "default"
const pulumiAPISecretName = "pulumi-api-secret"
const pulumiAWSSecretName = "pulumi-aws-secrets"
const k8sOpTimeout = 10 * time.Second
const stackExecTimeout = 3 * time.Minute
const interval = time.Second * 5

var _ = Describe("Stack Controller", func() {
	whoami, err := exec.Command("pulumi", "whoami").Output()
	if err != nil {
		Fail(fmt.Sprintf("Must get pulumi authorized user: %v", err))
	}
	stackOrg := strings.TrimSuffix(string(whoami), "\n")
	fmt.Printf("stackOrg: %s\n", stackOrg)

	_, path, _, ok := runtime.Caller(0)
	if !ok {
		Fail("Failed to determine current directory")
	}
	// Absolute path to base directory for the repository locally.
	baseDir = filepath.FromSlash(filepath.Join(path, "..", ".."))
	fmt.Printf("Basedir: %s\n", baseDir)
	commit, err := getCurrentCommit(baseDir)
	if err != nil {
		fmt.Printf("%+v", err)
		Fail(fmt.Sprintf("Couldn't resolve current commit: %v", err))
	}

	fmt.Printf("Current commit is: %s\n", commit)
	ctx := context.Background()
	var pulumiAPISecret *corev1.Secret
	var pulumiAWSSecret *corev1.Secret
	var passphraseSecret *corev1.Secret

	BeforeEach(func() {
		// Get the AWS and Pulumi access envvars.
		awsAccessKeyID, awsSecretAccessKey, awsSessionToken = os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_SESSION_TOKEN")
		if awsAccessKeyID == "" || awsSecretAccessKey == "" {
			Fail("Missing environment variable required for tests. AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must both be set. AWS_SESSION_TOKEN is optionally set.")
		}

		pulumiAccessToken = os.Getenv("PULUMI_ACCESS_TOKEN")
		if pulumiAccessToken == "" {
			Fail("Missing environment variable required for tests. PULUMI_ACCESS_TOKEN must be set.")
		}

		// Create the Pulumi API k8s secret
		pulumiAPISecret = generateSecret(pulumiAPISecretName, namespace,
			map[string][]byte{
				"accessToken": []byte(pulumiAccessToken),
			},
		)
		Expect(k8sClient.Create(ctx, pulumiAPISecret)).Should(Succeed())

		// Check that the Pulumi API secret created
		fetched := &corev1.Secret{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: pulumiAPISecret.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			return !fetched.CreationTimestamp.IsZero() && fetched.Data != nil
		}, k8sOpTimeout, interval).Should(BeTrue())

		// Create the Pulumi AWS k8s secret
		pulumiAWSSecret = generateSecret(pulumiAWSSecretName, namespace,
			map[string][]byte{
				"AWS_ACCESS_KEY_ID":     []byte(awsAccessKeyID),
				"AWS_SECRET_ACCESS_KEY": []byte(awsSecretAccessKey),
				"AWS_SESSION_TOKEN":     []byte(awsSessionToken),
			},
		)
		Expect(k8sClient.Create(ctx, pulumiAWSSecret)).Should(Succeed())

		// Check that the Pulumi AWS creds secret created
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: pulumiAWSSecret.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			return !fetched.CreationTimestamp.IsZero() && fetched.Data != nil
		}, k8sOpTimeout, interval).Should(BeTrue())

		// Create the passphrase secret
		passphraseSecret = generateSecret("passphrase-secret", namespace,
			map[string][]byte{
				"PULUMI_CONFIG_PASSPHRASE": []byte("the quick brown fox jumps over the lazy dog"),
			},
		)
		Expect(k8sClient.Create(ctx, passphraseSecret)).Should(Succeed())

		// Check that the passphrase secret was created
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: passphraseSecret.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			return !fetched.CreationTimestamp.IsZero() && fetched.Data != nil
		}, k8sOpTimeout, interval).Should(BeTrue())
	})

	AfterEach(func() {
		By("Deleting left over stacks")
		Expect(k8sClient.DeleteAllOf(
			ctx,
			&pulumiv1alpha1.Stack{},
			client.InNamespace(namespace),
			&client.DeleteAllOfOptions{},
		)).Should(Succeed())

		Eventually(func() bool {
			var stacksList pulumiv1alpha1.StackList
			if err = k8sClient.List(ctx, &stacksList, client.InNamespace(namespace)); err != nil {
				return false
			}
			return len(stacksList.Items) == 0
		}, stackExecTimeout, interval).Should(BeTrue()) // stacks will be finalized, so allow time for that to happen

		if pulumiAPISecret != nil {
			By("Deleting the Stack Pulumi API Secret")
			Expect(k8sClient.Delete(ctx, pulumiAPISecret)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: pulumiAPISecret.Name, Namespace: namespace}, pulumiAPISecret)
				return k8serrors.IsNotFound(err)
			}, k8sOpTimeout, interval).Should(BeTrue())
		}
		if pulumiAWSSecret != nil {
			By("Deleting the Stack AWS Credentials Secret")
			Expect(k8sClient.Delete(ctx, pulumiAWSSecret)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: pulumiAWSSecret.Name, Namespace: namespace}, pulumiAWSSecret)
				return k8serrors.IsNotFound(err)
			}, k8sOpTimeout, interval).Should(BeTrue())
		}
		if passphraseSecret != nil {
			By("Deleting the Passphrase Secret")
			Expect(k8sClient.Delete(ctx, passphraseSecret)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: passphraseSecret.Name, Namespace: namespace}, passphraseSecret)
				return k8serrors.IsNotFound(err)
			}, k8sOpTimeout, interval).Should(BeTrue())
		}
	})

	var toDelete []string

	defer func() {
		for _, d := range toDelete {
			_ = os.RemoveAll(d)
		}
	}()

	It("Should deploy a simple stack locally successfully", func() {
		// Uses v1alpha1 to both create and fetch
		var stack *pulumiv1alpha1.Stack
		// Use a local backend for this test.
		// Local backend doesn't allow setting slashes in stack name.
		const stackName = "dev"
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)

		// Tests run from outside the container. Using the safest existing directory to store
		// local state for the sake of this test.
		backendDir, err := ioutil.TempDir("", "local-state")
		Ω(err).ShouldNot(HaveOccurred())
		toDelete = append(toDelete, backendDir)

		// Define the stack spec
		localSpec := shared.StackSpec{
			Backend: fmt.Sprintf("file://%s", backendDir),
			Stack:   stackName,
			GitSource: &shared.GitSource{
				ProjectRepo: baseDir,
				RepoDir:     "test/testdata/empty-stack",
				Commit:      commit,
			},
			SecretsProvider: "passphrase",
			SecretEnvs: []string{
				passphraseSecret.Name,
			},
			Refresh: true,
		}

		// Create the stack
		name := "local-backend-stack"
		stack = generateStackV1Alpha1(name, namespace, localSpec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		fetched := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}

			return stackUpdatedToCommit(fetched.Status.LastUpdate, stack.Spec.Commit)
		}, stackExecTimeout, interval).Should(BeTrue())
		// Validate outputs.
		Expect(fetched.Status.Outputs).Should(BeEquivalentTo(shared.StackOutputs{
			"region":       v1.JSON{Raw: []byte(`"us-west-2"`)},
			"notSoSecret":  v1.JSON{Raw: []byte(`"safe"`)},
			"secretVal":    v1.JSON{Raw: []byte(`"[secret]"`)},
			"nestedSecret": v1.JSON{Raw: []byte(`"[secret]"`)},
		}))

		// Delete the Stack
		toDelete := &pulumiv1alpha1.Stack{}
		By(fmt.Sprintf("Deleting the Stack: %s", stack.Name))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, toDelete)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)
			return k8serrors.IsNotFound(err)
		}, stackExecTimeout, interval).Should(BeTrue()) // allow time for finalizer to run
	})

	It("Should deploy an AWS S3 Stack successfully, then deploy a commit update successfully", func() {
		// Use v1alpha1 to create and v1 to fetch
		var stack *pulumiv1alpha1.Stack
		stackName := fmt.Sprintf("%s/s3-op-project/dev-commit-change-%s", stackOrg, randString())
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)

		// Define the stack spec
		spec := shared.StackSpec{
			AccessTokenSecret: pulumiAPISecret.Name,
			SecretEnvs: []string{
				pulumiAWSSecret.Name,
			},
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Stack: stackName,
			GitSource: &shared.GitSource{
				ProjectRepo: baseDir,
				RepoDir:     "test/testdata/test-s3-op-project",
				Commit:      commit,
			},
			DestroyOnFinalize: true,
		}

		// Create stack
		name := "stack-test-aws-s3-commit-change"
		stack = generateStackV1Alpha1(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		original := &pulumiv1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, original)
			if err != nil {
				return false
			}
			return stackUpdatedToCommit(original.Status.LastUpdate, stack.Spec.Commit)
		}, stackExecTimeout, interval).Should(BeTrue())

		// Update the stack config (this time to cause a failure)
		original.Spec.Config["aws:region"] = "us-nonexistent-1"
		Expect(k8sClient.Update(ctx, original)).Should(Succeed(), "%+v", original)

		// Check that the stack tried to update but failed
		configChanged := &pulumiv1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, configChanged)
			if err != nil {
				return false
			}
			if configChanged.Status.LastUpdate != nil {
				return configChanged.Status.LastUpdate.LastSuccessfulCommit == stack.Spec.Commit &&
					configChanged.Status.LastUpdate.LastAttemptedCommit == stack.Spec.Commit &&
					configChanged.Status.LastUpdate.State == shared.FailedStackStateMessage
			}
			return false
		}, stackExecTimeout, interval).Should(BeTrue())

		// Update the stack commit to a different commit. Need retries because of
		// competing retries within the operator due to failure.
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, configChanged); err != nil {
				return false
			}
			configChanged.Spec.Commit = commit
			if err := k8sClient.Update(ctx, configChanged); err != nil {
				return false
			}
			return true
		}, stackExecTimeout, interval).Should(BeTrue(), "%#v", configChanged)

		// Check that the stack update was attempted but failed
		fetched := &pulumiv1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			if fetched.Status.LastUpdate != nil {
				return fetched.Status.LastUpdate.LastSuccessfulCommit == stack.Spec.Commit &&
					fetched.Status.LastUpdate.LastAttemptedCommit == commit &&
					fetched.Status.LastUpdate.State == shared.FailedStackStateMessage
			}
			return false
		}, stackExecTimeout, interval).Should(BeTrue())

		Eventually(func() bool {
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched); err != nil {
				return false
			}
			// Update the stack config to now be valid
			fetched.Spec.Config["aws:region"] = "us-east-2"
			if err := k8sClient.Update(ctx, fetched); err != nil {
				return false
			}
			return true
		}, k8sOpTimeout, interval).Should(BeTrue())

		// Check that the stack update attempted and succeeded after the region fix
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			return stackUpdatedToCommit(fetched.Status.LastUpdate, commit)
		}, stackExecTimeout, interval).Should(BeTrue())

		// Delete the Stack
		toDelete := &pulumiv1.Stack{}
		By(fmt.Sprintf("Deleting the Stack: %s", stack.Name))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, toDelete)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)
			return k8serrors.IsNotFound(err)
		}, stackExecTimeout, interval).Should(BeTrue())
	})

	It("Should deploy an AWS S3 Stack successfully with credentials passed through EnvRefs", func() {
		// Use v1 to create and v1alpha1 to fetch
		var stack *pulumiv1.Stack
		stackName := fmt.Sprintf("%s/s3-op-project/dev-env-ref-%s", stackOrg, randString())
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)

		// Write a secret to a temp directory. This is a stand-in for other mechanisms to reify the secrets
		// on the file system. This is not a recommended way to store/pass secrets.
		Expect(os.WriteFile(filepath.Join(secretsDir, pulumiAPISecretName), []byte(pulumiAccessToken), 0600)).To(Succeed())

		// Define the stack spec
		spec := shared.StackSpec{
			// Cover all variations of resource refs
			EnvRefs: map[string]shared.ResourceRef{
				"PULUMI_ACCESS_TOKEN":   shared.NewFileSystemResourceRef(filepath.Join(secretsDir, pulumiAPISecretName)),
				"AWS_ACCESS_KEY_ID":     shared.NewLiteralResourceRef(awsAccessKeyID),
				"AWS_SECRET_ACCESS_KEY": shared.NewSecretResourceRef(namespace, pulumiAWSSecret.Name, "AWS_SECRET_ACCESS_KEY"),
				"AWS_SESSION_TOKEN":     shared.NewEnvResourceRef("AWS_SESSION_TOKEN"),
			},
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Stack: stackName,
			GitSource: &shared.GitSource{
				ProjectRepo: baseDir,
				RepoDir:     "test/testdata/test-s3-op-project",
				Commit:      commit,
			},
			DestroyOnFinalize: true,
		}
		fmt.Printf("ProjectRepo: %q\n", spec.RepoDir)

		// Create stack
		name := "stack-test-aws-s3-file-secrets"
		stack = generateStackV1(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		Eventually(func() bool {
			fetched := &pulumiv1alpha1.Stack{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			return stackUpdatedToCommit(fetched.Status.LastUpdate, stack.Spec.Commit)
		}, stackExecTimeout, interval).Should(BeTrue())
	})

	It("Should deploy an AWS S3 Stack successfully using S3 backend", func() {
		// Use v1 to create and fetch
		var stack *pulumiv1.Stack

		s3Backend, ok := os.LookupEnv("PULUMI_S3_BACKEND_BUCKET")
		if !ok {
			Skip("No S3 backend bucket set in env variable: PULUMI_S3_BACKEND_BUCKET.")
		}
		kmsKey, ok := os.LookupEnv("PULUMI_KMS_KEY")
		if !ok {
			Skip("No KMS Key specified in env variable: PULUMI_KMS_KEY")
		}

		stackName := "s3backend.s3-op-project"
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)

		// Define the stack spec
		spec := shared.StackSpec{
			EnvRefs: map[string]shared.ResourceRef{
				"AWS_ACCESS_KEY_ID": {
					SelectorType: shared.ResourceSelectorSecret,
					ResourceSelector: shared.ResourceSelector{
						SecretRef: &shared.SecretSelector{
							Name:      pulumiAWSSecret.Name,
							Namespace: pulumiAWSSecret.Namespace,
							Key:       "AWS_ACCESS_KEY_ID",
						},
					},
				},
				"AWS_SECRET_ACCESS_KEY": {
					SelectorType: shared.ResourceSelectorSecret,
					ResourceSelector: shared.ResourceSelector{
						SecretRef: &shared.SecretSelector{
							Name:      pulumiAWSSecret.Name,
							Namespace: pulumiAWSSecret.Namespace,
							Key:       "AWS_SECRET_ACCESS_KEY",
						},
					},
				},
				"AWS_SESSION_TOKEN": {
					SelectorType: shared.ResourceSelectorSecret,
					ResourceSelector: shared.ResourceSelector{
						SecretRef: &shared.SecretSelector{
							Name:      pulumiAWSSecret.Name,
							Namespace: pulumiAWSSecret.Namespace,
							Key:       "AWS_SESSION_TOKEN",
						},
					},
				},
			},
			Backend:         fmt.Sprintf(`s3://%s`, s3Backend),
			SecretsProvider: fmt.Sprintf(`awskms:///%s?region=us-east-2`, kmsKey),
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Refresh: true,
			Stack:   stackName,
			GitSource: &shared.GitSource{
				ProjectRepo: baseDir,
				RepoDir:     "test/testdata/test-s3-op-project",
				Commit:      commit,
			},
			DestroyOnFinalize: true,
		}

		// Create stack
		name := "stack-test-aws-s3-s3backend"
		stack = generateStackV1(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		initial := &pulumiv1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, initial)
			if err != nil {
				return false
			}
			return stackUpdatedToCommit(initial.Status.LastUpdate, stack.Spec.Commit)
		}, stackExecTimeout, interval).Should(BeTrue())

		// Check that secrets are not leaked
		Expect(initial.Status.Outputs).Should(HaveKeyWithValue(
			"bucketsAsSecrets", v1.JSON{Raw: []byte(`"[secret]"`)}))

		// Delete the Stack
		toDelete := &pulumiv1.Stack{}
		By(fmt.Sprintf("Deleting the Stack: %s", stack.Name))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, toDelete)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)
			return k8serrors.IsNotFound(err)
		}, stackExecTimeout, interval).Should(BeTrue())
	})
})

func getCurrentCommit(path string) (string, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}

	ref, err := r.Head()
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

func stackUpdatedToCommit(lastUpdate *shared.StackUpdateState, commit string) bool {
	if lastUpdate != nil {
		return lastUpdate.LastSuccessfulCommit == commit &&
			lastUpdate.LastAttemptedCommit == commit &&
			lastUpdate.State == shared.SucceededStackStateMessage
	}
	return false
}

func generateStackV1Alpha1(name, namespace string, spec shared.StackSpec) *pulumiv1alpha1.Stack {
	return &pulumiv1alpha1.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{name, randString()}, "-"),
			Namespace: namespace,
		},
		Spec: spec,
	}
}

func generateStackV1(name, namespace string, spec shared.StackSpec) *pulumiv1.Stack {
	return &pulumiv1.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{name, randString()}, "-"),
			Namespace: namespace,
		},
		Spec: spec,
	}
}

func generateSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{name, randString()}, "-"),
			Namespace: namespace,
		},
		Data: data,
		Type: "Opaque",
	}
}

func randString() string {
	rand.Seed(time.Now().UnixNano())
	c := 10
	b := make([]byte, c)
	rand.Read(b)
	length := 6
	return strings.ToLower(base32.StdEncoding.EncodeToString(b)[:length])
}
