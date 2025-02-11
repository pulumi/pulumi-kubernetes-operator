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

package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/client"
	agentpb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ConnectionManager struct {
	factory client.TokenSourceFactory
}

type ConnectionManagerOptions struct {
	// The service account to impersonate for authentication purposes (i.e. the operator's KSA).
	ServiceAccount types.NamespacedName
}

func NewConnectionManager(config *rest.Config, opts ConnectionManagerOptions) (*ConnectionManager, error) {
	if config == nil {
		return nil, errors.New("must specify Config")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	factory := client.NewServiceAccountTokenFactory(clientset.CoreV1(), opts.ServiceAccount.Namespace, opts.ServiceAccount.Name)
	
	return &ConnectionManager{
		factory: factory,
	}, nil
}

func (cm *ConnectionManager) Connect(ctx context.Context, w *autov1alpha1.Workspace) (*grpc.ClientConn, error) {
	audience := audienceForWorkspace(w)
	creds := client.NewTokenCredentials(cm.factory.TokenSource(audience))

	addr := fmt.Sprintf("%s:%d", fqdnForService(w), WorkspaceGrpcPort)
	l.Info("Connecting", "addr", addr)
	if os.Getenv("WORKSPACE_LOCALHOST") != "" {
		addr = os.Getenv("WORKSPACE_LOCALHOST")
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

func audienceForWorkspace(w *autov1alpha1.Workspace) string {
	return fmt.Sprintf("%s.%s", w.Name, w.Namespace)
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
	return !reflect.DeepEqual(e.ObjectOld.GetOwnerReferences(), e.ObjectNew.GetOwnerReferences())
}

func (OwnerReferencesChangedPredicate) Generic(_ event.GenericEvent) bool {
	return false
}
