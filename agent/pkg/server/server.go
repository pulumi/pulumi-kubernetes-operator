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
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mitchellh/go-ps"
	pb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/agent/version"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/debug"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

var _userAgent = fmt.Sprintf("pulumi-kubernetes-operator/%s", version.Version)

// configValueJSON is the shape of the --json output for a configuration value.
// This matches the format expected by `pulumi config set-all --json`.
// When the value is a simple string, only Value is set.
// When the value is a structured type (object, array, number, boolean), both
// Value (JSON-encoded string) and ObjectValue (natural value) are set.
type configValueJSON struct {
	Value       *string `json:"value,omitempty"`
	ObjectValue any     `json:"objectValue,omitempty"`
	Secret      bool    `json:"secret"`
}

type Server struct {
	log            *zap.SugaredLogger
	plog           *zap.Logger
	stopping       atomic.Bool
	ws             auto.Workspace
	stackLock      sync.Mutex
	stack          *auto.Stack
	pulumiLogLevel uint

	pb.UnimplementedAutomationServiceServer
}

var _ = pb.AutomationServiceServer(&Server{})

type Options struct {
	// StackName is the name of the stack to select or create (optional).
	// If the stack exists, it will be selected. If it doesn't exist, it will be created.
	StackName string
	// SecretsProvider is the secrets provider to use when creating new stacks (optional).
	// This is only applied when creating a new stack, not when selecting an existing stack.
	// Examples: "passphrase", "awskms://...", "azurekeyvault://...", "gcpkms://...", "hashivault://..."
	SecretsProvider string

	// PulumiLogLevel is the log level to use for Pulumi CLI operations.
	PulumiLogLevel uint
}

// NewServer creates a new automation server for the given workspace.
func NewServer(ctx context.Context, ws auto.Workspace, opts *Options) (*Server, error) {
	if opts == nil {
		opts = &Options{}
	}

	// create loggers for the server methods and for capturing pulumi logs
	log := zap.L().Named("server").Sugar()
	plog := zap.L().Named("pulumi")

	server := &Server{
		log:            log,
		plog:           plog,
		ws:             ws,
		pulumiLogLevel: opts.PulumiLogLevel,
	}

	// select the initial stack, if provided
	if opts.StackName != "" {
		stack, err := auto.SelectStack(ctx, opts.StackName, ws)
		if err != nil {
			if auto.IsSelectStack404Error(err) {
				stack, err = initStack(ctx, ws, opts.StackName, opts.SecretsProvider)
				if err != nil {
					return nil, fmt.Errorf("failed to create stack: %w", err)
				}
				server.log.Infow("created and selected stack", "name", stack.Name())
			} else {
				return nil, fmt.Errorf("failed to select stack: %w", err)
			}
		} else {
			server.log.Infow("selected existing stack", "name", stack.Name())
		}
		server.stack = &stack
	}

	proj, err := ws.ProjectSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load project: %w", err)
	}
	log.Infow("project serving", "project", proj.Name, "runtime", proj.Runtime.Name())

	return server, nil
}

const pulumiHomeEnv = "PULUMI_HOME"

func (s *Server) ensureStack(ctx context.Context) (auto.Stack, error) {
	s.stackLock.Lock()
	defer s.stackLock.Unlock()

	summary, err := s.ws.Stack(ctx)
	if err != nil {
		return auto.Stack{}, err
	}
	if summary == nil {
		return auto.Stack{}, status.Error(codes.FailedPrecondition, "no stack is selected")
	}
	if s.stack != nil && s.stack.Name() == summary.Name {
		return *s.stack, nil
	}
	return auto.SelectStack(ctx, summary.Name, s.ws)
}

func (s *Server) clearStack() {
	s.stackLock.Lock()
	defer s.stackLock.Unlock()
	s.stack = nil
}

// Cancel attempts to send an interrupt to all of our outstanding Automation
// API subprocesses -- simulating a user's ctrl-C. This call is advisory and
// non-blocking; it is intended to be used alongside grpc.GracefulStop to allow
// outstanding handlers to return. If a handler needs to spawn multiple
// long-running Automation API subprocesses it should check whether the server
// is stopping before doing so.
func (s *Server) Cancel() {
	if s.stopping.Load() {
		// We've already started shutting down, nothing else to do.
		return
	}
	s.stopping.Store(true)

	// Optimistically send an interrupt to all of our children.
	pid := os.Getpid()
	procs, err := ps.Processes()
	if err != nil {
		s.log.Debug()
	}
	s.log.Debugw("problem listing processes", "err", err)
	for _, proc := range procs {
		if proc.PPid() != pid {
			continue
		}
		child, err := os.FindProcess(proc.Pid())
		if err != nil {
			s.log.Debugw("problem finding child process", "err", err)
			continue
		}
		err = child.Signal(os.Interrupt)
		if err != nil {
			s.log.Debugw("problem interrupting child", "err", err)
		}
	}
}

func (s *Server) WhoAmI(ctx context.Context, in *pb.WhoAmIRequest) (*pb.WhoAmIResult, error) {
	whoami, err := s.ws.WhoAmIDetails(ctx)
	if err != nil {
		s.log.Errorw("whoami completed with an error", zap.Error(err))
		st := status.Newf(codes.Unknown, "whoami failed: %v", err)
		return nil, withPulumiErrorInfo(st, err).Err()
	}
	resp := &pb.WhoAmIResult{
		User:          whoami.User,
		Organizations: whoami.Organizations,
		Url:           whoami.URL,
	}
	return resp, nil
}

func (s *Server) SelectStack(ctx context.Context, in *pb.SelectStackRequest) (*pb.SelectStackResult, error) {
	if in.StackName == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid stack name")
	}
	stack, err := func() (auto.Stack, error) {
		stack, err := auto.SelectStack(ctx, in.StackName, s.ws)
		if err != nil {
			if auto.IsSelectStack404Error(err) {
				if !in.GetCreate() {
					return auto.Stack{}, status.Error(codes.NotFound, "stack not found")
				}
				return initStack(ctx, s.ws, in.StackName, in.GetSecretsProvider())
			}
			return auto.Stack{}, err
		}
		return stack, nil
	}()
	if err != nil {
		return nil, err
	}

	s.stackLock.Lock()
	defer s.stackLock.Unlock()
	s.stack = &stack
	s.log.Infow("selected a stack", "name", stack.Name())

	info, err := stack.Info(ctx)
	if err != nil {
		return nil, err
	}
	resp := &pb.SelectStackResult{
		Summary: marshalStackSummary(info),
	}
	return resp, nil
}

// initStack creates a new stack with the specified secrets provider.
// It uses `pulumi stack init --secrets-provider` to properly initialize the secrets provider from the beginning.
// This is preferred over auto.NewStack which does not support specifying a secrets provider at creation time.
func initStack(ctx context.Context, ws auto.Workspace, stackName, secretsProvider string) (auto.Stack, error) {
	log := zap.L().Named("initStack").Sugar()

	log.Debugw("initializing stack", "stackName", stackName, "secretsProvider", secretsProvider)

	// Create the stack with secrets provider using pulumi stack init
	args := []string{"stack", "init", stackName}
	if secretsProvider != "" {
		args = append(args, "--secrets-provider", secretsProvider)
	}
	var env []string
	if ws.PulumiHome() != "" {
		homeEnv := fmt.Sprintf("%s=%s", pulumiHomeEnv, ws.PulumiHome())
		env = append(env, homeEnv)
	}
	if envvars := ws.GetEnvVars(); envvars != nil {
		for k, v := range envvars {
			e := []string{k, v}
			env = append(env, strings.Join(e, "="))
		}
	}

	_, _, errCode, err := ws.PulumiCommand().Run(ctx, ws.WorkDir(), nil, nil, nil, env, args...)
	if err != nil {
		log.Errorw("failed to create stack", "stackName", stackName, "exitCode", errCode, zap.Error(err))
		return auto.Stack{}, fmt.Errorf("failed to create stack (exit code %d): %w", errCode, err)
	}

	log.Debugw("stack initialized successfully", "stackName", stackName)

	// Now select the newly created stack
	stack, err := auto.SelectStack(ctx, stackName, ws)
	if err != nil {
		log.Errorw("failed to select newly created stack", "stackName", stackName, zap.Error(err))
		return auto.Stack{}, err
	}

	return stack, nil
}

func (s *Server) Info(ctx context.Context, in *pb.InfoRequest) (*pb.InfoResult, error) {
	stack, err := s.ensureStack(ctx)
	if err != nil {
		return nil, err
	}

	info, err := stack.Info(ctx)
	if err != nil {
		return nil, err
	}

	resp := &pb.InfoResult{
		Summary: marshalStackSummary(info),
	}
	return resp, nil
}

func (s *Server) SetAllConfig(ctx context.Context, in *pb.SetAllConfigRequest) (*pb.SetAllConfigResult, error) {
	stack, err := s.ensureStack(ctx)
	if err != nil {
		return nil, err
	}

	p, err := stack.Workspace().ProjectSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting project settings: %w", err)
	}

	// Check if any config items contain JSON values
	hasJson := false
	for _, item := range in.Config {
		if hasJsonValue(item) {
			hasJson = true
			break
		}
	}

	s.log.Debugw("setting all config", "count", len(in.Config), "hasJson", hasJson)

	if hasJson {
		// Use JSON-aware config setting
		config, err := unmarshalConfigItemsJson(p.Name.String(), in.Config)
		if err != nil {
			return nil, fmt.Errorf("config items: %w", err)
		}

		s.log.Debugw("setting config with JSON values", "keys", slices.Collect(maps.Keys(config)))

		// Marshal the config map to a JSON string
		configJson, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshaling config to JSON: %w", err)
		}

		// Use the new JSON config method (requires Pulumi CLI v3.202.0+)
		err = stack.SetAllConfigJson(ctx, string(configJson), &auto.ConfigOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		// Use path-based config setting
		config, err := unmarshalConfigItems(p.Name.String(), in.Config)
		if err != nil {
			return nil, fmt.Errorf("config items: %w", err)
		}

		s.log.Debugw("setting config with path support", "keys", slices.Collect(maps.Keys(config)))

		err = stack.SetAllConfigWithOptions(ctx, config, &auto.ConfigOptions{Path: true})
		if err != nil {
			return nil, err
		}
	}

	return &pb.SetAllConfigResult{}, nil
}

// unmarshalConfigItems maps proto ConfigItems to an auto.ConfigMap whose
// structure is appropriate for invoking with `pulumi config set-all --path`
// for efficiency. Config items whose keys are not meant to be treated as paths
// are escaped so they are treated as verbatim property paths.
//
// If any ConfigItem contains JSON values, this function will detect that and
// return an error, signaling the caller should use unmarshalConfigItemsJson instead.
func unmarshalConfigItems(project string, items []*pb.ConfigItem) (auto.ConfigMap, error) {
	out := auto.ConfigMap{}

	for _, item := range items {
		if hasJsonValue(item) {
			return nil, fmt.Errorf("config contains JSON values")
		}

		v, err := unmarshalConfigItem(item) // Resolves valueFrom.
		if err != nil {
			return nil, fmt.Errorf("unmarshalling %q config: %w", item.Key, err)
		}

		// ParseKey requires all keys to be explicitly namespaced, so scope any
		// implicit keys to our project.
		key := item.Key
		if !strings.Contains(key, tokens.TokenDelimiter) {
			key = project + ":" + key
		}
		k, err := config.ParseKey(key)
		if err != nil {
			return nil, fmt.Errorf("parsing %q key: %w", key, err)
		}

		// Keys which are not meant to be interpreted as paths as escaped so
		// subsequent path parsing treats them as-is. In particular "foo"
		// becomes `["<foo>"]`. Items which are already escaped in this way are
		// untouched. See
		// https://github.com/pulumi/pulumi/blob/4424d69177c47385ea4a01d8ad1b0b825d394f63/sdk/go/common/resource/properties_path.go#L32-L64
		path := false
		if item.Path != nil {
			path = *item.Path
		}
		if !path && !(strings.HasPrefix(k.Name(), `["`) && strings.HasSuffix(k.Name(), `"]`)) {
			k = config.MustMakeKey(k.Namespace(), fmt.Sprintf("[%q]", k.Name()))
		}

		out[k.String()] = v
	}

	return out, nil
}

// hasJsonValue checks if a ConfigItem contains or should be treated as a JSON value
func hasJsonValue(item *pb.ConfigItem) bool {
	switch vv := item.V.(type) {
	case *pb.ConfigItem_Value:
		// Check if the value is not a simple string (i.e., it's a structured value)
		val := vv.Value.AsInterface()
		switch val.(type) {
		case string, nil:
			return false
		default:
			// Maps, arrays, numbers, booleans are JSON values
			return true
		}
	case *pb.ConfigItem_ValueFrom:
		// Check if json flag is set
		return vv.ValueFrom.GetJson()
	}
	return false
}

// unmarshalConfigItemsJson builds a JSON configuration document for use with
// SetAllConfigJson. This is used when at least one config item
// contains a structured (JSON) value.
func unmarshalConfigItemsJson(project string, items []*pb.ConfigItem) (map[string]configValueJSON, error) {
	out := make(map[string]configValueJSON)

	for _, item := range items {
		// Validate: path and json are incompatible
		if item.GetPath() && hasJsonValue(item) {
			return nil, status.Errorf(codes.InvalidArgument,
				"config item %q: path=true is incompatible with JSON values", item.Key)
		}

		// ParseKey requires all keys to be explicitly namespaced
		key := item.Key
		if !strings.Contains(key, tokens.TokenDelimiter) {
			key = project + ":" + key
		}
		k, err := config.ParseKey(key)
		if err != nil {
			return nil, fmt.Errorf("parsing %q key: %w", key, err)
		}

		var cv configValueJSON
		cv.Secret = item.GetSecret()

		switch vv := item.V.(type) {
		case *pb.ConfigItem_Value:
			val := vv.Value.AsInterface()
			switch v := val.(type) {
			case string:
				// Simple string value - set only Value field
				cv.Value = &v
			default:
				// Structured value (object, array, number, boolean)
				// Set both Value (JSON string) and ObjectValue (natural value)
				// Use protojson to handle protobuf semantics, then normalize with json.Marshal
				protoJsonBytes, err := protojson.Marshal(vv.Value)
				if err != nil {
					return nil, fmt.Errorf("marshaling %q value to JSON: %w", item.Key, err)
				}
				// Unmarshal to Go value to normalize the representation
				var obj interface{}
				if err := json.Unmarshal(protoJsonBytes, &obj); err != nil {
					return nil, fmt.Errorf("unmarshaling %q protojson: %w", item.Key, err)
				}
				// Re-marshal with standard json.Marshal for deterministic compact output
				jsonBytes, err := json.Marshal(obj)
				if err != nil {
					return nil, fmt.Errorf("re-marshaling %q value: %w", item.Key, err)
				}
				jsonStr := string(jsonBytes)
				cv.Value = &jsonStr
				cv.ObjectValue = obj
			}
		case *pb.ConfigItem_ValueFrom:
			// Read the value from environment or filesystem
			var data string
			switch from := vv.ValueFrom.F.(type) {
			case *pb.ConfigValueFrom_Env:
				envData, ok := os.LookupEnv(from.Env)
				if !ok {
					return nil, status.Errorf(codes.InvalidArgument,
						"missing value for environment variable: %s", from.Env)
				}
				data = envData
			case *pb.ConfigValueFrom_Path:
				fileData, err := os.ReadFile(from.Path)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument,
						"unreadable path: %s", from.Path)
				}
				data = string(fileData)
			default:
				return nil, status.Error(codes.InvalidArgument, "invalid config value")
			}

			// If json flag is set, parse as JSON and set both Value and ObjectValue
			if vv.ValueFrom.GetJson() {
				var obj interface{}
				if err := json.Unmarshal([]byte(data), &obj); err != nil {
					return nil, fmt.Errorf("parsing %q as JSON: %w", item.Key, err)
				}
				cv.Value = &data
				cv.ObjectValue = obj
			} else {
				// Simple string - set only Value field
				cv.Value = &data
			}
		default:
			return nil, status.Error(codes.InvalidArgument, "invalid config value")
		}

		out[k.String()] = cv
	}

	return out, nil
}

func (s *Server) AddEnvironments(ctx context.Context, in *pb.AddEnvironmentsRequest) (*pb.AddEnvironmentsResult, error) {
	stack, err := s.ensureStack(ctx)
	if err != nil {
		return nil, err
	}

	err = stack.AddEnvironments(ctx, in.Environment...)
	if err != nil {
		return nil, err
	}

	return &pb.AddEnvironmentsResult{}, nil
}

func (s *Server) Install(ctx context.Context, in *pb.InstallRequest) (*pb.InstallResult, error) {
	stdout := &zapio.Writer{Log: s.plog, Level: zap.InfoLevel}
	defer stdout.Close()
	stderr := &zapio.Writer{Log: s.plog, Level: zap.WarnLevel}
	defer stderr.Close()
	opts := &auto.InstallOptions{
		Stdout: stdout,
		Stderr: stderr,
	}

	s.log.Infow("installing the project dependencies")
	if err := s.ws.Install(ctx, opts); err != nil {
		s.log.Errorw("install completed with an error", zap.Error(err))
		return nil, status.Error(codes.Aborted, err.Error())
	}
	s.log.Infow("installation completed")

	resp := &pb.InstallResult{}
	return resp, nil
}

// Preview implements proto.AutomationServiceServer.
func (s *Server) Preview(in *pb.PreviewRequest, srv pb.AutomationService_PreviewServer) error {
	ctx := srv.Context()
	stack, err := s.ensureStack(ctx)
	if err != nil {
		return err
	}

	// determine the options to pass to the preview operation
	opts := []optpreview.Option{
		optpreview.UserAgent(_userAgent),
		optpreview.Diff(), /* richer result? */
	}
	if s.pulumiLogLevel > 0 {
		// We need to conidtionally enable debug logging as upstream pu/pu will set the log level to be
		// at least 1 if debug.LoggingOptions is not nil.
		opts = append(opts, optpreview.DebugLogging(debug.LoggingOptions{LogLevel: &s.pulumiLogLevel, LogToStdErr: true}))
	}
	if in.Parallel != nil {
		opts = append(opts, optpreview.Parallel(int(*in.Parallel)))
	}
	if in.GetExpectNoChanges() {
		opts = append(opts, optpreview.ExpectNoChanges())
	}
	if in.Replace != nil {
		opts = append(opts, optpreview.Replace(in.Replace))
	}
	if in.Target != nil {
		opts = append(opts, optpreview.Target(in.Target))
	}
	if in.GetTargetDependents() {
		opts = append(opts, optpreview.TargetDependents())
	}
	// TODO:PolicyPack
	if in.GetRefresh() {
		opts = append(opts, optpreview.Refresh())
	}
	if in.Message != nil {
		opts = append(opts, optpreview.Message(*in.Message))
	}

	// wire up the logging
	stdout := &zapio.Writer{Log: s.plog, Level: zap.InfoLevel}
	defer stdout.Close()
	opts = append(opts, optpreview.ProgressStreams(stdout))
	stderr := &zapio.Writer{Log: s.plog, Level: zap.WarnLevel}
	defer stderr.Close()
	opts = append(opts, optpreview.ErrorProgressStreams(stderr))

	// stream the engine events to the client
	events := make(chan events.EngineEvent)
	opts = append(opts, optpreview.EventStreams(events))
	go func() {
		for evt := range events {
			data, err := marshalEngineEvent(evt.EngineEvent)
			if err != nil {
				s.log.Errorw("failed to marshal an engine event", "sequence", evt.Sequence, zap.Error(err))
				continue
			}
			msg := &pb.PreviewStream{Response: &pb.PreviewStream_Event{Event: data}}
			if err := srv.Send(msg); err != nil {
				s.log.Errorw("failed to send an engine event", "sequence", evt.Sequence, zap.Error(err))
				continue
			}
		}
	}()

	res, err := stack.Preview(ctx, opts...)
	if err != nil {
		s.log.Errorw("preview completed with an error", zap.Error(err))
		st := status.Newf(codes.Unknown, "preview failed: %v", err)
		return withPulumiErrorInfo(st, err).Err()
	}
	stdout.Close() //nolint:gosec // Close always returns nil err
	stderr.Close() //nolint:gosec // Close always returns nil err
	s.log.Infow("preview completed")

	resp := &pb.PreviewResult{
		Stdout: res.StdOut,
		Stderr: res.StdErr,
	}
	// TODO: ChangeSummary
	permalink, err := res.GetPermalink()
	if err != nil && err != auto.ErrParsePermalinkFailed {
		return err
	}
	if permalink != "" {
		resp.Permalink = ptr.To(permalink)
	}

	msg := &pb.PreviewStream{Response: &pb.PreviewStream_Result{Result: resp}}
	if err := srv.Send(msg); err != nil {
		s.log.Errorw("unable to send the preview result", zap.Error(err))
		return err
	}
	return nil
}

// Refresh implements proto.AutomationServiceServer.
func (s *Server) Refresh(in *pb.RefreshRequest, srv pb.AutomationService_RefreshServer) error {
	ctx := srv.Context()
	stack, err := s.ensureStack(ctx)
	if err != nil {
		return err
	}

	// determine the options to pass to the preview operation
	opts := []optrefresh.Option{
		optrefresh.UserAgent(_userAgent),
	}
	if s.pulumiLogLevel > 0 {
		// We need to conidtionally enable debug logging as upstream pu/pu will set the log level to be
		// at least 1 if debug.LoggingOptions is not nil.
		opts = append(opts, optrefresh.DebugLogging(debug.LoggingOptions{LogLevel: &s.pulumiLogLevel, LogToStdErr: true}))
	}
	if in.Parallel != nil {
		opts = append(opts, optrefresh.Parallel(int(*in.Parallel)))
	}
	if in.Message != nil {
		opts = append(opts, optrefresh.Message(*in.Message))
	}
	if in.GetExpectNoChanges() {
		opts = append(opts, optrefresh.ExpectNoChanges())
	}
	if in.Target != nil {
		opts = append(opts, optrefresh.Target(in.Target))
	}

	// wire up the logging
	stdout := &zapio.Writer{Log: s.plog, Level: zap.InfoLevel}
	defer stdout.Close()
	opts = append(opts, optrefresh.ProgressStreams(stdout))
	stderr := &zapio.Writer{Log: s.plog, Level: zap.WarnLevel}
	defer stderr.Close()
	opts = append(opts, optrefresh.ErrorProgressStreams(stderr))

	// stream the engine events to the client
	events := make(chan events.EngineEvent)
	opts = append(opts, optrefresh.EventStreams(events))
	go func() {
		for evt := range events {
			data, err := marshalEngineEvent(evt.EngineEvent)
			if err != nil {
				s.log.Errorw("failed to marshal an engine event", "sequence", evt.Sequence, zap.Error(err))
				continue
			}
			msg := &pb.RefreshStream{Response: &pb.RefreshStream_Event{Event: data}}
			if err := srv.Send(msg); err != nil {
				s.log.Errorw("failed to send an engine event", "sequence", evt.Sequence, zap.Error(err))
				continue
			}
		}
	}()

	res, err := stack.Refresh(ctx, opts...)
	if err != nil {
		s.log.Errorw("refresh completed with an error", zap.Error(err))
		st := status.Newf(codes.Unknown, "refresh failed: %v", err)
		return withPulumiErrorInfo(st, err).Err()
	}
	s.log.Infow("refresh completed", "result", res.Summary.Result, "message", res.Summary.Message)

	resp := &pb.RefreshResult{
		Stdout:  res.StdOut,
		Stderr:  res.StdErr,
		Summary: marshalUpdateSummary(res.Summary),
	}
	permalink, err := res.GetPermalink()
	if err != nil && err != auto.ErrParsePermalinkFailed {
		return err
	}
	if permalink != "" {
		resp.Permalink = ptr.To(permalink)
	}

	msg := &pb.RefreshStream{Response: &pb.RefreshStream_Result{Result: resp}}
	if err := srv.Send(msg); err != nil {
		s.log.Errorw("unable to send the refresh result", zap.Error(err))
		return err
	}
	return nil
}

// Up implements proto.AutomationServiceServer.
func (s *Server) Up(in *pb.UpRequest, srv pb.AutomationService_UpServer) error {
	ctx := srv.Context()
	stack, err := s.ensureStack(ctx)
	if err != nil {
		return err
	}

	// determine the options to pass to the preview operation
	opts := []optup.Option{
		optup.UserAgent(_userAgent),
		optup.SuppressProgress(),
		optup.Diff(), /* richer result? */
	}
	if s.pulumiLogLevel > 0 {
		// We need to conidtionally enable debug logging as upstream pu/pu will set the log level to be
		// at least 1 if debug.LoggingOptions is not nil.
		opts = append(opts, optup.DebugLogging(debug.LoggingOptions{LogLevel: &s.pulumiLogLevel, LogToStdErr: true}))
	}
	if in.Parallel != nil {
		opts = append(opts, optup.Parallel(int(*in.Parallel)))
	}
	if in.Message != nil {
		opts = append(opts, optup.Message(*in.Message))
	}
	if in.GetExpectNoChanges() {
		opts = append(opts, optup.ExpectNoChanges())
	}
	if in.Replace != nil {
		opts = append(opts, optup.Replace(in.Replace))
	}
	if in.Target != nil {
		opts = append(opts, optup.Target(in.Target))
	}
	if in.GetTargetDependents() {
		opts = append(opts, optup.TargetDependents())
	}
	// TODO:PolicyPack
	if in.GetRefresh() {
		opts = append(opts, optup.Refresh())
	}
	if in.GetContinueOnError() {
		opts = append(opts, optup.ContinueOnError())
	}

	// wire up the logging
	stdout := &zapio.Writer{Log: s.plog, Level: zap.InfoLevel}
	defer stdout.Close()
	stderr := &zapio.Writer{Log: s.plog, Level: zap.WarnLevel}
	defer stderr.Close()
	opts = append(opts, optup.ProgressStreams(stdout))
	opts = append(opts, optup.ErrorProgressStreams(stderr))

	// stream the engine events to the client
	events := make(chan events.EngineEvent)
	opts = append(opts, optup.EventStreams(events))
	go func() {
		for evt := range events {
			data, err := marshalEngineEvent(evt.EngineEvent)
			if err != nil {
				s.log.Errorw("failed to marshal an engine event", "sequence", evt.Sequence, zap.Error(err))
				continue
			}
			msg := &pb.UpStream{Response: &pb.UpStream_Event{Event: data}}
			if err := srv.Send(msg); err != nil {
				s.log.Errorw("failed to send an engine event", "sequence", evt.Sequence, zap.Error(err))
				continue
			}
		}
	}()

	// run the update to deploy our program
	res, err := stack.Up(ctx, opts...)
	if err != nil {
		s.log.Errorw("up completed with an error", zap.Error(err))
		st := status.Newf(codes.Unknown, "up failed: %v", err)
		return withPulumiErrorInfo(st, err).Err()
	}
	stdout.Close() //nolint:gosec // Close always returns nil err
	stderr.Close() //nolint:gosec // Close always returns nil err

	s.log.Infow("up completed", "result", res.Summary.Result, "message", res.Summary.Message)

	outputs, err := marshalOutputs(res.Outputs)
	if err != nil {
		return fmt.Errorf("marshaling outputs: %w", err)
	}

	resp := &pb.UpResult{
		Stdout:  res.StdOut,
		Stderr:  res.StdErr,
		Outputs: outputs,
		Summary: marshalUpdateSummary(res.Summary),
	}
	permalink, err := res.GetPermalink()
	if err != nil && err != auto.ErrParsePermalinkFailed {
		return err
	}
	if permalink != "" {
		resp.Permalink = ptr.To(permalink)
	}

	msg := &pb.UpStream{Response: &pb.UpStream_Result{Result: resp}}
	if err := srv.Send(msg); err != nil {
		s.log.Errorw("unable to send the up result", zap.Error(err))
		return err
	}

	return nil
}

// Destroy implements proto.AutomationServiceServer.
func (s *Server) PulumiVersion(ctx context.Context, in *pb.PulumiVersionRequest) (*pb.PulumiVersionResult, error) {
	version := s.ws.PulumiVersion()
	return &pb.PulumiVersionResult{Version: version}, nil
}

func (s *Server) Destroy(in *pb.DestroyRequest, srv pb.AutomationService_DestroyServer) error {
	ctx := srv.Context()
	stack, err := s.ensureStack(ctx)
	if err != nil {
		return err
	}

	// determine the options to pass to the preview operation
	opts := []optdestroy.Option{
		optdestroy.UserAgent(_userAgent),
	}
	if s.pulumiLogLevel > 0 {
		// We need to conidtionally enable debug logging as upstream pu/pu will set the log level to be
		// at least 1 if debug.LoggingOptions is not nil.
		opts = append(opts, optdestroy.DebugLogging(debug.LoggingOptions{LogLevel: &s.pulumiLogLevel, LogToStdErr: true}))
	}
	if in.Parallel != nil {
		opts = append(opts, optdestroy.Parallel(int(*in.Parallel)))
	}
	if in.Message != nil {
		opts = append(opts, optdestroy.Message(*in.Message))
	}
	if in.Target != nil {
		opts = append(opts, optdestroy.Target(in.Target))
	}
	if in.GetTargetDependents() {
		opts = append(opts, optdestroy.TargetDependents())
	}
	if in.GetRefresh() {
		opts = append(opts, optdestroy.Refresh())
	}
	if in.GetContinueOnError() {
		opts = append(opts, optdestroy.ContinueOnError())
	}
	if in.GetRemove() {
		opts = append(opts, optdestroy.Remove())
	}

	// wire up the logging
	stdout := &zapio.Writer{Log: s.plog, Level: zap.InfoLevel}
	defer stdout.Close()
	opts = append(opts, optdestroy.ProgressStreams(stdout))
	stderr := &zapio.Writer{Log: s.plog, Level: zap.WarnLevel}
	defer stderr.Close()
	opts = append(opts, optdestroy.ErrorProgressStreams(stderr))

	// stream the engine events to the client
	events := make(chan events.EngineEvent)
	opts = append(opts, optdestroy.EventStreams(events))
	go func() {
		for evt := range events {
			data, err := marshalEngineEvent(evt.EngineEvent)
			if err != nil {
				s.log.Errorw("failed to marshal an engine event", "sequence", evt.Sequence, zap.Error(err))
				continue
			}
			msg := &pb.DestroyStream{Response: &pb.DestroyStream_Event{Event: data}}
			if err := srv.Send(msg); err != nil {
				s.log.Errorw("failed to send an engine event", "sequence", evt.Sequence, zap.Error(err))
				continue
			}
		}
	}()

	// run the update to deploy our program
	res, err := stack.Destroy(ctx, opts...)
	if err != nil {
		s.log.Errorw("destroy completed with an error", zap.Error(err))
		st := status.Newf(codes.Unknown, "destroy failed: %v", err)
		return withPulumiErrorInfo(st, err).Err()
	}
	s.log.Infow("destroy completed", "result", res.Summary.Result, "message", res.Summary.Message)

	if in.GetRemove() && res.Summary.Result == "succeeded" {
		// the stack was removed, so unselect the current stack.
		s.clearStack()
	}

	resp := &pb.DestroyResult{
		Stdout:  res.StdOut,
		Stderr:  res.StdErr,
		Summary: marshalUpdateSummary(res.Summary),
	}
	permalink, err := res.GetPermalink()
	if err != nil && err != auto.ErrParsePermalinkFailed {
		return err
	}
	if permalink != "" {
		resp.Permalink = ptr.To(permalink)
	}

	msg := &pb.DestroyStream{Response: &pb.DestroyStream_Result{Result: resp}}
	if err := srv.Send(msg); err != nil {
		s.log.Errorw("unable to send the destroy result", zap.Error(err))
		return err
	}
	return nil
}

func unmarshalConfigItem(inv *pb.ConfigItem) (auto.ConfigValue, error) {
	v := auto.ConfigValue{
		Secret: inv.GetSecret(),
	}
	switch vv := inv.V.(type) {
	case *pb.ConfigItem_Value:
		// FUTURE: use JSON values
		v.Value = fmt.Sprintf("%v", vv.Value.AsInterface())
	case *pb.ConfigItem_ValueFrom:
		switch from := vv.ValueFrom.F.(type) {
		case *pb.ConfigValueFrom_Env:
			data, ok := os.LookupEnv(from.Env)
			if !ok {
				return auto.ConfigValue{}, status.Errorf(codes.InvalidArgument, "missing value for environment variable: %s", from.Env)
			}
			v.Value = data
		case *pb.ConfigValueFrom_Path:
			data, err := os.ReadFile(from.Path)
			if err != nil {
				return auto.ConfigValue{}, status.Errorf(codes.InvalidArgument, "unreadable path: %s", from.Path)
			}
			v.Value = string(data)
		default:
			return auto.ConfigValue{}, status.Error(codes.InvalidArgument, "invalid config value")
		}
	default:
		return auto.ConfigValue{}, status.Error(codes.InvalidArgument, "invalid config value")
	}
	return v, nil
}

func marshalStackSummary(info auto.StackSummary) *pb.StackSummary {
	data := &pb.StackSummary{
		Name:             info.Name,
		LastUpdate:       parseTime(&info.LastUpdate),
		UpdateInProgress: info.UpdateInProgress,
		Url:              ptr.To(info.URL),
	}
	if info.ResourceCount != nil {
		data.ResourceCount = ptr.To(int32(*info.ResourceCount)) //nolint:gosec // We don't reasonably expect an overflow.
	}
	return data
}

func marshalUpdateSummary(info auto.UpdateSummary) *pb.UpdateSummary {
	res := &pb.UpdateSummary{
		Result:    info.Result,
		Message:   info.Message,
		StartTime: parseTime(&info.StartTime),
		EndTime:   parseTime(info.EndTime),
	}
	return res
}

func parseTime(s *string) *timestamppb.Timestamp {
	if s == nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, *s)
	if err != nil {
		return nil
	}
	return timestamppb.New(t)
}

func marshalEngineEvent(evt apitype.EngineEvent) (*structpb.Struct, error) {
	m := make(map[string]any)
	j, err := json.Marshal(evt)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(j, &m)
	if err != nil {
		return nil, err
	}
	return structpb.NewStruct(m)
}

// marshalOutputs serializes outputs as a resource.PropertyMap to make
// downstream secret handling easier.
func marshalOutputs(outputs auto.OutputMap) (map[string]*pb.OutputValue, error) {
	if len(outputs) == 0 {
		return nil, nil
	}

	o := make(map[string]*pb.OutputValue, len(outputs))
	for k, v := range outputs {
		value, err := json.Marshal(v.Value)
		if err != nil {
			return nil, err
		}
		o[k] = &pb.OutputValue{
			Value:  value,
			Secret: v.Secret,
		}
	}

	return o, nil
}
