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
	"net"
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
	maxRPCMessageSize    = 1024 * 1024 * 400

	// TCP keepalive settings for connections to workspace pods. A Pulumi operation is a
	// long-lived server-streaming RPC that can legitimately produce no traffic for a long
	// time (a slow provider call, say), so a peer that silently vanishes -- a lost node, a
	// partition -- is indistinguishable from a quiet one except by probing.
	//
	// net.Dialer already enables keepalive with a 15s idle, but leaves the probe interval and
	// count to the system: on Linux that is 75s x 9, so noticing takes about 11 minutes. These
	// values bring that down to about a minute. Note that this only bounds a *broken*
	// connection; a workspace whose connection is healthy while its Pulumi operation is wedged
	// is invisible at this layer, and is what the update controller's idle timeout is for.
	//
	// These are deliberately TCP-level rather than gRPC-level keepalives. gRPC pings are
	// policed by the server's keepalive.EnforcementPolicy, whose default MinTime is 5
	// minutes; pinging more often than a workspace's agent allows earns a GOAWAY with
	// ENHANCE_YOUR_CALM and kills the stream. Since the agent image is pinned per Stack, the
	// operator routinely talks to older agents whose policy it cannot know. TCP probes are
	// invisible to HTTP/2, so they are safe against every agent version.
	//
	// A broken connection is detected after roughly workspaceKeepaliveIdle +
	// workspaceKeepaliveInterval*workspaceKeepaliveCount.
	workspaceKeepaliveIdle     = 30 * time.Second
	workspaceKeepaliveInterval = 10 * time.Second
	workspaceKeepaliveCount    = 3
)

// dialWorkspace establishes a TCP connection to a workspace pod with keepalive probes
// enabled, so that losing the pod or the network surfaces as a stream error rather than an
// indefinite block. See the workspaceKeepalive* constants.
func dialWorkspace(ctx context.Context, addr string) (net.Conn, error) {
	d := &net.Dialer{
		KeepAliveConfig: net.KeepAliveConfig{
			Enable:   true,
			Idle:     workspaceKeepaliveIdle,
			Interval: workspaceKeepaliveInterval,
			Count:    workspaceKeepaliveCount,
		},
	}
	return d.DialContext(ctx, "tcp", addr)
}

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
		grpc.WithPerRPCCredentials(creds),
		grpc.WithContextDialer(dialWorkspace),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxRPCMessageSize)))
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
