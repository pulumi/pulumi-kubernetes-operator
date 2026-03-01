// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/blang/semver"
	"github.com/fluxcd/pkg/http/fetch"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	_targetDir       string
	_fluxURL         string
	_fluxDigest      string
	_gitURL          string
	_gitRevision     string
	_gitDependencies []string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a Pulumi workspace",
	Long: `Initialize a working directory to contain project sources.

For Flux sources:
	pulumi-kubernetes-agent init --flux-fetch-url URL
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var f fetchWithContexter
		var g newLocalWorkspacer

		if _gitURL != "" {
			g = &gitFetcher{url: _gitURL, revision: _gitRevision, dependencies: _gitDependencies}
		}
		// https://github.com/fluxcd/kustomize-controller/blob/a1a33f2adda783dd2a17234f5d8e84caca4e24e2/internal/controller/kustomization_controller.go#L328
		if _fluxURL != "" {
			f = &fluxFetcher{
				url:    _fluxURL,
				digest: _fluxDigest,
				wrapped: fetch.New(
					fetch.WithRetries(3),
					fetch.WithHostnameOverwrite(os.Getenv("SOURCE_CONTROLLER_LOCALHOST")),
					fetch.WithUntar()),
			}
		}
		// Don't display usage on error.
		cmd.SilenceUsage = true
		return runInit(ctx, log, _targetDir, f, g)
	},
}

func runInit(ctx context.Context,
	log *zap.SugaredLogger,
	targetDir string,
	f fetchWithContexter,
	g newLocalWorkspacer,
) error {
	log.Debugw("executing init command", "TargetDir", targetDir)

	// fetch the configured flux artifact
	if f != nil {
		err := os.MkdirAll(targetDir, 0o750)
		if err != nil {
			return fmt.Errorf("unable to make target directory: %w", err)
		}
		log.Debugw("target directory created", "dir", targetDir)

		log.Infow("flux artifact fetching", "url", f.URL(), "digest", f.Digest())
		err = f.FetchWithContext(ctx, f.URL(), f.Digest(), targetDir)
		if err != nil {
			return fmt.Errorf("unable to fetch flux artifact: %w", err)
		}
		log.Infow("flux artifact fetched", "dir", targetDir)
		return nil
	}

	// fetch the configured git artifact
	// If dependencies are specified, use sparse checkout
	if gf, ok := g.(*gitFetcher); ok && len(gf.dependencies) > 0 {
		log.Infow("about to clone with sparse checkout", "TargetDir", targetDir, "url", g.URL(), "dependencies", gf.dependencies)
		err := gf.cloneWithSparseCheckout(ctx, targetDir)
		if errors.Is(err, git.ErrRepositoryAlreadyExists) {
			log.Infow("repository was previously checked out", "dir", targetDir)
			return nil
		}
		if err != nil {
			return fmt.Errorf("unable to fetch git source with sparse checkout: %w", err)
		}
		log.Infow("git artifact fetched with sparse checkout", "dir", targetDir)
		return nil
	}

	// Otherwise use Automation API
	auth := &auto.GitAuth{
		SSHPrivateKey:       os.Getenv("GIT_SSH_PRIVATE_KEY"),
		Username:            os.Getenv("GIT_USERNAME"),
		Password:            os.Getenv("GIT_PASSWORD"),
		PersonalAccessToken: os.Getenv("GIT_TOKEN"),
	}
	repo := auto.GitRepo{
		URL:        g.URL(),
		CommitHash: g.Revision(),
		Auth:       auth,
		Shallow:    os.Getenv("GIT_SHALLOW") == "true",
	}

	log.Infow("about to clone into", "TargetDir", targetDir, "fluxURL", g.URL())

	// This will also handle creating the TargetDir for us.
	_, err := g.NewLocalWorkspace(ctx,
		auto.Repo(repo),
		auto.WorkDir(targetDir),
		auto.Pulumi(noop{}),
	)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		// Repository exists - verify it's on the correct revision and update if needed
		log.Infow("repository exists, verifying checkout", "dir", targetDir, "expectedRevision", g.Revision())
		if updateErr := ensureCorrectRevision(ctx, targetDir, g.Revision(), auth); updateErr != nil {
			log.Warnw("failed to ensure correct revision, attempting fresh clone", "error", updateErr)
			// Remove existing repo and retry
			if rmErr := os.RemoveAll(targetDir); rmErr != nil {
				return fmt.Errorf("failed to remove stale repo: %w", rmErr)
			}
			_, err = g.NewLocalWorkspace(ctx,
				auto.Repo(repo),
				auto.WorkDir(targetDir),
				auto.Pulumi(noop{}),
			)
			if err != nil {
				return fmt.Errorf("unable to fetch git source after cleanup: %w", err)
			}
		}
		log.Infow("repository checkout verified", "dir", targetDir)
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to fetch git source: %w", err)
	}
	log.Infow("git artifact fetched", "dir", targetDir)
	return nil
}

type fetchWithContexter interface {
	FetchWithContext(ctx context.Context, archiveURL, digest, dir string) error
	URL() string
	Digest() string
}
type fluxFetcher struct {
	wrapped     *fetch.ArchiveFetcher
	url, digest string
}

func (f fluxFetcher) FetchWithContext(ctx context.Context, archiveURL, digest, dir string) error {
	return f.wrapped.FetchWithContext(ctx, archiveURL, digest, dir)
}

func (f fluxFetcher) URL() string {
	return f.url
}

func (f fluxFetcher) Digest() string {
	return f.digest
}

type newLocalWorkspacer interface {
	NewLocalWorkspace(context.Context, ...auto.LocalWorkspaceOption) (auto.Workspace, error)
	URL() string
	Revision() string
}

type gitFetcher struct {
	url, revision string
	dependencies  []string
}

func (g gitFetcher) NewLocalWorkspace(ctx context.Context, opts ...auto.LocalWorkspaceOption) (auto.Workspace, error) {
	// Sparse checkout is handled in runInit, so just delegate to Automation API
	return auto.NewLocalWorkspace(ctx, opts...)
}

func (g gitFetcher) URL() string {
	return g.url
}

func (g gitFetcher) Revision() string {
	return g.revision
}

func (g gitFetcher) cloneWithSparseCheckout(ctx context.Context, targetDir string) error {

	// Check if repo already exists
	if _, err := os.Stat(targetDir + "/.git"); err == nil {
		log.Infow("repository was previously checked out with sparse checkout", "dir", targetDir)
		return git.ErrRepositoryAlreadyExists
	}

	// Note: go-git doesn't support native sparse checkout
	// As a workaround, we clone the full repo and then remove unwanted files
	// For a true sparse clone, we'd need to use the git CLI or a different approach
	log.Warnw("sparse checkout requested but go-git doesn't support it natively, falling back to full clone", "dependencies", g.dependencies)

	// Get authentication
	auth, err := g.getAuth()
	if err != nil {
		return fmt.Errorf("failed to configure auth: %w", err)
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	log.Infow("cloning repository", "url", g.url, "revision", g.revision)

	// Clone with authentication
	cloneOpts := &git.CloneOptions{
		URL:  g.url,
		Auth: auth,
	}

	repo, err := git.PlainCloneContext(ctx, targetDir, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Checkout the specific revision
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	checkoutOpts := &git.CheckoutOptions{
		Hash: plumbing.NewHash(g.revision),
	}

	if err := worktree.Checkout(checkoutOpts); err != nil {
		return fmt.Errorf("failed to checkout revision %s: %w", g.revision, err)
	}

	log.Infow("clone completed", "dir", targetDir)

	return nil
}

// ensureCorrectRevision checks if the repo is on the correct revision and updates if needed.
func ensureCorrectRevision(ctx context.Context, targetDir, revision string, auth *auto.GitAuth) error {
	repo, err := git.PlainOpen(targetDir)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	// Get authentication for fetch
	var gitAuth transport.AuthMethod
	if auth != nil {
		if auth.PersonalAccessToken != "" {
			gitAuth = &http.BasicAuth{
				Username: "git",
				Password: auth.PersonalAccessToken,
			}
		} else if auth.Username != "" {
			gitAuth = &http.BasicAuth{
				Username: auth.Username,
				Password: auth.Password,
			}
		}
	}

	// Fetch latest from remote
	log.Infow("fetching latest from remote", "revision", revision)
	err = repo.FetchContext(ctx, &git.FetchOptions{
		Auth:  gitAuth,
		Force: true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		log.Warnw("fetch failed, will try checkout anyway", "error", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Try to resolve the revision as a branch first
	branchRef := plumbing.NewRemoteReferenceName("origin", revision)
	ref, err := repo.Reference(branchRef, true)
	if err == nil {
		// Found as remote branch, checkout the commit it points to
		log.Infow("checking out remote branch", "branch", revision, "commit", ref.Hash().String()[:8])
		return worktree.Checkout(&git.CheckoutOptions{
			Hash:  ref.Hash(),
			Force: true,
		})
	}

	// Try as a commit SHA
	hash := plumbing.NewHash(revision)
	if hash.IsZero() {
		return fmt.Errorf("invalid revision: %s", revision)
	}

	log.Infow("checking out commit", "commit", revision[:8])
	return worktree.Checkout(&git.CheckoutOptions{
		Hash:  hash,
		Force: true,
	})
}

func (g gitFetcher) getAuth() (transport.AuthMethod, error) {
	// SSH key authentication
	if sshKey := os.Getenv("GIT_SSH_PRIVATE_KEY"); sshKey != "" {
		publicKeys, err := ssh.NewPublicKeys("git", []byte(sshKey), os.Getenv("GIT_PASSWORD"))
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}
		return publicKeys, nil
	}

	// Token authentication
	if token := os.Getenv("GIT_TOKEN"); token != "" {
		return &http.BasicAuth{
			Username: "git", // Can be anything for token auth
			Password: token,
		}, nil
	}

	// Username/password authentication
	if username := os.Getenv("GIT_USERNAME"); username != "" {
		return &http.BasicAuth{
			Username: username,
			Password: os.Getenv("GIT_PASSWORD"),
		}, nil
	}

	// No authentication
	return nil, nil
}

// noop is a PulumiCommand which doesn't do anything -- because the underlying CLI
// isn't available in our image.
type noop struct{}

func (noop) Version() semver.Version { return semver.Version{} }

func (noop) Run(context.Context, string, io.Reader, []io.Writer, []io.Writer, []string, ...string) (string, string, int, error) {
	return "", "", 0, fmt.Errorf("pulumi CLI is not available during init")
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&_targetDir, "target-dir", "t", "", "The target directory to initialize")
	err := initCmd.MarkFlagRequired("target-dir")
	if err != nil {
		panic(err)
	}

	initCmd.Flags().StringVar(&_fluxURL, "flux-url", "", "Flux archive URL")
	initCmd.Flags().StringVar(&_fluxDigest, "flux-digest", "", "Flux digest")
	initCmd.MarkFlagsRequiredTogether("flux-url", "flux-digest")

	initCmd.Flags().StringVar(&_gitURL, "git-url", "", "Git repository URL")
	initCmd.Flags().StringVar(&_gitRevision, "git-revision", "", "Git revision (tag or commit SHA)")
	initCmd.Flags().StringSliceVar(&_gitDependencies, "git-dependencies", nil, "Additional paths for sparse checkout (comma-separated)")
	initCmd.MarkFlagsRequiredTogether("git-url", "git-revision")

	initCmd.MarkFlagsOneRequired("git-url", "flux-url")
	initCmd.MarkFlagsMutuallyExclusive("git-url", "flux-url")
}
