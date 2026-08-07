// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"net"
	"time"

	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	pb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

const (
	// keepaliveTime is how long the server waits for traffic before pinging the client, and
	// keepaliveTimeout is how long it then waits for the reply before closing the transport.
	keepaliveTime    = 30 * time.Second
	keepaliveTimeout = 20 * time.Second
)

// GRPC serves the automation service.
type GRPC struct {
	*grpc.Server
	wrapped *Server
	log     *zap.SugaredLogger
}

// NewGRPC constructs a new gRPC server with logging and authentication support.
func NewGRPC(rootLogger *zap.SugaredLogger, server *Server, authF grpc_auth.AuthFunc) *GRPC {
	log := rootLogger.Named("grpc")
	// Configure the grpc server.
	// Apply zap logging and use filters to reduce log verbosity as needed.
	serverOpts := []grpc_zap.Option{
		grpc_zap.WithDecider(func(fullMethodName string, err error) bool {
			return true
		}),
	}
	grpc_zap.ReplaceGrpcLoggerV2WithVerbosity(log.Desugar(), int(log.Level()))

	// Apply a default authentication function.
	if authF == nil {
		authF = func(ctx context.Context) (context.Context, error) {
			return ctx, nil
		}
	}

	// Create the gRPC server.
	s := grpc.NewServer(
		// Probe the client during long-running operations. A Pulumi operation streams for as
		// long as it takes, and can be silent for minutes at a stretch, so without probes a
		// disappeared operator leaves the handler -- and the `pulumi` subprocess it started --
		// running with nobody to deliver the result to. Detecting it closes the transport,
		// which cancels the stream context and lets the operation unwind.
		//
		// Server-initiated pings are not subject to any keepalive enforcement policy, so this
		// needs no cooperation from the client and is safe against every operator version. Note
		// that no EnforcementPolicy is set here on purpose: tightening the policy's MinTime
		// would let this agent reject pings from a client that is behaving correctly for the
		// default policy it was written against.
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    keepaliveTime,
			Timeout: keepaliveTimeout,
		}),
		grpc.ChainUnaryInterceptor(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zap.UnaryServerInterceptor(log.Desugar(), serverOpts...),
			grpc_auth.UnaryServerInterceptor(authF),
		),
		grpc.ChainStreamInterceptor(
			grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zap.StreamServerInterceptor(log.Desugar(), serverOpts...),
			grpc_auth.StreamServerInterceptor(authF),
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
