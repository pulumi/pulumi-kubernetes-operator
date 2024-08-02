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
	// create loggers for the server methods and for capturing pulumi logs
	log := zap.L().Named("auto.server").Sugar()
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
			err = stack.ChangeSecretsProvider(ctx, opts.SecretsProvider, &auto.ChangeSecretsProviderOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to set secrets provider: %w", err)
			}
		}
		server.stack = &stack
		server.log.Infow("selected a stack", "name", stack.Name())
	}

	return server, nil
}

func (s *Server) ensureStack() (*auto.Stack, error) {
	s.stackLock.Lock()
	defer s.stackLock.Unlock()
	if s.stack == nil {
		return nil, status.Error(codes.FailedPrecondition, "no stack is selected")
	}
	return s.stack, nil
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
	stack, err := s.ensureStack()
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
	stack, err := s.ensureStack()
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
		return nil, err
	}
	s.log.Infow("installation completed")

	resp := &pb.InstallResult{}
	return resp, nil
}

// Preview implements proto.AutomationServiceServer.
func (s *Server) Preview(in *pb.PreviewRequest, srv pb.AutomationService_PreviewServer) error {
	ctx := srv.Context()
	stack, err := s.ensureStack()
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
	if in.ExpectNoChanges {
		opts = append(opts, optpreview.ExpectNoChanges())
	}
	if in.Replace != nil {
		opts = append(opts, optpreview.Replace(in.Replace))
	}
	if in.Target != nil {
		opts = append(opts, optpreview.Target(in.Target))
	}
	if in.TargetDependents {
		opts = append(opts, optpreview.TargetDependents())
	}
	// TODO:PolicyPack
	if in.Refresh {
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
	stack, err := s.ensureStack()
	if err != nil {
		return err
	}

	fmt.Println("Starting refresh")
	res, err := stack.Refresh(ctx, optrefresh.UserAgent(UserAgent))
	if err != nil {
		fmt.Printf("Failed to refresh stack: %v\n", err)
		return err
	}
	resp := &pb.RefreshResult{
		Stdout: res.StdOut,
		Stderr: res.StdErr,
		Summary: &pb.UpdateSummary{
			Result:    res.Summary.Result,
			Message:   res.Summary.Message,
			StartTime: parseTime(&res.Summary.StartTime),
			EndTime:   parseTime(res.Summary.EndTime),
		},
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
	stack, err := s.ensureStack()
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

	resp := &pb.UpResult{
		Stdout: res.StdOut,
		Stderr: res.StdErr,
		Summary: &pb.UpdateSummary{
			Result:    res.Summary.Result,
			Message:   res.Summary.Message,
			StartTime: parseTime(&res.Summary.StartTime),
			EndTime:   parseTime(res.Summary.EndTime),
		},
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
	stack, err := s.ensureStack()
	if err != nil {
		return err
	}

	fmt.Println("Starting destroy")

	// run the update to deploy our program
	res, err := stack.Destroy(ctx, optdestroy.UserAgent(UserAgent))
	if err != nil {
		fmt.Printf("Failed to destroy stack: %v\n\n", err)
		return err
	}
	resp := &pb.DestroyResult{
		Stdout: res.StdOut,
		Stderr: res.StdErr,
		Summary: &pb.UpdateSummary{
			Result:    res.Summary.Result,
			Message:   res.Summary.Message,
			StartTime: parseTime(&res.Summary.StartTime),
			EndTime:   parseTime(res.Summary.EndTime),
		},
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
	j, _ := json.Marshal(evt)
	_ = json.Unmarshal(j, &m)
	data, err := structpb.NewStruct(m)
	if err != nil {
		return nil, err
	}
	return data, nil
}
