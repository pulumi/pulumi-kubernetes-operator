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
	"time"

	"github.com/go-logr/logr"
	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

const (
	UserAgent   = "pulumi-kubernetes-operator"
	FluxRetries = 3
)

type Server struct {
	log           logr.Logger
	cancelContext context.Context
	cancelFunc    context.CancelFunc
	workspace     auto.Workspace

	pb.UnimplementedAutomationServiceServer
}

var _ = pb.AutomationServiceServer(&Server{})

func NewServer(ctx context.Context, workDir string) (*Server, error) {
	log := logr.FromContextOrDiscard(ctx)
	opts := []auto.LocalWorkspaceOption{}
	opts = append(opts, auto.WorkDir(workDir))
	w, err := auto.NewLocalWorkspace(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("auto.NewLocalWorkspace: %w", err)
	}

	proj, err := w.ProjectSettings(ctx)
	if err != nil {
		return nil, err
	}
	log.Info("serving the Pulumi project", "workspace", workDir,
		"project", proj.Name, "runtime", proj.Runtime.Name())

	cancelContext, cancelFunc := context.WithCancel(context.Background())
	server := &Server{
		log:           log,
		workspace:     w,
		cancelContext: cancelContext,
		cancelFunc:    cancelFunc,
	}
	return server, nil
}

func (s *Server) Cancel() {
	s.cancelFunc()
}

func (s *Server) WhoAmI(ctx context.Context, in *pb.WhoAmIRequest) (*pb.WhoAmIResult, error) {
	whoami, err := s.workspace.WhoAmIDetails(ctx)
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

func (s *Server) Info(ctx context.Context, in *pb.InfoRequest) (*pb.InfoResult, error) {
	stack, err := auto.UpsertStack(ctx, in.Stack, s.workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to select stack: %w", err)
	}

	info, err := stack.Info(ctx)
	if err != nil {
		return nil, err
	}

	resp := &pb.InfoResult{
		Summary: &pb.StackSummary{
			Name:       info.Name,
			LastUpdate: parseTime(&info.LastUpdate),
		},
	}
	return resp, nil
}

// Preview implements proto.AutomationServiceServer.
func (s *Server) Preview(in *pb.PreviewRequest, srv pb.AutomationService_PreviewServer) error {
	ctx := srv.Context()
	stack, err := auto.UpsertStack(ctx, in.Stack, s.workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack: %w", err)
	}

	opts := []optpreview.Option{
		optpreview.UserAgent(UserAgent),
		optpreview.ProgressStreams(os.Stdout),
		optpreview.ErrorProgressStreams(os.Stderr),
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

	// event streaming
	prevCh := make(chan events.EngineEvent)
	opts = append(opts, optpreview.EventStreams(prevCh))
	go func() {
		for evt := range prevCh {
			m := make(map[string]any)
			j, _ := json.Marshal(evt.EngineEvent)
			_ = json.Unmarshal(j, &m)
			data, err := structpb.NewStruct(m)
			if err != nil {
				panic(fmt.Errorf("failed to marshal event: %w", err))
			}
			msg := &pb.PreviewStream{Response: &pb.PreviewStream_Event{Event: &pb.EngineEvent{Event: data}}}
			if err := srv.Send(msg); err != nil {
				panic(err)
			}
		}
	}()

	fmt.Println("Starting preview")
	res, err := stack.Preview(ctx, opts...)
	if err != nil {
		fmt.Printf("Failed to refresh stack: %v\n", err)
		return err
	}
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
		return err
	}
	s.log.Info("Preview succeeded!")
	fmt.Println("Preview succeeded!")

	return nil
}

// Refresh implements proto.AutomationServiceServer.
func (s *Server) Refresh(in *pb.RefreshRequest, srv pb.AutomationService_RefreshServer) error {
	ctx := srv.Context()
	stack, err := auto.UpsertStack(ctx, in.Stack, s.workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack: %w", err)
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

	if err := srv.Send(resp); err != nil {
		return err
	}
	fmt.Println("Refresh succeeded!")

	return nil
}

// Up implements proto.AutomationServiceServer.
func (s *Server) Up(in *pb.UpRequest, srv pb.AutomationService_UpServer) error {
	ctx := srv.Context()
	stack, err := auto.UpsertStack(ctx, in.Stack, s.workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack: %w", err)
	}

	fmt.Println("Starting update")

	// wire up our update to stream progress to stdout
	stdoutStreamer := optup.ProgressStreams(os.Stdout)

	// run the update to deploy our program
	res, err := stack.Up(ctx, stdoutStreamer, optup.UserAgent(UserAgent))
	if err != nil {
		fmt.Printf("Failed to update stack: %v\n\n", err)
		return err
	}
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

	if err := srv.Send(resp); err != nil {
		return err
	}
	fmt.Println("Update succeeded!")

	return nil
}

// Up implements proto.AutomationServiceServer.
func (s *Server) Destroy(in *pb.DestroyRequest, srv pb.AutomationService_DestroyServer) error {
	ctx := srv.Context()
	stack, err := auto.UpsertStack(ctx, in.Stack, s.workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack: %w", err)
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

	if err := srv.Send(resp); err != nil {
		return err
	}
	fmt.Println("Destroy succeeded!")

	return nil
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

func mergeContext(a, b context.Context) (context.Context, context.CancelFunc) {
	mctx, mcancel := context.WithCancel(a) // will cancel if `a` cancels

	go func() {
		select {
		case <-mctx.Done(): // don't leak go-routine on clean gRPC run
		case <-b.Done():
			mcancel() // b canceled, so cancel mctx
		}
	}()

	return mctx, mcancel
}
