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

package server

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/utils/ptr"

	pb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	TestStackName = "test"
)

func TestNewServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
		opts       *Options
		wantErr    any
	}{
		{
			name:       "simple",
			projectDir: "./testdata/simple",
		},
		{
			name:       "uninstallable",
			projectDir: "./testdata/uninstallable",
		},
		{
			name:       "empty",
			projectDir: "",
			wantErr:    gomega.ContainSubstring("unable to find project settings in workspace"),
		},
		{
			name:       "new stack",
			projectDir: "./testdata/simple",
			opts:       &Options{StackName: "new"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)

			ctx := newContext(t)
			ws := newWorkspace(ctx, t, tt.projectDir)

			_, err := NewServer(ctx, ws, tt.opts)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				if tt.opts != nil && tt.opts.StackName != "" {
					// ensure that the stack was actually selected
					current, err := ws.Stack(ctx)
					g.Expect(err).ToNot(gomega.HaveOccurred())
					g.Expect(current.Name).To(gomega.Equal(tt.opts.StackName))
				}
			}
		})
	}
}

func TestWhoAmI(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	u, err := user.Current()
	g.Expect(err).ToNot(gomega.HaveOccurred())

	tests := []struct {
		name    string
		req     *pb.WhoAmIRequest
		wantErr any
		want    string
	}{
		{
			name: "logged in",
			req:  &pb.WhoAmIRequest{},
			want: u.Username,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple"})

			res, err := tc.server.WhoAmI(ctx, tt.req)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(res.User).To(gomega.Equal(tt.want))
			}
		})
	}
}

func TestSelectStack(t *testing.T) {
	t.Parallel()

	// hasSummary := func(name string) types.GomegaMatcher {
	// 	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
	// 		"Summary": gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
	// 			"Name": gomega.Equal(name),
	// 		})),
	// 	})
	// }

	tests := []struct {
		name    string
		stacks  []string
		req     *pb.SelectStackRequest
		wantErr any
	}{
		{
			name:   "already selected stack",
			stacks: []string{"one"},
			req: &pb.SelectStackRequest{
				StackName: "one",
			},
		},
		{
			name:   "existent stack",
			stacks: []string{"one", "two"},
			req: &pb.SelectStackRequest{
				StackName: "one",
			},
		},
		{
			name: "non-existent stack",
			req: &pb.SelectStackRequest{
				StackName: "unexpected",
			},
			wantErr: status.Error(codes.NotFound, "stack not found"),
		},
		{
			name: "non-existent stack with create",
			req: &pb.SelectStackRequest{
				StackName: "one",
				Create:    ptr.To(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple", Stacks: tt.stacks})

			res, err := tc.server.SelectStack(ctx, tt.req)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())

				// check the response for accuracy
				g.Expect(res.Summary.Name).To(gomega.Equal(tt.req.StackName))

				// ensure that the stack was actually selected
				current, err := tc.ws.Stack(ctx)
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(current.Name).To(gomega.Equal(tt.req.StackName))
			}
		})
	}
}

func TestInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stacks  []string
		req     *pb.InfoRequest
		want    types.GomegaMatcher
		wantErr any
	}{
		{
			name:    "no active stack",
			stacks:  []string{},
			req:     &pb.InfoRequest{},
			wantErr: status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:   "active stack",
			stacks: []string{TestStackName},
			req:    &pb.InfoRequest{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple", Stacks: tt.stacks})
			res, err := tc.server.Info(ctx, tt.req)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				// check the response for accuracy
				current, err := tc.ws.Stack(ctx)
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(res.Summary.Name).To(gomega.Equal(current.Name))
			}
		})
	}
}

func TestSetAllConfig(t *testing.T) {
	t.Parallel()

	// SetAllConfig updates the config based on literals, envs, and files

	strVal := structpb.NewStringValue("bar")
	boolVal := structpb.NewBoolValue(true)

	tests := []struct {
		name    string
		stacks  []string
		req     *pb.SetAllConfigRequest
		wantErr error
		want    auto.ConfigMap
	}{
		{
			name:    "no active stack",
			stacks:  []string{},
			req:     &pb.SetAllConfigRequest{},
			wantErr: status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:   "literal",
			stacks: []string{TestStackName},
			req: &pb.SetAllConfigRequest{
				Config: []*pb.ConfigItem{{
					Key: "foo",
					V:   &pb.ConfigItem_Value{Value: strVal},
				}},
			},
			want: auto.ConfigMap{
				"simple:foo": {Value: "bar"},
			},
		},
		{
			name:   "env",
			stacks: []string{TestStackName},
			req: &pb.SetAllConfigRequest{
				// note that TestMain sets the FOO environment variable to a test value
				Config: []*pb.ConfigItem{{
					Key: "foo",
					V:   &pb.ConfigItem_ValueFrom{ValueFrom: &pb.ConfigValueFrom{F: &pb.ConfigValueFrom_Env{Env: "FOO"}}},
				}},
			},
			want: auto.ConfigMap{
				"simple:foo": {Value: "bar"},
			},
		},
		{
			name:   "file",
			stacks: []string{TestStackName},
			req: &pb.SetAllConfigRequest{
				// note that TestMain sets the FOO environment variable to a test value
				Config: []*pb.ConfigItem{{
					Key: "foo",
					V:   &pb.ConfigItem_ValueFrom{ValueFrom: &pb.ConfigValueFrom{F: &pb.ConfigValueFrom_Path{Path: "./testdata/foo.txt"}}},
				}},
			},
			want: auto.ConfigMap{
				"simple:foo": {Value: "bar"},
			},
		},
		{
			name:   "path-based",
			stacks: []string{TestStackName},
			req: &pb.SetAllConfigRequest{
				Config: []*pb.ConfigItem{
					{
						Key:  "a.b",
						Path: ptr.To(true),
						V:    &pb.ConfigItem_Value{Value: strVal},
					},
				},
			},
			want: auto.ConfigMap{
				"simple:a": {Value: `{"b":"bar"}`},
			},
		},
		{
			name:   "path-like",
			stacks: []string{TestStackName},
			req: &pb.SetAllConfigRequest{
				Config: []*pb.ConfigItem{
					{
						Key:  "a.b",
						Path: ptr.To(false),
						V:    &pb.ConfigItem_Value{Value: strVal},
					}, {
						Key:  `"quoted[key]"`,
						Path: ptr.To(false),
						V:    &pb.ConfigItem_Value{Value: strVal},
					}, {
						Key:  `["already.escaped"]`,
						Path: ptr.To(false),
						V:    &pb.ConfigItem_Value{Value: strVal},
					},
				},
			},
			want: auto.ConfigMap{
				"simple:a.b":             {Value: "bar"},
				`simple:"quoted[key]"`:   {Value: "bar"},
				`simple:already.escaped`: {Value: "bar"},
			},
		},
		{
			name:   "path-overlay",
			stacks: []string{"nested"},
			req: &pb.SetAllConfigRequest{
				Config: []*pb.ConfigItem{
					{
						Key:  "myList[0]",
						Path: ptr.To(true),
						V:    &pb.ConfigItem_Value{Value: structpb.NewStringValue("1")},
					},
					{
						Key:  "myList[1]",
						Path: ptr.To(true),
						V:    &pb.ConfigItem_Value{Value: structpb.NewStringValue("2")},
					},
					{
						Key:  "myList[3]",
						Path: ptr.To(true),
						V:    &pb.ConfigItem_Value{Value: structpb.NewStringValue("four")},
					},
					{
						Key:  "outer.more.inner",
						Path: ptr.To(true),
						V:    &pb.ConfigItem_Value{Value: structpb.NewStringValue("wrapped")},
					},
				},
			},
			want: auto.ConfigMap{
				"aws:region":    {Value: `us-west-2`},
				"simple:foo":    {Value: `aha`},
				"simple:bar":    {Value: `1234`},
				"simple:myList": {Value: `[1,2,"three","four"]`},
				"simple:outer":  {Value: `{"inner":"my_value","more":{"inner":"wrapped"},"other":"something_else"}`},
			},
		},
		{
			name:   "secret",
			stacks: []string{TestStackName},
			req: &pb.SetAllConfigRequest{
				Config: []*pb.ConfigItem{{
					Key: "foo",
					V:   &pb.ConfigItem_Value{Value: strVal}, Secret: ptr.To(true),
				}},
			},
			want: auto.ConfigMap{
				"simple:foo": {Value: "bar", Secret: true},
			},
		},
		{
			name:   "typed",
			stacks: []string{TestStackName},
			req: &pb.SetAllConfigRequest{
				Config: []*pb.ConfigItem{{
					Key: "enabled",
					V:   &pb.ConfigItem_Value{Value: boolVal},
				}},
			},
			want: auto.ConfigMap{
				"simple:enabled": {Value: "true"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple", Stacks: tt.stacks})
			_, err := tc.server.SetAllConfig(ctx, tt.req)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			actual, err := tc.ws.GetAllConfig(ctx, tc.stack.Name())
			assert.NoError(t, err)
			assert.Equal(t, tt.want, actual)
		})
	}
}

func TestSetEnvironments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stacks  []string
		req     *pb.AddEnvironmentsRequest
		wantErr error
		want    []string
	}{
		{
			name:    "no active stack",
			stacks:  []string{},
			req:     &pb.AddEnvironmentsRequest{},
			wantErr: status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:   "added environment",
			stacks: []string{TestStackName},
			req: &pb.AddEnvironmentsRequest{
				Environment: []string{"test"},
			},
			want: []string{"test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple", Stacks: tt.stacks})
			_, err := tc.server.AddEnvironments(ctx, tt.req)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			// note: the file backend does not support environments
			assert.ErrorContains(t, err, "does not support environments")
			// actual, err := tc.ws.ListEnvironments(ctx, tc.stack.Name())
			// assert.NoError(t, err)
			// assert.Equal(t, tt.want, actual)
		})
	}
}

func TestInstall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
		req        *pb.InstallRequest
		wantErr    types.GomegaMatcher
	}{
		{
			name:       "simple",
			projectDir: "./testdata/simple",
			req:        &pb.InstallRequest{},
		},
		{
			name:       "uninstallable",
			projectDir: "./testdata/uninstallable",
			req:        &pb.InstallRequest{},
			wantErr:    HasStatusCode(codes.Aborted, gomega.ContainSubstring("go.mod file not found")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: tt.projectDir})
			_, err := tc.server.Install(ctx, tt.req)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err).To(tt.wantErr)
				// g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
			}
		})
	}
}

func TestUp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
		stacks     []string
		serverOpts *Options
		req        *pb.UpRequest
		wantErr    any
		want       auto.ConfigMap
	}{
		{
			name:       "no active stack",
			projectDir: "./testdata/simple",
			stacks:     []string{},
			req:        &pb.UpRequest{},
			wantErr:    status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:       "simple",
			projectDir: "./testdata/simple",
			stacks:     []string{TestStackName},
			req:        &pb.UpRequest{},
		},
		{
			name:       "Pulumi CLI verbose logging enabled: lvl 11",
			projectDir: "./testdata/simple",
			stacks:     []string{TestStackName},
			serverOpts: &Options{PulumiLogLevel: 11},
			req:        &pb.UpRequest{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: tt.projectDir, Stacks: tt.stacks, ServerOpts: tt.serverOpts})

			srv := &upStream{
				ctx:    ctx,
				events: make([]*structpb.Struct, 0, 100),
			}
			err := tc.server.Up(tt.req, srv)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(srv.events).ToNot(gomega.BeEmpty())
				g.Expect(srv.result).ToNot(gomega.BeNil())
				g.Expect(srv.result.Summary).ToNot(gomega.BeNil())
				g.Expect(srv.result.Summary.Result).To(gomega.Equal("succeeded"))

				if tt.serverOpts != nil {
					// ensure that the extra logs are present
					g.Expect(srv.result).To(gomega.ContainSubstring("log level 11 will print sensitive information"))
				}
			}
		})
	}
}

type upStream struct {
	grpc.ServerStream
	ctx    context.Context
	mu     sync.RWMutex
	events []*structpb.Struct
	result *pb.UpResult
}

var _ pb.AutomationService_UpServer = (*upStream)(nil)

func (m *upStream) Context() context.Context {
	return m.ctx
}

func (m *upStream) Send(resp *pb.UpStream) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch r := resp.Response.(type) {
	case *pb.UpStream_Event:
		m.events = append(m.events, r.Event)
	case *pb.UpStream_Result:
		m.result = r.Result
	}
	return nil
}

func TestPreview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
		stacks     []string
		req        *pb.PreviewRequest
		serverOpts *Options
		wantErr    any
		want       auto.ConfigMap
	}{
		{
			name:       "no active stack",
			projectDir: "./testdata/simple",
			stacks:     []string{},
			req:        &pb.PreviewRequest{},
			wantErr:    status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:       "simple",
			projectDir: "./testdata/simple",
			stacks:     []string{TestStackName},
			req:        &pb.PreviewRequest{},
		},
		{
			name:       "Pulumi CLI verbose logging enabled",
			projectDir: "./testdata/simple",
			stacks:     []string{TestStackName},
			serverOpts: &Options{PulumiLogLevel: 11},
			req:        &pb.PreviewRequest{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: tt.projectDir, Stacks: tt.stacks, ServerOpts: tt.serverOpts})

			srv := &previewStream{
				ctx:    ctx,
				events: make([]*structpb.Struct, 0, 100),
			}
			err := tc.server.Preview(tt.req, srv)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(srv.events).ToNot(gomega.BeEmpty())
				g.Expect(srv.result).ToNot(gomega.BeNil())

				if tt.serverOpts != nil {
					// ensure that the extra logs are present
					g.Expect(srv.result).To(gomega.ContainSubstring("log level 11 will print sensitive information"))
				}
			}
		})
	}
}

type previewStream struct {
	grpc.ServerStream
	ctx    context.Context
	mu     sync.RWMutex
	events []*structpb.Struct
	result *pb.PreviewResult
}

var _ pb.AutomationService_PreviewServer = (*previewStream)(nil)

func (m *previewStream) Context() context.Context {
	return m.ctx
}

func (m *previewStream) Send(resp *pb.PreviewStream) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch r := resp.Response.(type) {
	case *pb.PreviewStream_Event:
		m.events = append(m.events, r.Event)
	case *pb.PreviewStream_Result:
		m.result = r.Result
	}
	return nil
}

func TestRefresh(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
		stacks     []string
		req        *pb.RefreshRequest
		serverOpts *Options
		wantErr    any
		want       auto.ConfigMap
	}{
		{
			name:       "no active stack",
			projectDir: "./testdata/simple",
			stacks:     []string{},
			req:        &pb.RefreshRequest{},
			wantErr:    status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:       "simple",
			projectDir: "./testdata/simple",
			stacks:     []string{TestStackName},
			req:        &pb.RefreshRequest{},
		},
		{
			name:       "Pulumi CLI verbose logging enabled",
			projectDir: "./testdata/simple",
			stacks:     []string{TestStackName},
			serverOpts: &Options{PulumiLogLevel: 11},
			req:        &pb.RefreshRequest{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: tt.projectDir, Stacks: tt.stacks, ServerOpts: tt.serverOpts})

			srv := &refreshStream{
				ctx:    ctx,
				events: make([]*structpb.Struct, 0, 100),
			}
			err := tc.server.Refresh(tt.req, srv)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(srv.events).ToNot(gomega.BeEmpty())
				g.Expect(srv.result).ToNot(gomega.BeNil())
				g.Expect(srv.result.Summary).ToNot(gomega.BeNil())
				g.Expect(srv.result.Summary.Result).To(gomega.Equal("succeeded"))

				if tt.serverOpts != nil {
					// ensure that the extra logs are present
					g.Expect(srv.result).To(gomega.ContainSubstring("log level 11 will print sensitive information"))
				}
			}
		})
	}
}

type refreshStream struct {
	grpc.ServerStream
	ctx    context.Context
	mu     sync.RWMutex
	events []*structpb.Struct
	result *pb.RefreshResult
}

var _ pb.AutomationService_RefreshServer = (*refreshStream)(nil)

func (m *refreshStream) Context() context.Context {
	return m.ctx
}

func (m *refreshStream) Send(resp *pb.RefreshStream) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch r := resp.Response.(type) {
	case *pb.RefreshStream_Event:
		m.events = append(m.events, r.Event)
	case *pb.RefreshStream_Result:
		m.result = r.Result
	}
	return nil
}

func TestDestroy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
		stacks     []string
		req        *pb.DestroyRequest
		serverOpts *Options
		wantErr    any
		want       auto.ConfigMap
	}{
		{
			name:       "no active stack",
			projectDir: "./testdata/simple",
			stacks:     []string{},
			req:        &pb.DestroyRequest{},
			wantErr:    status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:       "simple",
			projectDir: "./testdata/simple",
			stacks:     []string{TestStackName},
			req:        &pb.DestroyRequest{},
		},
		{
			name:       "Pulumi CLI verbose logging enabled",
			projectDir: "./testdata/simple",
			stacks:     []string{TestStackName},
			serverOpts: &Options{PulumiLogLevel: 11},
			req:        &pb.DestroyRequest{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: tt.projectDir, Stacks: tt.stacks})

			srv := &destroyStream{
				ctx:    ctx,
				events: make([]*structpb.Struct, 0, 100),
			}
			err := tc.server.Destroy(tt.req, srv)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(srv.events).ToNot(gomega.BeEmpty())
				g.Expect(srv.result).ToNot(gomega.BeNil())
				g.Expect(srv.result.Summary).ToNot(gomega.BeNil())
				g.Expect(srv.result.Summary.Result).To(gomega.Equal("succeeded"))
			}
		})
	}
}

type destroyStream struct {
	grpc.ServerStream
	ctx    context.Context
	mu     sync.RWMutex
	events []*structpb.Struct
	result *pb.DestroyResult
}

var _ pb.AutomationService_DestroyServer = (*destroyStream)(nil)

func (m *destroyStream) Context() context.Context {
	return m.ctx
}

func (m *destroyStream) Send(resp *pb.DestroyStream) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch r := resp.Response.(type) {
	case *pb.DestroyStream_Event:
		m.events = append(m.events, r.Event)
	case *pb.DestroyStream_Result:
		m.result = r.Result
	}
	return nil
}

func newContext(t *testing.T) context.Context {
	ctx := context.Background()
	if deadline, ok := t.Deadline(); ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		t.Cleanup(cancel)
	}
	return ctx
}

func newWorkspace(ctx context.Context, t *testing.T, templateDir string) auto.Workspace {
	opts := []auto.LocalWorkspaceOption{}

	// generate a project based on the template, with a file backend
	tempDir := t.TempDir()
	if templateDir != "" {
		err := os.MkdirAll(filepath.Join(tempDir, ".pulumi"), 0o750)
		if err != nil {
			t.Fatalf("failed to create state backend directory: %v", err)
		}
		if err := fsutil.CopyFile(tempDir, templateDir, nil); err != nil {
			t.Fatalf("failed to copy project file(s): %v", err)
		}
		proj, err := workspace.LoadProject(filepath.Join(tempDir, "Pulumi.yaml"))
		if err != nil {
			t.Fatalf("failed to load project: %v", err)
		}
		proj.Backend = &workspace.ProjectBackend{
			URL: fmt.Sprintf("file://%s", filepath.Join(tempDir, ".pulumi")),
		}
		err = proj.Save(filepath.Join(tempDir, "Pulumi.yaml"))
		if err != nil {
			t.Fatalf("failed to save project: %v", err)
		}
	}
	opts = append(opts, auto.WorkDir(tempDir))

	ws, err := auto.NewLocalWorkspace(ctx, opts...)
	if err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	t.Logf("created workspace in %q", tempDir)

	return ws
}

func HasStatusCode(code codes.Code, msg types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(err error) status.Status {
		s, _ := status.FromError(err)
		return *s
	}, gomega.And(
		gomega.WithTransform(func(s status.Status) codes.Code { return s.Code() }, gomega.Equal(code)),
		gomega.WithTransform(func(s status.Status) string { return s.Message() }, msg),
	))
}

type testContext struct {
	ws     auto.Workspace
	stack  *auto.Stack
	server *Server
}

type tcOptions struct {
	ProjectDir string
	Stacks     []string // pre-create some stacks; last one will be selected
	ServerOpts *Options
}

func newTC(ctx context.Context, t *testing.T, opts tcOptions) *testContext {
	ws := newWorkspace(ctx, t, opts.ProjectDir)
	if opts.ServerOpts == nil {
		opts.ServerOpts = &Options{}
	}
	server, err := NewServer(ctx, ws, opts.ServerOpts)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	tc := &testContext{
		ws:     ws,
		server: server,
	}
	for _, name := range opts.Stacks {
		stack, err := auto.NewStack(ctx, name, ws)
		if err != nil {
			t.Fatalf("failed to create server: %v", err)
		}
		tc.stack = &stack
	}
	return tc
}

func TestMain(m *testing.M) {
	format.RegisterCustomFormatter(func(value interface{}) (string, bool) {
		if v, ok := value.(proto.Message); ok {
			return v.String(), true
		}
		return "", false
	})

	// configure the file backend
	err := os.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
	if err != nil {
		panic(err)
	}

	// create a FOO variable for test purposes
	err = os.Setenv("FOO", "bar")
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}
