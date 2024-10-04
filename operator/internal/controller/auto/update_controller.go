/*
Copyright 2024.

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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/go-logr/logr"
	agentpb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	SecretOutputsAnnotation = "pulumi.com/secrets"
	UpdateIndexerWorkspace  = "index.spec.workspaceRef"

	UpdateConditionTypeComplete    = "Complete"
	UpdateConditionTypeFailed      = "Failed"
	UpdateConditionTypeProgressing = "Progressing"

	UpdateConditionReasonComplete    = "Complete"
	UpdateConditionReasonUpdated     = "Updated"
	UpdateConditionReasonProgressing = "Progressing"
)

// UpdateReconciler reconciles a Update object
type UpdateReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=auto.pulumi.com,resources=updates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=updates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=updates/finalizers,verbs=update
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=create

// Reconcile manages the Update CRD and initiates Pulumi operations.
func (r *UpdateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("Reconciling Update")

	obj := &autov1alpha1.Update{}
	err := r.Get(ctx, req.NamespacedName, obj)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	rs := newReconcileSession(r.Client, obj)

	if rs.complete.Status == metav1.ConditionTrue {
		l.V(1).Info("Ignoring completed update")
		return ctrl.Result{}, nil
	}

	// guard against retrying an incomplete update
	if rs.progressing.Status == metav1.ConditionTrue {
		l.Info("was progressing; marking as failed")
		rs.progressing.Status = metav1.ConditionFalse
		rs.progressing.Reason = "Failed"
		rs.failed.Status = metav1.ConditionTrue
		rs.failed.Reason = "unknown"
		rs.complete.Status = metav1.ConditionTrue
		rs.complete.Reason = "Aborted"
		return ctrl.Result{}, rs.updateStatus(ctx, obj)
	}

	l.Info("Updating the status")
	rs.progressing.Status = metav1.ConditionTrue
	rs.progressing.Reason = UpdateConditionReasonProgressing
	rs.failed.Status = metav1.ConditionFalse
	rs.failed.Reason = UpdateConditionReasonProgressing
	rs.complete.Status = metav1.ConditionFalse
	rs.complete.Reason = UpdateConditionReasonProgressing
	err = rs.updateStatus(ctx, obj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", err)
	}

	// TODO check the w status before proceeding
	w := &autov1alpha1.Workspace{}
	err = r.Client.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: obj.Spec.WorkspaceName}, w)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get workspace: %w", err)
	}

	// Connect to the workspace's GRPC server
	addr := fmt.Sprintf("%s:%d", fqdnForService(w), WorkspaceGrpcPort)
	l.Info("Connecting", "addr", addr)
	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()
	conn, err := connect(connectCtx, addr)
	if err != nil {
		l.Error(err, "unable to connect; retrying later", "addr", addr)
		rs.progressing.Status = metav1.ConditionFalse
		rs.progressing.Reason = "TransientFailure"
		rs.failed.Status = metav1.ConditionFalse
		rs.failed.Reason = UpdateConditionReasonProgressing
		rs.complete.Status = metav1.ConditionFalse
		rs.complete.Reason = UpdateConditionReasonProgressing
		return ctrl.Result{RequeueAfter: 5 * time.Second}, rs.updateStatus(ctx, obj)
	}
	defer func() {
		_ = conn.Close()
	}()
	client := agentpb.NewAutomationServiceClient(conn)

	l.Info("Selecting the stack", "stackName", obj.Spec.StackName)
	_, err = client.SelectStack(ctx, &agentpb.SelectStackRequest{
		StackName: obj.Spec.StackName,
		Create:    ptr.To(true),
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed request to workspace: %w", err)
	}

	l.Info("Applying the update", "type", obj.Spec.Type)
	switch obj.Spec.Type {
	case autov1alpha1.PreviewType:
		return rs.Preview(ctx, obj, client)
	case autov1alpha1.UpType:
		return rs.Update(ctx, obj, client, r.Client)
	case autov1alpha1.RefreshType:
		return rs.Refresh(ctx, obj, client)
	case autov1alpha1.DestroyType:
		return rs.Destroy(ctx, obj, client)
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported update type %q", obj.Spec.Type)
	}
}

type reconcileSession struct {
	progressing *metav1.Condition
	complete    *metav1.Condition
	failed      *metav1.Condition
	client      client.Client
}

// newReconcileSession creates a new reconcileSession.
func newReconcileSession(client client.Client, obj *autov1alpha1.Update) *reconcileSession {
	rs := &reconcileSession{client: client}
	rs.progressing = meta.FindStatusCondition(obj.Status.Conditions, UpdateConditionTypeProgressing)
	if rs.progressing == nil {
		rs.progressing = &metav1.Condition{
			Type:   UpdateConditionTypeProgressing,
			Status: metav1.ConditionUnknown,
		}
	}
	rs.failed = meta.FindStatusCondition(obj.Status.Conditions, UpdateConditionTypeFailed)
	if rs.failed == nil {
		rs.failed = &metav1.Condition{
			Type:   UpdateConditionTypeFailed,
			Status: metav1.ConditionUnknown,
		}
	}
	rs.complete = meta.FindStatusCondition(obj.Status.Conditions, UpdateConditionTypeComplete)
	if rs.complete == nil {
		rs.complete = &metav1.Condition{
			Type:   UpdateConditionTypeComplete,
			Status: metav1.ConditionUnknown,
		}
	}
	return rs
}

func (rs *reconcileSession) updateStatus(ctx context.Context, obj *autov1alpha1.Update) error {
	obj.Status.ObservedGeneration = obj.Generation
	rs.progressing.ObservedGeneration = obj.Generation
	meta.SetStatusCondition(&obj.Status.Conditions, *rs.progressing)
	rs.failed.ObservedGeneration = obj.Generation
	meta.SetStatusCondition(&obj.Status.Conditions, *rs.failed)
	rs.complete.ObservedGeneration = obj.Generation
	meta.SetStatusCondition(&obj.Status.Conditions, *rs.complete)
	err := rs.client.Status().Update(ctx, obj)
	if err == nil {
		return nil
	}
	return fmt.Errorf("updating status: %w", err)
}

func (u *reconcileSession) Preview(ctx context.Context, obj *autov1alpha1.Update, client agentpb.AutomationServiceClient) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	l.V(1).Info("Configure the preview operation")
	autoReq := &agentpb.PreviewRequest{
		Parallel:         obj.Spec.Parallel,
		Message:          obj.Spec.Message,
		ExpectNoChanges:  obj.Spec.ExpectNoChanges,
		Replace:          obj.Spec.Replace,
		Target:           obj.Spec.Target,
		TargetDependents: obj.Spec.TargetDependents,
		Refresh:          obj.Spec.Refresh,
	}

	l.Info("Executing preview operation", "request", autoReq)
	res, err := client.Preview(ctx, autoReq, grpc.WaitForReady(true))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed request to workspace: %w", err)
	}
	defer func() { _ = res.CloseSend() }()

	reader := streamReader[agentpb.PreviewStream]{receiver: res, l: l, u: u, obj: obj}
	_, err = reader.Result()
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, u.updateStatus(ctx, obj)
}

type upper interface {
	Up(ctx context.Context, in *agentpb.UpRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[agentpb.UpStream], error)
}

type recver[T any] interface {
	Recv() (*T, error)
	grpc.ClientStream
}

type uprecver = recver[agentpb.UpStream]

type creater interface {
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
}

func (u *reconcileSession) Update(ctx context.Context, obj *autov1alpha1.Update, client upper, kclient creater) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	l.V(1).Info("Configure the up operation")
	autoReq := &agentpb.UpRequest{
		Parallel:         obj.Spec.Parallel,
		Message:          obj.Spec.Message,
		ExpectNoChanges:  obj.Spec.ExpectNoChanges,
		Replace:          obj.Spec.Replace,
		Target:           obj.Spec.Target,
		TargetDependents: obj.Spec.TargetDependents,
		Refresh:          obj.Spec.Refresh,
		ContinueOnError:  obj.Spec.ContinueOnError,
	}

	l.Info("Executing update operation", "request", autoReq)
	res, err := client.Up(ctx, autoReq, grpc.WaitForReady(true))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed request to workspace: %w", err)
	}
	defer func() { _ = res.CloseSend() }()

	reader := streamReader[agentpb.UpStream]{receiver: res, l: l, u: u, obj: obj}
	result, err := reader.Result()
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create a secret with result.Outputs
	if r, ok := result.(*agentpb.UpResult); ok && r.Outputs != nil {
		secret, err := outputsToSecret(obj, r.Outputs)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("marshaling outputs: %w", err)
		}
		err = kclient.Create(ctx, secret)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("creating output secret: %w", err)
		}
		obj.Status.Outputs = secret.Name
	}

	return ctrl.Result{}, u.updateStatus(ctx, obj)
}

func (u *reconcileSession) Refresh(ctx context.Context, obj *autov1alpha1.Update, client agentpb.AutomationServiceClient) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	l.V(1).Info("Configure the refresh operation")
	autoReq := &agentpb.RefreshRequest{
		Parallel:        obj.Spec.Parallel,
		Message:         obj.Spec.Message,
		ExpectNoChanges: obj.Spec.ExpectNoChanges,
		Target:          obj.Spec.Target,
	}

	l.Info("Executing refresh operation", "request", autoReq)
	res, err := client.Refresh(ctx, autoReq, grpc.WaitForReady(true))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed request to workspace: %w", err)
	}
	defer func() { _ = res.CloseSend() }()

	reader := streamReader[agentpb.RefreshStream]{receiver: res, l: l, u: u, obj: obj}
	_, err = reader.Result()
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, u.updateStatus(ctx, obj)
}

func (u *reconcileSession) Destroy(ctx context.Context, obj *autov1alpha1.Update, client agentpb.AutomationServiceClient) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	l.V(1).Info("Configure the destroy operation")
	autoReq := &agentpb.DestroyRequest{
		Parallel:         obj.Spec.Parallel,
		Message:          obj.Spec.Message,
		Target:           obj.Spec.Target,
		TargetDependents: obj.Spec.TargetDependents,
		Refresh:          obj.Spec.Refresh,
		ContinueOnError:  obj.Spec.ContinueOnError,
		Remove:           obj.Spec.Remove,
	}

	l.Info("Executing destroy operation", "request", autoReq)
	res, err := client.Destroy(ctx, autoReq, grpc.WaitForReady(true))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed request to workspace: %w", err)
	}
	defer func() { _ = res.CloseSend() }()

	reader := streamReader[agentpb.DestroyStream]{receiver: res, l: l, u: u, obj: obj}
	_, err = reader.Result()
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, u.updateStatus(ctx, obj)
}

// SetupWithManager sets up the controller with the Manager.
func (r *UpdateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// index the updates by workspace
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &autov1alpha1.Update{},
		UpdateIndexerWorkspace, indexUpdateByWorkspace); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&autov1alpha1.Update{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&autov1alpha1.Workspace{},
			handler.EnqueueRequestsFromMapFunc(r.mapWorkspaceToUpdate),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}

func indexUpdateByWorkspace(obj client.Object) []string {
	w := obj.(*autov1alpha1.Update)
	return []string{w.Spec.WorkspaceName}
}

func (r *UpdateReconciler) mapWorkspaceToUpdate(ctx context.Context, obj client.Object) []reconcile.Request {
	l := log.FromContext(ctx)

	objs := &autov1alpha1.UpdateList{}
	err := r.Client.List(ctx, objs, client.InNamespace(obj.GetNamespace()), client.MatchingFields{UpdateIndexerWorkspace: obj.GetName()})
	if err != nil {
		l.Error(err, "unable to list updates")
		return nil
	}
	requests := make([]reconcile.Request, len(objs.Items))
	for i, mapped := range objs.Items {
		requests[i] = reconcile.Request{NamespacedName: types.NamespacedName{Name: mapped.Name, Namespace: mapped.Namespace}}
	}
	return requests
}

// outputsToSecret returns a Secret object whose keys are stack output names
// and values are JSON-encoded bytes. An annotation is recorded with all secret
// outputs overwritten with "[secret]"; this annotation is consumed by the
// Stack API and recorded on the stack's status.
func outputsToSecret(owner *autov1alpha1.Update, outputs map[string]*agentpb.OutputValue) (*corev1.Secret, error) {
	s := &corev1.Secret{Immutable: ptr.To(true), Data: map[string][]byte{}}
	s.SetName(owner.Name + "-stack-outputs")
	s.SetNamespace(owner.Namespace)
	s.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: owner.APIVersion,
		Kind:       owner.Kind,
		Name:       owner.Name,
		UID:        owner.UID,
	}})

	secrets := []string{}
	for k, v := range outputs {
		// v.Value is already JSON-encoded bytes,
		s.Data[k] = v.Value
		if v.Secret {
			secrets = append(secrets, k)
		}
	}

	annotation, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("marshaling output mask: %w", err)
	}
	s.SetAnnotations(map[string]string{
		SecretOutputsAnnotation: string(annotation),
	})

	return s, nil
}

// stream is an interface constraint for the response streams consumable by a
// streamReader.
type stream interface {
	agentpb.UpStream | agentpb.DestroyStream | agentpb.PreviewStream | agentpb.RefreshStream
}

// streamReader reads an update stream until a result is received. The
// reconcile session and underlying Update object are updated to reflect the
// result, but no changes are written back to the API server.
type streamReader[T stream] struct {
	receiver grpc.ServerStreamingClient[T]
	obj      *autov1alpha1.Update
	u        *reconcileSession
	l        logr.Logger
}

// Recv reads one message from the stream which may or may not contain a
// result.
func (s streamReader[T]) Recv() (getResulter[T], error) {
	stream, err := s.receiver.Recv()
	return getResulter[T]{stream}, err
}

// Result reads from the underlying stream until a Result is received or an
// error is encountered. A non-nil error is returned if the stream is closed
// prematurely or if a gRPC error is encountered. Importantly, if the
// Automation API returns an Unknown error it is assumed that the operation
// failed; in this case a failed Result is returned with nil error.
func (s streamReader[T]) Result() (result, error) {
	var res result

	for {
		stream, err := s.Recv()
		if err == io.EOF {
			break
		}
		if err != nil && status.Code(err) != codes.Unknown {
			// Surface gRPC errors.
			return nil, err
		}
		if err != nil {
			// For all other errors treat the operation as failed.
			s.l.Error(err, "Update failed")
			s.obj.Status.Message = status.Convert(err).Message()
			s.u.progressing.Status = metav1.ConditionFalse
			s.u.progressing.Reason = UpdateConditionReasonComplete
			s.u.complete.Status = metav1.ConditionTrue
			s.u.complete.Reason = UpdateConditionReasonComplete
			s.u.failed.Status = metav1.ConditionTrue
			s.u.failed.Reason = status.Code(err).String()
			s.u.failed.Message = s.obj.Status.Message
			return res, nil
		}

		res = stream.GetResult()
		if res == nil {
			continue // No result yet.
		}

		s.l.Info("Result received", "result", res)

		s.obj.Status.StartTime = metav1.NewTime(res.GetSummary().StartTime.AsTime())
		s.obj.Status.EndTime = metav1.NewTime(res.GetSummary().EndTime.AsTime())
		if link := res.GetPermalink(); link != "" {
			s.obj.Status.Permalink = link
		}
		s.obj.Status.Message = res.GetSummary().Message
		s.u.progressing.Status = metav1.ConditionFalse
		s.u.progressing.Reason = UpdateConditionReasonComplete
		s.u.complete.Status = metav1.ConditionTrue
		s.u.complete.Reason = UpdateConditionReasonUpdated
		switch res.GetSummary().Result {
		case string(apitype.StatusSucceeded):
			s.u.failed.Status = metav1.ConditionFalse
			s.u.failed.Reason = res.GetSummary().Result
			s.u.failed.Message = res.GetSummary().Message
		default:
			s.u.failed.Status = metav1.ConditionTrue
			s.u.failed.Reason = res.GetSummary().Result
			s.u.failed.Message = res.GetSummary().Message
		}
		return res, nil
	}

	return res, fmt.Errorf("didn't receive a result")
}

// getResulter glues our various result types to a common interface.
type getResulter[T stream] struct {
	stream *T
}

// result captures behavior for all of our stream results.
type result interface {
	GetSummary() *agentpb.UpdateSummary
	GetPermalink() string
}

// getResult returns nil if the underlying stream doesn't yet have a result;
// otherwise it returns a result interface wrapping the underlying type. See
// Update for an example of how to customize result handling.
func (gr getResulter[T]) GetResult() result {
	var res result
	switch s := any(gr.stream).(type) {
	case *agentpb.UpStream:
		if r := s.GetResult(); r != nil {
			res = r
		}
	case *agentpb.DestroyStream:
		if r := s.GetResult(); r != nil {
			res = r
		}
	case *agentpb.PreviewStream:
		if r := s.GetResult(); r != nil {
			res = r
		}
	case *agentpb.RefreshStream:
		if r := s.GetResult(); r != nil {
			res = r
		}
	}
	return res
}
