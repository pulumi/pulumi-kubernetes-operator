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
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

const (
	UserAgent = "pulumi-kubernetes-operator"
)

type Server struct {
	log        *zap.SugaredLogger
	plog       *zap.Logger
	cancelCtx  context.Context
	cancelFunc context.CancelFunc
	ws         auto.Workspace
	stackLock  sync.Mutex
	stack      *auto.Stack

	pb.UnimplementedAutomationServiceServer
}

var _ = pb.AutomationServiceServer(&Server{})

type Options struct {
	// StackName is the name of the stack to upsert (optional).
	StackName string
	// SecretsProvider is the secrets provider to use for new stacks (optional).
	SecretsProvider string
}

// NewServer creates a new automation server for the given workspace.
func NewServer(ctx context.Context, ws auto.Workspace, opts *Options) (*Server, error) {
	if opts == nil {
		opts = &Options{}
	}

	// create loggers for the server methods and for capturing pulumi logs
	log := zap.L().Named("server").Sugar()
	plog := zap.L().Named("pulumi")

	//  create a context for sending SIGINT to any outstanding Pulumi operations
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	server := &Server{
		log:        log,
		plog:       plog,
		ws:         ws,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	// select the initial stack, if provided
	if opts.StackName != "" {
		stack, err := auto.UpsertStack(ctx, opts.StackName, ws)
		if err != nil {
			return nil, fmt.Errorf("failed to select stack: %w", err)
		}
		if opts.SecretsProvider != "" {
			// We must always make sure the secret provider is initialized in the workspace
			// before we set any configs. Otherwise secret provider will mysteriously reset.
			// https://github.com/pulumi/pulumi-kubernetes-operator/issues/135
			err = stack.ChangeSecretsProvider(ctx, opts.SecretsProvider, &auto.ChangeSecretsProviderOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to set secrets provider: %w", err)
			}
		}
		server.stack = &stack
		server.log.Infow("selected a stack", "name", stack.Name())
	}

	proj, err := ws.ProjectSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load project: %w", err)
	}
	log.Infow("project serving", "project", proj.Name, "runtime", proj.Runtime.Name())

	return server, nil
}

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

// Cancel cancels outstanding operations by sending a SIGINT to Pulumi.
// This call is advisory and non-blocking.
func (s *Server) Cancel() {
	s.cancelFunc()
}

func (s *Server) WhoAmI(ctx context.Context, in *pb.WhoAmIRequest) (*pb.WhoAmIResult, error) {
	whoami, err := s.ws.WhoAmIDetails(ctx)
	if err != nil {
		return nil, err
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
				return auto.NewStack(ctx, in.StackName, s.ws)
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

	config := make(map[string]auto.ConfigValue)
	for k, inv := range in.GetConfig() {
		if k == "" {
			return nil, status.Errorf(codes.InvalidArgument, "invalid config key: %s", k)
		}
		v, err := unmarshalConfigValue(inv)
		if err != nil {
			return nil, err
		}
		config[k] = v
	}
	s.log.Debugw("setting all config", "config", config)

	err = stack.SetAllConfigWithOptions(ctx, config, &auto.ConfigOptions{
		Path: in.GetPath(),
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
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
		optpreview.UserAgent(UserAgent),
		optpreview.Diff(), /* richer result? */
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
		return err
	}
	stdout.Close()
	stderr.Close()
	s.log.Infow("preview completed", "summary", res.ChangeSummary)

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
		optrefresh.UserAgent(UserAgent),
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
		return err
	}
	s.log.Infow("refresh completed", "summary", res.Summary)

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
		optup.UserAgent(UserAgent),
		optup.SuppressProgress(),
		optup.Diff(), /* richer result? */
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
		return err
	}
	stdout.Close()
	stderr.Close()

	s.log.Infow("up completed", "summary", res.Summary)

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
func (s *Server) Destroy(in *pb.DestroyRequest, srv pb.AutomationService_DestroyServer) error {
	ctx := srv.Context()
	stack, err := s.ensureStack(ctx)
	if err != nil {
		return err
	}

	// determine the options to pass to the preview operation
	opts := []optdestroy.Option{
		optdestroy.UserAgent(UserAgent),
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
		return err
	}
	s.log.Infow("destroy completed", "summary", res.Summary)

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

func unmarshalConfigValue(inv *pb.ConfigValue) (auto.ConfigValue, error) {
	v := auto.ConfigValue{
		Secret: inv.GetSecret(),
	}
	switch vv := inv.V.(type) {
	case *pb.ConfigValue_Value:
		// FUTURE: use JSON values
		v.Value = fmt.Sprintf("%v", vv.Value.AsInterface())
	case *pb.ConfigValue_ValueFrom:
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
		data.ResourceCount = ptr.To(int32(*info.ResourceCount))
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
