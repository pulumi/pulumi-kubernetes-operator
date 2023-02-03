package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The go build cache will grow whenever it compiles a "new" file (new content, or new path; some
// other criteria). This test is to check that, without changing anything, rerunning a Go-based
// stack will _not_ grow the go cache. Working in sympathy with the go build cache is important
// because it means disk use is lower and more predictable, and build times much faster.
var _ = Describe("go build caching", func() {
	var (
		stack      *pulumiv1.Stack
		gitdir     string
		kubeconfig string
	)

	BeforeEach(func() {
		tmpdir, err := os.MkdirTemp("", "pulumi-test")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, tmpdir)

		gitdir = filepath.Join(tmpdir, "git")
		makeFixtureIntoRepo(gitdir, "testdata/go-build")

		// For the duration of the test, set the go build cache to a new location, so we have more
		// confidence when it changes (or doesn't change), we know why.
		gobuildcache := os.Getenv("GOCACHE")
		os.Setenv("GOCACHE", filepath.Join(tmpdir, "gocache"))
		DeferCleanup(os.Setenv, "GOCACHE", gobuildcache)

		kubeconfig = writeKubeconfig(tmpdir)

		stack = &pulumiv1.Stack{
			Spec: shared.StackSpec{
				Stack:   "test",
				Backend: fmt.Sprintf("file://%s", tmpdir),
				GitSource: &shared.GitSource{
					ProjectRepo: gitdir,
					RepoDir:     "testdata/go-build",
					Branch:      "default",
				},
				EnvRefs: map[string]shared.ResourceRef{
					"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("password"),
					"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
				},
				ContinueResyncOnCommitMatch: true,
			},
		}
		stack.Name = "go-build-" + randString()
		stack.Namespace = "default"

		Expect(k8sClient.Create(context.TODO(), stack)).To(Succeed())
		DeferCleanup(deleteAndWaitForFinalization, stack)
	})

	When("a go stack is run multiple times", func() {
		var (
			beforeSize string
		)

		checkCacheSize := func() string {
			out, err := exec.Command("go", "env", "GOCACHE").Output()
			ExpectWithOffset(1, err).ToNot(HaveOccurred())
			cachedir := string(out)
			cachedir = strings.TrimSpace(cachedir)
			GinkgoWriter.Println("Cache:", cachedir)
			// there's just one source file in the project, so the difference between the cache
			// working and it not working will be pretty small. `-k` measures in 1KiB blocks, which
			// should be fine enough.
			out, err = exec.Command("du", "-s", "-k", cachedir).Output()
			ExpectWithOffset(1, err).ToNot(HaveOccurred())
			s := string(out)
			return s
		}

		BeforeEach(func() {
			waitForStackSuccess(stack, "90s") // it just takes a while to build a Go project
			beforeSize = checkCacheSize()
			GinkgoWriter.Println("Before:", beforeSize)
			// make sure the cache is actually used!
			Expect(beforeSize).NotTo(HavePrefix("0\t"))
		})

		It("doesn't grow the Go build cache", func() {
			// run it again and check that we get the same result from du -sh <cache>
			stack.Spec.ResyncFrequencySeconds = 30 // just to provoke reprocessing
			resetWaitForStack()
			Expect(k8sClient.Update(context.TODO(), stack)).To(Succeed())
			waitForStackSuccess(stack, "90s")
			s := checkCacheSize()
			GinkgoWriter.Println("After:", s)
			Expect(s).To(Equal(beforeSize))
		})
	})
})
