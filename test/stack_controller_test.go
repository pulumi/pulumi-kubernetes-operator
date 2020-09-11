package tests

import (
	"context"
	"encoding/base32"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
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
const pulumiAWSSecretName = "pulumi-aws-secrets"
const timeout = time.Minute * 10
const interval = time.Second * 1

var _ = Describe("Stack Controller", func() {
	whoami, err := exec.Command("pulumi", "whoami").Output()
	if err != nil {
		Fail(fmt.Sprintf("Must get pulumi authorized user: %v", err))
	}
	stackOrg := strings.TrimSuffix(string(whoami), "\n")
	fmt.Printf("stackOrg: %s", stackOrg)

	ctx := context.Background()
	var pulumiAPISecret *corev1.Secret
	var pulumiAWSSecret *corev1.Secret

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
	})

	AfterEach(func() {
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
	})

	// Tip: avoid adding tests for vanilla CRUD operations because they would
	// test the Kubernetes API server, which isn't the goal here.

	It("Should deploy an AWS S3 Stack successfully", func() {
		var stack *pulumiv1alpha1.Stack
		stackName := fmt.Sprintf("%s/s3-op-project/dev-%s", stackOrg, randString())
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)

		// Define the stack spec
		spec := pulumiv1alpha1.StackSpec{
			AccessTokenSecret: pulumiAPISecret.ObjectMeta.Name,
			SecretEnvs: []string{
				pulumiAWSSecret.ObjectMeta.Name,
			},
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Stack:             stackName,
			ProjectRepo:       "https://github.com/metral/test-s3-op-project", // TODO: relocate to some other repo
			Commit:            "bd1edfac28577d62068b7ace0586df595bda33be",
			DestroyOnFinalize: true,
			Refresh:           true,
		}

		// Create the stack
		name := "stack-test-aws-s3"
		stack = generateStack(name, namespace, spec)
		Expect(k8sClient.Create(ctx, stack)).Should(Succeed())

		// Check that the stack updated successfully
		fetched := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			if fetched.Status.LastUpdate != nil {
				return fetched.Status.LastUpdate.State == stack.Spec.Commit
			}
			return false
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

	It("Should deploy an AWS S3 Stack successfully, then deploy a commit update successfully", func() {
		var stack *pulumiv1alpha1.Stack
		stackName := fmt.Sprintf("%s/s3-op-project/dev-commit-change-%s", stackOrg, randString())
		fmt.Fprintf(GinkgoWriter, "Stack.Name: %s\n", stackName)
		commit := "cc5442870f1195216d6bc340c14f8ae7d28cf3e2"

		// Define the stack spec
		spec := pulumiv1alpha1.StackSpec{
			AccessTokenSecret: pulumiAPISecret.ObjectMeta.Name,
			SecretEnvs: []string{
				pulumiAWSSecret.ObjectMeta.Name,
			},
			Config: map[string]string{
				"aws:region": "us-east-2",
			},
			Stack:             stackName,
			ProjectRepo:       "https://github.com/metral/test-s3-op-project", // TODO: relocate to some other repo
			Commit:            "bd1edfac28577d62068b7ace0586df595bda33be",
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
			if original.Status.LastUpdate != nil {
				return original.Status.LastUpdate.State == stack.Spec.Commit
			}
			return false
		}, timeout, interval).Should(BeTrue())

		// Update the stack commit to a different commit.
		original.Spec.Commit = commit
		Expect(k8sClient.Update(ctx, original)).Should(Succeed())

		// Check that the stack updated
		fetched := &pulumiv1alpha1.Stack{}
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: stack.Name, Namespace: namespace}, fetched)
			if err != nil {
				return false
			}
			if fetched.Status.LastUpdate != nil {
				return fetched.Status.LastUpdate.State == commit
			}
			return false
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
