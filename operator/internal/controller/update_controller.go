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
	"fmt"
	"io"
	"time"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/v1alpha1"
	"google.golang.org/grpc"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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

// Reconcile
func (r *UpdateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	obj := &autov1alpha1.Update{}
	err := r.Get(ctx, req.NamespacedName, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	progressing := meta.FindStatusCondition(obj.Status.Conditions, UpdateConditionTypeProgressing)
	if progressing == nil {
		progressing = &metav1.Condition{
			Type:   UpdateConditionTypeProgressing,
			Status: metav1.ConditionUnknown,
		}
	}
	failed := meta.FindStatusCondition(obj.Status.Conditions, UpdateConditionTypeFailed)
	if failed == nil {
		failed = &metav1.Condition{
			Type:   UpdateConditionTypeFailed,
			Status: metav1.ConditionUnknown,
		}
	}
	complete := meta.FindStatusCondition(obj.Status.Conditions, UpdateConditionTypeComplete)
	if complete == nil {
		complete = &metav1.Condition{
			Type:   UpdateConditionTypeComplete,
			Status: metav1.ConditionUnknown,
		}
	}
	updateStatus := func() error {
		obj.Status.ObservedGeneration = obj.Generation
		progressing.ObservedGeneration = obj.Generation
		meta.SetStatusCondition(&obj.Status.Conditions, *progressing)
		failed.ObservedGeneration = obj.Generation
		meta.SetStatusCondition(&obj.Status.Conditions, *failed)
		complete.ObservedGeneration = obj.Generation
		meta.SetStatusCondition(&obj.Status.Conditions, *complete)
		return r.Status().Update(ctx, obj)
	}

	if complete.Status == metav1.ConditionTrue {
		l.Info("is completed")
		return ctrl.Result{}, nil
	}

	// guard against retrying an incomplete update
	if progressing.Status == metav1.ConditionTrue {
		l.Info("was progressing; marking as failed")
		progressing.Status = metav1.ConditionFalse
		progressing.Reason = "Failed"
		failed.Status = metav1.ConditionTrue
		failed.Reason = "unknown"
		complete.Status = metav1.ConditionTrue
		complete.Reason = "Aborted"
		return ctrl.Result{}, updateStatus()
	}

	l.Info("Updating the status")
	progressing.Status = metav1.ConditionTrue
	progressing.Reason = "Progressing"
	failed.Status = metav1.ConditionFalse
	failed.Reason = "Progressing"
	complete.Status = metav1.ConditionFalse
	complete.Reason = "Progressing"
	err = updateStatus()
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
		progressing.Status = metav1.ConditionFalse
		progressing.Reason = "TransientFailure"
		failed.Status = metav1.ConditionFalse
		failed.Reason = "Progressing"
		complete.Status = metav1.ConditionFalse
		complete.Reason = "Progressing"
		return ctrl.Result{RequeueAfter: 5 * time.Second}, updateStatus()
	}
	defer func() {
		_ = conn.Close()
	}()
	client := agentpb.NewAutomationServiceClient(conn)

	l.Info("Executing update operation")
	stream, err := client.Up(ctx, &agentpb.UpRequest{
		Stack: obj.Spec.StackName,
	}, grpc.WaitForReady(true))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed request to workspace: %w", err)
	}
	l.Info("got stream")
	done := make(chan error)
	go func() {
		for {
			result, err := stream.Recv()
			if err == io.EOF {
				close(done)
				return
			}
			l.Info("Result received", "result", result)

			obj.Status.StartTime = metav1.NewTime(result.Summary.StartTime.AsTime())
			obj.Status.EndTime = metav1.NewTime(result.Summary.EndTime.AsTime())
			if result.Permalink != nil {
				obj.Status.Permalink = *result.Permalink
			}
			progressing.Status = metav1.ConditionFalse
			progressing.Reason = "Complete"
			complete.Status = metav1.ConditionTrue
			complete.Reason = "Updated"
			switch result.Summary.Result {
			case "succeeded":
				failed.Status = metav1.ConditionFalse
				failed.Reason = result.Summary.Result
			default:
				failed.Status = metav1.ConditionTrue
				failed.Reason = result.Summary.Result
			}
			err = updateStatus()
			if err != nil {
				done <- fmt.Errorf("failed to update the status: %w", err)
				return
			}
		}
	}()
	err = <-done
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("response error: %w", err)
	}

	l.Info("reconciled update")

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
