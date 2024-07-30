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

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var verbose bool
var zapLog *zap.Logger
var Log logr.Logger

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "agent",
	Short: "Pulumi Kubernetes Operator Agent",
	Long: `Provides tooling and a gRPC service for the Pulumi Kubernetes Operator 
to use to perform stack operations.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		zc := zap.NewDevelopmentConfig()
		if !verbose {
			zc.Level.SetLevel(zap.InfoLevel)
		}
		zapLog, err = zc.Build()
		if err != nil {
			return err
		}
		log.Global = zapr.NewLogger(zapLog)

		Log = log.Global.WithName("cmd")
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
}
