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

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	UpdateIndexerWorkspace = "index.spec.workspaceRef"

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

// Reconcile manages the Update CRD and initiates Pulumi operations.
func (r *UpdateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	obj := &autov1alpha1.Update{}
	err := r.Get(ctx, req.NamespacedName, obj)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	l.V(1).Info("Reconciling Update", "update", req.NamespacedName, "generation", obj.Generation)

	rs := &reconcileSession{}
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
	rs.updateStatus = func() error {
		obj.Status.ObservedGeneration = obj.Generation
		rs.progressing.ObservedGeneration = obj.Generation
		meta.SetStatusCondition(&obj.Status.Conditions, *rs.progressing)
		rs.failed.ObservedGeneration = obj.Generation
		meta.SetStatusCondition(&obj.Status.Conditions, *rs.failed)
		rs.complete.ObservedGeneration = obj.Generation
		meta.SetStatusCondition(&obj.Status.Conditions, *rs.complete)
		return r.Status().Update(ctx, obj)
	}

	if rs.complete.Status == metav1.ConditionTrue {
		l.V(1).Info("Ignoring completed update", "update", obj.Name)
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
		return ctrl.Result{}, rs.updateStatus()
	}

	l.Info("Updating the status")
	rs.progressing.Status = metav1.ConditionTrue
	rs.progressing.Reason = UpdateConditionReasonProgressing
	rs.failed.Status = metav1.ConditionFalse
	rs.failed.Reason = UpdateConditionReasonProgressing
	rs.complete.Status = metav1.ConditionFalse
	rs.complete.Reason = UpdateConditionReasonProgressing
	err = rs.updateStatus()
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
		return ctrl.Result{RequeueAfter: 5 * time.Second}, rs.updateStatus()
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
	progressing  *metav1.Condition
	complete     *metav1.Condition
	failed       *metav1.Condition
	updateStatus func() error
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
	done := make(chan error)
	go func() {
		for {
			stream, err := res.Recv()
			if err == io.EOF {
				close(done)
				return
			}

			switch r := stream.Response.(type) {
			case *agentpb.PreviewStream_Event:
				continue
			case *agentpb.PreviewStream_Result:
				l.Info("Result received", "result", r.Result)

				obj.Status.StartTime = metav1.NewTime(r.Result.Summary.StartTime.AsTime())
				obj.Status.EndTime = metav1.NewTime(r.Result.Summary.EndTime.AsTime())
				if r.Result.Permalink != nil {
					obj.Status.Permalink = *r.Result.Permalink
				}
				obj.Status.Message = r.Result.Summary.Message
				u.progressing.Status = metav1.ConditionFalse
				u.progressing.Reason = UpdateConditionReasonComplete
				u.complete.Status = metav1.ConditionTrue
				u.complete.Reason = UpdateConditionReasonUpdated
				switch r.Result.Summary.Result {
				case string(apitype.StatusSucceeded):
					u.failed.Status = metav1.ConditionFalse
					u.failed.Reason = r.Result.Summary.Result
				default:
					u.failed.Status = metav1.ConditionTrue
					u.failed.Reason = r.Result.Summary.Result
				}
				err = u.updateStatus()
				if err != nil {
					done <- fmt.Errorf("failed to update the status: %w", err)
					return
				}
			}
		}
	}()
	err = <-done
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("response error: %w", err)
	}

	return ctrl.Result{}, nil
}

func (u *reconcileSession) Update(ctx context.Context, obj *autov1alpha1.Update, client agentpb.AutomationServiceClient, kclient client.Client) (ctrl.Result, error) {
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
	defer func() { _ = res.CloseSend() }()

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed request to workspace: %w", err)
	}

	for {
		stream, err := res.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("response error: %w", err)
		}

		result := stream.GetResult()
		if result == nil {
			continue
		}

		l.Info("Result received", "result", result)

		// Create a secret with result.Outputs
		if result.Outputs != nil {
			secret, err := outputsToSecret(obj, result.Outputs)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("marshaling outputs: %w", err)
			}
			err = kclient.Create(ctx, secret)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("creating output secret: %w", err)
			}
			obj.Status.Outputs = secret.Name
		}

		obj.Status.StartTime = metav1.NewTime(result.Summary.StartTime.AsTime())
		obj.Status.EndTime = metav1.NewTime(result.Summary.EndTime.AsTime())
		if result.Permalink != nil {
			obj.Status.Permalink = *result.Permalink
		}
		obj.Status.Message = result.Summary.Message
		u.progressing.Status = metav1.ConditionFalse
		u.progressing.Reason = UpdateConditionReasonComplete
		u.complete.Status = metav1.ConditionTrue
		u.complete.Reason = UpdateConditionReasonUpdated
		switch result.Summary.Result {
		case string(apitype.StatusSucceeded):
			u.failed.Status = metav1.ConditionFalse
			u.failed.Reason = result.Summary.Result
		default:
			u.failed.Status = metav1.ConditionTrue
			u.failed.Reason = result.Summary.Result
		}
		err = u.updateStatus()
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update the status: %w", err)
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, fmt.Errorf("didn't receive a result")
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
	done := make(chan error)
	go func() {
		for {
			stream, err := res.Recv()
			if err == io.EOF {
				close(done)
				return
			}

			switch r := stream.Response.(type) {
			case *agentpb.RefreshStream_Event:
				continue
			case *agentpb.RefreshStream_Result:
				l.Info("Result received", "result", r.Result)

				obj.Status.StartTime = metav1.NewTime(r.Result.Summary.StartTime.AsTime())
				obj.Status.EndTime = metav1.NewTime(r.Result.Summary.EndTime.AsTime())
				if r.Result.Permalink != nil {
					obj.Status.Permalink = *r.Result.Permalink
				}
				obj.Status.Message = r.Result.Summary.Message
				u.progressing.Status = metav1.ConditionFalse
				u.progressing.Reason = UpdateConditionReasonComplete
				u.complete.Status = metav1.ConditionTrue
				u.complete.Reason = UpdateConditionReasonUpdated
				switch r.Result.Summary.Result {
				case string(apitype.StatusSucceeded):
					u.failed.Status = metav1.ConditionFalse
					u.failed.Reason = r.Result.Summary.Result
				default:
					u.failed.Status = metav1.ConditionTrue
					u.failed.Reason = r.Result.Summary.Result
				}
				err = u.updateStatus()
				if err != nil {
					done <- fmt.Errorf("failed to update the status: %w", err)
					return
				}
			}
		}
	}()
	err = <-done
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("response error: %w", err)
	}

	return ctrl.Result{}, nil
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

	l.Info("Executing refresh operation", "request", autoReq)
	res, err := client.Destroy(ctx, autoReq, grpc.WaitForReady(true))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed request to workspace: %w", err)
	}
	done := make(chan error)
	go func() {
		for {
			stream, err := res.Recv()
			if err == io.EOF {
				close(done)
				return
			}

			switch r := stream.Response.(type) {
			case *agentpb.DestroyStream_Event:
				continue
			case *agentpb.DestroyStream_Result:
				l.Info("Result received", "result", r.Result)

				obj.Status.StartTime = metav1.NewTime(r.Result.Summary.StartTime.AsTime())
				obj.Status.EndTime = metav1.NewTime(r.Result.Summary.EndTime.AsTime())
				if r.Result.Permalink != nil {
					obj.Status.Permalink = *r.Result.Permalink
				}
				obj.Status.Message = r.Result.Summary.Message
				u.progressing.Status = metav1.ConditionFalse
				u.progressing.Reason = UpdateConditionReasonComplete
				u.complete.Status = metav1.ConditionTrue
				u.complete.Reason = UpdateConditionReasonUpdated
				switch r.Result.Summary.Result {
				case string(apitype.StatusSucceeded):
					u.failed.Status = metav1.ConditionFalse
					u.failed.Reason = r.Result.Summary.Result
				default:
					u.failed.Status = metav1.ConditionTrue
					u.failed.Reason = r.Result.Summary.Result
				}
				err = u.updateStatus()
				if err != nil {
					done <- fmt.Errorf("failed to update the status: %w", err)
					return
				}
			}
		}
	}()
	err = <-done
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("response error: %w", err)
	}

	return ctrl.Result{}, nil
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

	scrubbed := make(map[string]any, len(outputs))
	for k, v := range outputs {
		// v.Value is already JSON-encoded bytes,
		s.Data[k] = v.Value
		if v.Secret {
			scrubbed[k] = "[secret]"
		} else {
			scrubbed[k] = json.RawMessage(v.Value)
		}
	}

	annotation, err := json.Marshal(scrubbed)
	if err != nil {
		return nil, fmt.Errorf("marshaling scrubbed output: %w", err)
	}
	s.SetAnnotations(map[string]string{
		"scrubbed": string(annotation),
	})

	return s, nil
}
