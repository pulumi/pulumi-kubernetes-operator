// Copyright 2023, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const buildDirectoryPrefix = "pulumi-working"

var _ = Describe("Stack isolation", func() {
	var (
		tmpDir             string
		gitDir, backendDir string
		stack              *pulumiv1.Stack
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "pulumi-test")
		Expect(err).ToNot(HaveOccurred())

		// This makes a git repo to clone from, so to avoid relying on something at GitHub that could
		// change or be inaccessible.
		gitDir = filepath.Join(tmpDir, "repo")
		backendDir = filepath.Join(tmpDir, "state")
		Expect(os.Mkdir(backendDir, 0777)).To(Succeed())

		// Prepare a stack
		Expect(makeFixtureIntoRepo(gitDir, "testdata/empty-stack")).To(Succeed())
		stack = &pulumiv1.Stack{
			ObjectMeta: v1.ObjectMeta{
				Name:      "testworkdir",
				Namespace: "default",
			},
			Spec: shared.StackSpec{
				Stack:   "dev", // NB this relies on the file "Pulumi.dev.yaml" being present in the repo dir
				Backend: fmt.Sprintf("file://%s", backendDir),
				GitSource: &shared.GitSource{
					ProjectRepo: gitDir,
					RepoDir:     "testdata/empty-stack",
					Branch:      "default",
				},
				EnvRefs: map[string]shared.ResourceRef{
					"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
				},
			},
		}
	})

	// This is done just before testing the outcome, so that any adjustments to the stack
	// can be made before the stack is processed.
	JustBeforeEach(func() {
		stack.Name += ("-" + randString())
	})

	AfterEach(func() {
		if stack.Name != "" { // assume that if it's been named, it was created in the cluster
			deleteAndWaitForFinalization(stack)
		}
		if strings.HasPrefix(tmpDir, os.TempDir()) {
			os.RemoveAll(tmpDir)
		}
	})

	getRootDir := func(stack *pulumiv1.Stack) string {
		return filepath.Join(os.TempDir(), buildDirectoryPrefix, stack.GetNamespace(), stack.GetName())
	}

	exists := func(filePath string) bool {
		_, err := os.Stat(filePath)
		return !os.IsNotExist(err)
	}

	It("should maintain a root directory", func() {
		rootDir := getRootDir(stack)
		Expect(exists(rootDir)).To(BeFalse())
		Expect(k8sClient.Create(context.TODO(), stack)).To(Succeed())
		waitForStackSuccess(stack)
		Expect(exists(rootDir)).To(BeTrue())
		Expect(exists(filepath.Join(rootDir, ".pulumi"))).To(BeTrue())
	})

	It("should purge the workspace dir", func() {
		rootDir := getRootDir(stack)
		Expect(k8sClient.Create(context.TODO(), stack)).To(Succeed())
		waitForStackSuccess(stack)
		Expect(exists(filepath.Join(rootDir, "workspace"))).To(BeFalse())
	})

	finalizationTest := func() {
		It("should purge the root directory", func() {
			rootDir := getRootDir(stack)
			Expect(k8sClient.Create(context.TODO(), stack)).To(Succeed())
			waitForStackSuccess(stack)
			deleteAndWaitForFinalization(stack)
			Expect(exists(rootDir)).To(BeFalse())
		})
	}

	Describe("finalization (slow path)", func() {
		BeforeEach(func() {
			stack.Spec.DestroyOnFinalize = true
		})
		finalizationTest()
	})

	Describe("finalization (fast path)", func() {
		BeforeEach(func() {
			stack.Spec.DestroyOnFinalize = false
		})
		finalizationTest()
	})
})
