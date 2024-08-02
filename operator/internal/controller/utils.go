package controller

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func connect(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	if os.Getenv("WORKSPACE_LOCALHOST") != "" {
		addr = os.Getenv("WORKSPACE_LOCALHOST")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to workspace: %w", err)
	}

	// wait for the connection to be ready or the context deadline be reached
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Idle:
			conn.Connect()
		case connectivity.Ready:
			return conn, nil
		case connectivity.Shutdown:
			fallthrough
		case connectivity.TransientFailure:
			_ = conn.Close()
			return nil, fmt.Errorf("unable to connect to workspace: %s", state)
		}
		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			return nil, ctx.Err()
		}
	}
}
