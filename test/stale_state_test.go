// Copyright 2022, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	// corev1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("mechanism for refreshing stale state", func() {
	// This models a situation in which the program constructs a provider using credentials which
	// are then rotated. Pulumi has difficulty with this if you want to refresh the state, since the
	// provider will be constructed from the credentials in the state, which are out of date. The
	// program is not run during refresh; only the state and the resources it refers to are
	// examined.
	//
	// In this case, the provider is RabbitMQ. Credentials are created and stored in a secret in a
	// first phase (using a stack, but only for convenience). In the second phase, the credentials
	// are read from the secret and used to connect to RabbitMQ. So far so good. Then, the
	// credentials are rotated -- the secret is updated and RabbitMQ is restarted with the new
	// credentials. Asking the second phase stack to refresh will cause it to fail, as explained
	// above.

	var (
		tmp        string
		setupStack *pulumiv1.Stack
		env        = map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "foobarbaz",
		}
	)

	createStack := func(fixture string) *pulumiv1.Stack {
		// NB fixture here is the directory _under_ testdata, e.g., `run-rabbitmq`
		targetPath := filepath.Join(tmp, fixture)
		repoPath := filepath.Join(targetPath, "repo")

		ExpectWithOffset(1, makeFixtureIntoRepo(repoPath, filepath.Join("testdata", fixture))).To(Succeed())

		backend := fmt.Sprintf("file://%s", targetPath)

		envRefs := map[string]shared.ResourceRef{}
		for k, v := range env {
			envRefs[k] = shared.NewLiteralResourceRef(v)
		}

		stackObj := &pulumiv1.Stack{
			Spec: shared.StackSpec{
				Stack: "test",
				GitSource: &shared.GitSource{
					ProjectRepo: repoPath,
					Branch:      "default",
				},
				EnvRefs:                     envRefs,
				Backend:                     backend,
				DestroyOnFinalize:           true,
				ContinueResyncOnCommitMatch: true, // ) necessary so it will run the stack again when the config changes,
				ResyncFrequencySeconds:      3600, // ) but not otherwise.
			},
		}
		stackObj.Name = fixture
		stackObj.Namespace = "default"

		return stackObj
	}

	BeforeEach(func() {
		ctx := context.TODO()
		// Run a setup program that: creates a password, puts that in a secret for the client program
		// to use; and runs RabbitMQ in a container.
		var err error
		tmp, err = os.MkdirTemp("", "pulumi-test")
		Expect(err).NotTo(HaveOccurred())
		// Set this so the default Kubernetes provider uses the test env API server.
		kubeconfig := writeKubeconfig(tmp)
		env["KUBECONFIG"] = kubeconfig

		setupStack = createStack("run-rabbitmq")
		setupStack.Spec.Config = map[string]string{"passwordLength": "16"} // changing this will force it to regenerate the password
		Expect(k8sClient.Create(ctx, setupStack)).To(Succeed())
		waitForStackSuccess(setupStack)
	})

	AfterEach(func() {
		if setupStack != nil {
			err := k8sClient.Delete(context.TODO(), setupStack)
			Expect(err).ToNot(HaveOccurred())
			// block until the API server has deleted it, so that the controller has a chance to
			// destroy its resources
			Eventually(func() bool {
				var s pulumiv1.Stack
				k := types.NamespacedName{Name: setupStack.Name, Namespace: setupStack.Namespace}
				return k8serrors.IsNotFound(k8sClient.Get(context.TODO(), k, &s))
			}, "20s", "1s").Should(BeTrue())
		}
		if strings.HasPrefix(tmp, os.TempDir()) {
			Expect(os.RemoveAll(tmp)).To(Succeed())
		}
	})

	It("fails to complete when the credentials are rotated", func() {
		ctx := context.TODO()
		// this first step should succeed -- the program will run and get the credentials from the
		// secret.
		stack := createStack("use-rabbitmq")
		Expect(k8sClient.Create(ctx, stack)).To(Succeed())
		waitForStackSuccess(stack)

		// now "rotate" the credentials, by rerunning the setup stack in such a way as to recreate the passphrase
		Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
			Name:      setupStack.Name,
			Namespace: setupStack.Namespace,
		}, setupStack)).To(Succeed())
		setupStack.Spec.Config = map[string]string{"passwordLength": "18"}
		waitForStackSince = time.Now()
		Expect(k8sClient.Update(ctx, setupStack)).To(Succeed())
		waitForStackSuccess(setupStack)

		Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
			Name:      stack.Name,
			Namespace: stack.Namespace,
		}, stack)).To(Succeed())
		stack.Spec.Refresh = true
		Expect(k8sClient.Update(ctx, stack)).To(Succeed())
		Eventually(func() bool {
			k := types.NamespacedName{Name: stack.Name, Namespace: stack.Namespace}
			var s pulumiv1.Stack
			err := k8sClient.Get(context.TODO(), k, &s)
			if err != nil {
				return false
			}
			return s.Status.LastUpdate != nil &&
				s.Status.LastUpdate.State == shared.FailedStackStateMessage &&
				s.Status.LastUpdate.LastResyncTime.Time.After(waitForStackSince)
		}, "20s", "1s").Should(BeTrue(), "stack fails when set to refresh")
	})
})
