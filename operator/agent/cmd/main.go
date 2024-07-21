// Copyright 2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"time"

	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/server"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/version"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

const (
	defaultGracefulShutdownTimeout = 5 * time.Minute
)

var (
	host    = pflag.String("host", "0.0.0.0", "The server host")
	port    = pflag.Int("port", 50051, "The server port")
	workdir = pflag.String("workspace", "", "The workspace directory")
)

func printVersion() {
	log.Printf("Agent Version: %s\n", version.Version)
	log.Printf("Go Version: %s\n", runtime.Version())
	log.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func main() {

	// Add flags registered by imported packages
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	printVersion()

	if *workdir == "" {
		w, err := os.MkdirTemp("", "pulumi-k8s-operator-*")
		if err != nil {
			log.Fatalf("failed to create temporary directory: %v", err)
		}
		workdir = &w
	}
	log.Printf("workspace dir: %q\n", *workdir)

	cancelCtx := signals.SetupSignalHandler()

	// Start the grpc server
	log.Println("Starting the Agent")
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *host, *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterAutomationServiceServer(s, server.NewServer(cancelCtx, *workdir))

	log.Printf("server listening at %v\n", lis.Addr())

	go func() {
		<-cancelCtx.Done()
		log.Println("Shutting down the Agent")
		s.GracefulStop()
		log.Printf("server stopped")
		os.Exit(0)
	}()
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
