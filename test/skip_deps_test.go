package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Skip install dependencies", func() {

	When("a Stack source has vendored dependencies", func() {

		var (
			repoDir string
			stack   *pulumiv1.Stack
		)

		BeforeEach(func() {
			tmp, err := os.MkdirTemp("", "pulumi-op-test")
			Expect(err).ToNot(HaveOccurred())
			DeferCleanup(os.RemoveAll, tmp)

			repoDir = filepath.Join(tmp, "repo")
			Expect(os.MkdirAll(repoDir, 0777)).To(Succeed())

			backendDir := filepath.Join(tmp, "backend")
			Expect(os.MkdirAll(backendDir, 0777)).To(Succeed())

			// Make a repo that we'll refer to from the stack. Then we'll vendor its dependencies,
			// so the operator doesn't need to install them; then we'll use the
			// skipInstallDependencies flag and see if the stack succeeds.
			Expect(makeFixtureIntoRepo(repoDir, "testdata/skip-deps")).To(Succeed())
			// makeFixtureIntoRepo will (at present) include the containing directories
			projectDir := filepath.Join("testdata", "skip-deps")

			npmCmd := exec.Command("npm", "ci")
			npmCmd.Dir = filepath.Join(repoDir, projectDir)
			npmCmd.Stdout = GinkgoWriter
			npmCmd.Stderr = GinkgoWriter
			Expect(npmCmd.Run()).To(Succeed())

			repo, err := git.PlainOpen(repoDir)
			Expect(err).ToNot(HaveOccurred())
			wt, err := repo.Worktree()
			Expect(err).ToNot(HaveOccurred())
			wt.Add("node_modules")
			wt.Commit("Vendor node_modules", &git.CommitOptions{
				Author: &object.Signature{
					Name:  "Pulumi Test",
					Email: "pulumi.test@example.com",
				},
			})

			kubeconfig := writeKubeconfig(tmp)

			stack = &pulumiv1.Stack{
				Spec: shared.StackSpec{
					Stack:   "skip-deps",
					Backend: fmt.Sprintf("file://%s", backendDir),
					EnvRefs: map[string]shared.ResourceRef{
						"KUBECONFIG":               shared.NewLiteralResourceRef(kubeconfig),
						"PULUMI_CONFIG_PASSPHRASE": shared.NewLiteralResourceRef("foobar"),
					},
					GitSource: &shared.GitSource{
						ProjectRepo: repoDir,
						Branch:      "default",
						RepoDir:     projectDir,
					},
				},
			}
			stack.Name = "skip-deps-" + randString()
			stack.Namespace = "default"

			Expect(k8sClient.Create(context.TODO(), stack)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(context.TODO(), stack)).To(Succeed())
			})
		})

		It("the stack succeeds", func() {
			waitForStackSuccess(stack)
		})
	})
})
