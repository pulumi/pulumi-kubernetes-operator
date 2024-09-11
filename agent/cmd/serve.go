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
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/server"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/version"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
)

var (
	_workDir     string
	_skipInstall bool
	_stack       string
	_host        string
	_port        int
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the agent RPC service",
	Long: `Start the agent gRPC server.
`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		log.Infow("Pulumi Kubernetes Agent", "version", version.Version)
		log.Debugw("executing serve command", "WorkDir", _workDir)

		// open the workspace using auto api
		workspaceOpts := []auto.LocalWorkspaceOption{}
		workDir, err := filepath.EvalSymlinks(_workDir) // resolve the true location of the workspace
		if err != nil {
			log.Fatalw("unable to resolve the workspace directory", zap.Error(err))
			os.Exit(1)
		}
		workspaceOpts = append(workspaceOpts, auto.WorkDir(workDir))
		workspace, err := auto.NewLocalWorkspace(ctx, workspaceOpts...)
		if err != nil {
			log.Fatalw("unable to open the workspace", zap.Error(err))
			os.Exit(1)
		}
		proj, err := workspace.ProjectSettings(ctx)
		if err != nil {
			log.Fatalw("unable to get the project settings", zap.Error(err))
			os.Exit(1)
		}
		log.Infow("opened a local workspace", "workspace", workDir,
			"project", proj.Name, "runtime", proj.Runtime.Name())

		// TODO: Do this during init?
		if !_skipInstall {
			plog := zap.L().Named("pulumi")
			stdout := &zapio.Writer{Log: plog, Level: zap.InfoLevel}
			defer stdout.Close()
			stderr := &zapio.Writer{Log: plog, Level: zap.WarnLevel}
			defer stderr.Close()
			opts := &auto.InstallOptions{
				Stdout: stdout,
				Stderr: stderr,
			}
			log.Infow("installing project dependencies")
			if err := workspace.Install(ctx, opts); err != nil {
				log.Fatalw("installation failed", zap.Error(err))
				os.Exit(1)
			}
			log.Infow("installation completed")
		} else {
			log.Infow("installation skipped",
				"project", proj.Name, "runtime", proj.Runtime.Name())
		}

		// Create the automation service
		autoServer, err := server.NewServer(ctx, workspace, &server.Options{
			StackName: _stack,
		})
		if err != nil {
			log.Fatalw("unable to make an automation server", zap.Error(err))
			os.Exit(1)
		}
		address := fmt.Sprintf("%s:%d", _host, _port)
		log.Infow("starting the RPC server", "address", address)

		s := server.NewGRPC(autoServer, log)

		// Start the grpc server
		lis, err := net.Listen("tcp", address)
		if err != nil {
			log.Errorw("fatal: unable to start the RPC server", zap.Error(err))
			os.Exit(1)
		}
		log.Infow("server listening", "address", lis.Addr(), "workspace", workDir)

		if err := s.Serve(ctx, lis); err != nil {
			log.Errorw("fatal: server failure", zap.Error(err))
			os.Exit(1)
		}

		log.Infow("server stopped")
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&_workDir, "workspace", "w", "", "The workspace directory to serve")
	err := serveCmd.MarkFlagRequired("workspace")
	if err != nil {
		panic(err)
	}

	serveCmd.Flags().BoolVar(&_skipInstall, "skip-install", false, "Skip installation of project dependencies")

	serveCmd.Flags().StringVarP(&_stack, "stack", "s", "", "Select (or create) the stack to use")

	serveCmd.Flags().StringVar(&_host, "host", "0.0.0.0", "Server bind address (default: 0.0.0.0)")
	serveCmd.Flags().IntVar(&_port, "port", 50051, "Server port (default: 50051)")
}
