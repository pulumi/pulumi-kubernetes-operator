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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	createStack := func(fixture string, suffix ...string) *pulumiv1.Stack {
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
		stackObj.Name = fixture + strings.Join(suffix, "-") + "-" + randString()
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

	AfterEach(func() {
		deleteAndWaitForFinalization(setupStack)
		if strings.HasPrefix(tmp, os.TempDir()) {
			Expect(os.RemoveAll(tmp)).To(Succeed())
		}
	})

	When("the credentials are rotated", func() {

		BeforeEach(func() {
			ctx := context.TODO()

			// now "rotate" the credentials, by rerunning the setup stack in such a way as to recreate the passphrase
			refetch(setupStack)
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
			refetch(useRabbitStack)
			useRabbitStack.Spec.Refresh = true
			Expect(k8sClient.Update(ctx, useRabbitStack)).To(Succeed())
			waitForStackFailure(useRabbitStack)
		})

		It("succeeds at refreshing when a targeted stack is run first", func() {
			ctx := context.TODO()

			// Get the URN of the provider, which we're going to target in this "helper" stack
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

		It("succeeds at refreshing when a targeted stack is given as a prerequisite", func() {
			ctx := context.TODO()

			// Get the URN of the provider, which we're going to target in this "helper" stack
			refetch(useRabbitStack)
			providerURNJSON := useRabbitStack.Status.Outputs["providerURN"]
			var providerURN string
			Expect(json.Unmarshal(providerURNJSON.Raw, &providerURN)).To(Succeed())
			Expect(providerURN).ToNot(BeEmpty())

			// this runs a stack which we'll use to refresh the state of the credentials. It
			// refreshes the _same_ state because it uses the same backend stack name, since the
			// stack is named for the fixture (in createStack).
			targetedStack := createStack("use-rabbitmq", "helper")
			targetedStack.Spec.Targets = []string{providerURN}
			Expect(k8sClient.Create(ctx, targetedStack)).To(Succeed())
			waitForStackSuccess(targetedStack)

			// this is a cheat: set the succeeded time for the helper to a long time ago, so the
			// prerequisite mechanism will kick in. Otherwise, it will see this as succeeded
			// already, and not bother.
			refetch(targetedStack)
			targetedStack.Status.LastUpdate.LastResyncTime = metav1.NewTime(time.Time{})
			Expect(k8sClient.Status().Update(ctx, targetedStack)).To(Succeed())

			// _Now_ change the password again; the targeted stack succeeded, and won't be run again
			// unless it's requeued (by the prerequisite mechanism, assumably).
			refetch(setupStack)
			setupStack.Spec.Config["password"] = randString()
			waitForStackSince = time.Now()
			Expect(k8sClient.Update(ctx, setupStack)).To(Succeed())
			waitForStackSuccess(setupStack)

			// the following both sets "refresh" on the stack, meaning it will try to refresh state
			// before running the program, and gives the above stack as a prerequisite. Since it
			// also mutates the object, the update serves to requeue it for the controller to
			// reconcile.
			refetch(useRabbitStack) // this is fetched above; but, avoid future coincidences ..
			useRabbitStack.Spec.Refresh = true
			useRabbitStack.Spec.Prerequisites = []shared.PrerequisiteRef{
				{
					Name: targetedStack.Name,
					Requirement: &shared.RequirementSpec{
						SucceededWithinDuration: &metav1.Duration{10 * time.Minute},
					},
				},
			}
			waitForStackSince = time.Now()
			Expect(k8sClient.Update(ctx, useRabbitStack)).To(Succeed())

			// TODO check that it's marked as reconciling pending a prerequisite

			// We'd expect this to fail, because its state was invalidated by running the setupStack
			// with a fresh password; _unless_ something prompts the targeted stack to be run again,
			// updating the state.
			waitForStackSuccess(useRabbitStack)
		})
	})
})
