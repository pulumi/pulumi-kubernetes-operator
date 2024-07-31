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
	"path/filepath"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

var (
	WorkDir string
	Host    string
	Port    int
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
		l := zap.L().Named("serve").Sugar()
		cancelCtx := signals.SetupSignalHandler()

		// Create the automation service
		workDir, err := filepath.EvalSymlinks(WorkDir)
		if err != nil {
			l.Errorw("fatal: unable to resolve the workspace directory", zap.Error(err))
			os.Exit(1)
		}
		autoServer, err := server.NewServer(cancelCtx, workDir)
		if err != nil {
			l.Errorw("fatal: unable to open the workspace", zap.Error(err))
			os.Exit(1)
		}

		// Configure the grpc server.
		// Apply zap logging and use filters to reduce log verbosity as needed.
		opts := []grpc_zap.Option{
			grpc_zap.WithDecider(func(fullMethodName string, err error) bool {
				return true
			}),
		}
		decider := func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
			return true
		}
		s := grpc.NewServer(
			grpc.ChainUnaryInterceptor(
				grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
				grpc_zap.UnaryServerInterceptor(rpcLogger, opts...),
				grpc_zap.PayloadUnaryServerInterceptor(rpcLogger, decider),
			),
			grpc.ChainStreamInterceptor(
				grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
				grpc_zap.StreamServerInterceptor(rpcLogger, opts...),
				grpc_zap.PayloadStreamServerInterceptor(rpcLogger, decider),
			),
		)
		pb.RegisterAutomationServiceServer(s, autoServer)

		// Start the grpc server
		address := fmt.Sprintf("%s:%d", Host, Port)
		l.Debugw("starting the RPC server", "address", address)
		lis, err := net.Listen("tcp", address)
		if err != nil {
			l.Errorw("fatal: unable to start the RPC server", zap.Error(err))
			os.Exit(1)
		}
		l.Infow("server listening", "address", lis.Addr(), "workspace", workDir)

		go func() {
			<-cancelCtx.Done()
			l.Infow("shutting down the server")
			s.GracefulStop()
			l.Infow("server stopped")
			os.Exit(0)
		}()
		if err := s.Serve(lis); err != nil {
			l.Errorw("fatal: server failure", zap.Error(err))
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&WorkDir, "workspace", "w", "", "The workspace directory to serve")
	serveCmd.MarkFlagRequired("workspace")

	serveCmd.Flags().StringVar(&Host, "host", "0.0.0.0", "Server bind address (default: 0.0.0.0)")
	serveCmd.Flags().IntVar(&Port, "port", 50051, "Server port (default: 50051)")
}
