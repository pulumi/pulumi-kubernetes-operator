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
	"os"

	"github.com/fluxcd/pkg/http/fetch"
	"github.com/spf13/cobra"
)

const (
	DefaultFluxRetries = 3
)

var (
	TargetDir  string
	FluxUrl    string
	FluxDigest string
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
		ctx := context.Background()

		err := os.MkdirAll(TargetDir, 0777)
		if err != nil {
			Log.Error(err, "fatal: unable to make target directory")
			os.Exit(1)
		}
		Log.V(1).Info("target directory created", "dir", TargetDir)

		// fetch the configured flux source
		if FluxUrl != "" {
			// https://github.com/fluxcd/kustomize-controller/blob/a1a33f2adda783dd2a17234f5d8e84caca4e24e2/internal/controller/kustomization_controller.go#L328
			fetcher := fetch.New(
				fetch.WithRetries(DefaultFluxRetries),
				fetch.WithHostnameOverwrite(os.Getenv("SOURCE_CONTROLLER_LOCALHOST")),
				fetch.WithUntar())

			Log.Info("flux source fetching", "url", FluxUrl, "digest", FluxDigest)
			err := fetcher.FetchWithContext(ctx, FluxUrl, FluxDigest, TargetDir)
			if err != nil {
				Log.Error(err, "fatal: unable to fetch flux source")
				os.Exit(2)
			}
			Log.Info("flux source fetched", "dir", TargetDir)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&TargetDir, "target-dir", "t", "", "The target directory to initialize")
	initCmd.MarkFlagRequired("target-dir")

	initCmd.Flags().StringVar(&FluxUrl, "flux-url", "", "Flux archive URL")
	initCmd.Flags().StringVar(&FluxDigest, "flux-digest", "", "Flux digest")
	initCmd.MarkFlagsRequiredTogether("flux-url", "flux-digest")
}
