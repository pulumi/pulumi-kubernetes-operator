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
	"strconv"
	"time"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1b2 "github.com/fluxcd/source-controller/api/v1beta2"
	agentpb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	WorkspaceIndexerFluxSource  = "index.spec.flux.sourceRef"
	WorkspaceConditionTypeReady = "Ready"
	PodAnnotationInitialized    = "auto.pulumi.com/initialized"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories,verbs=get;list;watch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=buckets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	w := &autov1alpha1.Workspace{}
	err := r.Get(ctx, req.NamespacedName, w)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ready := meta.FindStatusCondition(w.Status.Conditions, WorkspaceConditionTypeReady)
	if ready == nil {
		ready = &metav1.Condition{
			Type:   WorkspaceConditionTypeReady,
			Status: metav1.ConditionUnknown,
		}
	}
	updateConditions := func() error {
		w.Status.ObservedGeneration = w.Generation
		ready.ObservedGeneration = w.Generation
		meta.SetStatusCondition(&w.Status.Conditions, *ready)
		return r.Status().Update(ctx, w)
	}

	// service
	svc, err := newService(w)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := controllerutil.SetControllerReference(w, svc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	err = r.Patch(ctx, svc, client.Apply, client.FieldOwner(FieldManager))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to apply service: %w", err)
	}

	// statefulset
	ss, err := newStatefulSet(w)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := controllerutil.SetControllerReference(w, ss, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	err = r.Patch(ctx, ss, client.Apply, client.FieldOwner(FieldManager))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to apply statefulset: %w", err)
	}

	if ss.Status.AvailableReplicas < 1 {
		l.Info("no replicas available; retry later")
		ready.Status = metav1.ConditionFalse
		ready.Reason = "WaitingForReplicas"
		return ctrl.Result{}, updateConditions()
	}

	// Locate the workspace pod, to figure out whether workspace initialization is needed.
	// The workspace is stored in pod ephemeral storage, which has the same lifecycle as that of the pod.
	podName := fmt.Sprintf("%s-0", nameForStatefulSet(w))
	pod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: podName, Namespace: w.Namespace}, pod)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to find the workspace pod: %w", err)
	}
	if pod.DeletionTimestamp != nil {
		l.Info("pod is being deleted; retry later")
		ready.Status = metav1.ConditionFalse
		ready.Reason = "PodDeleting"
		return ctrl.Result{}, updateConditions()
	}

	// Connect to the workspace's GRPC server
	addr := fmt.Sprintf("%s:%d", fqdnForService(w), WorkspaceGrpcPort)
	l.Info("Connecting", "addr", addr)
	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()
	conn, err := connect(connectCtx, addr)
	if err != nil {
		l.Error(err, "unable to connect; retrying later", "addr", addr)
		ready.Status = metav1.ConditionFalse
		ready.Reason = "ConnectionFailed"
		ready.Message = err.Error()
		return ctrl.Result{RequeueAfter: 5 * time.Second}, updateConditions()
	}
	defer func() {
		_ = conn.Close()
	}()
	workspaceClient := agentpb.NewAutomationServiceClient(conn)

	initializedV, ok := pod.Annotations[PodAnnotationInitialized]
	initialized, _ := strconv.ParseBool(initializedV)
	if !ok || !initialized {
		l.Info("initializing the source")
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Initializing"
		ready.Message = ""
		if err := updateConditions(); err != nil {
			return ctrl.Result{}, err
		}

		initReq := &agentpb.InitializeRequest{}
		if w.Spec.Git != nil {
			source := &agentpb.InitializeRequest_Git{
				Git: &agentpb.GitSource{
					Url: w.Spec.Git.ProjectRepo,
					Dir: &w.Spec.Git.RepoDir,
				},
			}
			if w.Spec.Git.RepoDir != "" {
				source.Git.Dir = &w.Spec.Git.RepoDir
			}
			if w.Spec.Git.Commit != "" {
				source.Git.Ref = &agentpb.GitSource_CommitHash{CommitHash: w.Spec.Git.Commit}
			} else if w.Spec.Git.Branch != "" {
				source.Git.Ref = &agentpb.GitSource_Branch{Branch: w.Spec.Git.Branch}
			}
			initReq.Source = source
		}
		if w.Spec.Flux != nil {
			// Resolve the source reference and requeue the reconciliation if the source is not found.
			artifactSource, err := r.getSource(ctx, w)
			if err != nil {
				ready.Status = metav1.ConditionFalse
				ready.Reason = "ArtifactFailed"
				ready.Message = err.Error()
				if apierrors.IsNotFound(err) {
					return ctrl.Result{}, updateConditions()
				}
				// Retry with backoff on transient errors.
				_ = updateConditions()
				return ctrl.Result{}, err
			}

			// Requeue the reconciliation if the source artifact is not found.
			artifact := artifactSource.GetArtifact()
			if artifact == nil {
				ready.Status = metav1.ConditionFalse
				ready.Reason = "ArtifactFailed"
				ready.Message = "Source artifact not found, retrying later"
				return ctrl.Result{}, updateConditions()
			}

			source := &agentpb.InitializeRequest_Flux{
				Flux: &agentpb.FluxSource{
					Url:    artifact.URL,
					Digest: artifact.Digest,
				},
			}
			if w.Spec.Flux.Dir != "" {
				source.Flux.Dir = &w.Spec.Flux.Dir
			}

			initReq.Source = source
		}

		l.Info("initializing the workspace")
		_, err = workspaceClient.Initialize(ctx, initReq)
		if err != nil {
			l.Error(err, "unable to initialize; deleting the workspace pod to retry later")
			ready.Status = metav1.ConditionFalse
			ready.Reason = "InitializationFailed"
			ready.Message = err.Error()

			err = r.Client.Delete(ctx, pod)
			if err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, updateConditions()
		}

		// set the "initalized" annotation
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[PodAnnotationInitialized] = "true"

		err = r.Update(ctx, ss, client.FieldOwner(FieldManager))
		if err != nil {
			l.Error(err, "unable to update the pod; deleting the workspace pod to retry later")
			err = r.Client.Delete(ctx, pod)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, fmt.Errorf("failed to patch the pod: %w", err)
		}
		l.Info("initialized")
	}

	ready.Status = metav1.ConditionTrue
	ready.Reason = "Initialized"
	ready.Message = ""
	l.Info("Ready")

	return ctrl.Result{}, updateConditions()
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// index the workspaces by flux source
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &autov1alpha1.Workspace{},
		WorkspaceIndexerFluxSource, indexWorkspaceByFluxSource); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&autov1alpha1.Workspace{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Service{},
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Owns(&appsv1.StatefulSet{},
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Watches(&sourcev1b2.OCIRepository{},
			handler.EnqueueRequestsFromMapFunc(r.mapFluxSourceToWorkspace),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Watches(&sourcev1.GitRepository{},
			handler.EnqueueRequestsFromMapFunc(r.mapFluxSourceToWorkspace),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Watches(&sourcev1b2.Bucket{},
			handler.EnqueueRequestsFromMapFunc(r.mapFluxSourceToWorkspace),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}

func indexWorkspaceByFluxSource(obj client.Object) []string {
	w := obj.(*autov1alpha1.Workspace)
	if w.Spec.Flux == nil {
		return nil
	}
	key := fmt.Sprintf("%s/%s/%s", w.Spec.Flux.SourceRef.APIVersion, w.Spec.Flux.SourceRef.Kind, w.Spec.Flux.SourceRef.Name)
	return []string{key}
}

func (r *WorkspaceReconciler) mapFluxSourceToWorkspace(ctx context.Context, obj client.Object) []reconcile.Request {
	l := log.FromContext(ctx)

	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	key := fmt.Sprintf("%s/%s/%s", apiVersion, kind, obj.GetName())
	objs := &autov1alpha1.WorkspaceList{}
	err := r.Client.List(ctx, objs, client.InNamespace(obj.GetNamespace()), client.MatchingFields{WorkspaceIndexerFluxSource: key})
	if err != nil {
		l.Error(err, "unable to list workspaces")
		return nil
	}
	requests := make([]reconcile.Request, len(objs.Items))
	for i, mapped := range objs.Items {
		requests[i] = reconcile.Request{NamespacedName: types.NamespacedName{Name: mapped.Name, Namespace: mapped.Namespace}}
	}
	return requests
}

const (
	FieldManager           = "pulumi-kubernetes-operator"
	WorkspaceContainerName = "server"
	WorkspaceTmpVolumeName = "tmp"
	WorkspaceTmpMountPath  = "/tmp"
	WorkspaceGrpcPort      = 50051
)

func nameForStatefulSet(w *autov1alpha1.Workspace) string {
	return w.Name + "-workspace"
}

func nameForService(w *autov1alpha1.Workspace) string {
	return w.Name + "-workspace"
}

func fqdnForService(w *autov1alpha1.Workspace) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", nameForService(w), w.Namespace)
}

func labelsForStatefulSet(w *autov1alpha1.Workspace) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "pulumi",
		"app.kubernetes.io/component":  "workspace",
		"app.kubernetes.io/instance":   w.Name,
		"app.kubernetes.io/managed-by": "pulumi-kubernetes-operator",
	}
}

func newStatefulSet(w *autov1alpha1.Workspace) (*appsv1.StatefulSet, error) {
	labels := labelsForStatefulSet(w)
	statefulset := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameForStatefulSet(w),
			Namespace: w.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			ServiceName: nameForService(w),
			Replicas:    pointer.Int32(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            w.Spec.ServiceAccountName,
					TerminationGracePeriodSeconds: pointer.Int64(30),
					Containers: []corev1.Container{
						{
							Name:            WorkspaceContainerName,
							Image:           w.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      WorkspaceTmpVolumeName,
									MountPath: WorkspaceTmpMountPath,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "grpc",
									ContainerPort: WorkspaceGrpcPort,
								},
							},
							Env: w.Spec.Env,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: WorkspaceTmpVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	return statefulset, nil
}

func newService(w *autov1alpha1.Workspace) (*corev1.Service, error) {
	labels := labelsForStatefulSet(w)
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameForService(w),
			Namespace: w.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name: "grpc",
					Port: WorkspaceGrpcPort,
				},
			},
		},
	}

	return service, nil
}

func (r *WorkspaceReconciler) getSource(ctx context.Context,
	obj *autov1alpha1.Workspace) (sourcev1.Source, error) {
	var src sourcev1.Source
	namespacedName := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.Spec.Flux.SourceRef.Name,
	}

	switch obj.Spec.Flux.SourceRef.Kind {
	case sourcev1b2.OCIRepositoryKind:
		var repository sourcev1b2.OCIRepository
		err := r.Client.Get(ctx, namespacedName, &repository)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return src, err
			}
			return src, fmt.Errorf("unable to get source '%s': %w", namespacedName, err)
		}
		src = &repository
	case sourcev1.GitRepositoryKind:
		var repository sourcev1.GitRepository
		err := r.Client.Get(ctx, namespacedName, &repository)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return src, err
			}
			return src, fmt.Errorf("unable to get source '%s': %w", namespacedName, err)
		}
		src = &repository
	case sourcev1b2.BucketKind:
		var bucket sourcev1b2.Bucket
		err := r.Client.Get(ctx, namespacedName, &bucket)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return src, err
			}
			return src, fmt.Errorf("unable to get source '%s': %w", namespacedName, err)
		}
		src = &bucket
	default:
		return src, fmt.Errorf("source `%s` kind '%s' not supported",
			obj.Spec.Flux.SourceRef.Name, obj.Spec.Flux.SourceRef.Kind)
	}
	return src, nil
}
