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
package server

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type GRPC struct {
	*grpc.Server
	wrapped *Server
	log     *zap.SugaredLogger
}

func NewGRPC(server *Server, rootLogger *zap.SugaredLogger) *GRPC {
	log := rootLogger.Named("grpc")
	// Configure the grpc server.
	// Apply zap logging and use filters to reduce log verbosity as needed.
	serverOpts := []grpc_zap.Option{
		grpc_zap.WithDecider(func(fullMethodName string, err error) bool {
			return true
		}),
	}

	grpc_zap.ReplaceGrpcLoggerV2WithVerbosity(log.Desugar(), int(log.Level()))

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zap.UnaryServerInterceptor(log.Desugar(), serverOpts...),
		),
		grpc.ChainStreamInterceptor(
			grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zap.StreamServerInterceptor(log.Desugar(), serverOpts...),
		),
	)
	pb.RegisterAutomationServiceServer(s, server)

	return &GRPC{Server: s, wrapped: server, log: log}
}

// Serve wraps the underlying gRPC server with graceful shutdown. When a
// SIGTERM is received it is propagated to all child processes (spawned by
// Automation API) and requests are given an opportunity to exit cleanly.
func (s *GRPC) Serve(ctx context.Context, l net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	setupSignalHandler(cancel)

	go func() {
		<-ctx.Done()
		s.log.Infow("shutting down the server")
		s.wrapped.Cancel() // Non-blocking.
		s.GracefulStop()   // Blocks until outstanding requests have finished.
	}()

	return s.Server.Serve(l)
}

// SetupSignalHandler registers for SIGTERM and SIGINT. The fist signal invokes
// the cancel method. If a second signal is caught, the program is terminated
// with exit code 1.
func setupSignalHandler(cancel func()) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // Second signal, exit directly.
	}()
}
