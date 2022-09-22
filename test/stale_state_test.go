// Copyright 2022, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"encoding/json"
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

var _ = When("a stack uses a provider with credentials kept in state", func() {
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
		useRabbitStack *pulumiv1.Stack
	)

	createStack := func(fixture string) *pulumiv1.Stack {
		// NB fixture here is the directory _under_ testdata, e.g., `run-rabbitmq`
		targetPath := filepath.Join(tmp, fixture)
		repoPath := filepath.Join(targetPath, randString())

		ExpectWithOffset(1, makeFixtureIntoRepo(repoPath, filepath.Join("testdata", fixture))).To(Succeed())

		// This is deterministic so that tests can create Stack objects that target the same stack
		// in the same backend.
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
		stackObj.Name = randString()
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
		setupStack.Spec.Config = map[string]string{"password": randString()} // changing this will force it to restart RabbitMQ with the new creds
		Expect(k8sClient.Create(ctx, setupStack)).To(Succeed())
		waitForStackSuccess(setupStack)

		// running this the first time should succeed -- the program will run and get the
		// credentials from the secret, which then go into the stack state.
		useRabbitStack = createStack("use-rabbitmq")
		Expect(k8sClient.Create(context.TODO(), useRabbitStack)).To(Succeed())
		waitForStackSuccess(useRabbitStack)
	})

	cleanupStack := func(stack *pulumiv1.Stack) {
		if stack != nil {
			err := k8sClient.Delete(context.TODO(), stack)
			Expect(err).ToNot(HaveOccurred())
			// block until the API server has deleted it, so that the controller has a chance to
			// destroy its resources
			EventuallyWithOffset(1, func() bool {
				var s pulumiv1.Stack
				k := types.NamespacedName{Name: stack.Name, Namespace: stack.Namespace}
				return k8serrors.IsNotFound(k8sClient.Get(context.TODO(), k, &s))
			}, "20s", "1s").Should(BeTrue())
		}
	}

	AfterEach(func() {
		cleanupStack(setupStack)
		if strings.HasPrefix(tmp, os.TempDir()) {
			Expect(os.RemoveAll(tmp)).To(Succeed())
		}
	})

	When("the credentials are rotated", func() {

		BeforeEach(func() {
			ctx := context.TODO()

			// now "rotate" the credentials, by rerunning the setup stack in such a way as to recreate the passphrase
			Expect(k8sClient.Get(context.TODO(), types.NamespacedName{
				Name:      setupStack.Name,
				Namespace: setupStack.Namespace,
			}, setupStack)).To(Succeed())
			setupStack.Spec.Config = map[string]string{"password": randString()}
			waitForStackSince = time.Now()
			Expect(k8sClient.Update(ctx, setupStack)).To(Succeed())
			waitForStackSuccess(setupStack)
		})

		It("fails to refresh without intervention", func() {
			ctx := context.TODO()
			// the following both sets "refresh" on the stack, meaning it will try to refresh state
			// before running the program; and, since it mutates the object, serves to requeue it for
			// the controller to reconcile.
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      useRabbitStack.Name,
				Namespace: useRabbitStack.Namespace,
			}, useRabbitStack)).To(Succeed())
			useRabbitStack.Spec.Refresh = true
			Expect(k8sClient.Update(ctx, useRabbitStack)).To(Succeed())

			Eventually(func() bool {
				k := types.NamespacedName{Name: useRabbitStack.Name, Namespace: useRabbitStack.Namespace}
				var s pulumiv1.Stack
				err := k8sClient.Get(ctx, k, &s)
				if err != nil {
					return false
				}
				return s.Status.LastUpdate != nil &&
					s.Status.LastUpdate.State == shared.FailedStackStateMessage &&
					s.Status.LastUpdate.LastResyncTime.Time.After(waitForStackSince)
			}, "20s", "1s").Should(BeTrue(), "stack fails when set to refresh")
		})

		It("succeeds at refreshing when a targeted stack is run first", func() {
			ctx := context.TODO()

			// Get the URN of the provider, which we're gong to target in this "helper" stack
			var s pulumiv1.Stack
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      useRabbitStack.Name,
				Namespace: useRabbitStack.Namespace,
			}, &s)).To(Succeed())
			providerURNJSON := s.Status.Outputs["providerURN"]
			var providerURN string
			Expect(json.Unmarshal(providerURNJSON.Raw, &providerURN)).To(Succeed())
			Expect(providerURN).ToNot(BeEmpty())

			// this runs a stack which we'll use to refresh the state of the credentials. It
			// refreshes the _same_ state because it uses the same backend stack name, since the
			// stack is named for the fixture (in createStack).
			targetedStack := createStack("use-rabbitmq")
			targetedStack.Spec.Targets = []string{providerURN}
			Expect(k8sClient.Create(ctx, targetedStack)).To(Succeed())
			waitForStackSuccess(targetedStack)

			// the following both sets "refresh" on the stack, meaning it will try to refresh state
			// before running the program; and, since it mutates the object, serves to requeue it for
			// the controller to reconcile.
			waitForStackSince = time.Now()
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      useRabbitStack.Name,
				Namespace: useRabbitStack.Namespace,
			}, useRabbitStack)).To(Succeed())
			useRabbitStack.Spec.Refresh = true
			Expect(k8sClient.Update(ctx, useRabbitStack)).To(Succeed())

			waitForStackSuccess(useRabbitStack)
		})
	})
})
