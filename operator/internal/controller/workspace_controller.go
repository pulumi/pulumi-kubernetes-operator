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
	"crypto/md5"
	"encoding/hex"
	"fmt"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1b2 "github.com/fluxcd/source-controller/api/v1beta2"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	hashutil "k8s.io/kubernetes/pkg/util/hash"
	"k8s.io/utils/ptr"
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
	// PodAnnotationInitialized    = "auto.pulumi.com/initialized"
	PodAnnotationSourceHash = "auto.pulumi.com/source-hash"

	// TODO: get from configuration
	WorkspaceAgentImage = "pulumi/pulumi-kubernetes-agent:latest"

	// Termination grace period for the workspace pod and any update running in it.
	// Upon an update to the workspec spec or content, the statefulset will be updated,
	// leading to graceful pod replacement. The pod receives a SIGTERM signal and has
	// this much time to shut down before it is killed.
	WorkspacePodTerminationGracePeriodSeconds = 10 * 60
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
	updateStatus := func() error {
		w.Status.ObservedGeneration = w.Generation
		ready.ObservedGeneration = w.Generation
		meta.SetStatusCondition(&w.Status.Conditions, *ready)
		return r.Status().Update(ctx, w)
	}

	if w.DeletionTimestamp != nil {
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Deleting"
		ready.Message = "Workspace is being deleted"
		return ctrl.Result{}, updateStatus()
	}

	// determine the source revision to use in later steps.
	source := &sourceSpec{}
	if force, ok := w.Annotations[autov1alpha1.ForceRequestAnnotation]; ok && force != "" {
		source.ForceRequest = force
	}

	// if w.Spec.Git != nil {
	// 	source.Git = &agentpb.GitSource{
	// 		Url: w.Spec.Git.ProjectRepo,
	// 		Dir: &w.Spec.Git.RepoDir,
	// 	}
	// 	if w.Spec.Git.RepoDir != "" {
	// 		source.Git.Dir = &w.Spec.Git.RepoDir
	// 	}
	// 	if w.Spec.Git.Commit != "" {
	// 		source.Git.Ref = &agentpb.GitSource_CommitHash{CommitHash: w.Spec.Git.Commit}
	// 	} else if w.Spec.Git.Branch != "" {
	// 		source.Git.Ref = &agentpb.GitSource_Branch{Branch: w.Spec.Git.Branch}
	// 	}
	// }
	if w.Spec.Flux != nil {
		// Resolve the source reference and requeue the reconciliation if the source is not found.
		artifactSource, err := r.getSource(ctx, w)
		if err != nil {
			ready.Status = metav1.ConditionFalse
			ready.Reason = "ArtifactFailed"
			ready.Message = err.Error()
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, updateStatus()
			}
			// Retry with backoff on transient errors.
			_ = updateStatus()
			return ctrl.Result{}, err
		}

		// Requeue the reconciliation if the source artifact is not found.
		artifact := artifactSource.GetArtifact()
		if artifact == nil {
			ready.Status = metav1.ConditionFalse
			ready.Reason = "ArtifactFailed"
			ready.Message = "Source artifact not found, retrying later"
			return ctrl.Result{}, updateStatus()
		}

		source.Flux = &fluxSource{
			Url:    artifact.URL,
			Digest: artifact.Digest,
			Dir:    w.Spec.Flux.Dir,
		}
	}
	sourceHash := source.Hash()
	l.Info("Applying StatefulSet", "hash", sourceHash, "source", source)

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

	// make a statefulset, incorporating the source revision into the pod spec.
	// whenever the revision changes, the statefulset will be updated.
	// once the statefulset is updated, initialize the pod to that source revision.
	ss, err := newStatefulSet(w, source)
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

	if ss.Status.ObservedGeneration != ss.Generation || ss.Status.UpdateRevision != ss.Status.CurrentRevision {
		ready.Status = metav1.ConditionFalse
		ready.Reason = "RollingUpdate"
		ready.Message = "Waiting for the statefulset to be updated"
		return ctrl.Result{}, updateStatus()
	}
	if ss.Status.AvailableReplicas < 1 {
		ready.Status = metav1.ConditionFalse
		ready.Reason = "WaitingForReplicas"
		ready.Message = "Waiting for the workspace pod to be available"
		return ctrl.Result{}, updateStatus()
	}

	// // Locate the workspace pod, to figure out whether workspace initialization is needed.
	// // The workspace is stored in pod ephemeral storage, which has the same lifecycle as that of the pod.
	// podName := fmt.Sprintf("%s-0", nameForStatefulSet(w))
	// pod := &corev1.Pod{}
	// err = r.Get(ctx, types.NamespacedName{Name: podName, Namespace: w.Namespace}, pod)
	// if err != nil {
	// 	return ctrl.Result{}, fmt.Errorf("unable to find the workspace pod: %w", err)
	// }
	// podRevision := pod.Labels["controller-revision-hash"]
	// if podRevision != ss.Status.CurrentRevision {
	// 	// the pod cache must be stale because the statefulset is up-to-date yet the revision is mismatched.
	// 	l.Info("source revision mismatch; requeuing", "actual", podRevision, "expected", ss.Status.CurrentRevision)
	// 	return ctrl.Result{Requeue: true}, nil
	// }

	// // Connect to the workspace's GRPC server
	// addr := fmt.Sprintf("%s:%d", fqdnForService(w), WorkspaceGrpcPort)
	// l.Info("Connecting", "addr", addr)
	// w.Status.Address = addr

	// connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	// defer connectCancel()
	// conn, err := connect(connectCtx, addr)
	// if err != nil {
	// 	l.Error(err, "unable to connect; retrying later", "addr", addr)
	// 	ready.Status = metav1.ConditionFalse
	// 	ready.Reason = "ConnectionFailed"
	// 	ready.Message = err.Error()
	// 	return ctrl.Result{RequeueAfter: 5 * time.Second}, updateStatus()
	// }
	// defer func() {
	// 	_ = conn.Close()
	// }()
	// workspaceClient := agentpb.NewAutomationServiceClient(conn)

	// initializedV, ok := pod.Annotations[PodAnnotationInitialized]
	// initialized, _ := strconv.ParseBool(initializedV)
	// if !ok || !initialized {
	// 	l.Info("initializing the source", "hash", sourceHash)
	// 	ready.Status = metav1.ConditionFalse
	// 	ready.Reason = "Initializing"
	// 	ready.Message = ""
	// 	if err := updateStatus(); err != nil {
	// 		return ctrl.Result{}, err
	// 	}

	// 	initReq := &agentpb.InitializeRequest{}
	// 	if source.Git != nil {
	// 		initReq.Source = &agentpb.InitializeRequest_Git{
	// 			Git: source.Git,
	// 		}
	// 	}
	// 	if source.Flux != nil {
	// 		initReq.Source = &agentpb.InitializeRequest_Flux{
	// 			Flux: source.Flux,
	// 		}
	// 	}

	// 	l.Info("initializing the workspace")
	// 	_, err = workspaceClient.Initialize(ctx, initReq)
	// 	if err != nil {
	// 		l.Error(err, "unable to initialize; deleting the workspace pod to retry later")
	// 		ready.Status = metav1.ConditionFalse
	// 		ready.Reason = "InitializationFailed"
	// 		ready.Message = err.Error()

	// 		err = r.Client.Delete(ctx, pod)
	// 		if err != nil {
	// 			return ctrl.Result{}, err
	// 		}

	// 		return ctrl.Result{}, updateStatus()
	// 	}

	// 	// set the "initalized" annotation
	// 	if pod.Annotations == nil {
	// 		pod.Annotations = make(map[string]string)
	// 	}
	// 	pod.Annotations[PodAnnotationInitialized] = "true"
	// 	err = r.Update(ctx, pod, client.FieldOwner(FieldManager))
	// 	if err != nil {
	// 		l.Error(err, "unable to update the workspace pod; deleting the pod to retry later")
	// 		err = r.Client.Delete(ctx, pod)
	// 		if err != nil {
	// 			return ctrl.Result{}, err
	// 		}
	// 		return ctrl.Result{}, fmt.Errorf("failed to patch the pod: %w", err)
	// 	}
	// 	l.Info("initialized")
	// }

	addr := fmt.Sprintf("%s:%d", fqdnForService(w), WorkspaceGrpcPort)
	w.Status.Address = addr
	ready.Status = metav1.ConditionTrue
	ready.Reason = "Succeeded"
	ready.Message = ""
	l.Info("Ready")

	return ctrl.Result{}, updateStatus()
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
			r.newFluxMapper(sourcev1b2.GroupVersion, sourcev1b2.OCIRepositoryKind),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Watches(&sourcev1.GitRepository{},
			r.newFluxMapper(sourcev1.GroupVersion, sourcev1.GitRepositoryKind),
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Watches(&sourcev1b2.Bucket{},
			r.newFluxMapper(sourcev1b2.GroupVersion, sourcev1b2.BucketKind),
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

func (r *WorkspaceReconciler) newFluxMapper(gv schema.GroupVersion, kind string) handler.EventHandler {
	m := &fluxMapper{
		Client:           r.Client,
		GroupVersionKind: schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: kind},
	}
	return handler.EnqueueRequestsFromMapFunc(m.mapFluxSourceToWorkspace)
}

type fluxMapper struct {
	client.Client
	schema.GroupVersionKind
}

func (r *fluxMapper) mapFluxSourceToWorkspace(ctx context.Context, obj client.Object) []reconcile.Request {
	l := log.FromContext(ctx)
	apiVersion, kind := r.ToAPIVersionAndKind()
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
	FieldManager             = "pulumi-kubernetes-operator"
	WorkspaceContainerName   = "server"
	WorkspaceShareVolumeName = "share"
	WorkspaceShareMountPath  = "/share"
	WorkspaceGrpcPort        = 50051
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

func newStatefulSet(w *autov1alpha1.Workspace, source *sourceSpec) (*appsv1.StatefulSet, error) {
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
			Replicas:    ptr.To[int32](1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						// this annotation is used to cause pod replacement when the source has changed.
						PodAnnotationSourceHash: source.Hash(),
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            w.Spec.ServiceAccountName,
					TerminationGracePeriodSeconds: ptr.To[int64](WorkspacePodTerminationGracePeriodSeconds),
					InitContainers: []corev1.Container{
						{
							Name:            "bootstrap",
							Image:           WorkspaceAgentImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      WorkspaceShareVolumeName,
									MountPath: WorkspaceShareMountPath,
								},
							},
							Command: []string{"cp", "/agent", "/share/agent"},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "pulumi",
							Image:           w.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      WorkspaceShareVolumeName,
									MountPath: WorkspaceShareMountPath,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "grpc",
									ContainerPort: WorkspaceGrpcPort,
								},
							},
							Env:     w.Spec.Env,
							Command: []string{"/share/agent", "serve", "--workspace", "/share/workspace"},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: WorkspaceShareVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	if source.Flux != nil {
		script := `
/share/agent init -t /share/source --flux-url $FLUX_URL --flux-digest $FLUX_DIGEST &&
ln -s /share/source/$FLUX_DIR /share/workspace
		`
		container := corev1.Container{
			Name:            "fetch",
			Image:           WorkspaceAgentImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      WorkspaceShareVolumeName,
					MountPath: WorkspaceShareMountPath,
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "FLUX_URL",
					Value: source.Flux.Url,
				},
				{
					Name:  "FLUX_DIGEST",
					Value: source.Flux.Digest,
				},
				{
					Name:  "FLUX_DIR",
					Value: source.Flux.Dir,
				},
			},
			Command: []string{"sh", "-c", script},
		}
		statefulset.Spec.Template.Spec.InitContainers = append(statefulset.Spec.Template.Spec.InitContainers, container)
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

type sourceSpec struct {
	ForceRequest string
	// Git  *gitSource
	Flux *fluxSource
}

// type gitSource struct {
// 	Url    string
// 	Digest string
// }

type fluxSource struct {
	Url    string
	Digest string
	Dir    string
}

func (s *sourceSpec) Hash() string {
	hasher := md5.New()
	hashutil.DeepHashObject(hasher, s)
	return hex.EncodeToString(hasher.Sum(nil)[0:])
}

type source interface {
	sourcev1.Source
}

func (r *WorkspaceReconciler) getSource(ctx context.Context,
	obj *autov1alpha1.Workspace) (source, error) {
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
