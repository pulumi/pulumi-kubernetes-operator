/*
Copyright © 2024 Pulumi Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	_targetDir   string
	_fluxURL     string
	_fluxDigest  string
	_gitURL      string
	_gitRevision string
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
			g = &gitFetcher{url: _gitURL, revision: _gitRevision}
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
		err := os.MkdirAll(targetDir, 0o777)
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
		// TODO(https://github.com/pulumi/pulumi/issues/17288): Automation
		// API needs to ensure the existing checkout is valid.
		log.Infow("repository was previously checked out", "dir", targetDir)
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
}

func (g gitFetcher) NewLocalWorkspace(ctx context.Context, opts ...auto.LocalWorkspaceOption) (auto.Workspace, error) {
	return auto.NewLocalWorkspace(ctx, opts...)
}

func (g gitFetcher) URL() string {
	return g.url
}

func (g gitFetcher) Revision() string {
	return g.revision
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
	initCmd.MarkFlagRequired("target-dir")

	initCmd.Flags().StringVar(&_fluxURL, "flux-url", "", "Flux archive URL")
	initCmd.Flags().StringVar(&_fluxDigest, "flux-digest", "", "Flux digest")
	initCmd.MarkFlagsRequiredTogether("flux-url", "flux-digest")

	initCmd.Flags().StringVar(&_gitURL, "git-url", "", "Git repository URL")
	initCmd.Flags().StringVar(&_gitRevision, "git-revision", "", "Git revision (tag or commit SHA)")
	initCmd.MarkFlagsRequiredTogether("git-url", "git-revision")

	initCmd.MarkFlagsOneRequired("git-url", "flux-url")
	initCmd.MarkFlagsMutuallyExclusive("git-url", "flux-url")
}
