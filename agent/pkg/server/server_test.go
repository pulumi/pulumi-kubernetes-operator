package server_test

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/server"
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
		opts       *server.Options
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
			opts:       &server.Options{StackName: "new"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)

			ctx := newContext(t)
			ws := newWorkspace(ctx, t, tt.projectDir)

			_, err := server.NewServer(ctx, ws, tt.opts)
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
		req     pb.WhoAmIRequest
		wantErr any
		want    string
	}{
		{
			name: "logged in",
			req:  pb.WhoAmIRequest{},
			want: u.Username,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple"})

			res, err := tc.server.WhoAmI(ctx, &tt.req)
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
		req     pb.SelectStackRequest
		wantErr any
	}{
		{
			name:   "already selected stack",
			stacks: []string{"one"},
			req: pb.SelectStackRequest{
				StackName: "one",
			},
		},
		{
			name:   "existent stack",
			stacks: []string{"one", "two"},
			req: pb.SelectStackRequest{
				StackName: "one",
			},
		},
		{
			name: "non-existent stack",
			req: pb.SelectStackRequest{
				StackName: "unexpected",
			},
			wantErr: status.Error(codes.NotFound, "stack not found"),
		},
		{
			name: "non-existent stack with create",
			req: pb.SelectStackRequest{
				StackName: "one",
				Create:    Pointer(true),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple", Stacks: tt.stacks})

			res, err := tc.server.SelectStack(ctx, &tt.req)
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
		req     pb.InfoRequest
		want    types.GomegaMatcher
		wantErr any
	}{
		{
			name:    "no active stack",
			stacks:  []string{},
			req:     pb.InfoRequest{},
			wantErr: status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:   "active stack",
			stacks: []string{TestStackName},
			req:    pb.InfoRequest{},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple", Stacks: tt.stacks})
			res, err := tc.server.Info(ctx, &tt.req)
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
		req     pb.SetAllConfigRequest
		wantErr any
		want    auto.ConfigMap
	}{
		{
			name:    "no active stack",
			stacks:  []string{},
			req:     pb.SetAllConfigRequest{},
			wantErr: status.Error(codes.FailedPrecondition, "no stack is selected"),
		},
		{
			name:   "literal",
			stacks: []string{TestStackName},
			req: pb.SetAllConfigRequest{
				Config: map[string]*pb.ConfigValue{
					"foo": {V: &pb.ConfigValue_Value{Value: strVal}},
				},
			},
			want: auto.ConfigMap{
				"simple:foo": {Value: "bar"},
			},
		},
		{
			name:   "env",
			stacks: []string{TestStackName},
			req: pb.SetAllConfigRequest{
				// note that TestMain sets the FOO environment variable to a test value
				Config: map[string]*pb.ConfigValue{
					"foo": {V: &pb.ConfigValue_ValueFrom{ValueFrom: &pb.ConfigValueFrom{F: &pb.ConfigValueFrom_Env{Env: "FOO"}}}},
				},
			},
			want: auto.ConfigMap{
				"simple:foo": {Value: "bar"},
			},
		},
		{
			name:   "file",
			stacks: []string{TestStackName},
			req: pb.SetAllConfigRequest{
				// note that TestMain sets the FOO environment variable to a test value
				Config: map[string]*pb.ConfigValue{
					"foo": {V: &pb.ConfigValue_ValueFrom{ValueFrom: &pb.ConfigValueFrom{F: &pb.ConfigValueFrom_Path{Path: "./testdata/foo.txt"}}}},
				},
			},
			want: auto.ConfigMap{
				"simple:foo": {Value: "bar"},
			},
		},
		{
			name:   "path-based",
			stacks: []string{TestStackName},
			req: pb.SetAllConfigRequest{
				Path: Pointer(true),
				Config: map[string]*pb.ConfigValue{
					"a.b": {V: &pb.ConfigValue_Value{Value: strVal}},
				},
			},
			want: auto.ConfigMap{
				"simple:a": {Value: `{"b":"bar"}`},
			},
		},
		{
			name:   "secret",
			stacks: []string{TestStackName},
			req: pb.SetAllConfigRequest{
				Config: map[string]*pb.ConfigValue{
					"foo": {V: &pb.ConfigValue_Value{Value: strVal}, Secret: Pointer(true)},
				},
			},
			want: auto.ConfigMap{
				"simple:foo": {Value: "bar", Secret: true},
			},
		},
		{
			name:   "typed",
			stacks: []string{TestStackName},
			req: pb.SetAllConfigRequest{
				Config: map[string]*pb.ConfigValue{
					"enabled": {V: &pb.ConfigValue_Value{Value: boolVal}},
				},
			},
			want: auto.ConfigMap{
				"simple:enabled": {Value: "true"},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: "./testdata/simple", Stacks: tt.stacks})
			_, err := tc.server.SetAllConfig(ctx, &tt.req)
			if tt.wantErr != nil {
				g.Expect(err).To(gomega.MatchError(tt.wantErr))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred())

				actual, err := tc.ws.GetAllConfig(ctx, tc.stack.Name())
				g.Expect(err).ToNot(gomega.HaveOccurred())
				g.Expect(actual).To(gomega.Equal(tt.want))
			}
		})
	}
}

func TestInstall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
		req        pb.InstallRequest
		wantErr    types.GomegaMatcher
	}{
		{
			name:       "simple",
			projectDir: "./testdata/simple",
			req:        pb.InstallRequest{},
		},
		{
			name:       "uninstallable",
			projectDir: "./testdata/uninstallable",
			req:        pb.InstallRequest{},
			wantErr:    HasStatusCode(codes.Aborted, gomega.ContainSubstring("go.mod file not found")),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			ctx := newContext(t)
			tc := newTC(ctx, t, tcOptions{ProjectDir: tt.projectDir})
			_, err := tc.server.Install(ctx, &tt.req)
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
		err := os.MkdirAll(filepath.Join(tempDir, ".pulumi"), 0755)
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

func Pointer[T any](d T) *T {
	return &d
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
	server *server.Server
}

type tcOptions struct {
	ProjectDir string
	Stacks     []string // pre-create some stacks; last one will be selected
	ServerOpts *server.Options
}

func newTC(ctx context.Context, t *testing.T, opts tcOptions) *testContext {
	ws := newWorkspace(ctx, t, opts.ProjectDir)
	if opts.ServerOpts == nil {
		opts.ServerOpts = &server.Options{}
	}
	server, err := server.NewServer(ctx, ws, opts.ServerOpts)
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
	os.Setenv("PULUMI_CONFIG_PASSPHRASE", "")

	// create a FOO variable for test purposes
	os.Setenv("FOO", "bar")

	os.Exit(m.Run())
}
