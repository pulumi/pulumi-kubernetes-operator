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
const timeout = time.Minute * 10
const interval = time.Second * 1

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
		}, timeout, interval).Should(BeTrue())

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
		}, timeout, interval).Should(BeTrue())

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
		}, timeout, interval).Should(BeTrue())
	})

	AfterEach(func() {
		By("Deleting left over stacks")
		deletionPolicy := metav1.DeletePropagationForeground
		Expect(k8sClient.DeleteAllOf(
			ctx,
			&pulumiv1alpha1.Stack{},
			client.InNamespace(namespace),
			&client.DeleteAllOfOptions{
				DeleteOptions: client.DeleteOptions{PropagationPolicy: &deletionPolicy},
			},
		)).Should(Succeed())

		Eventually(func() bool {
			var stacksList pulumiv1alpha1.StackList
			if err = k8sClient.List(ctx, &stacksList, client.InNamespace(namespace)); err != nil {
				return false
			}
			return len(stacksList.Items) == 0
		}, timeout, interval).Should(BeTrue())

		if pulumiAPISecret != nil {
			By("Deleting the Stack Pulumi API Secret")
			Expect(k8sClient.Delete(ctx, pulumiAPISecret)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: pulumiAPISecret.Name, Namespace: namespace}, pulumiAPISecret)
				return k8serrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
		if pulumiAWSSecret != nil {
			By("Deleting the Stack AWS Credentials Secret")
			Expect(k8sClient.Delete(ctx, pulumiAWSSecret)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: pulumiAWSSecret.Name, Namespace: namespace}, pulumiAWSSecret)
				return k8serrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
		if passphraseSecret != nil {
			By("Deleting the Passphrase Secret")
			Expect(k8sClient.Delete(ctx, passphraseSecret)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: passphraseSecret.Name, Namespace: namespace}, passphraseSecret)
				return k8serrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
	})

	var toDelete []string

	defer func() {
		for _, d := range toDelete {
			_ = os.RemoveAll(d)
		}
	}()

	// Tip: avoid adding tests for vanilla CRUD operations because they would
	// test the Kubernetes API server, which isn't the goal here.

	It("Should deploy a simple stack locally successfully", func() {
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
		localSpec := pulumiv1alpha1.StackSpec{
			Backend:         fmt.Sprintf("file://%s", backendDir),
			Stack:           stackName,
			ProjectRepo:     baseDir,
			RepoDir:         "test/testdata/empty-stack",
			Commit:          commit,
			SecretsProvider: "passphrase",
			SecretEnvs: []string{
				passphraseSecret.Name,
			},
			Refresh: true,
		}

		// Create the stack
		name := "local-backend-stack"
		stack = generateStack(name, namespace, localSpec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		fetched := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}

			if fetched.Status.LastUpdate != nil {
				return fetched.Status.LastUpdate.LastSuccessfulCommit != "" &&
					fetched.Status.LastUpdate.LastAttemptedCommit != "" &&
					fetched.Status.LastUpdate.LastSuccessfulCommit == fetched.Status.LastUpdate.LastAttemptedCommit &&
					fetched.Status.LastUpdate.State == pulumiv1alpha1.SucceededStackStateMessage
			}

			return false
		}, timeout, interval).Should(BeTrue())
		// Validate outputs.
		Expect(fetched.Status.Outputs).Should(BeEquivalentTo(pulumiv1alpha1.StackOutputs{
			"region":      v1.JSON{Raw: []byte(`"us-west-2"`)},
			"notSoSecret": v1.JSON{Raw: []byte(`"safe"`)},
			"secretVal":   v1.JSON{Raw: []byte(`"[secret]"`)},
		}))

		// Delete the Stack
		toDelete := &pulumiv1alpha1.Stack{}
		By(fmt.Sprintf("Deleting the Stack: %s", stack.Name))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, toDelete)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)
			return k8serrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
	})

	It("Should deploy an AWS S3 Stack successfully, then deploy a commit update successfully", func() {
		var stack *pulumiv1alpha1.Stack
		stackName := fmt.Sprintf("%s/s3-op-project/dev-commit-change-%s", stackOrg, randString())
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)

		// Define the stack spec
		spec := pulumiv1alpha1.StackSpec{
			AccessTokenSecret: pulumiAPISecret.Name,
			SecretEnvs: []string{
				pulumiAWSSecret.Name,
			},
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Refresh:           true,
			Stack:             stackName,
			ProjectRepo:       baseDir,
			RepoDir:           "test/testdata/test-s3-op-project",
			Commit:            commit,
			DestroyOnFinalize: true,
		}

		// Create stack
		name := "stack-test-aws-s3-commit-change"
		stack = generateStack(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		original := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, original)
			if err != nil {
				return false
			}
			return stackUpdatedToCommit(original, stack.Spec.Commit)
		}, timeout, interval).Should(BeTrue())

		// Update the stack config (this time to cause a failure)
		original.Spec.Config["aws:region"] = "us-nonexistent-1"
		Expect(k8sClient.Update(ctx, original)).Should(Succeed(), "%+v", original)

		// Check that the stack tried to update but failed
		configChanged := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, configChanged)
			if err != nil {
				return false
			}
			if configChanged.Status.LastUpdate != nil {
				return configChanged.Status.LastUpdate.LastSuccessfulCommit == stack.Spec.Commit &&
					configChanged.Status.LastUpdate.LastAttemptedCommit == stack.Spec.Commit &&
					configChanged.Status.LastUpdate.State == pulumiv1alpha1.FailedStackStateMessage
			}
			return false
		})

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
		}, timeout, interval).Should(BeTrue(), "%#v", configChanged)

		// Check that the stack update was attempted but failed
		fetched := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			if fetched.Status.LastUpdate != nil {
				return fetched.Status.LastUpdate.LastSuccessfulCommit == stack.Spec.Commit &&
					fetched.Status.LastUpdate.LastAttemptedCommit == commit &&
					fetched.Status.LastUpdate.State == pulumiv1alpha1.FailedStackStateMessage
			}
			return false
		}, timeout, interval).Should(BeTrue())

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
		}, timeout, interval).Should(BeTrue())

		// Check that the stack update attempted and succeeded after the region fix
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			return stackUpdatedToCommit(fetched, commit)
		}, timeout, interval).Should(BeTrue())

		// Delete the Stack
		toDelete := &pulumiv1alpha1.Stack{}
		By(fmt.Sprintf("Deleting the Stack: %s", stack.Name))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, toDelete)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)
			return k8serrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
	})

	It("Should deploy an AWS S3 Stack successfully with credentials passed through EnvRefs", func() {
		var stack *pulumiv1alpha1.Stack
		stackName := fmt.Sprintf("%s/s3-op-project/dev-env-ref-%s", stackOrg, randString())
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)

		// Write a secret to a temp directory. This is a stand-in for other mechanisms to reify the secrets
		// on the file system. This is not a recommended way to store/pass secrets.
		Eventually(func() bool {
			if err = os.WriteFile(filepath.Join(secretsDir, pulumiAPISecretName), []byte(pulumiAccessToken), 0600); err != nil {
				return false
			}
			return true
		}, timeout, interval).Should(BeTrue())

		// Define the stack spec
		spec := pulumiv1alpha1.StackSpec{
			// Cover all variations of resource refs
			EnvRefs: map[string]pulumiv1alpha1.ResourceRef{
				"PULUMI_ACCESS_TOKEN":   pulumiv1alpha1.NewFileSystemResourceRef(filepath.Join(secretsDir, pulumiAPISecretName)),
				"AWS_ACCESS_KEY_ID":     pulumiv1alpha1.NewLiteralResourceRef(awsAccessKeyID),
				"AWS_SECRET_ACCESS_KEY": pulumiv1alpha1.NewSecretResourceRef(namespace, pulumiAWSSecret.Name, "AWS_SECRET_ACCESS_KEY"),
				"AWS_SESSION_TOKEN":     pulumiv1alpha1.NewEnvResourceRef("AWS_SESSION_TOKEN"),
			},
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Stack:             stackName,
			ProjectRepo:       baseDir,
			RepoDir:           "test/testdata/test-s3-op-project",
			Commit:            commit,
			DestroyOnFinalize: true,
		}
		fmt.Printf("ProjectRepo: %q\n", spec.RepoDir)

		// Create stack
		name := "stack-test-aws-s3-file-secrets"
		stack = generateStack(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		Eventually(func() bool {
			fetched := &pulumiv1alpha1.Stack{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			return stackUpdatedToCommit(fetched, stack.Spec.Commit)
		}, timeout, interval).Should(BeTrue())
	})

	It("Should deploy an AWS S3 Stack successfully using S3 backend", func() {
		var stack *pulumiv1alpha1.Stack

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
		spec := pulumiv1alpha1.StackSpec{
			AccessTokenSecret: pulumiAPISecret.Name,
			SecretEnvs: []string{
				pulumiAWSSecret.Name,
			},
			Backend:         fmt.Sprintf(`s3://%s`, s3Backend),
			SecretsProvider: fmt.Sprintf(`awskms:///%s?region=us-east-2`, kmsKey),
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Refresh:           true,
			Stack:             stackName,
			ProjectRepo:       baseDir,
			RepoDir:           "test/testdata/test-s3-op-project",
			Commit:            commit,
			DestroyOnFinalize: true,
		}

		// Create stack
		name := "stack-test-aws-s3-s3backend"
		stack = generateStack(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		initial := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, initial)
			if err != nil {
				return false
			}
			return stackUpdatedToCommit(initial, stack.Spec.Commit)
		}, timeout, interval).Should(BeTrue())

		// Check that secrets are not leaked
		Expect(initial.Status.Outputs).Should(HaveKeyWithValue(
			"bucketsAsSecrets", v1.JSON{Raw: []byte(`"[secret]"`)}))

		// Delete the Stack
		toDelete := &pulumiv1alpha1.Stack{}
		By(fmt.Sprintf("Deleting the Stack: %s", stack.Name))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, toDelete)).Should(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)
			return k8serrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
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

func stackUpdatedToCommit(stack *pulumiv1alpha1.Stack, commit string) bool {
	if stack.Status.LastUpdate != nil {
		return stack.Status.LastUpdate.LastSuccessfulCommit == commit &&
			stack.Status.LastUpdate.LastAttemptedCommit == commit &&
			stack.Status.LastUpdate.State == pulumiv1alpha1.SucceededStackStateMessage
	}
	return false
}

func generateStack(name, namespace string, spec pulumiv1alpha1.StackSpec) *pulumiv1alpha1.Stack {
	return &pulumiv1alpha1.Stack{
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
