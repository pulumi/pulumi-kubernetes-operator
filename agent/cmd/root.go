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
	"os"

	"flag"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	verbose     bool
	kubeContext string
)

// a command-specific logger
var log *zap.SugaredLogger

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "agent",
	Short: "Pulumi Kubernetes Operator Agent",
	Long: `Provides tooling and a gRPC service for the Pulumi Kubernetes Operator 
to use to perform stack operations.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		// initialize the global logger
		zc := zap.NewDevelopmentConfig()
		zc.DisableCaller = true
		zc.DisableStacktrace = true
		if !verbose {
			zc.Level.SetLevel(zap.InfoLevel)
		}
		zapLog, err := zc.Build()
		if err != nil {
			return err
		}
		zap.ReplaceGlobals(zapLog)

		// initialize a command-specific logger
		log = zap.L().Named("cmd").Named(cmd.Name()).Sugar()
		cmd.SilenceErrors = true
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// ignore sync errors: https://github.com/uber-go/zap/pull/347
		_ = zap.L().Sync()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		if log != nil {
			log.Error(err.Error())
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	// register the Kubernetes flags (e.g. for serve command when using Kubernetes RBAC for authorization)
	fs := flag.NewFlagSet("kubernetes", flag.ExitOnError)
	config.RegisterFlags(fs)
	rootCmd.PersistentFlags().AddGoFlagSet(fs)
	rootCmd.PersistentFlags().StringVar(&kubeContext, "context", "", "Kubernetes context override")
}

func GetKubeConfig() (*rest.Config, error) {
	return config.GetConfigWithContext(kubeContext)
}
