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
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDialWorkspaceTunesKeepalive checks that connections to workspace pods carry tightened TCP
// keepalive probes, so that a workspace lost to a node failure or partition is noticed in about
// a minute rather than the ~11 minutes the system defaults allow. That bounds how long such a
// peer can keep an update controller worker parked in Recv().
// See https://github.com/pulumi/pulumi-kubernetes-operator/issues/1293.
func TestDialWorkspaceTunesKeepalive(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })

	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := lis.Accept()
		if err != nil {
			return
		}
		accepted <- c
	}()

	conn, err := dialWorkspace(t.Context(), lis.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	tcpConn, ok := conn.(*net.TCPConn)
	require.True(t, ok, "expected a TCP connection, got %T", conn)
	assertKeepaliveTuned(t, tcpConn)

	select {
	case c := <-accepted:
		_ = c.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("the server never accepted the connection")
	}
}

// TestWorkspaceKeepaliveBoundsDetection guards the intent of the keepalive constants: they must
// bound how long a lost workspace can go unnoticed. A regression that disabled or greatly
// relaxed them would reintroduce the indefinite block, so pin the resulting detection window.
func TestWorkspaceKeepaliveBoundsDetection(t *testing.T) {
	assert.Positive(t, workspaceKeepaliveIdle)
	assert.Positive(t, workspaceKeepaliveInterval)
	assert.Positive(t, workspaceKeepaliveCount)

	detection := workspaceKeepaliveIdle +
		workspaceKeepaliveInterval*time.Duration(workspaceKeepaliveCount)
	assert.LessOrEqual(t, detection, 2*time.Minute,
		"a dead workspace should be detected in minutes, not hours")
}

// TestDialWorkspaceHonorsContext checks that a dial is abandoned when its context is done, so
// that Connect's own timeout still governs how long connecting may take.
func TestDialWorkspaceHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// 203.0.113.0/24 is reserved for documentation, so this cannot connect.
	_, err := dialWorkspace(ctx, "203.0.113.1:50051")
	assert.ErrorIs(t, err, context.Canceled)
}
