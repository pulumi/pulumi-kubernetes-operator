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

//go:build unix

package controller

import (
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertKeepaliveTuned checks the probe interval and count on the socket.
//
// Deliberately not asserting SO_KEEPALIVE itself: net.Dialer turns keepalive on by default
// (with a 15s idle), so that would pass no matter what this package does. The interval and
// count are the parts that are not defaults -- on Linux the defaults are 75s and 9 probes,
// giving ~11 minutes to notice a black-holed peer -- so they are what pins the change.
func assertKeepaliveTuned(t *testing.T, conn *net.TCPConn) {
	t.Helper()

	raw, err := conn.SyscallConn()
	require.NoError(t, err)

	get := func(name int) int {
		t.Helper()
		var (
			value  int
			optErr error
		)
		require.NoError(t, raw.Control(func(fd uintptr) {
			value, optErr = syscall.GetsockoptInt(int(fd), syscall.IPPROTO_TCP, name)
		}))
		require.NoError(t, optErr)
		return value
	}

	assert.Equal(t, int(workspaceKeepaliveInterval.Seconds()), get(syscall.TCP_KEEPINTVL),
		"probe interval should be the tuned value, not the system default")
	assert.Equal(t, workspaceKeepaliveCount, get(syscall.TCP_KEEPCNT),
		"probe count should be the tuned value, not the system default")
}
