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

package pulumi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/operator-framework/operator-lib/handler"
	autov1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/auto/v1alpha1"
	"github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/api/pulumi/v1"
	auto "github.com/pulumi/pulumi-kubernetes-operator/v2/operator/internal/controller/auto"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlhandler "sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	errRequirementNotRun    = fmt.Errorf("prerequisite has not run to completion")
	errRequirementFailed    = fmt.Errorf("prerequisite failed")
	errRequirementOutOfDate = fmt.Errorf("prerequisite succeeded but not recently enough")
)

const (
	pulumiFinalizer                = "finalizer.stack.pulumi.com"
	defaultMaxConcurrentReconciles = 10
	programRefIndexFieldName       = ".spec.programRef.name"      // this is an arbitrary string, named for the field it indexes
	fluxSourceIndexFieldName       = ".spec.fluxSource.sourceRef" // an arbitrary name, named for the field it indexes
)

const (
	FieldManager = "pulumi-kubernetes-operator"
)

// prerequisiteIndexFieldName is the name used for indexing the prerequisites field.
const prerequisiteIndexFieldName = "spec.prerequisites.name"

// SetupWithManager sets up the controller with the Manager.
func (r *StackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	blder := ctrl.NewControllerManagedBy(mgr).Named("stack-controller")
	opts := controller.Options{}

	opts.MaxConcurrentReconciles = defaultMaxConcurrentReconciles
	if maxConcurrentReconcilesStr, ok := os.LookupEnv("MAX_CONCURRENT_RECONCILES"); ok {
		opts.MaxConcurrentReconciles, err = strconv.Atoi(maxConcurrentReconcilesStr)
		if err != nil {
			return err
		}
	}

	// Filter for update events where an object's metadata.generation is changed (no spec change!),
	// or the "force reconcile" annotation is used (and not marked as handled).
	predicates := []predicate.Predicate{
		predicate.Or(
			predicate.And(predicate.GenerationChangedPredicate{}, predicate.Not(&finalizerAddedPredicate{})),
			ReconcileRequestedPredicate{}),
	}

	// Track metrics about stacks.
	stackInformer, err := mgr.GetCache().GetInformer(context.Background(), &pulumiv1.Stack{})
	if err != nil {
		return err
	}
	if _, err = stackInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    newStackCallback,
		UpdateFunc: updateStackCallback,
		DeleteFunc: deleteStackCallback,
	}); err != nil {
		return err
	}

	// Maintain an index of stacks->dependents; so that when a stack succeeds, we can requeue any
	// stacks that might be waiting for it.
	indexer := mgr.GetFieldIndexer()
	if err = indexer.IndexField(context.Background(), &pulumiv1.Stack{}, prerequisiteIndexFieldName, func(o client.Object) []string {
		stack := o.(*pulumiv1.Stack)
		names := make([]string, len(stack.Spec.Prerequisites), len(stack.Spec.Prerequisites))
		for i := range stack.Spec.Prerequisites {
			names[i] = stack.Spec.Prerequisites[i].Name
		}
		return names
	}); err != nil {
		return err
	}

	enqueueDependents := func(ctx context.Context, stack client.Object) []reconcile.Request {
		var dependentStacks pulumiv1.StackList
		err := mgr.GetClient().List(ctx, &dependentStacks,
			client.InNamespace(stack.GetNamespace()),
			client.MatchingFields{prerequisiteIndexFieldName: stack.GetName()})
		if err == nil {
			reqs := make([]reconcile.Request, len(dependentStacks.Items), len(dependentStacks.Items))
			for i := range dependentStacks.Items {
				reqs[i].NamespacedName = client.ObjectKeyFromObject(&dependentStacks.Items[i])
			}
			return reqs
		}
		// we don't get to return an error; only to fail quietly
		mgr.GetLogger().Error(err, "failed to fetch dependents for object", "name", stack.GetName(), "namespace", stack.GetNamespace())
		return nil
	}

	// Watch for changes to primary resource Stack
	blder = blder.Watches(&pulumiv1.Stack{}, &handler.InstrumentedEnqueueRequestForObject[client.Object]{}, builder.WithPredicates(predicates...))

	// Watch stacks so that dependent stacks can be requeued when they change
	blder = blder.Watches(&pulumiv1.Stack{}, ctrlhandler.EnqueueRequestsFromMapFunc(enqueueDependents) /* , builder.WithPredicates(dependentStatusUpdate...) */)

	// Watch Programs, and look up which (if any) Stack refers to them when they change

	// Index stacks against the names of programs they reference
	if err = indexer.IndexField(context.Background(), &pulumiv1.Stack{}, programRefIndexFieldName, func(o client.Object) []string {
		stack := o.(*pulumiv1.Stack)
		if stack.Spec.ProgramRef != nil {
			return []string{stack.Spec.ProgramRef.Name}
		}
		return nil
	}); err != nil {
		return err
	}

	// this encodes the "use an index to look up the stacks used by a source" pattern which both
	// ProgramRef and FluxSource need.
	enqueueStacksForSourceFunc := func(indexName string, getFieldKey func(client.Object) string) func(context.Context, client.Object) []reconcile.Request {
		return func(ctx context.Context, src client.Object) []reconcile.Request {
			var stacks pulumiv1.StackList
			err := mgr.GetClient().List(ctx, &stacks,
				client.InNamespace(src.GetNamespace()),
				client.MatchingFields{indexName: getFieldKey(src)})
			if err == nil {
				reqs := make([]reconcile.Request, len(stacks.Items), len(stacks.Items))
				for i := range stacks.Items {
					reqs[i].NamespacedName = client.ObjectKeyFromObject(&stacks.Items[i])
				}
				return reqs
			}
			// we don't get to return an error; only to fail quietly
			mgr.GetLogger().Error(err, "failed to fetch stack referring to source",
				"gvk", src.GetObjectKind().GroupVersionKind(),
				"name", src.GetName(),
				"namespace", src.GetNamespace())
			return nil
		}
	}
	blder = blder.Watches(&pulumiv1.Program{}, ctrlhandler.EnqueueRequestsFromMapFunc(
		enqueueStacksForSourceFunc(programRefIndexFieldName,
			func(obj client.Object) string {
				return obj.GetName()
			})),
		builder.WithPredicates(&auto.DebugPredicate{Controller: "stack-controller"}))

	// Watch the stack's workspace and update objects
	blder = blder.Watches(&autov1alpha1.Workspace{},
		ctrlhandler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &pulumiv1.Stack{}),
		builder.WithPredicates(&workspaceReadyPredicate{}, &auto.DebugPredicate{Controller: "stack-controller"}))
	blder = blder.Watches(&autov1alpha1.Update{},
		ctrlhandler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &pulumiv1.Stack{}),
		builder.WithPredicates(&updateCompletePredicate{}, &auto.DebugPredicate{Controller: "stack-controller"}))

	c, err := blder.WithOptions(opts).Build(r)
	if err != nil {
		return err
	}

	// Lazily watch Flux sources we get told about, and look up the Stack(s) using them when they change

	// Index the stacks against the type and name of sources they reference.
	if err = indexer.IndexField(context.Background(), &pulumiv1.Stack{}, fluxSourceIndexFieldName, func(o client.Object) []string {
		stack := o.(*pulumiv1.Stack)
		if source := stack.Spec.FluxSource; source != nil {
			gvk, err := getSourceGVK(source.SourceRef)
			if err != nil {
				mgr.GetLogger().Error(err, "unable to parse .sourceRef.apiVersion in Flux source")
				return nil
			}
			// the keys include the type, because the references are not of a fixed type of object
			return []string{fluxSourceKey(gvk, source.SourceRef.Name)}
		}
		return nil
	}); err != nil {
		return err
	}

	// We can't watch a specific type (i.e., using source.Kind) here; what we have to do is wait
	// until we see stacks that refer to particular kinds, then watch those. Technically this can
	// "leak" watches -- we may end up watching kinds that are no longer mentioned in stacks. My
	// assumption is that the number of distinct types that might be mentioned (including typos) is
	// low enough that this remains acceptably cheap.

	// Keep track of types we've already watched, so we don't install more than one handler for a
	// type.
	watched := make(map[schema.GroupVersionKind]struct{})
	watchedMu := sync.Mutex{}

	// Calling this will attempt to install a watch for the kind given in the source reference. It
	// will return an error if there's something wrong with the source reference or if the watch
	// could not be attempted otherwise. If the kind cannot be found then this will keep trying in
	// the background until the context given to controller.Start is cancelled, rather than return
	// an error.
	r.maybeWatchFluxSourceKind = func(src shared.FluxSourceReference) error {
		gvk, err := getSourceGVK(src)
		if err != nil {
			return err
		}
		watchedMu.Lock()
		_, ok := watched[gvk]
		if !ok {
			watched[gvk] = struct{}{}
		}
		watchedMu.Unlock()
		if !ok {
			var sourceKind unstructured.Unstructured
			sourceKind.SetGroupVersionKind(gvk)
			mgr.GetLogger().Info("installing watcher for newly seen source kind", "GroupVersionKind", gvk)
			err = c.Watch(source.Kind[client.Object](mgr.GetCache(), &sourceKind,
				ctrlhandler.TypedEnqueueRequestsFromMapFunc(
					enqueueStacksForSourceFunc(fluxSourceIndexFieldName, func(obj client.Object) string {
						gvk := obj.GetObjectKind().GroupVersionKind()
						return fluxSourceKey(gvk, obj.GetName())
					})), &fluxSourceReadyPredicate{}, &auto.DebugPredicate{Controller: "stack-controller"}))
			if err != nil {
				watchedMu.Lock()
				delete(watched, gvk)
				watchedMu.Unlock()
				mgr.GetLogger().Error(err, "failed to watch source kind", "GroupVersionKind", gvk)
				return err
			}
		}
		return nil
	}

	return nil
}

// isRequirementSatisfied checks the given readiness requirement against the given stack, and
// returns nil if the requirement is satisfied, and an error otherwise. The requirement can be nil
// itself, in which case the prerequisite is only that the stack succeeded on its last run.
func isRequirementSatisfied(req *shared.RequirementSpec, stack pulumiv1.Stack) error {
	if stack.Status.LastUpdate == nil || stack.Status.LastUpdate.Generation != stack.GetGeneration() {
		return errRequirementNotRun
	}
	if stack.Status.LastUpdate.State != shared.SucceededStackStateMessage {
		return errRequirementFailed
	}
	if req != nil && req.SucceededWithinDuration != nil {
		lastRun := stack.Status.LastUpdate.LastResyncTime
		if lastRun.IsZero() || time.Since(lastRun.Time) > req.SucceededWithinDuration.Duration {
			return errRequirementOutOfDate
		}
	}
	return nil
}

// ReconcileRequestedPredicate filters (returns true) for resources that are updated with a new or
// amended annotation value at `ReconcileRequestAnnotation`.
//
// Reconciliation request protocol:
//
// This gives a means of prompting the controller to reconsider a resource that otherwise might not
// be queued, e.g., because it has already reached a success state. This is useful for command-line
// tooling (e.g., you can trigger updates with `kubectl annotate`), and is the mechanism used to
// requeue prerequisites that are not up to date.
//
// The protocol works like this:

//   - when you want the object to be considered for reconciliation, annotate it with the key
//     `shared.ReconcileRequestAnnotation` and any likely-to-be-unique value. This causes the object
//     to be queued for consideration by the controller;
//   - the controller shall save the value of the annotation to `.status.observedReconcileRequest`
//     whenever it processes a resource. This is so you can check the
//     status to see whether the annotation has been seen (similar to `.status.observedGeneration`).
//
// This protocol is the same mechanism used by many Flux controllers, as explained at
// https://pkg.go.dev/github.com/fluxcd/pkg/runtime/predicates#ReconcileRequestedPredicate and
// related documentation.
type ReconcileRequestedPredicate struct {
	predicate.Funcs
}

func getReconcileRequestAnnotation(obj client.Object) (string, bool) {
	r, ok := obj.GetAnnotations()[shared.ReconcileRequestAnnotation]
	return r, ok
}

// Update filters update events based on whether the request reconciliation annotation has been
// added or amended.
func (p ReconcileRequestedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	if vNew, ok := getReconcileRequestAnnotation(e.ObjectNew); ok {
		if vOld, ok := getReconcileRequestAnnotation(e.ObjectOld); ok {
			return vNew != vOld
		}
		return true // new object has it, old one doesn't
	}
	return false // either removed, or present in neither object
}

func isWorkspaceReady(ws *autov1alpha1.Workspace) bool {
	if ws == nil || ws.Generation != ws.Status.ObservedGeneration {
		return false
	}
	return meta.IsStatusConditionTrue(ws.Status.Conditions, autov1alpha1.WorkspaceReady)
}

type workspaceReadyPredicate struct{}

var _ predicate.Predicate = &workspaceReadyPredicate{}

func (workspaceReadyPredicate) Create(e event.CreateEvent) bool {
	return isWorkspaceReady(e.Object.(*autov1alpha1.Workspace))
}

func (workspaceReadyPredicate) Delete(_ event.DeleteEvent) bool {
	return false
}

func (workspaceReadyPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return !isWorkspaceReady(e.ObjectOld.(*autov1alpha1.Workspace)) && isWorkspaceReady(e.ObjectNew.(*autov1alpha1.Workspace))
}

func (workspaceReadyPredicate) Generic(_ event.GenericEvent) bool {
	return false
}

func isUpdateComplete(update *autov1alpha1.Update) bool {
	if update == nil || update.Generation != update.Status.ObservedGeneration {
		return false
	}
	return meta.IsStatusConditionTrue(update.Status.Conditions, autov1alpha1.UpdateConditionTypeComplete)
}

type updateCompletePredicate struct{}

var _ predicate.Predicate = &updateCompletePredicate{}

func (updateCompletePredicate) Create(e event.CreateEvent) bool {
	return isUpdateComplete(e.Object.(*autov1alpha1.Update))
}

func (updateCompletePredicate) Delete(e event.DeleteEvent) bool {
	return false
}

func (updateCompletePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return !isUpdateComplete(e.ObjectOld.(*autov1alpha1.Update)) && isUpdateComplete(e.ObjectNew.(*autov1alpha1.Update))
}

func (updateCompletePredicate) Generic(e event.GenericEvent) bool {
	return false
}

// StackReconciler reconciles a Stack object
type StackReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// this is initialised by add(), to be available to Reconcile
	maybeWatchFluxSourceKind func(shared.FluxSourceReference) error
}

// StallError represents a problem that makes a Stack spec unprocessable, while otherwise being
// valid. For example: the spec refers to a secret in another namespace. This is used to signal
// "stall" failures within helpers -- that is, when the operator cannot process the object as it is
// specified.
type StallError struct {
	error
}

func newStallErrorf(format string, args ...interface{}) error {
	return StallError{fmt.Errorf(format, args...)}
}

func isStalledError(e error) bool {
	var s StallError
	return errors.As(e, &s)
}

var (
	errNamespaceIsolation          = newStallErrorf(`cross-namespace refs are not allowed`)
	errOtherThanOneSourceSpecified = newStallErrorf(`exactly one source (.spec.fluxSource, .spec.projectRepo, or .spec.programRef) for the stack must be given`)
)

var errProgramNotFound = fmt.Errorf("unable to retrieve program for stack")

//+kubebuilder:rbac:groups=pulumi.com,resources=stacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pulumi.com,resources=stacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pulumi.com,resources=stacks/finalizers,verbs=update
//+kubebuilder:rbac:groups=pulumi.com,resources=programs,verbs=get;list;watch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=buckets,verbs=get;list;watch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories,verbs=get;list;watch
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=auto.pulumi.com,resources=updates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

// Reconcile reads that state of the cluster for a Stack object and makes changes based on the state read
// and what is in the Stack.Spec
func (r *StackReconciler) Reconcile(ctx context.Context, request ctrl.Request) (res ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the Stack instance
	instance := &pulumiv1.Stack{}
	err := r.Client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		// Request object not found, could have been deleted after reconcile request.
		// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
		// Return and don't requeue
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	log = log.WithValues("revision", instance.ResourceVersion)
	log.Info("Reconciling Stack")

	// Update the observed generation and "reconcile request" of the object.
	instance.Status.ObservedGeneration = instance.GetGeneration()
	if req, ok := getReconcileRequestAnnotation(instance); ok {
		instance.Status.ObservedReconcileRequest = req
	}

	// Check if the Stack instance is marked to be deleted, which is indicated by the deletion
	// timestamp being set.
	isStackMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	// If there's no finalizer, it's either been cleaned up, never been seen, or never gotten far
	// enough to need cleaning up.
	if isStackMarkedToBeDeleted && !slices.Contains(instance.GetFinalizers(), pulumiFinalizer) {
		return reconcile.Result{}, nil
	}
	if !isStackMarkedToBeDeleted && controllerutil.AddFinalizer(instance, pulumiFinalizer) {
		if err = r.Update(ctx, instance, client.FieldOwner(FieldManager)); err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to add finalizer: %w", err)
		}
	}

	// This helper helps with updates, from here onwards.
	stack := instance.Spec
	sess := newStackReconcilerSession(log, stack, r.Client, r.Scheme, request.Namespace)

	// Plan a workspace object as an execution environment for the stack.
	// The workspace object is deleted during stack finalization.
	// Any problem here is unexpected, and treated as a controller error.
	err = sess.NewWorkspace(instance)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("unable to define workspace for stack: %w", err)
	}

	saveStatus := func() error {
		if err := r.Status().Update(ctx, instance); err != nil {
			log.Error(err, "unable to save object status")
			return err
		}
		log = ctrllog.FromContext(ctx).WithValues("revision", instance.ResourceVersion)
		log.V(1).Info("Status updated")
		return nil
	}

	// Check for an outstanding update, and absorb the result into status if the update is complete.
	if instance.Status.CurrentUpdate != nil {
		if err := sess.readCurrentUpdate(ctx, types.NamespacedName{
			Name:      instance.Status.CurrentUpdate.Name,
			Namespace: request.Namespace,
		}); err != nil {
			if apierrors.IsNotFound(err) {
				// the cache is probably out of date; wait for a watch event.
				log.Info("update object not found; will retry", "Name", instance.Status.CurrentUpdate.Name)
				instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingProcessingReason, pulumiv1.ReconcilingProcessingUpdateMessage)
				return reconcile.Result{}, saveStatus()
			}
			return reconcile.Result{}, fmt.Errorf("get current update: %w", err)
		}

		completed := sess.update.Generation == sess.update.Status.ObservedGeneration &&
			meta.IsStatusConditionTrue(sess.update.Status.Conditions, autov1alpha1.UpdateConditionTypeComplete)
		if !completed {
			// wait for the update to complete
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingProcessingReason, pulumiv1.ReconcilingProcessingUpdateMessage)
			return reconcile.Result{}, saveStatus()
		}

		// The update is complete. If it failed, we need to mark the stack as failed.
		if meta.IsStatusConditionTrue(sess.update.Status.Conditions, autov1alpha1.UpdateConditionTypeFailed) {
			// The update failed. We need to mark the stack as failed.
			r.markStackFailed(sess, instance, instance.Status.CurrentUpdate, sess.update)
		} else {
			// The update succeeded.
			err := r.markStackSucceeded(ctx, instance, instance.Status.CurrentUpdate, sess.update)
			if err != nil {
				return reconcile.Result{}, fmt.Errorf("marking stack succeeded: %w", err)
			}
		}

		instance.Status.CurrentUpdate = nil
	}

	// We can exit early if there is no clean-up to do.
	if isStackMarkedToBeDeleted && !stack.DestroyOnFinalize {
		if controllerutil.RemoveFinalizer(instance, pulumiFinalizer) {
			return reconcile.Result{}, r.Update(ctx, instance, client.FieldOwner(FieldManager))
		}
		return reconcile.Result{}, nil
	}

	// Check prerequisites, to make sure they are adequately up to date. Any prerequisite failing to
	// be met will cause this run to be abandoned and the stack under consideration to be requeued;
	// however, we go through all of the prerequisites anyway, so we can annotate all failing stacks
	// to be requeued themselves.
	var failedPrereqNames []string // in the case there's more than one, we report the names
	var failedPrereqErr error      // in caase there's just one, we report the specific error

	for _, prereq := range instance.Spec.Prerequisites {
		var prereqStack pulumiv1.Stack
		key := types.NamespacedName{Name: prereq.Name, Namespace: instance.Namespace}
		err := r.Client.Get(ctx, key, &prereqStack)
		if err != nil {
			prereqErr := fmt.Errorf("unable to fetch prerequisite %q: %w", prereq.Name, err)
			if apierrors.IsNotFound(err) {
				failedPrereqNames = append(failedPrereqNames, prereq.Name)
				failedPrereqErr = prereqErr
				continue
			}
			sess.logger.Error(prereqErr, "unable to fetch prerequisite", "name", prereq.Name)
			return reconcile.Result{}, fmt.Errorf("fetching prerequisite: %w", err)
		}

		// does the prerequisite stack satisfy the requirements given?
		requireErr := isRequirementSatisfied(prereq.Requirement, prereqStack)
		if requireErr != nil {
			failedPrereqNames = append(failedPrereqNames, prereq.Name)
			failedPrereqErr = fmt.Errorf("prerequisite not satisfied for %q: %w", prereq.Name, requireErr)
			// annotate the out of date stack so that it'll be queued. The value is arbitrary; this
			// value gives a bit of context which might be helpful when troubleshooting.
			v := fmt.Sprintf("update prerequisite of %s at %s", instance.Name, time.Now().Format(time.RFC3339))
			prereqStack1 := prereqStack.DeepCopy()
			a := prereqStack1.GetAnnotations()
			if a == nil {
				a = map[string]string{}
			}
			a[shared.ReconcileRequestAnnotation] = v
			prereqStack1.SetAnnotations(a)
			log.Info("requesting requeue of prerequisite", "name", prereqStack1.Name, "cause", requireErr.Error())
			if err := r.Client.Patch(ctx, prereqStack1, client.MergeFrom(&prereqStack)); err != nil {
				// A conflict here may mean the prerequisite has been changed, or it's just been
				// run. In any case, requeueing this object means we'll see the new state of the
				// world next time around.
				return reconcile.Result{}, fmt.Errorf("annotating prerequisite to force requeue: %w", err)
			}
		}
	}

	if len(failedPrereqNames) > 1 {
		failedPrereqErr = fmt.Errorf("multiple prerequisites were not satisfied %s", strings.Join(failedPrereqNames, ", "))
	}
	if failedPrereqErr != nil {
		instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingPrerequisiteNotSatisfiedReason, failedPrereqErr.Error())
		// Rely on the watcher watching prerequisites to requeue this, rather than requeuing
		// explicitly.
		return reconcile.Result{}, saveStatus()
	}

	// This value is reported in .status, and is set from some property of the source -- whether
	// it's the actual commit, or some analogue.
	var currentCommit string

	// Step 1. Set up the workspace, select the right stack and populate config if supplied.

	// Check which kind of source we have.

	switch {
	case !exactlyOneOf(stack.GitSource != nil, stack.FluxSource != nil, stack.ProgramRef != nil):
		err := errOtherThanOneSourceSpecified
		instance.Status.MarkStalledCondition(pulumiv1.StalledSpecInvalidReason, err.Error())
		return reconcile.Result{}, saveStatus()

	case stack.GitSource != nil:
		auth, err := sess.resolveGitAuth(ctx)
		if err != nil {
			r.emitEvent(instance, pulumiv1.StackConfigInvalidEvent(), err.Error())
			instance.Status.MarkStalledCondition(pulumiv1.StalledSpecInvalidReason, err.Error())
			return reconcile.Result{}, saveStatus()
		}

		gs, err := NewGitSource(*stack.GitSource, auth)
		if err != nil {
			r.emitEvent(instance, pulumiv1.StackConfigInvalidEvent(), err.Error())
			instance.Status.MarkStalledCondition(pulumiv1.StalledSpecInvalidReason, err.Error())
			return reconcile.Result{}, saveStatus()
		}

		currentCommit, err = gs.CurrentCommit(ctx)
		if err != nil {
			instance.Status.MarkStalledCondition(pulumiv1.StalledSourceUnavailableReason, err.Error())
			return reconcile.Result{}, saveStatus()
		}

		err = sess.setupWorkspaceFromGitSource(ctx, currentCommit)
		if err != nil {
			log.Error(err, "Failed to setup Pulumi workspace with git source")
			return reconcile.Result{}, err
		}

	case stack.FluxSource != nil:
		fluxSource := stack.FluxSource

		// Watch this kind of source, if we haven't already.
		if err := r.maybeWatchFluxSourceKind(fluxSource.SourceRef); err != nil {
			reterr := fmt.Errorf("cannot process source reference: %w", err)
			instance.Status.MarkStalledCondition(pulumiv1.StalledSpecInvalidReason, reterr.Error())
			return reconcile.Result{}, saveStatus()
		}

		var sourceObject unstructured.Unstructured
		sourceObject.SetAPIVersion(fluxSource.SourceRef.APIVersion)
		sourceObject.SetKind(fluxSource.SourceRef.Kind)
		if err := r.Client.Get(ctx, client.ObjectKey{
			Name:      fluxSource.SourceRef.Name,
			Namespace: request.Namespace,
		}, &sourceObject); err != nil {
			if apierrors.IsNotFound(err) {
				// this is marked as stalled and not requeued; the watch mechanism will requeue it if
				// the source it points to appears.
				reterr := fmt.Errorf("could not resolve sourceRef: %w", err)
				instance.Status.MarkStalledCondition(pulumiv1.StalledSourceUnavailableReason, reterr.Error())
				return reconcile.Result{}, saveStatus()
			}
			log.Error(err, "Failed to get Flux source", "Name", fluxSource.SourceRef.Name)
			return reconcile.Result{}, err
		}

		if !checkFluxSourceReady(&sourceObject) {
			// Wait until the source is ready, at which time the watch mechanism will requeue it.
			instance.Status.MarkStalledCondition(pulumiv1.StalledSourceUnavailableReason, "Flux source not ready")
			return reconcile.Result{}, saveStatus()
		}

		currentCommit, err = sess.SetupWorkspaceFromFluxSource(ctx, sourceObject, fluxSource)
		if err != nil {
			log.Error(err, "Failed to setup Pulumi workspace with flux source")
			return reconcile.Result{}, err
		}

	case stack.ProgramRef != nil:
		var program unstructured.Unstructured
		program.SetAPIVersion(pulumiv1.GroupVersion.String())
		program.SetKind("Program")
		if err := r.Client.Get(ctx, client.ObjectKey{
			Name:      stack.ProgramRef.Name,
			Namespace: request.Namespace,
		}, &program); err != nil {
			if apierrors.IsNotFound(err) {
				// this is marked as stalled and not requeued; the watch mechanism will requeue it if
				// the source it points to appears.
				instance.Status.MarkStalledCondition(pulumiv1.StalledSourceUnavailableReason, errProgramNotFound.Error())
				return reconcile.Result{}, saveStatus()
			}
			log.Error(err, "Failed to get Program object", "Name", stack.ProgramRef.Name)
			return reconcile.Result{}, err
		}

		// The Program.status is setup to mimic a FluxSource, so we can use the same function to
		// initiate the workspace.
		currentCommit, err = sess.SetupWorkspaceFromFluxSource(ctx, program, &shared.FluxSource{})
		if err != nil {
			log.Error(err, "Failed to setup Pulumi workspace")
			return reconcile.Result{}, err
		}
	}

	// Step 2. If there are extra environment variables, read them in now and use them for subsequent commands.
	err = sess.setupWorkspace(ctx)
	if err != nil {
		if errors.Is(err, errNamespaceIsolation) {
			instance.Status.MarkStalledCondition(pulumiv1.StalledCrossNamespaceRefForbiddenReason, err.Error())
			return reconcile.Result{}, saveStatus()
		}
		log.Error(err, "Failed to setup Pulumi workspace")
		return reconcile.Result{}, err
	}
	sess.SetEnvs(ctx, stack.Envs, request.Namespace)
	sess.SetSecretEnvs(ctx, stack.SecretEnvs, request.Namespace)

	// Step 3: Evaluate whether an update is needed. If not, we transition to Ready.
	if isSynced(instance, currentCommit) {
		// We don't mark the stack as read if its update failed so downstream
		// Stack dependencies aren't triggered.
		if instance.Status.LastUpdate.State == shared.SucceededStackStateMessage {
			instance.Status.MarkReadyCondition()
		} else {
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, fmt.Sprintf("%d update failure(s)", instance.Status.LastUpdate.Failures))
		}

		if isStackMarkedToBeDeleted {
			log.Info("Stack was destroyed; finalizing now.")
			_ = saveStatus()
			if controllerutil.RemoveFinalizer(instance, pulumiFinalizer) {
				return reconcile.Result{}, r.Update(ctx, instance, client.FieldOwner(FieldManager))
			}
			return reconcile.Result{}, nil
		}

		// Requeue reconciliation as necessary to detect branch updates and
		// resyncs.

		requeueAfter := time.Duration(0)

		if instance.Status.LastUpdate.State == shared.FailedStackStateMessage {
			requeueAfter = max(1*time.Second, time.Until(instance.Status.LastUpdate.LastResyncTime.Add(cooldown(instance))))
		}
		if sess.stack.ContinueResyncOnCommitMatch {
			requeueAfter = max(1*time.Second, time.Until(instance.Status.LastUpdate.LastResyncTime.Add(resyncFreq(instance))))
		}
		if stack.GitSource != nil {
			trackBranch := len(stack.GitSource.Branch) > 0
			if trackBranch {
				// Reconcile every resyncFreq to check for new commits to the branch.
				pollFreq := resyncFreq(instance)
				log.Info("Commit hash unchanged. Will poll for new commits.", "pollFrequency", pollFreq)
				requeueAfter = min(requeueAfter, pollFreq)
			} else {
				log.Info("Commit hash unchanged.")
			}
		} else if stack.FluxSource != nil {
			log.Info("Commit hash unchanged. Will wait for Source update or resync.")
		} else if stack.ProgramRef != nil {
			log.Info("Commit hash unchanged. Will wait for Program update or resync.")
		}

		return reconcile.Result{RequeueAfter: requeueAfter}, saveStatus()
	}

	if instance.Status.LastUpdate != nil && instance.Status.LastUpdate.LastSuccessfulCommit != currentCommit {
		r.emitEvent(instance, pulumiv1.StackUpdateDetectedEvent(), "New commit detected: %q.", currentCommit)
		log.Info("New commit hash found", "Current commit", currentCommit,
			"Last commit", instance.Status.LastUpdate.LastSuccessfulCommit)
	}

	// Step 4: Create or update the workspace in which to run an update.

	instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingProcessingReason, pulumiv1.ReconcilingProcessingWorkspaceMessage)

	if err := sess.CreateWorkspace(ctx); err != nil {
		log.Error(err, "cannot create workspace")
		return reconcile.Result{}, fmt.Errorf("unable to create workspace: %w", err)
	}

	if !isWorkspaceReady(sess.ws) {
		// watch the workspace for status updates
		log.V(1).Info("waiting for workspace to be ready")
		return reconcile.Result{}, saveStatus()
	}

	// Step 5: Create an Update object to run the update asynchronously

	instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingProcessingReason, pulumiv1.ReconcilingProcessingUpdateMessage)

	var update *autov1alpha1.Update
	if isStackMarkedToBeDeleted {
		update, err = sess.newDestroy(ctx, instance, "Stack Update (destroy)")
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to prepare update (destroy) for stack: %w", err)
		}
	} else {
		update, err = sess.newUp(ctx, instance, "Stack Update (up)")
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("unable to prepare update (up) for stack: %w", err)
		}
	}
	instance.Status.CurrentUpdate = &shared.CurrentStackUpdate{
		Generation: instance.Generation,
		Name:       update.Name,
		Commit:     currentCommit,
	}
	if err := saveStatus(); err != nil {
		// the status couldn't be updated, e.g. due to a conflct; try again later.
		return reconcile.Result{}, fmt.Errorf("unable to update the status: %w", err)
	}
	err = r.Create(ctx, update, client.FieldOwner(FieldManager))
	if err != nil {
		// the update object couldn't be created; remove the currentUpdate and try again later.
		log.Error(err, "failed to create an Update for the stack; will retry later")
		instance.Status.CurrentUpdate = nil
		_ = saveStatus()
		return reconcile.Result{}, fmt.Errorf("unable to create update for stack: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *StackReconciler) emitEvent(instance *pulumiv1.Stack, event pulumiv1.StackEvent, messageFmt string, args ...interface{}) {
	r.Recorder.Eventf(instance, event.EventType(), event.Reason(), messageFmt, args...)
}

// markStackFailed updates the status of the Stack object `instance` locally, to reflect a failure to process the stack.
func (r *StackReconciler) markStackFailed(sess *stackReconcilerSession, instance *pulumiv1.Stack, current *shared.CurrentStackUpdate, update *autov1alpha1.Update) {
	sess.logger.Info("Failed to update Stack", "Stack.Name", sess.stack.Stack, "Message", update.Status.Message)

	// Update Stack status with failed state
	last := instance.Status.LastUpdate
	instance.Status.LastUpdate = &shared.StackUpdateState{
		Generation:          current.Generation,
		Name:                update.Name,
		Type:                update.Spec.Type,
		State:               shared.FailedStackStateMessage,
		LastAttemptedCommit: current.Commit,
		Permalink:           shared.Permalink(update.Status.Permalink),
		LastResyncTime:      metav1.Now(),
	}
	if last != nil {
		instance.Status.LastUpdate.LastSuccessfulCommit = last.LastSuccessfulCommit
	}
	if last != nil && last.Generation == current.Generation {
		instance.Status.LastUpdate.Failures = last.Failures + 1
	}

	r.emitEvent(instance, pulumiv1.StackUpdateFailureEvent(), "Failed to update Stack: %s", update.Status.Message)
}

func (r *StackReconciler) markStackSucceeded(ctx context.Context, instance *pulumiv1.Stack, current *shared.CurrentStackUpdate, update *autov1alpha1.Update) error {
	// Step 5. Capture outputs onto the resulting status object.
	if update.Status.Outputs != "" {
		secret := corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{Namespace: update.Namespace, Name: update.Status.Outputs}, &secret)
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("getting output secret: %w", err)
		}
		outputs := shared.StackOutputs{}
		secrets := []string{}
		if annotation, ok := secret.GetAnnotations()[auto.SecretOutputsAnnotation]; ok {
			err := json.Unmarshal([]byte(annotation), &secrets)
			if err != nil {
				return fmt.Errorf("unmarshaling output mask: %w", err)
			}
		}
		for key, value := range secret.Data {
			if slices.Contains(secrets, key) {
				outputs[key] = apiextensionsv1.JSON{Raw: []byte(`"[secret]"`)}
			} else {
				outputs[key] = apiextensionsv1.JSON{Raw: json.RawMessage(value)}
			}
		}
		instance.Status.Outputs = outputs
	}

	instance.Status.LastUpdate = &shared.StackUpdateState{
		Generation:           current.Generation,
		Name:                 update.Name,
		Type:                 update.Spec.Type,
		State:                shared.SucceededStackStateMessage,
		LastAttemptedCommit:  current.Commit,
		LastSuccessfulCommit: current.Commit,
		Permalink:            shared.Permalink(update.Status.Permalink),
		LastResyncTime:       metav1.Now(),
		Failures:             0,
	}

	r.emitEvent(instance, pulumiv1.StackUpdateSuccessfulEvent(), "Successfully updated stack.")
	return nil
}

type stackReconcilerSession struct {
	logger     logr.Logger
	kubeClient client.Client
	scheme     *runtime.Scheme
	stack      shared.StackSpec
	namespace  string
	ws         *autov1alpha1.Workspace
	wss        *autov1alpha1.WorkspaceStack
	wspc       *corev1.Container
	update     *autov1alpha1.Update
}

func newStackReconcilerSession(
	logger logr.Logger,
	stack shared.StackSpec,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	namespace string,
) *stackReconcilerSession {
	return &stackReconcilerSession{
		logger:     logger,
		kubeClient: kubeClient,
		scheme:     scheme,
		stack:      stack,
		namespace:  namespace,
	}
}

// isSynced determines whether the stack is in sync with respect to the
// specification. i.e. the current spec generation has been applied, the update
// was successful, the latest commit has been applied, and (if resync is
// enabled) has been resynced recently.
func isSynced(stack *pulumiv1.Stack, currentCommit string) bool {
	if stack.Status.LastUpdate == nil {
		return false
	}

	if stack.Status.LastUpdate.Generation != stack.Generation {
		return false
	}

	if stack.Status.LastUpdate.State == shared.SucceededStackStateMessage {
		if stack.DeletionTimestamp != nil { // Marked for deletion.
			return true
		}
		if stack.Status.LastUpdate.LastSuccessfulCommit != currentCommit {
			return false
		}
		if !stack.Spec.ContinueResyncOnCommitMatch {
			return true
		}
		return time.Since(stack.Status.LastUpdate.LastResyncTime.Time) < resyncFreq(stack)
	}

	if stack.Status.LastUpdate.State == shared.FailedStackStateMessage {
		return time.Since(stack.Status.LastUpdate.LastResyncTime.Time) < cooldown(stack)
	}

	// We should never get here; if we do the Update is in an unknown state so
	// don't trigger work for it.
	return true
}

// cooldown returns the amount of time to wait before a failed Update should be
// retried. We start with a 1-minute cooldown and double that for each failed
// attempt, up to a max of 24 hours. Failed Updates are considered synced while
// inside this cooldown period. A zero-value duration is returned if the update
// succeeded.
func cooldown(stack *pulumiv1.Stack) time.Duration {
	cooldown := time.Duration(0)
	if stack.Status.LastUpdate == nil {
		return cooldown
	}
	if stack.Status.LastUpdate.State == shared.FailedStackStateMessage {
		cooldown = 1 * time.Minute
		cooldown *= time.Duration(math.Exp2(float64(stack.Status.LastUpdate.Failures)))
		cooldown = min(24*time.Hour, cooldown)
	}
	return cooldown
}

// resyncFreq determines how often a stack should be re-synced, for example to
// poll for new commits.
func resyncFreq(stack *pulumiv1.Stack) time.Duration {
	resyncFreq := time.Duration(stack.Spec.ResyncFrequencySeconds) * time.Second
	if resyncFreq.Seconds() < 60 {
		resyncFreq = time.Duration(60) * time.Second
	}
	return resyncFreq
}

// SetEnvs populates the environment the stack run with values
// from an array of Kubernetes ConfigMaps in a Namespace.
func (sess *stackReconcilerSession) SetEnvs(ctx context.Context, configMapNames []string, _ string) {
	for _, name := range configMapNames {
		sess.ws.Spec.EnvFrom = append(sess.ws.Spec.EnvFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
			},
		})
	}
}

// SetSecretEnvs populates the environment of the stack run with values
// from an array of Kubernetes Secrets in a Namespace.
func (sess *stackReconcilerSession) SetSecretEnvs(ctx context.Context, secretNames []string, _ string) {
	for _, name := range secretNames {
		sess.ws.Spec.EnvFrom = append(sess.ws.Spec.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
			},
		})
	}
}

// SetEnvRefsForWorkspace populates environment variables for workspace using items in
// the EnvRefs field in the stack specification.
func (sess *stackReconcilerSession) SetEnvRefsForWorkspace(ctx context.Context) error {
	envRefs := sess.stack.EnvRefs

	// envRefs is an unordered map, but we need to constrct env vars
	// deterministically to not thrash our underlying StatefulSet.
	keys := slices.Sorted(maps.Keys(envRefs))

	for _, key := range keys {
		ref := envRefs[key]

		value, valueFrom, err := sess.resolveResourceRefAsEnvVar(ctx, &ref)
		if err != nil {
			return fmt.Errorf("resolving env variable reference for %q: %w", key, err)
		}
		sess.ws.Spec.Env = append(sess.ws.Spec.Env, corev1.EnvVar{
			Name:      key,
			Value:     value,
			ValueFrom: valueFrom,
		})
	}
	return nil
}

func (sess *stackReconcilerSession) resolveResourceRefAsEnvVar(_ context.Context, ref *shared.ResourceRef) (string, *corev1.EnvVarSource, error) {
	switch ref.SelectorType {
	case shared.ResourceSelectorEnv:
		// DEPRECATED: this reads from the operator's own environment
		if ref.Env != nil {
			resolved := os.Getenv(ref.Env.Name)
			if resolved == "" {
				return "", nil, fmt.Errorf("missing value for environment variable: %s", ref.Env.Name)
			}
			return resolved, nil, nil
		}
		return "", nil, errors.New("missing env reference in ResourceRef")
	case shared.ResourceSelectorLiteral:
		if ref.LiteralRef != nil {
			return ref.LiteralRef.Value, nil, nil
		}
		return "", nil, errors.New("missing literal reference in ResourceRef")
	case shared.ResourceSelectorFS:
		// DEPRECATED: this reads from the operator's own filesystem
		if ref.FileSystem != nil {
			contents, err := os.ReadFile(ref.FileSystem.Path)
			if err != nil {
				return "", nil, fmt.Errorf("reading path %q: %w", ref.FileSystem.Path, err)
			}
			return string(contents), nil, nil
		}
		return "", nil, errors.New("Missing filesystem reference in ResourceRef")
	case shared.ResourceSelectorSecret:
		if ref.SecretRef != nil {
			// enforce namespace isolation
			if ref.SecretRef.Namespace != "" && ref.SecretRef.Namespace != sess.namespace {
				return "", nil, errNamespaceIsolation
			}
			return "", &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: ref.SecretRef.Name},
					Key:                  ref.SecretRef.Key,
				},
			}, nil
		}
		return "", nil, errors.New("Missing secret reference in ResourceRef")
	default:
		return "", nil, fmt.Errorf("Unsupported selector type: %v", ref.SelectorType)
	}
}

func makeSecretRefMountPath(secretRef *shared.SecretSelector) string {
	return "/var/run/secrets/stacks.pulumi.com/secrets/" + secretRef.Name
}

func (sess *stackReconcilerSession) resolveResourceRefAsConfigItem(ctx context.Context, ref *shared.ResourceRef) (*string, *autov1alpha1.ConfigValueFrom, error) {
	switch ref.SelectorType {
	case shared.ResourceSelectorEnv:
		// DEPRECATED: this reads from the operator's own environment
		if ref.Env != nil {
			resolved := os.Getenv(ref.Env.Name)
			if resolved == "" {
				return nil, nil, fmt.Errorf("missing value for environment variable: %s", ref.Env.Name)
			}
			return &resolved, nil, nil
		}
		return nil, nil, errors.New("missing env reference in ResourceRef")
	case shared.ResourceSelectorLiteral:
		if ref.LiteralRef != nil {
			return ptr.To(ref.LiteralRef.Value), nil, nil
		}
		return nil, nil, errors.New("missing literal reference in ResourceRef")
	case shared.ResourceSelectorFS:
		// DEPRECATED: this reads from the operator's own filesystem
		if ref.FileSystem != nil {
			contents, err := os.ReadFile(ref.FileSystem.Path)
			if err != nil {
				return nil, nil, fmt.Errorf("reading path %q: %w", ref.FileSystem.Path, err)
			}
			return ptr.To(string(contents)), nil, nil
		}
		return nil, nil, errors.New("Missing filesystem reference in ResourceRef")
	case shared.ResourceSelectorSecret:
		if ref.SecretRef != nil {
			// enforce namespace isolation
			if ref.SecretRef.Namespace != "" && ref.SecretRef.Namespace != sess.namespace {
				return nil, nil, errNamespaceIsolation
			}

			// mount the secret into the pulumi container
			volumeName := "secret-" + ref.SecretRef.Name
			mountPath := makeSecretRefMountPath(ref.SecretRef)
			if !slices.ContainsFunc(sess.wspc.VolumeMounts, func(mnt corev1.VolumeMount) bool {
				return mnt.Name == volumeName
			}) {
				sess.wspc.VolumeMounts = append(sess.wspc.VolumeMounts, corev1.VolumeMount{
					Name:      volumeName,
					MountPath: mountPath,
				})
				sess.ws.Spec.PodTemplate.Spec.Volumes = append(sess.ws.Spec.PodTemplate.Spec.Volumes, corev1.Volume{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: ref.SecretRef.Name,
						},
					},
				})
			}
			return nil, &autov1alpha1.ConfigValueFrom{
				Path: path.Join(mountPath, ref.SecretRef.Key),
			}, nil
		}
		return nil, nil, errors.New("Missing secret reference in ResourceRef")
	default:
		return nil, nil, fmt.Errorf("Unsupported selector type: %v", ref.SelectorType)
	}
}

// resolveSecretResourceRef reads a referenced object and returns its value as
// a string. The v1 controller allowed env and filesystem references which no
// longer make sense in the v2 agent/manager model, so only secret refs are
// currently supported.
func (sess *stackReconcilerSession) resolveSecretResourceRef(ctx context.Context, ref *shared.ResourceRef) (string, error) {
	switch ref.SelectorType {
	case shared.ResourceSelectorSecret:
		if ref.SecretRef == nil {
			return "", errors.New("missing secret reference in ResourceRef")
		}
		var config corev1.Secret
		namespace := ref.SecretRef.Namespace
		if namespace == "" {
			namespace = sess.namespace
		}
		if namespace != sess.namespace {
			return "", errNamespaceIsolation
		}

		if err := sess.kubeClient.Get(ctx, types.NamespacedName{Name: ref.SecretRef.Name, Namespace: namespace}, &config); err != nil {
			return "", fmt.Errorf("unable to get secret %s/%s: %w", namespace, ref.SecretRef.Name, err)
		}
		secretVal, ok := config.Data[ref.SecretRef.Key]
		if !ok {
			return "", fmt.Errorf("no key %q found in secret %s/%s", ref.SecretRef.Key, namespace, ref.SecretRef.Name)
		}
		return string(secretVal), nil
	default:
		return "", fmt.Errorf("%s selectors are no longer supported in v2, please use a secret reference instead", ref.SelectorType)
	}
}

func nameForWorkspace(stack *metav1.ObjectMeta) string {
	return stack.Name
}

func labelsForWorkspace(stack *metav1.ObjectMeta) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "pulumi",
		"app.kubernetes.io/component":  "stack",
		"app.kubernetes.io/instance":   stack.Name,
		"app.kubernetes.io/managed-by": "pulumi-kubernetes-operator",
	}
}

// NewWorkspace makes a new workspace for the given stack.
func (sess *stackReconcilerSession) NewWorkspace(stack *pulumiv1.Stack) error {
	labels := labelsForWorkspace(&stack.ObjectMeta)
	sess.ws = &autov1alpha1.Workspace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: autov1alpha1.GroupVersion.String(),
			Kind:       "Workspace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameForWorkspace(&stack.ObjectMeta),
			Namespace: sess.namespace,
			Labels:    labels,
		},
		Spec: autov1alpha1.WorkspaceSpec{
			PodTemplate: &autov1alpha1.EmbeddedPodTemplateSpec{
				Spec: &corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "pulumi",
						},
					},
				},
			},
		},
	}
	sess.wspc = &sess.ws.Spec.PodTemplate.Spec.Containers[0]
	if err := controllerutil.SetControllerReference(stack, sess.ws, sess.scheme); err != nil {
		return err
	}

	return nil
}

func (sess *stackReconcilerSession) CreateWorkspace(ctx context.Context) error {
	sess.ws.Spec.Stacks = append(sess.ws.Spec.Stacks, *sess.wss)

	if err := sess.kubeClient.Patch(ctx, sess.ws, client.Apply, client.FieldOwner(FieldManager)); err != nil {
		sess.logger.Error(err, "Failed to create workspace object")
		return err
	}
	return nil
}

// setupWorkspace sets all the extra configuration specified by the Stack object, after you have
// constructed a workspace from a source.
func (sess *stackReconcilerSession) setupWorkspace(ctx context.Context) error {
	w := sess.ws
	if sess.stack.Backend != "" {
		w.Spec.Env = append(w.Spec.Env, corev1.EnvVar{
			Name:  "PULUMI_BACKEND_URL",
			Value: sess.stack.Backend,
		})
	}
	if sess.stack.AccessTokenSecret != "" {
		w.Spec.Env = append(w.Spec.Env, corev1.EnvVar{
			Name: "PULUMI_ACCESS_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: sess.stack.AccessTokenSecret},
					Key:                  "accessToken",
				},
			},
		})
	}

	var err error
	if err = sess.SetEnvRefsForWorkspace(ctx); err != nil {
		return err
	}

	wss := &autov1alpha1.WorkspaceStack{
		Name:   sess.stack.Stack,
		Create: ptr.To(!sess.stack.UseLocalStackOnly),
	}
	// Prefer the secretsProvider in the stack config. To override an existing stack to the default
	// secret provider, the stack's secretsProvider field needs to be set to 'default'
	if sess.stack.SecretsProvider != "" {
		// We must always make sure the secret provider is initialized in the workspace
		// before we set any configs. Otherwise secret provider will mysteriously reset.
		// https://github.com/pulumi/pulumi-kubernetes-operator/issues/135
		wss.SecretsProvider = &sess.stack.SecretsProvider
	}

	sess.wss = wss
	sess.logger.V(1).Info("Setting workspace stack", "stack", wss)

	// Update the stack config and secret config values.
	err = sess.UpdateConfig(ctx)
	if err != nil {
		sess.logger.Error(err, "failed to set stack config", "Stack.Name", sess.stack.Stack)
		return fmt.Errorf("failed to set stack config: %w", err)
	}

	// Apply the user's workspace spec as a merge patch on top of what we've
	// already generated.
	if sess.stack.WorkspaceTemplate != nil {
		patched, err := patchObject(*sess.ws, *sess.stack.WorkspaceTemplate)
		if err != nil {
			return fmt.Errorf("patching workspace spec: %w", err)
		}
		sess.ws = patched
		for _, c := range sess.ws.Spec.PodTemplate.Spec.Containers {
			if c.Name == "pulumi" {
				sess.wspc = &c
				break
			}
		}
	}

	return nil
}

func (sess *stackReconcilerSession) UpdateConfig(ctx context.Context) error {
	ws := sess.wss

	// m := make(auto.ConfigMap)
	for k, v := range sess.stack.Config {
		ws.Config = append(ws.Config, autov1alpha1.ConfigItem{
			Key:    k,
			Value:  ptr.To(v),
			Secret: ptr.To(false),
		})
	}
	for k, v := range sess.stack.Secrets {
		ws.Config = append(ws.Config, autov1alpha1.ConfigItem{
			Key:    k,
			Value:  ptr.To(v),
			Secret: ptr.To(true),
		})
	}

	for k, ref := range sess.stack.SecretRefs {
		value, valueFrom, err := sess.resolveResourceRefAsConfigItem(ctx, &ref)
		if err != nil {
			return fmt.Errorf("updating secretRef for %q: %w", k, err)
		}
		ws.Config = append(ws.Config, autov1alpha1.ConfigItem{
			Key:       k,
			Value:     value,
			ValueFrom: valueFrom,
			Secret:    ptr.To(true),
		})
	}

	sess.logger.V(1).Info("Updated stack config", "Stack.Name", sess.stack.Stack, "config", ws.Config)
	return nil
}

// newUp runs `pulumi up` on the stack.
func (sess *stackReconcilerSession) newUp(ctx context.Context, o *pulumiv1.Stack, message string) (*autov1alpha1.Update, error) {
	update := &autov1alpha1.Update{
		TypeMeta: metav1.TypeMeta{
			APIVersion: autov1alpha1.GroupVersion.String(),
			Kind:       "Update",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      makeUpdateName(o),
			Namespace: sess.namespace,
		},
		Spec: autov1alpha1.UpdateSpec{
			WorkspaceName: sess.ws.Name,
			StackName:     sess.stack.Stack,
			Type:          autov1alpha1.UpType,
			Message:       ptr.To(message),
			// ExpectNoChanges:  ptr.To(o.Spec.ExpectNoRefreshChanges),
			Target:           o.Spec.Targets,
			TargetDependents: ptr.To(o.Spec.TargetDependents),
			Refresh:          ptr.To(o.Spec.Refresh),
		},
	}

	if err := controllerutil.SetControllerReference(o, update, sess.scheme); err != nil {
		return nil, err
	}

	return update, nil
}

// newUp runs `pulumi destroy` on the stack.
func (sess *stackReconcilerSession) newDestroy(ctx context.Context, o *pulumiv1.Stack, message string) (*autov1alpha1.Update, error) {
	update := &autov1alpha1.Update{
		TypeMeta: metav1.TypeMeta{
			APIVersion: autov1alpha1.GroupVersion.String(),
			Kind:       "Update",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      makeUpdateName(o),
			Namespace: sess.namespace,
		},
		Spec: autov1alpha1.UpdateSpec{
			WorkspaceName: sess.ws.Name,
			StackName:     sess.stack.Stack,
			Type:          autov1alpha1.DestroyType,
			Message:       ptr.To(message),
		},
	}

	if err := controllerutil.SetControllerReference(o, update, sess.scheme); err != nil {
		return nil, err
	}

	return update, nil
}

func makeUpdateName(o *pulumiv1.Stack) string {
	return fmt.Sprintf("%s-%s", o.Name, utilrand.String(8))
}

func (sess *stackReconcilerSession) readCurrentUpdate(ctx context.Context, name types.NamespacedName) error {
	u := &autov1alpha1.Update{}
	if err := sess.kubeClient.Get(ctx, name, u); err != nil {
		return err
	}
	sess.update = u
	return nil
}

func patchObject[T any, V any](base T, patch V) (*T, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON for base: %w", err)
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON for workspace template: %w", err)
	}

	// Calculate the patch result.
	var result T
	jsonResultBytes, err := strategicpatch.StrategicMergePatch(baseBytes, patchBytes, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to generate merge patch for workspace template: %w", err)
	}
	if err := json.Unmarshal(jsonResultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merged workspace template: %w", err)
	}

	return &result, nil
}

// finalizerAddedPredicate detects when a finalizer is added to an object.
// It is used to suppress reconciliation when the stack controller adds its finalizer, which causes
// a generation change that would otherwise trigger reconciliation.
type finalizerAddedPredicate struct{}

var _ predicate.Predicate = &finalizerAddedPredicate{}

func (p *finalizerAddedPredicate) Create(_ event.CreateEvent) bool {
	return false
}

func (p *finalizerAddedPredicate) Delete(_ event.DeleteEvent) bool {
	return false
}

func (p *finalizerAddedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}
	return !controllerutil.ContainsFinalizer(e.ObjectOld, pulumiFinalizer) && controllerutil.ContainsFinalizer(e.ObjectNew, pulumiFinalizer)
}

func (p *finalizerAddedPredicate) Generic(_ event.GenericEvent) bool {
	return false
}
