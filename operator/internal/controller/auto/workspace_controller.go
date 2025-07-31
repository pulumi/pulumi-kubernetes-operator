// Copyright 2016-2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	agentpb "github.com/pulumi/pulumi-kubernetes-operator/v2/agent/pkg/proto"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	autov1alpha1webhook "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/webhook/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/version"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"sigs.k8s.io/controller-runtime/pkg/event"
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
	Scheme            *runtime.Scheme
	Recorder          record.EventRecorder
	ConnectionManager *ConnectionManager
}

//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces/rpc,verbs=use
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	w := &autov1alpha1.Workspace{}
	err := r.Get(ctx, req.NamespacedName, w)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	l = l.WithValues("revision", w.ResourceVersion)
	l.Info("Reconciling Workspace")

	// apply defaults to the workspace spec
	// future: use a mutating webhook to apply defaults
	defaulter := autov1alpha1webhook.WorkspaceCustomDefaulter{}
	err = defaulter.Default(ctx, w)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to apply defaults: %w", err)
	}

	ready := meta.FindStatusCondition(w.Status.Conditions, WorkspaceConditionTypeReady)
	if ready == nil {
		ready = &metav1.Condition{
			Type:   WorkspaceConditionTypeReady,
			Status: metav1.ConditionUnknown,
		}
	}
	updateStatus := func() error {
		oldRevision := w.ResourceVersion
		w.Status.ObservedGeneration = w.Generation
		ready.ObservedGeneration = w.Generation
		meta.SetStatusCondition(&w.Status.Conditions, *ready)
		err := r.Status().Update(ctx, w)
		if err != nil {
			l.Error(err, "updating status")
		} else {
			if w.ResourceVersion != oldRevision {
				l = log.FromContext(ctx).WithValues("revision", w.ResourceVersion)
				l.Info("Status updated",
					"observedGeneration", w.Status.ObservedGeneration,
					"address", w.Status.Address,
					"conditions", w.Status.Conditions)
			}
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
	if w.Spec.Local != nil {
		source.Local = &localSource{
			Dir: w.Spec.Local.Dir,
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
		// migration logic for 2.0.0-beta to 2.0.0
		if apierrors.IsInvalid(err) {
			l.V(0).Info("replacing the workspace statefulset to update an immutable field")
			if err = r.Delete(ctx, ss); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete statefulset: %w", err)
			}
			emitEvent(r.Recorder, w, autov1alpha1.MigratedEvent(), "Replaced the workspace statefulset")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
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
	l.Info("Connecting to workspace pod")
	connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Second)
	defer connectCancel()
	conn, err := r.ConnectionManager.Connect(connectCtx, w)
	if err != nil {
		l.Error(err, "unable to connect; retrying later")
		emitEvent(r.Recorder, w, autov1alpha1.ConnectionFailureEvent(), err.Error())
		ready.Status = metav1.ConditionFalse
		ready.Reason = "ConnectionFailed"
		ready.Message = err.Error()
		return ctrl.Result{RequeueAfter: 5 * time.Second}, updateStatus()
	}
	defer func() {
		_ = conn.Close()
	}()
	l.Info("Connected to workspace pod", "addr", conn.Target())
	w.Status.Address = conn.Target()
	wc := agentpb.NewAutomationServiceClient(conn)

	initializedV, ok := pod.Annotations[PodAnnotationInitialized]
	initialized, _ := strconv.ParseBool(initializedV)
	if !ok || !initialized {
		l.Info("Running whoami to ensure authentication is setup correctly with the workspace pod")
		_, err = wc.WhoAmI(ctx, &agentpb.WhoAmIRequest{})
		if err != nil {
			l.Error(err, "unable to run whoami; retaining the workspace pod to retry later")
			emitEvent(r.Recorder, w, autov1alpha1.ConnectionFailureEvent(), err.Error())

			st := status.Convert(err)

			ready.Status = metav1.ConditionFalse
			ready.Reason = st.Code().String()
			ready.Message = st.Message()

			// Override with structured error from PulumiErrorInfo if provided.
			if len(st.Details()) > 0 {
				if info, ok := st.Details()[0].(*agentpb.PulumiErrorInfo); ok {
					ready.Reason = info.Reason
					ready.Message = info.Message
				}
			}

			if statusErr := updateStatus(); statusErr != nil {
				return ctrl.Result{}, statusErr
			}

			return ctrl.Result{}, err
		}

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
			emitEvent(r.Recorder, w, autov1alpha1.InstallationFailureEvent(), err.Error())
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

				if len(stack.Config) != 0 {
					l.V(1).Info("Setting the stack configuration")
					config := make([]*agentpb.ConfigItem, 0, len(stack.Config))
					for _, item := range stack.Config {
						config = append(config, marshalConfigItem(item))
					}
					_, err := wc.SetAllConfig(ctx, &agentpb.SetAllConfigRequest{
						Config: config,
					})
					if err != nil {
						return err
					}
				}

				if len(stack.Environment) != 0 {
					l.V(1).Info("Setting the stack environment")
					_, err := wc.AddEnvironments(ctx, &agentpb.AddEnvironmentsRequest{
						Environment: stack.Environment,
					})
					if err != nil {
						return err
					}
				}

				return nil
			}()
			if err != nil {
				l.Error(err, "unable to initialize the Pulumi stack")
				emitEvent(r.Recorder, w, autov1alpha1.StackInitializationFailureEvent(), "Failed to initialize stack %q: %v", stack.Name, err.Error())
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
		emitEvent(r.Recorder, w, autov1alpha1.InitializedEvent(), "Initialized workspace pod %q", pod.Name)
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
		Named("workspace-controller").
		For(&autov1alpha1.Workspace{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, &DebugPredicate{Controller: "workspace-controller"})).
		Owns(&appsv1.StatefulSet{},
			builder.WithPredicates(&statefulSetReadyPredicate{}, &DebugPredicate{Controller: "workspace-controller"})).
		Complete(r)
}

type statefulSetReadyPredicate struct{}

var _ predicate.Predicate = &statefulSetReadyPredicate{}

func isStatefulSetReady(ss *appsv1.StatefulSet) bool {
	if ss.Status.ObservedGeneration != ss.Generation || ss.Status.UpdateRevision != ss.Status.CurrentRevision {
		return false
	}
	if ss.Status.AvailableReplicas < 1 {
		return false
	}
	return true
}

func (statefulSetReadyPredicate) Create(e event.CreateEvent) bool {
	return isStatefulSetReady(e.Object.(*appsv1.StatefulSet))
}

func (statefulSetReadyPredicate) Delete(_ event.DeleteEvent) bool {
	return false
}

func (statefulSetReadyPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return !isStatefulSetReady(e.ObjectOld.(*appsv1.StatefulSet)) && isStatefulSetReady(e.ObjectNew.(*appsv1.StatefulSet))
}

func (statefulSetReadyPredicate) Generic(_ event.GenericEvent) bool {
	return false
}

const (
	FieldManager                 = "pulumi-kubernetes-operator"
	WorkspacePulumiContainerName = "pulumi"
	WorkspaceShareVolumeName     = "share"
	WorkspaceShareMountPath      = "/share"
	WorkspaceGrpcPort            = 50051
)

func nameForStatefulSet(w *autov1alpha1.Workspace) string {
	return w.Name + "-workspace"
}

func nameForService(w *autov1alpha1.Workspace) string {
	return w.Name + "-workspace"
}

func fqdnForService(w *autov1alpha1.Workspace) string {
	return fmt.Sprintf("%s.%s", nameForService(w), w.Namespace)
}

func labelsForStatefulSet(w *autov1alpha1.Workspace) map[string]string {
	return map[string]string{
		ComponentLabel:     WorkspaceComponent,
		WorkspaceNameLabel: w.Name,
	}
}

func agentImage() (string, corev1.PullPolicy) {
	image := os.Getenv("AGENT_IMAGE")
	if image == "" {
		image = "pulumi/pulumi-kubernetes-operator:" + version.Version
	}
	policy := os.Getenv("AGENT_IMAGE_PULL_POLICY")
	var imagePullPolicy corev1.PullPolicy
	switch {
	case strings.EqualFold(policy, string(corev1.PullAlways)):
		imagePullPolicy = corev1.PullAlways
	case strings.EqualFold(policy, string(corev1.PullNever)):
		imagePullPolicy = corev1.PullNever
	case strings.EqualFold(policy, string(corev1.PullIfNotPresent)):
		imagePullPolicy = corev1.PullIfNotPresent
	default:
		imagePullPolicy = corev1.PullIfNotPresent
	}
	return image, imagePullPolicy
}

func newStatefulSet(ctx context.Context, w *autov1alpha1.Workspace, source *sourceSpec) (*appsv1.StatefulSet, error) {
	image, imagePullPolicy := agentImage()
	labels := labelsForStatefulSet(w)

	command := []string{
		"/share/tini", "--", "sh", "-c",
	}
	args := []string{
		`set -a; [ -f "$PULUMI_ENV" ] && . "$PULUMI_ENV"; set +a; exec /share/agent "$@"`,
		"agent",
		"serve",
		"--workspace", "/share/workspace",
		"--skip-install",
	}
	env := w.Spec.Env

	// provide some pod information to the agent for informational purposes
	env = append(env, corev1.EnvVar{
		Name: "POD_NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	})
	env = append(env, corev1.EnvVar{
		Name: "POD_SA_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "spec.serviceAccountName",
			},
		},
	})

	// limit the memory usage to the reserved amount
	// https://github.com/pulumi/pulumi-kubernetes-operator/issues/698
	env = append(env, corev1.EnvVar{
		Name: "AGENT_MEMLIMIT",
		ValueFrom: &corev1.EnvVarSource{
			ResourceFieldRef: &corev1.ResourceFieldSelector{
				ContainerName: WorkspacePulumiContainerName,
				Resource:      "requests.memory",
			},
		},
	})

	// enable workspace endpoint protection
	args = append(args,
		"--auth-mode", "kube",
		"--kube-audience", audienceForWorkspace(w),
		"--kube-workspace-namespace", w.Namespace,
		"--kube-workspace-name", w.Name)

	// increase Pulumi CLI log verbosity if provided
	if w.Spec.PulumiLogVerbosity != 0 {
		args = append(args, "--pulumi-log-level", strconv.Itoa(int(w.Spec.PulumiLogVerbosity)))
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
			Selector:            &metav1.LabelSelector{MatchLabels: labels},
			ServiceName:         nameForService(w),
			Replicas:            ptr.To[int32](1),
			PodManagementPolicy: appsv1.ParallelPodManagement,
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
							Image:           image,
							ImagePullPolicy: imagePullPolicy,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      WorkspaceShareVolumeName,
									MountPath: WorkspaceShareMountPath,
								},
							},
							Args: []string{"cp", "/agent", "/tini", "/share/"},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            WorkspacePulumiContainerName,
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
							Env:        env,
							EnvFrom:    w.Spec.EnvFrom,
							Command:    command,
							Args:       args,
							WorkingDir: "/share/workspace",
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
			Image:           image,
			ImagePullPolicy: imagePullPolicy,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      WorkspaceShareVolumeName,
					MountPath: WorkspaceShareMountPath,
				},
			},
			Env:  env,
			Args: []string{"sh", "-c", script},
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
			Image:           image,
			ImagePullPolicy: imagePullPolicy,
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
			Args: []string{"sh", "-c", script},
		}
		statefulset.Spec.Template.Spec.InitContainers = append(statefulset.Spec.Template.Spec.InitContainers, container)
	}

	if source.Local != nil {
		script := `ln -s $LOCAL_DIR /share/workspace`
		container := corev1.Container{
			Name:            "fetch",
			Image:           image,
			ImagePullPolicy: imagePullPolicy,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      WorkspaceShareVolumeName,
					MountPath: WorkspaceShareMountPath,
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "LOCAL_DIR",
					Value: source.Local.Dir,
				},
			},
			Args: []string{"sh", "-c", script},
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

	// Add well-known environment variables to the containers and init containers.
	common := []corev1.EnvVar{
		{Name: "PULUMI_ENV", Value: "/share/.env"},
	}
	for i := range statefulset.Spec.Template.Spec.Containers {
		statefulset.Spec.Template.Spec.Containers[i].Env = append(
			statefulset.Spec.Template.Spec.Containers[i].Env,
			common...,
		)
	}
	for i := range statefulset.Spec.Template.Spec.InitContainers {
		statefulset.Spec.Template.Spec.InitContainers[i].Env = append(
			statefulset.Spec.Template.Spec.InitContainers[i].Env,
			common...,
		)
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
	Local        *localSource
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

type localSource struct {
	Dir string
}

func (s *sourceSpec) Hash() string {
	hasher := md5.New() //nolint:gosec
	hashutil.DeepHashObject(hasher, s)
	return hex.EncodeToString(hasher.Sum(nil)[0:])
}

func mergePodTemplateSpec(_ context.Context, base *corev1.PodTemplateSpec, patch *autov1alpha1.EmbeddedPodTemplateSpec) (*corev1.PodTemplateSpec, error) {
	if patch == nil {
		return base, nil
	}
	baseData, err := marshalJSON(base)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON for base: %w", err)
	}
	patchData, err := marshalJSON(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON for pod template: %w", err)
	}

	// Set the ordering of the init containers such that the base init containers come first.
	// https://github.com/kubernetes/design-proposals-archive/blob/main/cli/preserve-order-in-strategic-merge-patch.md
	setElementOrder := []any{}
	for _, v := range base.Spec.InitContainers {
		setElementOrder = append(setElementOrder, map[string]any{"name": v.Name})
	}
	for _, v := range patch.Spec.InitContainers {
		setElementOrder = append(setElementOrder, map[string]any{"name": v.Name})
	}
	_ = unstructured.SetNestedSlice(patchData, setElementOrder, "spec", "$setElementOrder/initContainers")

	// Calculate the patch result.
	schema, err := strategicpatch.NewPatchMetaFromStruct(&corev1.PodTemplateSpec{})
	if err != nil {
		return nil, err
	}
	mergedData, err := strategicpatch.StrategicMergeMapPatchUsingLookupPatchMeta(baseData, patchData, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to generate merge patch for pod template: %w", err)
	}

	patchResult := &corev1.PodTemplateSpec{}
	if err := unmarshalJSON(mergedData, patchResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merged pod template: %w", err)
	}

	return patchResult, nil
}
