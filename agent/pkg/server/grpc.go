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

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// GRPC serves the automation service.
type GRPC struct {
	*grpc.Server
	wrapped *Server
	log     *zap.SugaredLogger
}

// NewGRPC constructs a new gRPC server with logging and graceful shutdown.
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

// Serve wraps the underlying gRPC server with graceful shutdown. When the
// given context is canceled a SIGTERM is propagated to all child processes
// (spawned by Automation API) and requests are given an opportunity to exit
// cleanly.
func (s *GRPC) Serve(ctx context.Context, l net.Listener) error {
	go func() {
		<-ctx.Done()
		s.log.Infow("shutting down the server")
		s.wrapped.Cancel() // Non-blocking.
		s.GracefulStop()   // Blocks until outstanding requests have finished.
	}()

	return s.Server.Serve(l)
}
