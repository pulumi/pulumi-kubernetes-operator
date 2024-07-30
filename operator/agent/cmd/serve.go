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

	"github.com/go-logr/logr"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/log"
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

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve the agent RPC service",
	Long: `Start the agent gRPC server.
`,
	Run: func(cmd *cobra.Command, args []string) {
		cancelCtx := signals.SetupSignalHandler()

		// Create the automation service
		workDir, err := filepath.EvalSymlinks(WorkDir)
		if err != nil {
			Log.Error(err, "fatal: unable to resolve the workspace directory")
			os.Exit(1)
		}
		Log.V(1).Info("opening the workspace", "workspace", workDir)
		serverCtx := logr.NewContext(cmd.Context(), log.Global.WithName("server"))
		autoServer, err := server.NewServer(serverCtx, workDir)
		if err != nil {
			Log.Error(err, "fatal: unable to open the workspace")
			os.Exit(1)
		}

		// Start the grpc server
		loggerOpts := []logging.Option{
			logging.WithLogOnEvents(logging.StartCall, logging.PayloadReceived, logging.PayloadSent, logging.FinishCall),
		}
		address := fmt.Sprintf("%s:%d", Host, Port)
		Log.V(1).Info("starting the RPC server", "address", address)
		lis, err := net.Listen("tcp", address)
		if err != nil {
			Log.Error(err, "fatal: unable to start the RPC server")
			os.Exit(1)
		}
		s := grpc.NewServer(
			grpc.ChainUnaryInterceptor(
				logging.UnaryServerInterceptor(InterceptorLogger(Log), loggerOpts...),
			),
			grpc.ChainStreamInterceptor(
				logging.StreamServerInterceptor(InterceptorLogger(Log), loggerOpts...),
			),
		)

		pb.RegisterAutomationServiceServer(s, autoServer)
		Log.Info("server listening", "address", lis.Addr(), "workspace", workDir)

		go func() {
			<-cancelCtx.Done()
			Log.Info("shutting down the server")
			s.GracefulStop()
			Log.Info("server stopped")
			os.Exit(0)
		}()
		if err := s.Serve(lis); err != nil {
			Log.Error(err, "fatal: server failure")
			os.Exit(1)
		}
	},
}

const (
	debugVerbosity = int(-zap.DebugLevel)
	infoVerbosity  = int(-zap.InfoLevel)
	warnVerbosity  = int(-zap.WarnLevel)
	errorVerbosity = int(-zap.ErrorLevel)
)

// InterceptorLogger adapts logr logger to interceptor logger.
func InterceptorLogger(l logr.Logger) logging.Logger {
	return logging.LoggerFunc(func(_ context.Context, lvl logging.Level, msg string, fields ...any) {
		l := l.WithValues(fields...)
		switch lvl {
		case logging.LevelDebug:
			l.V(debugVerbosity).Info(msg)
		case logging.LevelInfo:
			l.V(infoVerbosity).Info(msg)
		case logging.LevelWarn:
			l.V(warnVerbosity).Info(msg)
		case logging.LevelError:
			l.V(errorVerbosity).Info(msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&WorkDir, "workspace", "w", "", "The workspace directory to serve")
	serveCmd.MarkFlagRequired("workspace")

	serveCmd.Flags().StringVar(&Host, "host", "0.0.0.0", "Server bind address (default: 0.0.0.0)")
	serveCmd.Flags().IntVar(&Port, "port", 50051, "Server port (default: 50051)")
}
