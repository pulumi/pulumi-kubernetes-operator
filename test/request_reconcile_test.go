// Copyright 2022, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = When("a Stack has reached a successful state, then been annotated", func() {

	var (
		tmp string
		env = map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "foobarbaz",
		}
		stackObj *pulumiv1.Stack
	)

	BeforeEach(func() {
		var err error
		tmp, err = os.MkdirTemp("", "pulumi-test")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			removeTempDir(tmp)
		})

		// Set this so the default Kubernetes provider uses the test env API server.
		kubeconfig := writeKubeconfig(tmp)
		env["KUBECONFIG"] = kubeconfig

		targetPath := filepath.Join(tmp, "reconcile-request-"+randString())
		repoPath := filepath.Join(targetPath, randString())

		Expect(makeFixtureIntoRepo(repoPath, filepath.Join("testdata", "success"))).To(Succeed())

		// This is deterministic so that tests can create Stack objects that target the same stack
		// in the same backend.
		backend := fmt.Sprintf("file://%s", targetPath)

		envRefs := map[string]shared.ResourceRef{}
		for k, v := range env {
			envRefs[k] = shared.NewLiteralResourceRef(v)
		}

		stackObj = &pulumiv1.Stack{
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
		stackObj.Name = "reconcile-request-" + randString()
		stackObj.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), stackObj)).To(Succeed())
		waitForStackSuccess(stackObj)
		DeferCleanup(func() {
			deleteAndWaitForFinalization(stackObj)
		})

		refetch(stackObj)
		stackObj.SetAnnotations(map[string]string{
			shared.ReconcileRequestAnnotation: randString(),
		})
		waitForStackSince = time.Now() // reset the base time, to be checked in subsequent waitForStack*
		Expect(k8sClient.Update(context.TODO(), stackObj)).To(Succeed())
	})

	It("is run again", func() {
		waitForStackSuccess(stackObj) // i.e., has been run again and succeeded since the base time was reset
	})

	It("has the request reconcile annotation observed in .status.observedReconcileRequest", func() {
		waitForStackSuccess(stackObj)
		refetch(stackObj)
		Expect(stackObj.Status.ObservedReconcileRequest).To(Equal(stackObj.GetAnnotations()[shared.ReconcileRequestAnnotation]))
	})
})
