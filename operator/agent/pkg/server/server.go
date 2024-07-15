package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	pb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/pointer"

	"github.com/fluxcd/pkg/http/fetch"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
)

const (
	UserAgent   = "pulumi-kubernetes-operator"
	FluxRetries = 3
)

type Server struct {
	WorkDir string

	initialized atomic.Bool
	workspace   auto.Workspace

	pb.UnimplementedAutomationServiceServer
}

var _ = pb.AutomationServiceServer(&Server{})

func NewServer(workDir string) *Server {
	return &Server{
		WorkDir: workDir,
	}
}

func (s *Server) isInitialized() bool {
	return s.initialized.Load() && s.workspace != nil
}

func (s *Server) Initialize(ctx context.Context, in *pb.InitializeRequest) (*pb.InitializeResult, error) {
	if !s.initialized.CompareAndSwap(false, true) {
		// TODO: persist to workdir
		return nil, fmt.Errorf("server already initialized")
	}

	workDir := s.WorkDir
	opts := []auto.LocalWorkspaceOption{}

	if src := in.GetFlux(); src != nil {
		// https://github.com/fluxcd/kustomize-controller/blob/a1a33f2adda783dd2a17234f5d8e84caca4e24e2/internal/controller/kustomization_controller.go#L328

		fetcher := fetch.New(
			fetch.WithRetries(FluxRetries),
			fetch.WithHostnameOverwrite(os.Getenv("SOURCE_CONTROLLER_LOCALHOST")),
			fetch.WithUntar())

		err := fetcher.FetchWithContext(ctx, src.Url, src.Digest, s.WorkDir)
		if err != nil {
			return nil, fmt.Errorf("flux fetch failed: %w", err)
		}

		if src.Dir != nil {
			workDir = filepath.Join(s.WorkDir, *src.Dir)
		}

		log.Printf("initializing the workspace directory with flux source (%q)", src.Url)

	} else if src := in.GetGit(); src != nil {
		repo := auto.GitRepo{
			URL:        src.Url,
			Branch:     src.GetBranch(),
			CommitHash: src.GetCommitHash(),
		}
		if src.Dir != nil {
			repo.ProjectPath = *src.Dir
		}
		opts = append(opts, auto.Repo(repo))

		log.Printf("initializing the workspace directory with git source (%q)", src.Url)
	}

	opts = append(opts, auto.WorkDir(workDir))
	w, err := auto.NewLocalWorkspace(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}
	s.workspace = w

	log.Printf("workspace initialized")

	resp := &pb.InitializeResult{}
	return resp, nil
}

func (s *Server) WhoAmI(ctx context.Context, in *pb.WhoAmIRequest) (*pb.WhoAmIResult, error) {
	if !s.isInitialized() {
		return nil, fmt.Errorf("server not initialized")
	}

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
	if !s.isInitialized() {
		return nil, fmt.Errorf("server not initialized")
	}

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

// Refresh implements proto.AutomationServiceServer.
func (s *Server) Refresh(in *pb.RefreshRequest, srv pb.AutomationService_RefreshServer) error {
	ctx := srv.Context()
	if !s.isInitialized() {
		return fmt.Errorf("server not initialized")
	}
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
		resp.Permalink = pointer.String(permalink)
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
	if !s.isInitialized() {
		return fmt.Errorf("server not initialized")
	}
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
		resp.Permalink = pointer.String(permalink)
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
	if !s.isInitialized() {
		return fmt.Errorf("server not initialized")
	}
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
		resp.Permalink = pointer.String(permalink)
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
