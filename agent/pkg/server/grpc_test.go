/*
Copyright © 2024 Pulumi Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package server_test

// TODO: Why are we using a black box test package?

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	pb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

//nolint:paralleltest // Kills child subprocesses.
func TestGracefulShutdown(t *testing.T) {
	// Give this test 10 seconds to spin up and shut down gracefully. Should be
	// more than enough -- typically takes ~2s.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup the server using an in-memory listener.
	tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/hang", Stacks: []string{"test"}})
	log := zap.L().Named("test").Sugar()
	lis := bufconn.Listen(1024)
	s := server.NewGRPC(log, tc.server, nil)
	go func() {
		// This should exit cleanly if we shut down gracefully.
		if err := s.Serve(ctx, lis); err != nil {
			t.Errorf("unexpected serve error: %s", err)
		}
	}()

	// Setup the client.
	conn, err := grpc.NewClient("passthrough://bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	client := pb.NewAutomationServiceClient(conn)

	// Initiate an update using a context separate from the server's.
	stream, err := client.Up(context.Background(), &pb.UpRequest{})
	require.NoError(t, err)

	// Stream events from our update. We will cause the server to shut down
	// once a resourcePreEvent is observed, and it should continue to send
	// events as it shuts down. If it exits cleanly, we expect it to return a
	// cancelEvent along with an non-nil error summarizing what was updated.
	sawCancelEvent := false
	sawSummary := false
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		// A non-nil error is expected if the server acked our cancellation.
		// This includes a final summary of any changes applied before the
		// update was canceled.
		if err != nil && sawCancelEvent {
			assert.ErrorContains(t, err, "urn:pulumi:test::hang::pulumi:pulumi:Stack::hang-test")
			assert.ErrorContains(t, err, "error: update canceled")
			sawSummary = true
			break
		}
		require.NoError(t, err)

		// Start shutting down our server once we see a resourcePreEvent.
		// Ideally this would invoke our signal handler but this is close
		// enough.
		if _, ok := msg.GetEvent().AsMap()["resourcePreEvent"]; ok {
			cancel()
			continue
		}

		// We should eventually see an ack for the cancellation.
		if _, ok := msg.GetEvent().AsMap()["cancelEvent"]; ok {
			sawCancelEvent = true
		}
	}

	assert.True(t, sawCancelEvent)
	assert.True(t, sawSummary)
}
