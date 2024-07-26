/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/server"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

const (
	defaultGracefulShutdownTimeout = 5 * time.Minute
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
			fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
			os.Exit(1)
		}
		log.Printf("Opening workspace at %s\n", workDir)
		autoServer, err := server.NewServer(context.Background(), workDir)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Server error: %s\n", err.Error())
			os.Exit(1)
		}

		// Start the grpc server
		log.Println("Starting the Agent")
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", Host, Port))
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		s := grpc.NewServer()
		pb.RegisterAutomationServiceServer(s, autoServer)

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
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().StringVarP(&WorkDir, "workspace", "w", "", "The workspace directory to serve")
	serveCmd.MarkFlagRequired("workspace")

	serveCmd.Flags().StringVar(&Host, "host", "0.0.0.0", "Server bind address (default: 0.0.0.0)")
	serveCmd.Flags().IntVar(&Port, "port", 50051, "Server port (default: 50051)")
}
