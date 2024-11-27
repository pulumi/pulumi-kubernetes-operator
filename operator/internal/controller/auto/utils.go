package controller

import (
	"context"
	"fmt"
	"os"
	"reflect"

	agentclient "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/client"
	agentpb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func connect(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	if os.Getenv("WORKSPACE_LOCALHOST") != "" {
		addr = os.Getenv("WORKSPACE_LOCALHOST")
	}

	token := os.Getenv("WORKSPACE_TOKEN")
	tokenFile := os.Getenv("WORKSPACE_TOKEN_FILE")
	if token == "" && tokenFile == "" {
		// use in-cluster configuration using the operator's service account token
		tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	}
	creds, err := agentclient.NewTokenCredentials(token, tokenFile)
	if err != nil {
		return nil, fmt.Errorf("unable to use token credentials: %w", err)
	}

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(creds))
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

func marshalConfigItem(item autov1alpha1.ConfigItem) *agentpb.ConfigItem {
	v := &agentpb.ConfigItem{
		Key:    item.Key,
		Path:   item.Path,
		Secret: item.Secret,
	}
	if item.Value != nil {
		v.V = &agentpb.ConfigItem_Value{
			Value: structpb.NewStringValue(*item.Value),
		}
	}
	if item.ValueFrom != nil {
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
		v.V = &agentpb.ConfigItem_ValueFrom{
			ValueFrom: f,
		}
	}
	return v
}

var l = log.Log.WithName("predicate").WithName("debug")

type DebugPredicate struct {
	Controller string
}

var _ predicate.Predicate = &DebugPredicate{}

func (p *DebugPredicate) Create(e event.CreateEvent) bool {
	l.V(1).Info("Create", "controller", p.Controller, "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName(), "revision", e.Object.GetResourceVersion())
	return true
}

func (p *DebugPredicate) Delete(e event.DeleteEvent) bool {
	l.V(1).Info("Delete", "controller", p.Controller, "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName(), "revision", e.Object.GetResourceVersion())
	return true
}

func (p *DebugPredicate) Update(e event.UpdateEvent) bool {
	l.V(1).Info("Update", "controller", p.Controller, "type", fmt.Sprintf("%T", e.ObjectOld), "name", e.ObjectOld.GetName(), "old-revision", e.ObjectOld.GetResourceVersion(), "new-revision", e.ObjectNew.GetResourceVersion())
	return true
}

func (p *DebugPredicate) Generic(e event.GenericEvent) bool {
	l.V(1).Info("Generic", "controller", p.Controller, "type", fmt.Sprintf("%T", e.Object), "name", e.Object.GetName(), "revision", e.Object.GetResourceVersion())
	return true
}

type OwnerReferencesChangedPredicate struct{}

var _ predicate.Predicate = &OwnerReferencesChangedPredicate{}

func (OwnerReferencesChangedPredicate) Create(e event.CreateEvent) bool {
	return false
}

func (OwnerReferencesChangedPredicate) Delete(_ event.DeleteEvent) bool {
	return false
}

func (OwnerReferencesChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return reflect.DeepEqual(e.ObjectOld.GetOwnerReferences(), e.ObjectNew.GetOwnerReferences())
}

func (OwnerReferencesChangedPredicate) Generic(_ event.GenericEvent) bool {
	return false
}
