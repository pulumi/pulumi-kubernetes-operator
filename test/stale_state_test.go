// Copyright 2022, Pulumi Corporation.  All rights reserved.

package tests

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	// "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	// pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	// corev1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/types"
)

// To avoid Pulumi automation API rewriting checked-in files (e.g., when a test uses a tempdir state
// backend), always make a copy of the pristine project files.
func copyProject(target, source string) error {
	// this is a cheat: assume the project is a JavaScript package
	cmd := exec.Command("npm", "ci")
	cmd.Dir = source
	Expect(cmd.Run()).To(Succeed())

	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		newPath := filepath.Join(target, path[len(source):])
		if entry.IsDir() {
			return os.MkdirAll(newPath, 0777)
		}
		s, err := os.Open(path)
		if err != nil {
			return err
		}
		defer s.Close()
		t, err := os.Create(newPath)
		if err != nil {
			return err
		}
		defer t.Close()
		_, err = io.Copy(t, s)
		return err
	})
}

var _ = Describe("mechanism for refreshing stale state", func() {
	// This models a situation in which the program constructs a provider using credentials which
	// are then rotated. Pulumi has difficulty with this if you want to refresh the state, since the
	// provider will be constructed from the credentials in the state, which are out of date. The
	// program is not run during refresh; only the state and the resources it refers to are
	// examined.

	var (
		tmp        string
		setupStack *auto.Stack
		env        = map[string]string{
			"PULUMI_CONFIG_PASSPHRASE": "foobarbaz",
		}
	)

	createStack := func(fixture string) auto.Stack {
		sourcePath := filepath.Join(".", "testdata", fixture)
		targetPath := filepath.Join(tmp, fixture)
		Expect(os.MkdirAll(targetPath, 0777)).To(Succeed())
		Expect(copyProject(targetPath, sourcePath)).To(Succeed())

		var proj workspace.Project
		contents, err := os.ReadFile(filepath.Join(targetPath, "Pulumi.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(yaml.Unmarshal(contents, &proj)).To(Succeed())

		// override the backend, to use the temp directory
		proj.Backend = &workspace.ProjectBackend{
			URL: fmt.Sprintf("file://%s", targetPath),
		}
		stack, err := auto.UpsertStackLocalSource(context.TODO(), "local", targetPath,
			auto.Project(proj),
			auto.EnvVars(env),
			auto.PulumiHome(tmp))
		Expect(err).NotTo(HaveOccurred())
		return stack
	}

	BeforeEach(func() {
		ctx := context.TODO()
		// Run a setup program that: creates a password, puts that in a secret for the client program
		// to use; and runs MinIO in a container publishing its API on :32000.
		var err error
		tmp, err = os.MkdirTemp("", "pulumi-test")
		Expect(err).NotTo(HaveOccurred())
		stack := createStack("run-rabbitmq")
		stack.SetConfig(ctx, "passwordLength", auto.ConfigValue{Value: "16"})
		setupStack = &stack
		_, err = stack.Up(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if setupStack != nil {
			_, err := setupStack.Destroy(context.TODO())
			Expect(err).ToNot(HaveOccurred())
		}
		if strings.HasPrefix(tmp, os.TempDir()) {
			Expect(os.RemoveAll(tmp)).To(Succeed())
		}
	})

	It("fails to complete when the credentials are rotated", func() {
		ctx := context.TODO()
		// this first step should succeed -- the credentials line up
		stack := createStack("use-rabbitmq")
		_, err := stack.Up(ctx)
		Expect(err).NotTo(HaveOccurred())

		// now "rotate" the credentials, by rerunning the setup stack in such a way as to recreate the passphrase
		setupStack.SetConfig(ctx, "passwordLength", auto.ConfigValue{Value: "18"})
		_, err = setupStack.Up(ctx)
		Expect(err).NotTo(HaveOccurred())

		// refreshing the bucket stack will fail, because it has the old credentials in the state
		_, err = stack.Refresh(ctx)
		Expect(err).ToNot(BeNil())
	})
})
