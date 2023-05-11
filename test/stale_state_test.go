// Copyright 2022, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

		rabbitPort      string
		credsSecretName string
	)

	createStack := func(fixture string, stack string) *pulumiv1.Stack {
		// NB fixture here is the directory _under_ testdata, e.g., `run-rabbitmq`
		targetPath := filepath.Join(tmp, stack)
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
					RepoDir:     filepath.Join("testdata", fixture),
				},
				EnvRefs:                     envRefs,
				Backend:                     backend,
				ContinueResyncOnCommitMatch: true, // ) necessary so it will run the stack again when the config changes,
				ResyncFrequencySeconds:      3600, // ) but not otherwise.
			},
		}
		stackObj.Name = fixture + "-" + stack + "-" + randString()
		stackObj.Namespace = "default"

		return stackObj
	}

	BeforeEach(func() {
		rabbitPort = fmt.Sprintf("%d", rand.Int31n(5000)+10000)
		credsSecretName = "rabbit-creds-" + randString()

		ctx := context.TODO()
		// Run a setup program that: creates a password, puts that in a secret for the client program
		// to use; and runs RabbitMQ in a container.
		var err error
		tmp, err = os.MkdirTemp("", "pulumi-test")
		Expect(err).NotTo(HaveOccurred())

		// Set this so the default Kubernetes provider uses the test env API server.
		kubeconfig := writeKubeconfig(tmp)
		env["KUBECONFIG"] = kubeconfig

		setupStack = createStack("run-rabbitmq", "setup")
		setupStack.Spec.DestroyOnFinalize = true // tear down the container afterwards
		setupStack.Spec.Config = map[string]string{
			"password":   randString(),
			"port":       rabbitPort,
			"secretName": credsSecretName,
		} // changing this will force it to restart RabbitMQ with the new creds
		Expect(k8sClient.Create(ctx, setupStack)).To(Succeed())
		waitForStackSuccess(setupStack, "120s")

		// running this the first time should succeed -- the program will run and get the
		// credentials from the secret, which then go into the stack state.
		useRabbitStack = createStack("use-rabbitmq", "use")
		useRabbitStack.Spec.Config = map[string]string{
			"port":       rabbitPort,
			"secretName": credsSecretName,
		}

		Expect(k8sClient.Create(context.TODO(), useRabbitStack)).To(Succeed())
		waitForStackSuccess(useRabbitStack, "120s")
	})

	AfterEach(func() {
		deleteAndWaitForFinalization(useRabbitStack)
		deleteAndWaitForFinalization(setupStack)
		os.RemoveAll(tmp)
	})

	When("the credentials are rotated", func() {

		BeforeEach(func() {
			ctx := context.TODO()

			// "rotate" the credentials by rerunning the setup stack in such a way as to recreate
			// the passphrase
			refetch(setupStack)
			setupStack.Spec.Config = map[string]string{
				"password":   randString(),
				"port":       rabbitPort,
				"secretName": credsSecretName,
			}
			resetWaitForStack()
			Expect(k8sClient.Update(ctx, setupStack)).To(Succeed())
			waitForStackSuccess(setupStack, "120s")
		})

		It("fails to refresh without intervention", func() {
			ctx := context.TODO()
			// the following both sets "refresh" on the stack, meaning it will try to refresh state
			// before running the program; and, since it mutates the object, serves to requeue it for
			// the controller to reconcile.
			refetch(useRabbitStack)
			useRabbitStack.Spec.Refresh = true
			Expect(k8sClient.Update(ctx, useRabbitStack)).To(Succeed())
			time.Sleep(30 * time.Second)
			waitForStackFailure(useRabbitStack, "120s")
		})
	})

	When("a targeted stack is created to update credentials", func() {

		var (
			targetedStack *pulumiv1.Stack
		)

		BeforeEach(func() {
			ctx := context.TODO()

			// Get the URN of the provider, which we're going to target in this "helper" stack
			refetch(useRabbitStack)
			providerURNJSON := useRabbitStack.Status.Outputs["providerURN"]
			var providerURN string
			Expect(json.Unmarshal(providerURNJSON.Raw, &providerURN)).To(Succeed())
			Expect(providerURN).ToNot(BeEmpty())

			// this runs a stack which we'll use to refresh the state of the credentials. It
			// refreshes the _same_ state because it uses the same backend stack name.
			targetedStack = createStack("use-rabbitmq", "use")
			targetedStack.Spec.Targets = []string{providerURN}
			targetedStack.Spec.Config = map[string]string{
				"port":       rabbitPort,
				"secretName": credsSecretName,
			}
			Expect(k8sClient.Create(ctx, targetedStack)).To(Succeed())
			DeferCleanup(func() {
				deleteAndWaitForFinalization(targetedStack)
			})
			waitForStackSuccess(targetedStack, "120s")

			// This is a cheat: set the succeeded time for the helper to longer ago than the prereq
			// wants (below), so it'll fail the requirement. Otherwise, it will see this as
			// succeeded already, and not bother to requeue it.
			refetch(targetedStack)
			targetedStack.Status.LastUpdate.LastResyncTime = metav1.NewTime(time.Now().Add(-11 * time.Minute))
			Expect(k8sClient.Status().Update(ctx, targetedStack)).To(Succeed())
		})

		When("the credentials are rotated", func() {

			BeforeEach(func() {
				ctx := context.TODO()

				// "rotate" the credentials by rerunning the setup stack in such a way as to recreate
				// the passphrase
				refetch(setupStack)
				setupStack.Spec.Config = map[string]string{
					"password":   randString(),
					"port":       rabbitPort,
					"secretName": credsSecretName,
				}
				resetWaitForStack()
				Expect(k8sClient.Update(ctx, setupStack)).To(Succeed())
				waitForStackSuccess(setupStack, "120s")
			})

			It("fails if the targeted stack isn't run again", func() {
				refetch(useRabbitStack) // this is fetched above; but, avoid future coincidences ..
				useRabbitStack.Spec.Refresh = true
				resetWaitForStack()
				Expect(k8sClient.Update(context.TODO(), useRabbitStack)).To(Succeed())
				time.Sleep(30 * time.Second)

				waitForStackFailure(useRabbitStack, "120s")
			})

			It("succeeds if the targeted stack is used as a prerequisite", func() {
				ctx := context.TODO()

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
							SucceededWithinDuration: &metav1.Duration{Duration: 10 * time.Minute},
						},
					},
				}
				resetWaitForStack()
				Expect(k8sClient.Update(ctx, useRabbitStack)).To(Succeed())

				// TODO check that it's marked as reconciling pending a prerequisite

				// We'd expect this to fail, because its state was invalidated by running the setupStack
				// with a fresh password; _unless_ something prompts the targeted stack to be run again,
				// updating the state.
				time.Sleep(30 * time.Second)
				waitForStackSuccess(useRabbitStack, "120s")
			})
		})
	})
})
