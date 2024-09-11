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
	"net/url"
	"os"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/repository"
	"github.com/fluxcd/pkg/http/fetch"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	DefaultFluxRetries = 3
)

var (
	TargetDir   string
	FluxUrl     string
	FluxDigest  string
	GitUrl      string
	GitRevision string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a Pulumi workspace",
	Long: `Initialize a working directory to contain project sources.

For Flux sources:
	pulumi-kubernetes-agent init --flux-fetch-url URL
`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		log.Debugw("executing init command", "TargetDir", TargetDir)

		err := os.MkdirAll(TargetDir, 0777)
		if err != nil {
			log.Errorw("fatal: unable to make target directory", zap.Error(err))
			os.Exit(1)
		}
		log.Debugw("target directory created", "dir", TargetDir)

		// fetch the configured flux artifact
		if FluxUrl != "" {
			// https://github.com/fluxcd/kustomize-controller/blob/a1a33f2adda783dd2a17234f5d8e84caca4e24e2/internal/controller/kustomization_controller.go#L328
			fetcher := fetch.New(
				fetch.WithRetries(DefaultFluxRetries),
				fetch.WithHostnameOverwrite(os.Getenv("SOURCE_CONTROLLER_LOCALHOST")),
				fetch.WithUntar())

			log.Infow("flux artifact fetching", "url", FluxUrl, "digest", FluxDigest)
			err := fetcher.FetchWithContext(ctx, FluxUrl, FluxDigest, TargetDir)
			if err != nil {
				log.Errorw("fatal: unable to fetch flux artifact", zap.Error(err))
				os.Exit(2)
			}
			log.Infow("flux artifact fetched", "dir", TargetDir)
		}

		// fetch the configured git artifact
		if GitUrl != "" {
			u, err := url.Parse(GitUrl)
			if err != nil {
				log.Errorw("fatal: unable to parse git url", zap.Error(err))
				os.Exit(2)
			}
			// Configure authentication strategy to access the source
			authData := map[string][]byte{}
			authOpts, err := git.NewAuthOptions(*u, authData)
			if err != nil {
				log.Errorw("fatal: unable to parse git auth options", zap.Error(err))
				os.Exit(2)
			}
			cloneOpts := repository.CloneConfig{
				RecurseSubmodules: false,
				ShallowClone:      true,
			}
			cloneOpts.Commit = GitRevision
			log.Infow("git source fetching", "url", GitUrl, "revision", GitRevision)
			_, err = gitCheckout(ctx, GitUrl, cloneOpts, authOpts, nil, TargetDir)
			if err != nil {
				log.Errorw("fatal: unable to fetch git source", zap.Error(err))
				os.Exit(2)
			}
			log.Infow("git artifact fetched", "dir", TargetDir)
		}
	},
}

func gitCheckout(ctx context.Context, url string, cloneOpts repository.CloneConfig,
	authOpts *git.AuthOptions, proxyOpts *transport.ProxyOptions, dir string) (*git.Commit, error) {

	clientOpts := []gogit.ClientOption{gogit.WithDiskStorage()}
	if authOpts.Transport == git.HTTP {
		clientOpts = append(clientOpts, gogit.WithInsecureCredentialsOverHTTP())
	}
	if proxyOpts != nil {
		clientOpts = append(clientOpts, gogit.WithProxy(*proxyOpts))
	}

	gitReader, err := gogit.NewClient(dir, authOpts, clientOpts...)
	if err != nil {
		return nil, err
	}
	defer gitReader.Close()

	commit, err := gitReader.Clone(ctx, url, cloneOpts)
	if err != nil {
		return nil, err
	}

	return commit, nil
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&TargetDir, "target-dir", "t", "", "The target directory to initialize")
	err := initCmd.MarkFlagRequired("target-dir")
	if err != nil {
		panic(err)
	}

	initCmd.Flags().StringVar(&FluxUrl, "flux-url", "", "Flux archive URL")
	initCmd.Flags().StringVar(&FluxDigest, "flux-digest", "", "Flux digest")
	initCmd.MarkFlagsRequiredTogether("flux-url", "flux-digest")

	initCmd.Flags().StringVar(&GitUrl, "git-url", "", "Git repository URL")
	initCmd.Flags().StringVar(&GitRevision, "git-revision", "", "Git revision (tag or commit SHA)")
	initCmd.MarkFlagsRequiredTogether("git-url", "git-revision")
}
