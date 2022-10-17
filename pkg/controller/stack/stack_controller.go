// Copyright 2021, Pulumi Corporation.  All rights reserved.

package stack

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/operator-framework/operator-lib/handler"
	libpredicate "github.com/operator-framework/operator-lib/predicate"
	"github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/shared"
	pulumiv1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1"
	"github.com/pulumi/pulumi-kubernetes-operator/pkg/logging"
	"github.com/pulumi/pulumi-kubernetes-operator/version"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	giturls "github.com/whilp/git-urls"
	git "gopkg.in/src-d/go-git.v4"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	yaml "sigs.k8s.io/yaml"
)

var (
	log       = logf.Log.WithName("controller_stack")
	execAgent = fmt.Sprintf("pulumi-kubernetes-operator/%s", version.Version)
)

const (
	pulumiFinalizer                = "finalizer.stack.pulumi.com"
	defaultMaxConcurrentReconciles = 10
)

const (
	// envInsecureNoNamespaceIsolation is the name of the environment entry which, when set to a
	// truthy value (1|true), shall allow multiple namespaces to be watched, and cross-namespace
	// references to be accepted.
	EnvInsecureNoNamespaceIsolation = "INSECURE_NO_NAMESPACE_ISOLATION"
)

func IsNamespaceIsolationWaived() bool {
	switch os.Getenv(EnvInsecureNoNamespaceIsolation) {
	case "1", "true":
		return true
	default:
		return false
	}
}

// Add creates a new Stack Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	// Use the ServiceAccount CA cert and token to setup $HOME/.kube/config.
	// This is used to deploy Pulumi Stacks of k8s resources
	// in-cluster that use the default, ambient kubeconfig.
	if err := setupInClusterKubeconfig(); err != nil {
		log.Error(err, "skipping in-cluster kubeconfig setup due to non-existent ServiceAccount")
	}
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileStack{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor("stack-controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	var err error
	maxConcurrentReconciles := defaultMaxConcurrentReconciles
	maxConcurrentReconcilesStr, set := os.LookupEnv("MAX_CONCURRENT_RECONCILES")
	if set {
		maxConcurrentReconciles, err = strconv.Atoi(maxConcurrentReconcilesStr)
		if err != nil {
			return err
		}
	}

	// Create a new controller
	c, err := controller.New("stack-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: maxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Filter out update events if an object's metadata.generation is
	// unchanged, or if the object never had a generation update.
	//  - https://github.com/operator-framework/operator-lib/blob/main/predicate/nogeneration.go#L29-L34
	//  - https://github.com/operator-framework/operator-sdk/issues/2795
	//  - https://github.com/kubernetes-sigs/kubebuilder/issues/1103
	//  - https://github.com/kubernetes-sigs/controller-runtime/pull/553
	//  - https://book-v1.book.kubebuilder.io/basics/status_subresource.html
	// Set up predicates.
	predicates := []predicate.Predicate{
		predicate.Or(predicate.GenerationChangedPredicate{}, libpredicate.NoGenerationPredicate{}),
	}

	stackInformer, err := mgr.GetCache().GetInformer(context.Background(), &pulumiv1.Stack{})
	if err != nil {
		return err
	}
	stackInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    newStackCallback,
		UpdateFunc: updateStackCallback,
		DeleteFunc: deleteStackCallback,
	})

	// Watch for changes to primary resource Stack
	err = c.Watch(&source.Kind{Type: &pulumiv1.Stack{}}, &handler.InstrumentedEnqueueRequestForObject{}, predicates...)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileStack implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileStack{}

// ReconcileStack reconciles a Stack object
type ReconcileStack struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
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

var errNamespaceIsolation = newStallErrorf(`refs are constrained to the object's namespace unless %s is set`, EnvInsecureNoNamespaceIsolation)
var errOtherThanOneSourceSpecified = newStallErrorf(`exactly one source (.spec.fluxSource, .spec.projectRepo, or .spec.programRef) for the stack must be given`)

var errProgramNotFound = fmt.Errorf("unable to retrieve program for stack")

// Reconcile reads that state of the cluster for a Stack object and makes changes based on the state read
// and what is in the Stack.Spec
func (r *ReconcileStack) Reconcile(ctx context.Context, request reconcile.Request) (retres reconcile.Result, reterr error) {
	reqLogger := logging.WithValues(log, "Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Stack")

	// Fetch the Stack instance
	instance := &pulumiv1.Stack{}
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Stack resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Deletion/finalization protocol: Usually
	// (https://book.kubebuilder.io/reference/using-finalizers.html) you would add a finalizer when
	// you first see an object; and, when an object is being deleted, do clean up and exit instead
	// of continuing on to process it. For this controller, clean-up may need some preparation
	// (e.g., fetching the git repo), and there is a risk that finalization cannot be completed if
	// that preparation fails (e.g., the git repo doesn't exist). To mitigate this, an object isn't
	// given a finalizer until preparation has succeeded once (see under Step 2).

	// Check if the Stack instance is marked to be deleted, which is indicated by the deletion
	// timestamp being set.
	isStackMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	// If there's no finalizer, it's either been cleaned up, never been seen, or never gotten far
	// enough to need cleaning up.
	if isStackMarkedToBeDeleted && !contains(instance.GetFinalizers(), pulumiFinalizer) {
		return reconcile.Result{}, nil
	}

	// This helper helps with updates, from here onwards.
	stack := instance.Spec
	sess := newReconcileStackSession(reqLogger, stack, r.client, request.Namespace)

	// We can exit early if there is no clean-up to do.
	if isStackMarkedToBeDeleted && !stack.DestroyOnFinalize {
		// We know `!(isStackMarkedToBeDeleted && !contains(finalizer))` from above, and now
		// `isStackMarkedToBeDeleted`, implying `contains(finalizer)`; but this would be correct
		// even if it's a no-op.
		err := sess.removeFinalizerAndUpdate(ctx, instance)
		if err != nil {
			sess.logger.Error(err, "Failed to delete Pulumi finalizer", "Stack.Name", instance.Spec.Stack)
		}
		return reconcile.Result{}, err
	}

	// This makes sure the status reflects the outcome of reconcilation. Any non-error return means
	// the object definition was observed, whether the object ended up in a ready state or not. An
	// error return (now we have successfully fetched the object) means it is "in progress" and not
	// ready.
	saveStatus := func() {
		if reterr == nil {
			instance.Status.ObservedGeneration = instance.GetGeneration()
		} else {
			// controller-runtime will requeue the object, so reflect that in the conditions by
			// saying it is still in progress.
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, reterr.Error())
		}
		if err := sess.patchStatus(ctx, instance); err != nil {
			log.Error(err, "unable to save object status")
		}
	}
	defer saveStatus()

	// We're ready to do some actual work. Until we have a definitive outcome, mark the stack as
	// reconciling.
	instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingProcessingReason, pulumiv1.ReconcilingProcessingMessage)
	if err = sess.patchStatus(ctx, instance); err != nil {
		return reconcile.Result{}, err
	}

	// This value is reported in .status, and is set from some property of the source -- whether
	// it's the actual commit, or some analogue.
	var currentCommit string

	// Step 1. Set up the workdir, select the right stack and populate config if supplied.

	exactlyOneOf := func(these ...bool) bool {
		var found bool
		for _, b := range these {
			if found && b {
				return false
			}
			found = found || b
		}
		return found
	}

	// Check which kind of source we have.

	switch {
	case !exactlyOneOf(stack.GitSource != nil, stack.FluxSource != nil, stack.ProgramRef != nil):
		err := errOtherThanOneSourceSpecified
		r.markStackFailed(sess, instance, err, "", "")
		instance.Status.MarkStalledCondition(pulumiv1.StalledSpecInvalidReason, err.Error())
		return reconcile.Result{}, nil

	case stack.GitSource != nil:
		gitSource := stack.GitSource
		// Validate that there is enough specified to be able to clone the git repo.
		if gitSource.ProjectRepo == "" || (gitSource.Commit == "" && gitSource.Branch == "") {

			msg := "Stack git source needs to specify 'projectRepo' and either 'branch' or 'commit'"
			r.emitEvent(instance, pulumiv1.StackConfigInvalidEvent(), msg)
			reqLogger.Info(msg)
			r.markStackFailed(sess, instance, errors.New(msg), "", "")
			instance.Status.MarkStalledCondition(pulumiv1.StalledSpecInvalidReason, msg)
			// this object won't be processable until the spec is changed, so no reason to requeue
			// explicitly
			return reconcile.Result{}, nil
		}

		gitAuth, err := sess.SetupGitAuth(ctx) // TODO be more explicit about what's being fed in here
		if err != nil {
			r.emitEvent(instance, pulumiv1.StackGitAuthFailureEvent(), "Failed to setup git authentication: %v", err.Error())
			reqLogger.Error(err, "Failed to setup git authentication", "Stack.Name", stack.Stack)
			r.markStackFailed(sess, instance, err, "", "")
			instance.Status.MarkStalledCondition(pulumiv1.StalledSourceUnavailableReason, err.Error())
			return reconcile.Result{}, nil
		}

		if gitAuth.SSHPrivateKey != "" {
			// Add the project repo's public SSH keys to the SSH known hosts
			// to perform the necessary key checking during SSH git cloning.
			sess.addSSHKeysToKnownHosts(sess.stack.ProjectRepo)
		}

		if currentCommit, err = sess.SetupWorkdirFromGitSource(ctx, gitAuth, gitSource); err != nil {
			r.emitEvent(instance, pulumiv1.StackInitializationFailureEvent(), "Failed to initialize stack: %v", err.Error())
			reqLogger.Error(err, "Failed to setup Pulumi workdir", "Stack.Name", stack.Stack)
			r.markStackFailed(sess, instance, err, "", "")
			if isStalledError(err) {
				instance.Status.MarkStalledCondition(pulumiv1.StalledCrossNamespaceRefForbiddenReason, err.Error())
				return reconcile.Result{}, nil
			}
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
			// this can fail for reasons which might go away without intervention; so, retry explicitly
			return reconcile.Result{Requeue: true}, nil
		}

	case stack.FluxSource != nil:
		fluxSource := stack.FluxSource
		var sourceObject unstructured.Unstructured
		sourceObject.SetAPIVersion(fluxSource.SourceRef.APIVersion)
		sourceObject.SetKind(fluxSource.SourceRef.Kind)
		if err := r.client.Get(ctx, client.ObjectKey{
			Name:      fluxSource.SourceRef.Name,
			Namespace: request.Namespace,
		}, &sourceObject); err != nil {
			r.markStackFailed(sess, instance, err, "", "")
			if client.IgnoreNotFound(err) != nil {
				return reconcile.Result{}, fmt.Errorf("could not resolve sourceRef: %w", err)
			}
			// TODO: revisit this, if sources are watched; perhaps it should be stalled?
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
			return reconcile.Result{Requeue: true}, nil
		}

		if err := checkFluxSourceReady(sourceObject); err != nil {
			r.markStackFailed(sess, instance, err, "", "")
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
			return reconcile.Result{Requeue: true}, nil
		}

		currentCommit, err = sess.SetupWorkdirFromFluxSource(ctx, sourceObject, fluxSource)
		if err != nil {
			r.emitEvent(instance, pulumiv1.StackInitializationFailureEvent(), "Failed to initialize stack: %v", err.Error())
			reqLogger.Error(err, "Failed to setup Pulumi workdir", "Stack.Name", stack.Stack)
			r.markStackFailed(sess, instance, err, "", "")
			if isStalledError(err) {
				instance.Status.MarkStalledCondition(pulumiv1.StalledCrossNamespaceRefForbiddenReason, err.Error())
				return reconcile.Result{}, nil
			}
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
			// this can fail for reasons which might go away without intervention; so, retry explicitly
			return reconcile.Result{Requeue: true}, nil
		}

	case stack.ProgramRef != nil:
		programRef := stack.ProgramRef
		if currentCommit, err = sess.SetupWorkdirFromYAML(ctx, *programRef); err != nil {
			r.emitEvent(instance, pulumiv1.StackInitializationFailureEvent(), "Failed to initialize stack: %v", err.Error())
			reqLogger.Error(err, "Failed to setup Pulumi workdir", "Stack.Name", stack.Stack)
			r.markStackFailed(sess, instance, err, "", "")
			if errors.Is(err, errProgramNotFound) {
				instance.Status.MarkStalledCondition(pulumiv1.StalledSourceUnavailableReason, err.Error())
				return reconcile.Result{}, nil
			}
			if isStalledError(err) {
				instance.Status.MarkStalledCondition(pulumiv1.StalledSpecInvalidReason, err.Error())
				return reconcile.Result{}, nil
			}
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
			// this can fail for reasons which might go away without intervention; so, retry explicitly
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// Delete the temporary directory after the reconciliation is completed (regardless of success or failure).
	defer sess.CleanupPulumiDir()

	// Step 2. If there are extra environment variables, read them in now and use them for subsequent commands.
	if err = sess.SetEnvs(ctx, stack.Envs, request.Namespace); err != nil {
		err := fmt.Errorf("could not find ConfigMap for Envs: %w", err)
		r.markStackFailed(sess, instance, err, currentCommit, "")
		instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
		return reconcile.Result{Requeue: true}, nil
	}
	if err = sess.SetSecretEnvs(ctx, stack.SecretEnvs, request.Namespace); err != nil {
		err := fmt.Errorf("could not find Secret for SecretEnvs: %w", err)
		r.markStackFailed(sess, instance, err, currentCommit, "")
		instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
		return reconcile.Result{Requeue: true}, nil
	}

	// This is enough preparation to be able to destroy the stack, if it's being deleted, or to
	// consider it destroyable, if not.

	if isStackMarkedToBeDeleted {
		if contains(instance.GetFinalizers(), pulumiFinalizer) {
			err := sess.finalize(ctx, instance)
			// Manage extra status here
			return reconcile.Result{}, err
		}
	} else {
		if !contains(instance.GetFinalizers(), pulumiFinalizer) {
			// Add finalizer to Stack if not being deleted
			err := sess.addFinalizerAndUpdate(ctx, instance)
			if err != nil {
				return reconcile.Result{}, err
			}
			time.Sleep(2 * time.Second) // arbitrary sleep after finalizer add to avoid stale obj for permalink
			// Add default permalink for the stack in the Pulumi Service.
			if err := sess.addDefaultPermalink(ctx, instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Proceed/Requeue logic: this depends on the kind of source, but broadly:
	// - if the fetched revision is the same as the last one, proceed only if
	//   `ContinueResyncOnCommitMatch`
	// - if not proceeding, requeue in ResyncFrequencySeconds (sic)

	// requeueForSourcePoll keeps track of whether this object will need to be requeued for the
	// purpose of polling its source.
	requeueForSourcePoll := true
	resyncFreqSeconds := sess.stack.ResyncFrequencySeconds
	if sess.stack.ResyncFrequencySeconds != 0 && sess.stack.ResyncFrequencySeconds < 60 {
		resyncFreqSeconds = 60
	}

	if stack.GitSource != nil {
		trackBranch := len(stack.GitSource.Branch) > 0
		// this object won't need to be requeued later if it's not tracking a branch
		requeueForSourcePoll = trackBranch

		// when tracking a branch, rather than an exact commit, always requeue
		if trackBranch || sess.stack.ContinueResyncOnCommitMatch {
			if resyncFreqSeconds == 0 {
				resyncFreqSeconds = 60
			}
		}

		if trackBranch && instance.Status.LastUpdate != nil {
			reqLogger.Info("Checking current HEAD commit hash", "Current commit", currentCommit)
			if instance.Status.LastUpdate.LastSuccessfulCommit == currentCommit && !sess.stack.ContinueResyncOnCommitMatch {
				reqLogger.Info("Commit hash unchanged. Will poll again.", "pollFrequencySeconds", resyncFreqSeconds)
				// Reconcile every resyncFreqSeconds to check for new commits to the branch.
				instance.Status.MarkReadyCondition() // FIXME: should this reflect the previous update state?
				return reconcile.Result{RequeueAfter: time.Duration(resyncFreqSeconds) * time.Second}, nil
			}

			if instance.Status.LastUpdate.LastSuccessfulCommit != currentCommit {
				r.emitEvent(instance, pulumiv1.StackUpdateDetectedEvent(), "New commit detected: %q.", currentCommit)
				reqLogger.Info("New commit hash found", "Current commit", currentCommit,
					"Last commit", instance.Status.LastUpdate.LastSuccessfulCommit)
			}
		}

	} else if stack.FluxSource != nil {
		if instance.Status.LastUpdate != nil {
			if instance.Status.LastUpdate.LastSuccessfulCommit == currentCommit && !stack.ContinueResyncOnCommitMatch {
				reqLogger.Info("Commit hash unchanged. Will poll again.", "pollFrequencySeconds", resyncFreqSeconds)
				// Reconcile every resyncFreqSeconds to check for new commits to the branch.
				instance.Status.MarkReadyCondition() // FIXME: should this reflect the previous update state?
				return reconcile.Result{RequeueAfter: time.Duration(resyncFreqSeconds) * time.Second}, nil
			}

			if instance.Status.LastUpdate.LastSuccessfulCommit != currentCommit {
				r.emitEvent(instance, pulumiv1.StackUpdateDetectedEvent(), "New commit detected: %q.", currentCommit)
				reqLogger.Info("New commit hash found", "Current commit", currentCommit,
					"Last commit", instance.Status.LastUpdate.LastSuccessfulCommit)
			}
		}
	} else if stack.ProgramRef != nil {
		if instance.Status.LastUpdate != nil {
			if instance.Status.LastUpdate.LastSuccessfulCommit == currentCommit && !stack.ContinueResyncOnCommitMatch {
				reqLogger.Info("Commit hash unchanged. Will poll again.", "pollFrequencySeconds", resyncFreqSeconds)
				// Reconcile every resyncFreqSeconds to check for new commits to the branch.
				instance.Status.MarkReadyCondition() // FIXME: should this reflect the previous update state?
				return reconcile.Result{RequeueAfter: time.Duration(resyncFreqSeconds) * time.Second}, nil
			}

			if instance.Status.LastUpdate.LastSuccessfulCommit != currentCommit {
				r.emitEvent(instance, pulumiv1.StackUpdateDetectedEvent(), "New commit detected: %q.", currentCommit)
				reqLogger.Info("New commit hash found", "Current commit", currentCommit,
					"Last commit", instance.Status.LastUpdate.LastSuccessfulCommit)
			}
		}
	}

	// Step 3. If a stack refresh is requested, run it now.
	if sess.stack.Refresh {
		permalink, err := sess.RefreshStack(ctx, sess.stack.ExpectNoRefreshChanges)
		if err != nil {
			r.markStackFailed(sess, instance, fmt.Errorf("refreshing stack: %w", err), currentCommit, permalink)
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
			return reconcile.Result{Requeue: true}, nil
		}
		if instance.Status.LastUpdate == nil {
			instance.Status.LastUpdate = &shared.StackUpdateState{}
		}
		instance.Status.LastUpdate.Permalink = permalink

		err = sess.patchStatus(ctx, instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update Stack status for refresh", "Stack.Name", stack.Stack)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Successfully refreshed Stack", "Stack.Name", stack.Stack)
	}

	// Step 4. Run a `pulumi up --skip-preview`.
	// TODO: is it possible to support a --dry-run with a preview?
	status, permalink, result, err := sess.UpdateStack(ctx)
	switch status {
	case shared.StackUpdateConflict:
		r.emitEvent(instance,
			pulumiv1.StackUpdateConflictDetectedEvent(),
			"Conflict with another concurrent update. "+
				"If Stack CR specifies 'retryOnUpdateConflict' a retry will trigger automatically.")
		if sess.stack.RetryOnUpdateConflict {
			reqLogger.Error(err, "Conflict with another concurrent update -- will retry shortly", "Stack.Name", stack.Stack)
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, "conflict with concurrent update, retryOnUpdateConflict set")
			return reconcile.Result{RequeueAfter: time.Second * 5}, nil
		}
		reqLogger.Error(err, "Conflict with another concurrent update -- NOT retrying", "Stack.Name", stack.Stack)
		instance.Status.MarkStalledCondition(pulumiv1.StalledConflictReason, "conflict with concurrent update, retryOnUpdateConflict not set")
		return reconcile.Result{}, nil
	case shared.StackNotFound:
		r.emitEvent(instance, pulumiv1.StackNotFoundEvent(), "Stack not found. Will retry.")
		reqLogger.Error(err, "Stack not found -- will retry shortly", "Stack.Name", stack.Stack, "Err:")
		instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, "stack not found in backend; retrying")
		return reconcile.Result{RequeueAfter: time.Second * 5}, nil
	default:
		if err != nil {
			r.markStackFailed(sess, instance, err, currentCommit, permalink)
			instance.Status.MarkReconcilingCondition(pulumiv1.ReconcilingRetryReason, err.Error())
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// At this point, the stack has been processed successfully. Mark it as ready, and rely on the
	// post-return hook `saveStatus` to account for any last minute exceptions.
	instance.Status.MarkReadyCondition()

	// Step 5. Capture outputs onto the resulting status object.
	outs, err := sess.GetStackOutputs(result.Outputs)
	if err != nil {
		r.emitEvent(instance, pulumiv1.StackOutputRetrievalFailureEvent(), "Failed to get Stack outputs: %v.", err.Error())
		reqLogger.Error(err, "Failed to get Stack outputs", "Stack.Name", stack.Stack)
		return reconcile.Result{}, err
	}
	if outs == nil {
		reqLogger.Info("Stack outputs are empty. Skipping status update", "Stack.Name", stack.Stack)
		return reconcile.Result{}, nil
	}

	instance.Status.Outputs = outs
	instance.Status.LastUpdate = &shared.StackUpdateState{
		State:                shared.SucceededStackStateMessage,
		LastAttemptedCommit:  currentCommit,
		LastSuccessfulCommit: currentCommit,
		Permalink:            permalink,
		LastResyncTime:       metav1.Now(),
	}

	r.emitEvent(instance, pulumiv1.StackUpdateSuccessfulEvent(), "Successfully updated stack.")
	if requeueForSourcePoll || sess.stack.ContinueResyncOnCommitMatch {
		// Reconcile every 60 seconds to check for new commits to the branch.
		reqLogger.Debug("Will requeue in", "seconds", resyncFreqSeconds)
		return reconcile.Result{RequeueAfter: time.Duration(resyncFreqSeconds) * time.Second}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileStack) emitEvent(instance *pulumiv1.Stack, event pulumiv1.StackEvent, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(instance, event.EventType(), event.Reason(), messageFmt, args...)
}

// markStackFailed updates the status of the Stack object `instance` locally, to reflect a failure to process the stack.
func (r *ReconcileStack) markStackFailed(sess *reconcileStackSession, instance *pulumiv1.Stack, err error, currentCommit string, permalink shared.Permalink) {
	r.emitEvent(instance, pulumiv1.StackUpdateFailureEvent(), "Failed to update Stack: %v.", err.Error())
	sess.logger.Error(err, "Failed to update Stack", "Stack.Name", sess.stack.Stack)
	// Update Stack status with failed state
	if instance.Status.LastUpdate == nil {
		instance.Status.LastUpdate = &shared.StackUpdateState{}
	}
	instance.Status.LastUpdate.LastAttemptedCommit = currentCommit
	instance.Status.LastUpdate.State = shared.FailedStackStateMessage
	instance.Status.LastUpdate.Permalink = permalink
	instance.Status.LastUpdate.LastResyncTime = metav1.Now()
}

func (sess *reconcileStackSession) finalize(ctx context.Context, stack *pulumiv1.Stack) error {
	sess.logger.Info("Finalizing the stack")
	// Run finalization logic for pulumiFinalizer. If the
	// finalization logic fails, don't remove the finalizer so
	// that we can retry during the next reconciliation.
	if err := sess.finalizeStack(ctx); err != nil {
		sess.logger.Error(err, "Failed to run Pulumi finalizer", "Stack.Name", stack.Spec.Stack)
		return err
	}

	return sess.removeFinalizerAndUpdate(ctx, stack)
}

// removeFinalizerAndUpdate makes sure this controller's finalizer is not present in the instance
// given, and updates the object with the Kubernetes API client. It will retry if there is a
// conflict, which is a possibility since other processes may be removing finalizers at the same
// time.
func (sess *reconcileStackSession) removeFinalizerAndUpdate(ctx context.Context, instance *pulumiv1.Stack) error {
	key := client.ObjectKeyFromObject(instance)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var stack pulumiv1.Stack
		if err := sess.kubeClient.Get(ctx, key, &stack); err != nil {
			return err
		}
		// TODO: in more recent controller-runtime, the following returns a bool, and we could avoid
		// the update if not necessary.
		controllerutil.RemoveFinalizer(&stack, pulumiFinalizer)
		return sess.kubeClient.Update(ctx, &stack)
	})
}

func (sess *reconcileStackSession) finalizeStack(ctx context.Context) error {
	// Destroy the stack resources and stack.
	if sess.stack.DestroyOnFinalize {
		if err := sess.DestroyStack(ctx); err != nil {
			return err
		}
	}
	sess.logger.Info("Successfully finalized stack")
	return nil
}

// addFinalizerAndUpdate adds this controller's finalizer to the given object, and updates it with
// the Kubernetes API client. The update is retried, in case e.g., somebody else has updated the
// spec.
func (sess *reconcileStackSession) addFinalizerAndUpdate(ctx context.Context, stack *pulumiv1.Stack) error {
	sess.logger.Debug("Adding Finalizer for the Stack", "Stack.Name", stack.Name)
	key := client.ObjectKeyFromObject(stack)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var stack pulumiv1.Stack
		if err := sess.kubeClient.Get(ctx, key, &stack); err != nil {
			return err
		}
		controllerutil.AddFinalizer(&stack, pulumiFinalizer)
		return sess.kubeClient.Update(ctx, &stack)
	})
}

type reconcileStackSession struct {
	logger     logging.Logger
	kubeClient client.Client
	stack      shared.StackSpec
	autoStack  *auto.Stack
	namespace  string
	workdir    string
	rootDir    string
}

func newReconcileStackSession(
	logger logging.Logger,
	stack shared.StackSpec,
	kubeClient client.Client,
	namespace string,
) *reconcileStackSession {
	return &reconcileStackSession{
		logger:     logger,
		kubeClient: kubeClient,
		stack:      stack,
		namespace:  namespace,
	}
}

// SetEnvs populates the environment the stack run with values
// from an array of Kubernetes ConfigMaps in a Namespace.
func (sess *reconcileStackSession) SetEnvs(ctx context.Context, configMapNames []string, namespace string) error {
	for _, env := range configMapNames {
		var config corev1.ConfigMap
		if err := sess.kubeClient.Get(ctx, types.NamespacedName{Name: env, Namespace: namespace}, &config); err != nil {
			return fmt.Errorf("Namespace=%s Name=%s: %w", namespace, env, err)
		}
		if err := sess.autoStack.Workspace().SetEnvVars(config.Data); err != nil {
			return fmt.Errorf("Namespace=%s Name=%s: %w", namespace, env, err)
		}
	}
	return nil
}

// SetSecretEnvs populates the environment of the stack run with values
// from an array of Kubernetes Secrets in a Namespace.
func (sess *reconcileStackSession) SetSecretEnvs(ctx context.Context, secrets []string, namespace string) error {
	for _, env := range secrets {
		var config corev1.Secret
		if err := sess.kubeClient.Get(ctx, types.NamespacedName{Name: env, Namespace: namespace}, &config); err != nil {
			return fmt.Errorf("Namespace=%s Name=%s: %w", namespace, env, err)
		}
		envvars := map[string]string{}
		for k, v := range config.Data {
			envvars[k] = string(v)
		}
		if err := sess.autoStack.Workspace().SetEnvVars(envvars); err != nil {
			return fmt.Errorf("Namespace=%s Name=%s: %w", namespace, env, err)
		}
	}
	return nil
}

// SetEnvRefsForWorkspace populates environment variables for workspace using items in
// the EnvRefs field in the stack specification.
func (sess *reconcileStackSession) SetEnvRefsForWorkspace(ctx context.Context, w auto.Workspace) error {
	envRefs := sess.stack.EnvRefs
	for envVar, ref := range envRefs {
		val, err := sess.resolveResourceRef(ctx, &ref)
		if err != nil {
			return fmt.Errorf("resolving env variable reference for %q: %w", envVar, err)
		}
		w.SetEnvVar(envVar, val)
	}
	return nil
}

func (sess *reconcileStackSession) resolveResourceRef(ctx context.Context, ref *shared.ResourceRef) (string, error) {
	switch ref.SelectorType {
	case shared.ResourceSelectorEnv:
		if ref.Env != nil {
			resolved := os.Getenv(ref.Env.Name)
			if resolved == "" {
				return "", fmt.Errorf("missing value for environment variable: %s", ref.Env.Name)
			}
			return resolved, nil
		}
		return "", errors.New("missing env reference in ResourceRef")
	case shared.ResourceSelectorLiteral:
		if ref.LiteralRef != nil {
			return ref.LiteralRef.Value, nil
		}
		return "", errors.New("missing literal reference in ResourceRef")
	case shared.ResourceSelectorFS:
		if ref.FileSystem != nil {
			contents, err := os.ReadFile(ref.FileSystem.Path)
			if err != nil {
				return "", fmt.Errorf("reading path %q: %w", ref.FileSystem.Path, err)
			}
			return string(contents), nil
		}
		return "", errors.New("Missing filesystem reference in ResourceRef")
	case shared.ResourceSelectorSecret:
		if ref.SecretRef != nil {
			var config corev1.Secret
			namespace := ref.SecretRef.Namespace
			if namespace == "" {
				namespace = sess.namespace
			}
			// enforce namespace isolation unless it's explicitly been waived
			if !IsNamespaceIsolationWaived() && namespace != sess.namespace {
				return "", errNamespaceIsolation
			}

			if err := sess.kubeClient.Get(ctx, types.NamespacedName{Name: ref.SecretRef.Name, Namespace: namespace}, &config); err != nil {
				return "", fmt.Errorf("Namespace=%s Name=%s: %w", ref.SecretRef.Namespace, ref.SecretRef.Name, err)
			}
			secretVal, ok := config.Data[ref.SecretRef.Key]
			if !ok {
				return "", fmt.Errorf("No key %s found in secret %s/%s", ref.SecretRef.Key, ref.SecretRef.Namespace, ref.SecretRef.Name)
			}
			return string(secretVal), nil
		}
		return "", errors.New("Mising secret reference in ResourceRef")
	default:
		return "", fmt.Errorf("Unsupported selector type: %v", ref.SelectorType)
	}
}

// runCmd runs the given command with stdout and stderr hooked up to the logger.
func (sess *reconcileStackSession) runCmd(title string, cmd *exec.Cmd, workspace auto.Workspace) (string, string, error) {
	// If not overridden, set the command to run in the working directory.
	if cmd.Dir == "" {
		cmd.Dir = workspace.WorkDir()
	}

	// Init environment variables.
	if len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	}
	// If there are extra environment variables, set them.
	if workspace != nil {
		for k, v := range workspace.GetEnvVars() {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Capture stdout and stderr.
	stdoutR, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	stderrR, err := cmd.StderrPipe()
	if err != nil {
		return "", "", err
	}

	// Start the command asynchronously.
	err = cmd.Start()
	if err != nil {
		return "", "", err
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// We want to echo both stderr and stdout as they are written; so at least one of them must be
	// in another goroutine.
	stderrClosed := make(chan struct{})
	errs := bufio.NewScanner(stderrR)
	go func() {
		for errs.Scan() {
			text := errs.Text()
			sess.logger.Debug(title, "Dir", cmd.Dir, "Path", cmd.Path, "Args", cmd.Args, "Text", text)
			stderr.WriteString(text + "\n")
		}
		close(stderrClosed)
	}()

	outs := bufio.NewScanner(stdoutR)
	for outs.Scan() {
		text := outs.Text()
		sess.logger.Debug(title, "Dir", cmd.Dir, "Path", cmd.Path, "Args", cmd.Args, "Stdout", text)
		stdout.WriteString(text + "\n")
	}
	<-stderrClosed

	// Now wait for the command to finish. No matter what, return everything written to stdout and
	// stderr, in addition to the resulting error, if any.
	err = cmd.Wait()
	return stdout.String(), stderr.String(), err
}

func (sess *reconcileStackSession) lookupPulumiAccessToken(ctx context.Context) (string, bool) {
	if sess.stack.AccessTokenSecret != "" {
		// Fetch the API token from the named secret.
		secret := &corev1.Secret{}
		if err := sess.kubeClient.Get(ctx,
			types.NamespacedName{Name: sess.stack.AccessTokenSecret, Namespace: sess.namespace}, secret); err != nil {
			sess.logger.Error(err, "Could not find secret for Pulumi API access",
				"Namespace", sess.namespace, "Stack.AccessTokenSecret", sess.stack.AccessTokenSecret)
			return "", false
		}

		accessToken := string(secret.Data["accessToken"])
		if accessToken == "" {
			err := errors.New("Secret accessToken data is empty")
			sess.logger.Error(err, "Illegal empty secret accessToken data for Pulumi API access",
				"Namespace", sess.namespace, "Stack.AccessTokenSecret", sess.stack.AccessTokenSecret)
			return "", false
		}
		return accessToken, true
	}

	return "", false
}

func (sess *reconcileStackSession) SetupWorkdirFromGitSource(ctx context.Context, gitAuth *auto.GitAuth, source *shared.GitSource) (_revision string, retErr error) {
	repo := auto.GitRepo{
		URL:         source.ProjectRepo,
		ProjectPath: source.RepoDir,
		CommitHash:  source.Commit,
		Branch:      source.Branch,
		Auth:        gitAuth,
	}

	sess.logger.Debug("Setting up pulumi workdir for stack", "stack", sess.stack)
	// Create a new workspace.
	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)

	// Create the temporary workdir
	dir, err := os.MkdirTemp("", "pulumi_auto")
	if err != nil {
		return "", fmt.Errorf("unable to create tmp directory for workspace: %w", err)
	}
	sess.rootDir = dir

	// Cleanup the rootdir on failure setting up the workspace.
	defer func() {
		if retErr != nil {
			_ = os.RemoveAll(dir)
			sess.rootDir = ""
		}
	}()

	var w auto.Workspace
	w, err = auto.NewLocalWorkspace(ctx, auto.WorkDir(dir), auto.Repo(repo), secretsProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create local workspace: %w", err)
	}

	revision, err := revisionAtWorkingDir(w.WorkDir())
	if err != nil {
		return "", err
	}

	return revision, sess.setupWorkspace(ctx, w)
}

// ProjectFile adds required Pulumi 'project' fields to the Program spec, making it valid to be given to Pulumi.
type ProjectFile struct {
	Name    string `json:"name"`
	Runtime string `json:"runtime"`
	pulumiv1.ProgramSpec
}

func (sess *reconcileStackSession) SetupWorkdirFromYAML(ctx context.Context, programRef shared.ProgramReference) (_commit string, retErr error) {
	sess.logger.Debug("Setting up pulumi workdir for stack", "stack", sess.stack)

	// Create a new workspace.
	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)

	// Create the temporary workdir
	dir, err := os.MkdirTemp("", "pulumi_auto")
	if err != nil {
		return "", fmt.Errorf("unable to create tmp directory for workspace: %w", err)
	}
	sess.rootDir = dir

	// Cleanup the rootdir on failure setting up the workspace.
	defer func() {
		if retErr != nil {
			_ = os.RemoveAll(sess.rootDir)
		}
	}()

	program := pulumiv1.Program{}
	programKey := client.ObjectKey{
		Name:      programRef.Name,
		Namespace: sess.namespace,
	}

	err = sess.kubeClient.Get(ctx, programKey, &program)
	if err != nil {
		return "", errProgramNotFound
	}

	var project ProjectFile
	project.Name = program.Name
	project.Runtime = "yaml"
	project.ProgramSpec = program.Program

	out, err := yaml.Marshal(&project)
	if err != nil {
		return "", fmt.Errorf("failed to marshal program object to YAML: %w", err)
	}

	err = os.WriteFile(filepath.Join(dir, "Pulumi.yaml"), out, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write YAML to file: %w", err)
	}

	var w auto.Workspace
	w, err = auto.NewLocalWorkspace(ctx, auto.WorkDir(dir), secretsProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create local workspace: %w", err)
	}

	revision := fmt.Sprintf("%s/%d", program.Name, program.ObjectMeta.Generation)

	return revision, sess.setupWorkspace(ctx, w)
}

// setupWorkspace sets all the extra configuration specified by the Stack object, after you have
// constructed a workspace from a source.
func (sess *reconcileStackSession) setupWorkspace(ctx context.Context, w auto.Workspace) error {
	sess.workdir = w.WorkDir()

	if sess.stack.Backend != "" {
		w.SetEnvVar("PULUMI_BACKEND_URL", sess.stack.Backend)
	}
	if accessToken, found := sess.lookupPulumiAccessToken(ctx); found {
		w.SetEnvVar("PULUMI_ACCESS_TOKEN", accessToken)
	}

	var err error
	if err = sess.SetEnvRefsForWorkspace(ctx, w); err != nil {
		return err
	}

	var a auto.Stack

	if sess.stack.UseLocalStackOnly {
		sess.logger.Info("Using local stack", "stack", sess.stack.Stack)
		a, err = auto.SelectStack(ctx, sess.stack.Stack, w)
	} else {
		sess.logger.Info("Upserting stack", "stack", sess.stack.Stack, "workspace", w)
		a, err = auto.UpsertStack(ctx, sess.stack.Stack, w)
	}
	if err != nil {
		return fmt.Errorf("failed to create and/or select stack %s: %w", sess.stack.Stack, err)
	}
	sess.autoStack = &a
	sess.logger.Debug("Setting autostack", "autostack", sess.autoStack)

	var c auto.ConfigMap
	c, err = sess.autoStack.GetAllConfig(ctx)
	if err != nil {
		return err
	}
	sess.logger.Debug("Initial autostack config", "config", c)

	// Ensure stack settings file in workspace is populated appropriately.
	if err = sess.ensureStackSettings(ctx, w); err != nil {
		return err
	}

	// Update the stack config and secret config values.
	err = sess.UpdateConfig(ctx)
	if err != nil {
		sess.logger.Error(err, "failed to set stack config", "Stack.Name", sess.stack.Stack)
		return fmt.Errorf("failed to set stack config: %w", err)
	}

	// Install project dependencies
	if err = sess.InstallProjectDependencies(ctx, sess.autoStack.Workspace()); err != nil {
		return fmt.Errorf("installing project dependencies: %w", err)
	}

	return nil
}

func (sess *reconcileStackSession) ensureStackSettings(ctx context.Context, w auto.Workspace) error {
	// We may have a project stack file already checked-in. Try and read that first
	// since we don't want to clobber it unnecessarily.
	// If not found, stackConfig will be a pointer to a zeroed-out workspace.ProjectStack.
	stackConfig, err := w.StackSettings(ctx, sess.stack.Stack)
	if err != nil {
		sess.logger.Info("Missing stack config file. Will assume no stack config checked-in.", "Cause", err)
		stackConfig = &workspace.ProjectStack{}
	}

	sess.logger.Debug("stackConfig loaded", "stack", sess.autoStack, "stackConfig", stackConfig)

	// Prefer the secretsProvider in the stack config. To override an existing stack to the default
	// secret provider, the stack's secretsProvider field needs to be set to 'default'
	if sess.stack.SecretsProvider != "" {
		// We must always make sure the secret provider is initialized in the workspace
		// before we set any configs. Otherwise secret provider will mysteriously reset.
		// https://github.com/pulumi/pulumi-kubernetes-operator/issues/135
		stackConfig.SecretsProvider = sess.stack.SecretsProvider
	}
	if err := w.SaveStackSettings(ctx, sess.stack.Stack, stackConfig); err != nil {
		return fmt.Errorf("failed to save stack settings: %w", err)
	}
	return nil
}

func (sess *reconcileStackSession) CleanupPulumiDir() {
	if sess.rootDir != "" {
		if err := os.RemoveAll(sess.rootDir); err != nil {
			sess.logger.Error(err, "Failed to delete temporary root dir: %s", sess.rootDir)
		}
	}
}

// Determine the actual commit information from the working directory (Spec commit etc. is optional).
func revisionAtWorkingDir(workingDir string) (string, error) {
	gitRepo, err := git.PlainOpenWithOptions(workingDir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", fmt.Errorf("failed to resolve git repository from working directory %s: %w", workingDir, err)
	}
	headRef, err := gitRepo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to determine revision for git repository at %s: %w", workingDir, err)
	}
	return headRef.Hash().String(), nil
}

func (sess *reconcileStackSession) InstallProjectDependencies(ctx context.Context, workspace auto.Workspace) error {
	project, err := workspace.ProjectSettings(ctx)
	if err != nil {
		return fmt.Errorf("unable to get project runtime: %w", err)
	}
	sess.logger.Debug("InstallProjectDependencies", "workspace", workspace.WorkDir())
	switch project.Runtime.Name() {
	case "nodejs":
		npm, _ := exec.LookPath("npm")
		if npm == "" {
			npm, _ = exec.LookPath("yarn")
		}
		if npm == "" {
			return errors.New("did not find 'npm' or 'yarn' on the PATH; can't install project dependencies")
		}
		// TODO: Consider using `npm ci` instead if there is a `package-lock.json` or `npm-shrinkwrap.json` present
		cmd := exec.Command(npm, "install")
		_, _, err := sess.runCmd("NPM/Yarn", cmd, workspace)
		return err
	case "python":
		python3, _ := exec.LookPath("python3")
		if python3 == "" {
			return errors.New("did not find 'python3' on the PATH; can't install project dependencies")
		}
		pip3, _ := exec.LookPath("pip3")
		if pip3 == "" {
			return errors.New("did not find 'pip3' on the PATH; can't install project dependencies")
		}
		venv := ""
		if project.Runtime.Options() != nil {
			venv, _ = project.Runtime.Options()["virtualenv"].(string)
		}
		if venv == "" {
			// TODO[pulumi/pulumi-kubernetes-operator#79]
			return errors.New("Python projects without a `virtualenv` project configuration are not yet supported in the Pulumi Kubernetes Operator")
		}
		// Emulate the same steps as the CLI does in https://github.com/pulumi/pulumi/blob/master/sdk/python/python.go#L97-L99.
		// TODO[pulumi/pulumi#5164]: Ideally the CLI would automatically do these - since it already knows how.
		cmd := exec.Command(python3, "-m", "venv", venv)
		_, _, err := sess.runCmd("Pip Install", cmd, workspace)
		if err != nil {
			return err
		}
		venvPython := filepath.Join(venv, "bin", "python")
		cmd = exec.Command(venvPython, "-m", "pip", "install", "--upgrade", "pip", "setuptools", "wheel")
		_, _, err = sess.runCmd("Pip Install", cmd, workspace)
		if err != nil {
			return err
		}
		cmd = exec.Command(venvPython, "-m", "pip", "install", "-r", "requirements.txt")
		_, _, err = sess.runCmd("Pip Install", cmd, workspace)
		if err != nil {
			return err
		}
		return nil
	case "go", "dotnet":
		// nothing needed
		return nil
	default:
		// Allow unknown runtimes without any pre-processing, but print a message indicating runtime was unknown
		sess.logger.Info(fmt.Sprintf("Handling unknown project runtime '%s'", project.Runtime.Name()),
			"Stack.Name", sess.stack.Stack)
		return nil
	}
}

func (sess *reconcileStackSession) UpdateConfig(ctx context.Context) error {
	m := make(auto.ConfigMap)
	for k, v := range sess.stack.Config {
		m[k] = auto.ConfigValue{
			Value:  v,
			Secret: false,
		}
	}
	for k, v := range sess.stack.Secrets {
		m[k] = auto.ConfigValue{
			Value:  v,
			Secret: true,
		}
	}

	for k, ref := range sess.stack.SecretRefs {
		resolved, err := sess.resolveResourceRef(ctx, &ref)
		if err != nil {
			return fmt.Errorf("updating secretRef for %q: %w", k, err)
		}
		m[k] = auto.ConfigValue{
			Value:  resolved,
			Secret: true,
		}
	}
	if err := sess.autoStack.SetAllConfig(ctx, m); err != nil {
		return err
	}
	sess.logger.Debug("Updated stack config", "Stack.Name", sess.stack.Stack, "config", m)
	return nil
}

func (sess *reconcileStackSession) RefreshStack(ctx context.Context, expectNoChanges bool) (shared.Permalink, error) {
	writer := sess.logger.LogWriterDebug("Pulumi Refresh")
	defer contract.IgnoreClose(writer)
	opts := []optrefresh.Option{optrefresh.ProgressStreams(writer), optrefresh.UserAgent(execAgent)}
	if expectNoChanges {
		opts = append(opts, optrefresh.ExpectNoChanges())
	}
	result, err := sess.autoStack.Refresh(ctx, opts...)
	if err != nil {
		return "", fmt.Errorf("refreshing stack %q: %w", sess.stack.Stack, err)
	}
	p, err := auto.GetPermalink(result.StdOut)
	if err != nil {
		// Successful update but no permalink suggests a backend which doesn't support permalinks. Ignore.
		sess.logger.Error(err, "No permalink found.", "Namespace", sess.namespace)
	}
	permalink := shared.Permalink(p)
	return permalink, nil
}

// UpdateStack runs the update on the stack and returns an update status code
// and error. In certain cases, an update may be unabled to proceed due to locking,
// in which case the operator will requeue itself to retry later.
func (sess *reconcileStackSession) UpdateStack(ctx context.Context) (shared.StackUpdateStatus, shared.Permalink, *auto.UpResult, error) {
	writer := sess.logger.LogWriterDebug("Pulumi Update")
	defer contract.IgnoreClose(writer)

	result, err := sess.autoStack.Up(ctx, optup.ProgressStreams(writer), optup.UserAgent(execAgent))
	if err != nil {
		// If this is the "conflict" error message, we will want to gracefully quit and retry.
		if auto.IsConcurrentUpdateError(err) {
			return shared.StackUpdateConflict, shared.Permalink(""), nil, err
		}
		// If this is the "not found" error message, we will want to gracefully quit and retry.
		if strings.Contains(result.StdErr, "error: [404] Not found") {
			return shared.StackNotFound, shared.Permalink(""), nil, err
		}
		return shared.StackUpdateFailed, shared.Permalink(""), nil, err
	}
	p, err := auto.GetPermalink(result.StdOut)
	if err != nil {
		// Successful update but no permalink suggests a backend which doesn't support permalinks. Ignore.
		sess.logger.Debug("No permalink found - ignoring.", "Stack.Name", sess.stack.Stack, "Namespace", sess.namespace)
	}
	permalink := shared.Permalink(p)
	return shared.StackUpdateSucceeded, permalink, &result, nil
}

// GetStackOutputs gets the stack outputs and parses them into a map.
func (sess *reconcileStackSession) GetStackOutputs(outs auto.OutputMap) (shared.StackOutputs, error) {
	o := make(shared.StackOutputs)
	for k, v := range outs {
		var value apiextensionsv1.JSON
		if v.Secret {
			value = apiextensionsv1.JSON{Raw: []byte(`"[secret]"`)}
		} else {
			// Marshal the OutputMap value only, to use in unmarshaling to StackOutputs
			valueBytes, err := json.Marshal(v.Value)
			if err != nil {
				return nil, fmt.Errorf("marshaling stack output value interface: %w", err)
			}
			if err := json.Unmarshal(valueBytes, &value); err != nil {
				return nil, fmt.Errorf("unmarshaling stack output value: %w", err)
			}
		}

		o[k] = value
	}
	return o, nil
}

func (sess *reconcileStackSession) DestroyStack(ctx context.Context) error {
	writer := sess.logger.LogWriterInfo("Pulumi Destroy")
	defer contract.IgnoreClose(writer)

	_, err := sess.autoStack.Destroy(ctx, optdestroy.ProgressStreams(writer), optdestroy.UserAgent(execAgent))
	if err != nil {
		return fmt.Errorf("destroying resources for stack %q: %w", sess.stack.Stack, err)
	}

	err = sess.autoStack.Workspace().RemoveStack(ctx, sess.stack.Stack)
	if err != nil {
		return fmt.Errorf("removing stack %q: %w", sess.stack.Stack, err)
	}
	return nil
}

// SetupGitAuth sets up the authentication option to use for the git source
// repository of the stack. If neither gitAuth or gitAuthSecret are set,
// a pointer to a zero value of GitAuth is returned — representing
// unauthenticated git access.
func (sess *reconcileStackSession) SetupGitAuth(ctx context.Context) (*auto.GitAuth, error) {
	gitAuth := &auto.GitAuth{}

	if sess.stack.GitAuth != nil {
		if sess.stack.GitAuth.SSHAuth != nil {
			privateKey, err := sess.resolveResourceRef(ctx, &sess.stack.GitAuth.SSHAuth.SSHPrivateKey)
			if err != nil {
				return nil, fmt.Errorf("resolving gitAuth SSH private key: %w", err)
			}
			gitAuth.SSHPrivateKey = privateKey

			if sess.stack.GitAuth.SSHAuth.Password != nil {
				password, err := sess.resolveResourceRef(ctx, sess.stack.GitAuth.SSHAuth.Password)
				if err != nil {
					return nil, fmt.Errorf("resolving gitAuth SSH password: %w", err)
				}
				gitAuth.Password = password
			}

			return gitAuth, nil
		}

		if sess.stack.GitAuth.PersonalAccessToken != nil {
			accessToken, err := sess.resolveResourceRef(ctx, sess.stack.GitAuth.PersonalAccessToken)
			if err != nil {
				return nil, fmt.Errorf("resolving gitAuth personal access token: %w", err)
			}
			gitAuth.PersonalAccessToken = accessToken
			return gitAuth, nil
		}

		if sess.stack.GitAuth.BasicAuth == nil {
			return nil, errors.New("gitAuth config must specify exactly one of " +
				"'personalAccessToken', 'sshPrivateKey' or 'basicAuth'")
		}

		userName, err := sess.resolveResourceRef(ctx, &sess.stack.GitAuth.BasicAuth.UserName)
		if err != nil {
			return nil, fmt.Errorf("resolving gitAuth username: %w", err)
		}

		password, err := sess.resolveResourceRef(ctx, &sess.stack.GitAuth.BasicAuth.Password)
		if err != nil {
			return nil, fmt.Errorf("resolving gitAuth password: %w", err)
		}

		gitAuth.Username = userName
		gitAuth.Password = password
	} else if sess.stack.GitAuthSecret != "" {
		namespacedName := types.NamespacedName{Name: sess.stack.GitAuthSecret, Namespace: sess.namespace}

		// Fetch the named secret.
		secret := &corev1.Secret{}
		if err := sess.kubeClient.Get(ctx, namespacedName, secret); err != nil {
			sess.logger.Error(err, "Could not find secret for access to the git repository",
				"Namespace", sess.namespace, "Stack.GitAuthSecret", sess.stack.GitAuthSecret)
			return nil, err
		}

		// First check if an SSH private key has been specified.
		if sshPrivateKey, exists := secret.Data["sshPrivateKey"]; exists {
			gitAuth = &auto.GitAuth{
				SSHPrivateKey: string(sshPrivateKey),
			}

			if password, exists := secret.Data["password"]; exists {
				gitAuth.Password = string(password)
			}
			// Then check if a personal access token has been specified.
		} else if accessToken, exists := secret.Data["accessToken"]; exists {
			gitAuth = &auto.GitAuth{
				PersonalAccessToken: string(accessToken),
			}
			// Then check if basic authentication has been specified.
		} else if username, exists := secret.Data["username"]; exists {
			if password, exists := secret.Data["password"]; exists {
				gitAuth = &auto.GitAuth{
					Username: string(username),
					Password: string(password),
				}
			} else {
				return nil, errors.New("creating gitAuth: missing 'password' secret entry")
			}
		}
	}

	return gitAuth, nil
}

// Add default permalink for the stack in the Pulumi Service.
func (sess *reconcileStackSession) addDefaultPermalink(ctx context.Context, stack *pulumiv1.Stack) error {
	// Get stack URL.
	info, err := sess.autoStack.Info(ctx)
	if err != nil {
		sess.logger.Error(err, "Failed to update Stack status with default permalink", "Stack.Name", stack.Spec.Stack)
		return err
	}
	// Set stack URL.
	if stack.Status.LastUpdate == nil {
		stack.Status.LastUpdate = &shared.StackUpdateState{}
	}
	stack.Status.LastUpdate.Permalink = shared.Permalink(info.URL)
	err = sess.patchStatus(ctx, stack)
	if err != nil {
		return err
	}
	sess.logger.Debug("Successfully updated Stack with default permalink", "Stack.Name", stack.Spec.Stack)
	return nil
}

// patchStatus updates the recorded status of a stack using a patch. The patch is calculated with
// respect to a freshly fetched object, to better avoid conflicts.
func (sess *reconcileStackSession) patchStatus(ctx context.Context, o *pulumiv1.Stack) error {
	var s pulumiv1.Stack
	if err := sess.kubeClient.Get(ctx, types.NamespacedName{
		Namespace: o.GetNamespace(),
		Name:      o.GetName(),
	}, &s); err != nil {
		return err
	}
	s1 := s.DeepCopy()
	s1.Status = o.Status
	patch := client.MergeFrom(&s)
	return sess.kubeClient.Status().Patch(ctx, s1, patch)
}

// addSSHKeysToKnownHosts scans the public SSH keys for the project repository URL
// and adds them to the SSH known hosts to perform strict key checking during SSH
// git cloning.
func (sess *reconcileStackSession) addSSHKeysToKnownHosts(projectRepoURL string) error {
	// Parse the Stack project repo SSH host and port (if exists) from the git SSH URL
	// e.g. git@github.com:foo/bar.git returns "github.com" for host
	// e.g. git@example.com:1234:foo/bar.git returns "example.com" for host and "1234" for port
	u, err := giturls.Parse(projectRepoURL)
	if err != nil {
		return fmt.Errorf("error parsing project repo URL to use with ssh-keyscan: %w", err)
	}
	hostPort := strings.Split(u.Host, ":")
	if len(hostPort) == 0 || len(hostPort) > 2 {
		return fmt.Errorf("error parsing project repo URL to use with ssh-keyscan: %w", err)
	}

	// SSH key scan the repo's URL (host port) to get the public keys.
	args := []string{}
	if len(hostPort) == 2 {
		args = append(args, "-p", hostPort[1])
	}
	args = append(args, "-H", hostPort[0])
	sshKeyScan, _ := exec.LookPath("ssh-keyscan")
	cmd := exec.Command(sshKeyScan, args...)
	cmd.Dir = os.Getenv("HOME")
	stdout, _, err := sess.runCmd("SSH Key Scan", cmd, nil)
	if err != nil {
		return fmt.Errorf("error running ssh-keyscan: %w", err)
	}

	// Add the repo public keys to the SSH known hosts to enforce key checking.
	filename := fmt.Sprintf("%s/%s", os.Getenv("HOME"), ".ssh/known_hosts")
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("error running ssh-keyscan: %w", err)
	}
	defer f.Close()
	if _, err = f.WriteString(stdout); err != nil {
		return fmt.Errorf("error running ssh-keyscan: %w")
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Use the ServiceAccount CA cert and token to setup $HOME/.kube/config.
// This makes the cert and token already available to the operator by its
// ServiceAccount into a consumable kubeconfig file written its filesystem for
// usage. This kubeconfig is used to deploy Pulumi Stacks of k8s resources
// in-cluster that use the default, ambient kubeconfig.
func setupInClusterKubeconfig() error {
	const certFp = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	const tokenFp = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	const namespaceFp = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	kubeFp := os.ExpandEnv("$HOME/.kube")
	kubeconfigFp := fmt.Sprintf("%s/config", kubeFp)

	cert, err := waitForFile(certFp)
	if err != nil {
		return fmt.Errorf("failed to open in-cluster ServiceAccount CA certificate: %w", err)
	}
	token, err := waitForFile(tokenFp)
	if err != nil {
		return fmt.Errorf("failed to open in-cluster ServiceAccount token: %w", err)
	}
	namespace, err := waitForFile(namespaceFp)
	if err != nil {
		return fmt.Errorf("failed to open in-cluster ServiceAccount namespace: %w", err)
	}

	// Compute the kubeconfig using the cert and token.
	s := fmt.Sprintf(`
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: https://%s
  name: local
contexts:
- context:
    cluster: local
    user: local
    %s
  name: local
current-context: local
kind: Config
users:
- name: local
  user:
    token: %s
`, string(base64.StdEncoding.EncodeToString(cert)), os.ExpandEnv("$KUBERNETES_PORT_443_TCP_ADDR"), inferNamespace(string(namespace)), string(token))

	err = os.Mkdir(os.ExpandEnv(kubeFp), 0755)
	if err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}
	file, err := os.Create(os.ExpandEnv(kubeconfigFp))
	if err != nil {
		return fmt.Errorf("failed to create kubeconfig file: %w", err)
	}
	return os.WriteFile(file.Name(), []byte(s), 0644)
}

// waitForFile waits for the existence of a file, and returns its contents if
// available.
func waitForFile(fp string) ([]byte, error) {
	retries := 3
	fileExists := false
	var err error
	for i := 0; i < retries; i++ {
		if _, err = os.Stat(fp); os.IsNotExist(err) {
			time.Sleep(2 * time.Second)
		} else {
			fileExists = true
			break
		}
	}

	if !fileExists {
		return nil, err
	}

	file, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", fp, err)
	}
	return file, err
}
