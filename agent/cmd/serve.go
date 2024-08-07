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
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/server"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/version"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"google.golang.org/grpc"
)

var (
	WorkDir     string
	SkipInstall bool
	Stack       string
	Host        string
	Port        int
)

var rpcLogger *zap.Logger

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the agent RPC service",
	Long: `Start the agent gRPC server.
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		rpcLogger = zap.L().Named("rpc")
		grpc_zap.ReplaceGrpcLoggerV2(rpcLogger)
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		log.Infow("Pulumi Kubernetes Agent", "version", version.Version)
		log.Debugw("executing serve command", "WorkDir", WorkDir)

		// open the workspace using auto api
		workspaceOpts := []auto.LocalWorkspaceOption{}
		workDir, err := filepath.EvalSymlinks(WorkDir) // resolve the true location of the workspace
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

		if !SkipInstall {
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
		}

		// Create the automation service
		autoServer, err := server.NewServer(ctx, workspace, &server.Options{
			StackName: Stack,
		})
		if err != nil {
			log.Fatalw("unable to make an automation server", zap.Error(err))
			os.Exit(1)
		}

		// Configure the grpc server.
		// Apply zap logging and use filters to reduce log verbosity as needed.
		address := fmt.Sprintf("%s:%d", Host, Port)
		log.Infow("starting the RPC server", "address", address)
		serverOpts := []grpc_zap.Option{
			grpc_zap.WithDecider(func(fullMethodName string, err error) bool {
				return true
			}),
		}
		decider := func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
			return verbose
		}
		s := grpc.NewServer(
			grpc.ChainUnaryInterceptor(
				grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
				grpc_zap.UnaryServerInterceptor(rpcLogger, serverOpts...),
				grpc_zap.PayloadUnaryServerInterceptor(rpcLogger, decider),
			),
			grpc.ChainStreamInterceptor(
				grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
				grpc_zap.StreamServerInterceptor(rpcLogger, serverOpts...),
				grpc_zap.PayloadStreamServerInterceptor(rpcLogger, decider),
			),
		)
		pb.RegisterAutomationServiceServer(s, autoServer)

		// Start the grpc server
		lis, err := net.Listen("tcp", address)
		if err != nil {
			log.Errorw("fatal: unable to start the RPC server", zap.Error(err))
			os.Exit(1)
		}
		log.Infow("server listening", "address", lis.Addr(), "workspace", workDir)

		cancelCtx := setupSignalHandler()
		go func() {
			<-cancelCtx.Done()
			log.Infow("shutting down the server")
			s.GracefulStop()
			autoServer.Cancel()
			log.Infow("server stopped")
			os.Exit(0)
		}()
		if err := s.Serve(lis); err != nil {
			log.Errorw("fatal: server failure", zap.Error(err))
			os.Exit(1)
		}
	},
}

// SetupSignalHandler registers for SIGTERM and SIGINT. A context is returned
// which is canceled on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&WorkDir, "workspace", "w", "", "The workspace directory to serve")
	serveCmd.MarkFlagRequired("workspace")

	serveCmd.Flags().BoolVar(&SkipInstall, "skip-install", false, "Skip installation of project dependencies")

	serveCmd.Flags().StringVarP(&Stack, "stack", "s", "", "Select (or create) the stack to use")

	serveCmd.Flags().StringVar(&Host, "host", "0.0.0.0", "Server bind address (default: 0.0.0.0)")
	serveCmd.Flags().IntVar(&Port, "port", 50051, "Server port (default: 50051)")
}
