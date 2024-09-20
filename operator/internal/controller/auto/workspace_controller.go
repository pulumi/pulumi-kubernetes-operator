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
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/operator/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/tools/record"
	hashutil "k8s.io/kubernetes/pkg/util/hash"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	WorkspaceIndexerFluxSource  = "index.spec.flux.sourceRef"
	WorkspaceConditionTypeReady = autov1alpha1.WorkspaceReady
	PodAnnotationInitialized    = "auto.pulumi.com/initialized"
	PodAnnotationRevisionHash   = "auto.pulumi.com/revision-hash"

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
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	w := &autov1alpha1.Workspace{}
	err := r.Get(ctx, req.NamespacedName, w)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	l.V(1).Info("Reconciling Workspace", "workspace", req.NamespacedName, "generation", w.Generation)

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
		err := r.Status().Update(ctx, w)
		if err != nil {
			l.Error(err, "updating status")
		}
		return err
	}

	if w.DeletionTimestamp != nil {
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Deleting"
		ready.Message = "Workspace is being deleted"
		return ctrl.Result{}, updateStatus()
	}

	// determine the source revision to use in later steps.
	source := &sourceSpec{
		Generation: w.Generation,
	}
	if w.Spec.Git != nil {
		source.Git = &gitSource{
			URL:     w.Spec.Git.URL,
			Dir:     w.Spec.Git.Dir,
			Ref:     w.Spec.Git.Ref,
			Shallow: w.Spec.Git.Shallow,
		}
		if w.Spec.Git.Auth != nil {
			source.Git.Password = w.Spec.Git.Auth.Password
			source.Git.SSHPrivateKey = w.Spec.Git.Auth.SSHPrivateKey
			source.Git.Token = w.Spec.Git.Auth.Token
			source.Git.Username = w.Spec.Git.Auth.Username
		}
	}
	if w.Spec.Flux != nil {
		source.Flux = &fluxSource{
			Url:    w.Spec.Flux.Url,
			Digest: w.Spec.Flux.Digest,
			Dir:    w.Spec.Flux.Dir,
		}
	}
	sourceHash := source.Hash()
	l.Info("Applying StatefulSet", "hash", sourceHash, "source", source)

	// service
	svc := newService(w)
	if err := controllerutil.SetControllerReference(w, svc, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	err = r.Patch(ctx, svc, client.Apply, client.FieldOwner(FieldManager))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to apply service: %w", err)
	}
	addr := fmt.Sprintf("%s:%d", fqdnForService(w), WorkspaceGrpcPort)

	// make a statefulset, incorporating the source revision into the pod spec.
	// whenever the revision changes, the statefulset will be updated.
	// once the statefulset is updated, initialize the pod to that source revision.
	ss, err := newStatefulSet(ctx, w, source)
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
		w.Status.Address = ""
		return ctrl.Result{}, updateStatus()
	}
	if ss.Status.AvailableReplicas < 1 {
		ready.Status = metav1.ConditionFalse
		ready.Reason = "WaitingForReplicas"
		ready.Message = "Waiting for the workspace pod to be available"
		w.Status.Address = ""
		return ctrl.Result{}, updateStatus()
	}

	// Locate the workspace pod, to figure out whether workspace initialization is needed.
	// The workspace is stored in pod ephemeral storage, which has the same lifecycle as that of the pod.
	podName := fmt.Sprintf("%s-0", nameForStatefulSet(w))
	pod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: podName, Namespace: w.Namespace}, pod)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to find the workspace pod: %w", err)
	}
	podRevision := pod.Labels["controller-revision-hash"]
	if podRevision != ss.Status.CurrentRevision {
		// the pod cache must be stale because the statefulset is up-to-date yet the revision is mismatched.
		l.Info("source revision mismatch; requeuing", "actual", podRevision, "expected", ss.Status.CurrentRevision)
		return ctrl.Result{Requeue: true}, nil
	}

	// Connect to the workspace's GRPC server
	l.Info("Connecting to workspace pod", "addr", addr)
	connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Second)
	defer connectCancel()
	conn, err := connect(connectCtx, addr)
	if err != nil {
		l.Error(err, "unable to connect; retrying later", "addr", addr)
		ready.Status = metav1.ConditionFalse
		ready.Reason = "ConnectionFailed"
		ready.Message = err.Error()
		return ctrl.Result{RequeueAfter: 5 * time.Second}, updateStatus()
	}
	defer func() {
		_ = conn.Close()
	}()
	w.Status.Address = addr
	wc := agentpb.NewAutomationServiceClient(conn)

	initializedV, ok := pod.Annotations[PodAnnotationInitialized]
	initialized, _ := strconv.ParseBool(initializedV)
	if !ok || !initialized {
		l.Info("Running pulumi install")
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Installing"
		ready.Message = "Installing packages and plugins required by the program"
		if err := updateStatus(); err != nil {
			return ctrl.Result{}, err
		}
		_, err = wc.Install(ctx, &agentpb.InstallRequest{})
		if err != nil {
			l.Error(err, "unable to install; deleting the workspace pod to retry later")
			ready.Status = metav1.ConditionFalse
			ready.Reason = "InstallationFailed"
			ready.Message = err.Error()
			err = r.Client.Delete(ctx, pod)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, updateStatus()
		}

		l.Info("Creating Pulumi stack(s)")
		ready.Status = metav1.ConditionFalse
		ready.Reason = "Initializing"
		ready.Message = "Initializing and selecting a Pulumi stack"
		if err := updateStatus(); err != nil {
			return ctrl.Result{}, err
		}
		for _, stack := range w.Spec.Stacks {
			l := l.WithValues("stackName", stack.Name)
			err := func() error {
				l.V(1).Info("Creating or selecting a stack",
					"create", stack.Create, "secretsProvider", stack.SecretsProvider)
				if _, err = wc.SelectStack(ctx, &agentpb.SelectStackRequest{
					StackName:       stack.Name,
					Create:          stack.Create,
					SecretsProvider: stack.SecretsProvider,
				}); err != nil {
					return err
				}
				l.V(1).Info("Setting the stack configuration")
				for _, item := range stack.Config {
					v := marshalConfigValue(item)
					if _, err = wc.SetAllConfig(ctx, &agentpb.SetAllConfigRequest{
						Path: item.Path,
						Config: map[string]*agentpb.ConfigValue{
							item.Key: v,
						},
					}); err != nil {
						return err
					}
				}
				return nil
			}()
			if err != nil {
				l.Error(err, "unable to initialize the Pulumi stack")
				ready.Status = metav1.ConditionFalse
				ready.Reason = "InitializationFailed"
				ready.Message = err.Error()
				err = r.Client.Delete(ctx, pod)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, updateStatus()
			}
		}

		// set the "initialized" annotation
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[PodAnnotationInitialized] = "true"
		err = r.Update(ctx, pod, client.FieldOwner(FieldManager))
		if err != nil {
			l.Error(err, "unable to update the workspace pod; deleting the pod to retry later")
			err = r.Client.Delete(ctx, pod)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, fmt.Errorf("failed to update the pod: %w", err)
		}
		l.Info("workspace pod initialized")
	}

	ready.Status = metav1.ConditionTrue
	ready.Reason = "Succeeded"
	ready.Message = ""
	l.Info("Ready")

	return ctrl.Result{}, updateStatus()
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&autov1alpha1.Workspace{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Service{},
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Owns(&appsv1.StatefulSet{},
			builder.WithPredicates(&predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
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

func newStatefulSet(ctx context.Context, w *autov1alpha1.Workspace, source *sourceSpec) (*appsv1.StatefulSet, error) {
	// TODO: get from configuration
	workspaceAgentImage := "pulumi/pulumi-kubernetes-operator-v2:" + version.Version

	labels := labelsForStatefulSet(w)

	command := []string{
		"/share/agent", "serve",
		"--workspace", "/share/workspace",
		"--skip-install",
	}

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
						PodAnnotationRevisionHash: source.Hash(),
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: w.Spec.ServiceAccountName,
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					TerminationGracePeriodSeconds: ptr.To[int64](WorkspacePodTerminationGracePeriodSeconds),
					InitContainers: []corev1.Container{
						{
							Name:            "bootstrap",
							Image:           workspaceAgentImage,
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
							ImagePullPolicy: w.Spec.ImagePullPolicy,
							Resources:       w.Spec.Resources,
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
							EnvFrom: w.Spec.EnvFrom,
							Command: command,
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

	// apply the 'fetch' init container
	if source.Git != nil {
		script := `
/share/agent init -t /share/source --git-url $GIT_URL --git-revision $GIT_REVISION &&
ln -s /share/source/$GIT_DIR /share/workspace
		`
		env := []corev1.EnvVar{
			{
				Name:  "GIT_URL",
				Value: source.Git.URL,
			},
			{
				Name:  "GIT_REVISION",
				Value: source.Git.Ref,
			},
			{
				Name:  "GIT_DIR",
				Value: source.Git.Dir,
			},
		}
		if source.Git.Shallow {
			env = append(env, corev1.EnvVar{
				Name:  "GIT_SHALLOW",
				Value: "true",
			})
		}
		if source.Git.SSHPrivateKey != nil {
			env = append(env, corev1.EnvVar{
				Name: "GIT_SSH_PRIVATE_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: source.Git.SSHPrivateKey,
				},
			})
		}
		if source.Git.Username != nil {
			env = append(env, corev1.EnvVar{
				Name: "GIT_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: source.Git.Username,
				},
			})
		}
		if source.Git.Password != nil {
			env = append(env, corev1.EnvVar{
				Name: "GIT_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: source.Git.Password,
				},
			})
		}
		if source.Git.Token != nil {
			env = append(env, corev1.EnvVar{
				Name: "GIT_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: source.Git.Token,
				},
			})
		}

		container := corev1.Container{
			Name:            "fetch",
			Image:           workspaceAgentImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      WorkspaceShareVolumeName,
					MountPath: WorkspaceShareMountPath,
				},
			},
			Env:     env,
			Command: []string{"sh", "-c", script},
		}
		statefulset.Spec.Template.Spec.InitContainers = append(statefulset.Spec.Template.Spec.InitContainers, container)
	}

	if source.Flux != nil {
		script := `
/share/agent init -t /share/source --flux-url $FLUX_URL --flux-digest $FLUX_DIGEST &&
ln -s /share/source/$FLUX_DIR /share/workspace
		`
		container := corev1.Container{
			Name:            "fetch",
			Image:           workspaceAgentImage,
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

	// apply the 'restricted' security profile as necessary
	if w.Spec.SecurityProfile == autov1alpha1.SecurityProfileRestricted {
		sc := statefulset.Spec.Template.Spec.SecurityContext
		sc.RunAsNonRoot = ptr.To(true)
		sc.RunAsUser = ptr.To(int64(1000))
		sc.RunAsGroup = ptr.To(int64(1000))

		initContainers := statefulset.Spec.Template.Spec.InitContainers
		for i := range initContainers {
			if initContainers[i].SecurityContext == nil {
				initContainers[i].SecurityContext = &corev1.SecurityContext{}
			}
			initContainers[i].SecurityContext.AllowPrivilegeEscalation = ptr.To(false)
			initContainers[i].SecurityContext.Capabilities = &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
			}
		}

		containers := statefulset.Spec.Template.Spec.Containers
		for i := range containers {
			if containers[i].SecurityContext == nil {
				containers[i].SecurityContext = &corev1.SecurityContext{}
			}
			containers[i].SecurityContext.AllowPrivilegeEscalation = ptr.To(false)
			containers[i].SecurityContext.Capabilities = &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
			}
		}
	}

	// merge the user-supplied template using strategic merge patch
	if w.Spec.PodTemplate != nil {
		podTemplate, err := mergePodTemplateSpec(ctx, &statefulset.Spec.Template, w.Spec.PodTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to merge pod template: %w", err)
		}
		statefulset.Spec.Template = *podTemplate
	}

	return statefulset, nil
}

func newService(w *autov1alpha1.Workspace) *corev1.Service {
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

	return service
}

type sourceSpec struct {
	Generation   int64
	ForceRequest string
	Git          *gitSource
	Flux         *fluxSource
}

type gitSource struct {
	URL           string
	Ref           string
	Dir           string
	Shallow       bool
	SSHPrivateKey *corev1.SecretKeySelector
	Username      *corev1.SecretKeySelector
	Password      *corev1.SecretKeySelector
	Token         *corev1.SecretKeySelector
}

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

func mergePodTemplateSpec(_ context.Context, base *corev1.PodTemplateSpec, patch *autov1alpha1.EmbeddedPodTemplateSpec) (*corev1.PodTemplateSpec, error) {
	if patch == nil {
		return base, nil
	}

	baseBytes, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON for base: %w", err)
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON for pod template: %w", err)
	}

	// Calculate the patch result.
	jsonResultBytes, err := strategicpatch.StrategicMergePatch(baseBytes, patchBytes, &corev1.PodTemplateSpec{})
	if err != nil {
		return nil, fmt.Errorf("failed to generate merge patch for pod template: %w", err)
	}

	patchResult := &corev1.PodTemplateSpec{}
	if err := json.Unmarshal(jsonResultBytes, patchResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merged pod template: %w", err)
	}

	return patchResult, nil
}
