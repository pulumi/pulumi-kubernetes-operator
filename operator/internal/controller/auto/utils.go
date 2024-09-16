package controller

import (
	"context"
	"fmt"
	"os"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
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

func marshalConfigValue(item autov1alpha1.ConfigItem) *agentpb.ConfigValue {
	v := &agentpb.ConfigValue{
		Secret: item.Secret,
	}
	if item.Value != nil {
		v.V = &agentpb.ConfigValue_Value{
			Value: structpb.NewStringValue(*item.Value),
		}
	} else if item.ValueFrom != nil {
		f := &agentpb.ConfigValueFrom{}
		if item.ValueFrom.Env != "" {
			f.F = &agentpb.ConfigValueFrom_Env{
				Env: item.ValueFrom.Env,
			}
		} else if item.ValueFrom.Path != "" {
			f.F = &agentpb.ConfigValueFrom_Path{
				Path: item.ValueFrom.Path,
			}
		}
		v.V = &agentpb.ConfigValue_ValueFrom{
			ValueFrom: f,
		}
	}
	return v
}