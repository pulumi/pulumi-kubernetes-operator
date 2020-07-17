package tests

import (
	"context"
	"encoding/base32"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var pulumiAccessToken = ""
var awsAccessKeyID = ""
var awsSecretAccessKey = ""
var awsSessionToken = ""
var namespace = "default"

const pulumiAPISecretName = "pulumi-api-secret"
const pulumiAWSSecretsName = "pulumi-aws-secrets"
const timeout = time.Minute * 10
const interval = time.Second * 1

var _ = Describe("Stack Controller", func() {
	ctx := context.Background()

	BeforeEach(func() {
		// failed test runs that don't clean up leave resources behind.
	})

	AfterEach(func() {
	})

	// Avoid adding tests for vanilla CRUD operations because they would
	// test Kubernetes API server, which isn't the goal here.

	It("Should create the Stack CR and deploy a successful update", func() {
		var stack *pulumiv1alpha1.Stack
		var pulumiAPISecret *corev1.Secret
		var pulumiAWSSecrets *corev1.Secret

		awsAccessKeyID, awsSecretAccessKey, awsSessionToken = os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_SESSION_TOKEN")
		if awsAccessKeyID == "" || awsSecretAccessKey == "" {
			Fail("Missing environment variable required for tests. AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must both be set. AWS_SESSION_TOKEN is optionally set.")
		}

		pulumiAccessToken = os.Getenv("PULUMI_ACCESS_TOKEN")
		if pulumiAccessToken == "" {
			Fail("Missing environment variable required for tests. PULUMI_ACCESS_TOKEN must be set.")
		}

		// Define the k8s secrets
		pulumiAPISecret = generateSecret(pulumiAPISecretName, namespace,
			map[string][]byte{
				"accessToken": []byte(pulumiAccessToken),
			},
		)
		pulumiAWSSecrets = generateSecret(pulumiAWSSecretsName, namespace,
			map[string][]byte{
				"AWS_ACCESS_KEY_ID":     []byte(awsAccessKeyID),
				"AWS_SECRET_ACCESS_KEY": []byte(awsSecretAccessKey),
				"AWS_SESSION_TOKEN":     []byte(awsSessionToken),
			},
		)
		Expect(k8sClient.Create(ctx, pulumiAPISecret)).Should(Succeed())
		time.Sleep(time.Second * 3) // give time for resource to create
		Expect(k8sClient.Create(ctx, pulumiAWSSecrets)).Should(Succeed())
		time.Sleep(time.Second * 3) // give time for resource to create

		name := "stack-test-aws-s3"
		stackName := fmt.Sprintf("metral/s3-op-project/dev-%s", randString())
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)

		// Define the stack spec
		spec := pulumiv1alpha1.StackSpec{
			AccessTokenSecret: pulumiAPISecret.ObjectMeta.Name,
			SecretEnvs: []string{
				pulumiAWSSecrets.ObjectMeta.Name,
			},
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Stack:             stackName,
			InitOnCreate:      true,
			ProjectRepo:       "https://github.com/metral/test-s3-op-project",
			Commit:            "bd1edfac28577d62068b7ace0586df595bda33be",
			DestroyOnFinalize: true,
		}

		// Create secrets and stack
		stack = generateStack(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())
		//time.Sleep(time.Second * 5)

		// Check that the stack updated
		fetched := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}

			// Check that the stack updated successfully
			if fetched.Status.LastUpdate != nil {
				return fetched.Status.LastUpdate.State == stack.Spec.Commit
			}
			return false
		}, timeout, interval).Should(BeTrue())

		if stack != nil {
			By(fmt.Sprintf("Deleting the Stack CR: %s", fetched.Name))
			toDelete := &pulumiv1alpha1.Stack{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, toDelete)).Should(Succeed())
			fetched := &pulumiv1alpha1.Stack{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
				return k8serrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
		if pulumiAPISecret != nil {
			By("Deleting the Stack Pulumi API Secret")
			Expect(k8sClient.Delete(ctx, pulumiAPISecret)).Should(Succeed())
		}
		if pulumiAWSSecrets != nil {
			By("Deleting the Stack AWS Secrets")
			Expect(k8sClient.Delete(ctx, pulumiAWSSecrets)).Should(Succeed())
		}
	})

	It("Should update the Stack CR commit", func() {
		var stack *pulumiv1alpha1.Stack
		var pulumiAPISecret *corev1.Secret
		var pulumiAWSSecrets *corev1.Secret

		awsAccessKeyID, awsSecretAccessKey, awsSessionToken = os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_SESSION_TOKEN")
		if awsAccessKeyID == "" || awsSecretAccessKey == "" {
			Fail("Missing environment variable required for tests. AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must both be set. AWS_SESSION_TOKEN is optionally set.")
		}

		pulumiAccessToken = os.Getenv("PULUMI_ACCESS_TOKEN")
		if pulumiAccessToken == "" {
			Fail("Missing environment variable required for tests. PULUMI_ACCESS_TOKEN must be set.")
		}

		// Define the k8s secrets
		pulumiAPISecret = generateSecret(pulumiAPISecretName, namespace,
			map[string][]byte{
				"accessToken": []byte(pulumiAccessToken),
			},
		)
		pulumiAWSSecrets = generateSecret(pulumiAWSSecretsName, namespace,
			map[string][]byte{
				"AWS_ACCESS_KEY_ID":     []byte(awsAccessKeyID),
				"AWS_SECRET_ACCESS_KEY": []byte(awsSecretAccessKey),
				"AWS_SESSION_TOKEN":     []byte(awsSessionToken),
			},
		)
		Expect(k8sClient.Create(ctx, pulumiAPISecret)).Should(Succeed())
		time.Sleep(time.Second * 3) // give time for resource to create
		Expect(k8sClient.Create(ctx, pulumiAWSSecrets)).Should(Succeed())
		time.Sleep(time.Second * 3) // give time for resource to create
		name := "stack-test-aws-s3-commit-change"
		stackName := fmt.Sprintf("metral/s3-op-project/dev-commit-change-%s", randString())
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)
		commit := "cc5442870f1195216d6bc340c14f8ae7d28cf3e2"

		// Define the stack spec
		spec := pulumiv1alpha1.StackSpec{
			AccessTokenSecret: pulumiAPISecret.ObjectMeta.Name,
			SecretEnvs: []string{
				pulumiAWSSecrets.ObjectMeta.Name,
			},
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Stack:             stackName,
			InitOnCreate:      true,
			ProjectRepo:       "https://github.com/metral/test-s3-op-project",
			Commit:            "bd1edfac28577d62068b7ace0586df595bda33be",
			DestroyOnFinalize: true,
		}

		// Create secrets and stack
		stack = generateStack(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		original := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, original)
			if err != nil {
				return false
			}

			// Check that the stack updated successfully
			if original.Status.LastUpdate != nil {
				return original.Status.LastUpdate.State == stack.Spec.Commit
			}
			return false
		}, timeout, interval).Should(BeTrue())

		// Update the stack commit
		original.Spec.Commit = commit
		Expect(k8sClient.Update(ctx, original)).Should(Succeed())
		time.Sleep(time.Second * 3) // give time for resource to create

		// Check that the stack updated
		fetched := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}

			// Check that the stack updated successfully to the new commit.
			if fetched.Status.LastUpdate != nil {
				return fetched.Status.LastUpdate.State == commit
			}
			return false
		}, timeout, interval).Should(BeTrue())

		if stack != nil {
			By(fmt.Sprintf("Deleting the Stack CR: %s", fetched.Name))
			toDelete := &pulumiv1alpha1.Stack{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, toDelete)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, toDelete)).Should(Succeed())
			fetched := &pulumiv1alpha1.Stack{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
				return k8serrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
		if pulumiAPISecret != nil {
			By("Deleting the Stack Pulumi API Secret")
			Expect(k8sClient.Delete(ctx, pulumiAPISecret)).Should(Succeed())
		}
		if pulumiAWSSecrets != nil {
			By("Deleting the Stack AWS Secrets")
			Expect(k8sClient.Delete(ctx, pulumiAWSSecrets)).Should(Succeed())
		}
	})
})

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
			APIVersion: "apps/v1beta1",
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
