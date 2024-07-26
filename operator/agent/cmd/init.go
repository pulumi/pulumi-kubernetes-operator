/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"io"
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

		// verify that the target directory doesn't exist, for safety and to dictate the file permissions.
		target, err := os.Stat(TargetDir)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
			os.Exit(1)
		}
		if target != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "fatal: target directory exists")
			os.Exit(1)
		}

		err = os.MkdirAll(TargetDir, 0777)
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "fatal: unable to create target directory")
			os.Exit(1)
		}

		// fetch the configured flux source
		if FluxUrl != "" {
			// https://github.com/fluxcd/kustomize-controller/blob/a1a33f2adda783dd2a17234f5d8e84caca4e24e2/internal/controller/kustomization_controller.go#L328
			fetcher := fetch.New(
				fetch.WithRetries(DefaultFluxRetries),
				fetch.WithHostnameOverwrite(os.Getenv("SOURCE_CONTROLLER_LOCALHOST")),
				fetch.WithUntar())

			fmt.Fprintln(cmd.OutOrStdout(), "Fetching Flux source...")
			err := fetcher.FetchWithContext(ctx, FluxUrl, FluxDigest, TargetDir)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Fetch error: %s\n", err.Error())
				os.Exit(1)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Source fetched")
		}
	},
}

func isEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&TargetDir, "target-dir", "t", "", "The target directory to initialize")
	initCmd.MarkFlagRequired("target-dir")

	initCmd.Flags().StringVar(&FluxUrl, "flux-url", "", "Flux archive URL")
	initCmd.Flags().StringVar(&FluxDigest, "flux-digest", "", "Flux digest")
	initCmd.MarkFlagsRequiredTogether("flux-url", "flux-digest")
}
