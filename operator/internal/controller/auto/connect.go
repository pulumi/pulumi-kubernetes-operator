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
	"time"

	"github.com/go-logr/logr"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/client"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	pruneTokensOlderThan = 2 * time.Hour
)

// ConnectionManager is responsible for managing connections to workspaces.
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
	factory := client.NewServiceAccount(clientset.CoreV1().ServiceAccounts(opts.ServiceAccount.Namespace), opts.ServiceAccount.Name)

	return &ConnectionManager{
		factory: factory,
	}, nil
}

func (cm *ConnectionManager) Connect(ctx context.Context, w *autov1alpha1.Workspace) (*grpc.ClientConn, error) {
	l := log.FromContext(ctx)
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

// Starts the connection manager, e.g. to periodically clean token caches.
func (m *ConnectionManager) Start(ctx context.Context) error {
	l := logr.FromContextOrDiscard(ctx)
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				unusedSince := time.Now().Add(-1 * pruneTokensOlderThan)
				n := m.factory.Prune(unusedSince)
				l.Info("pruned the token cache", "unusedSince", unusedSince, "pruned", n)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func audienceForWorkspace(w *autov1alpha1.Workspace) string {
	return fmt.Sprintf("%s.%s", w.Name, w.Namespace)
}
