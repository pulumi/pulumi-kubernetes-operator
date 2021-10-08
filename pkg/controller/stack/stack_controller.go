// Copyright 2021, Pulumi Corporation.  All rights reserved.

package stack

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/operator-framework/operator-lib/handler"
	libpredicate "github.com/operator-framework/operator-lib/predicate"
	"github.com/pkg/errors"
	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1alpha1"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log       = logf.Log.WithName("controller_stack")
	execAgent = fmt.Sprintf("pulumi-kubernetes-operator/%s", version.Version)
)

const (
	pulumiFinalizer                = "finalizer.stack.pulumi.com"
	defaultMaxConcurrentReconciles = 10
)

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
	return &ReconcileStack{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	var err error
	maxConcurrentReconciles := 10
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

	stackInformer, err := mgr.GetCache().GetInformer(context.Background(), &pulumiv1alpha1.Stack{})
	if err != nil {
		return err
	}
	stackInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    newStackCallback,
		UpdateFunc: updateStackCallback,
		DeleteFunc: deleteStackCallback,
	})

	// Watch for changes to primary resource Stack
	err = c.Watch(&source.Kind{Type: &pulumiv1alpha1.Stack{}}, &handler.InstrumentedEnqueueRequestForObject{}, predicates...)
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
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Stack object and makes changes based on the state read
// and what is in the Stack.Spec
func (r *ReconcileStack) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := logging.WithValues(log, "Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Stack")

	// Fetch the Stack instance
	instance := &pulumiv1alpha1.Stack{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
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

	stack := instance.Spec

	// Check if the Stack instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isStackMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil

	// Create a new reconciliation session.
	sess := newReconcileStackSession(reqLogger, stack, r.client, request.Namespace)

	// Ensure either branch or commit has been specified in the stack CR if stack is not marked for deletion
	if !isStackMarkedToBeDeleted &&
		sess.stack.Commit == "" &&
		sess.stack.Branch == "" {

		reqLogger.Info("Stack CustomResource needs to specify either 'branch' or 'commit' for the tracking repo.")
		return reconcile.Result{}, err
	}

	// Step 1. Set up the workdir, select the right stack and populate config if supplied.
	gitAuth, err := sess.SetupGitAuth()
	if err != nil {
		reqLogger.Error(err, "Failed to setup git authentication", "Stack.Name", stack.Stack)
		return reconcile.Result{}, err
	}

	if gitAuth.SSHPrivateKey != "" {
		// Add the project repo's public SSH keys to the SSH known hosts
		// to perform the necessary key checking during SSH git cloning.
		sess.addSSHKeysToKnownHosts(sess.stack.ProjectRepo)
	}

	if err = sess.SetupPulumiWorkdir(gitAuth); err != nil {
		reqLogger.Error(err, "Failed to setup Pulumi workdir", "Stack.Name", stack.Stack)
		return reconcile.Result{}, err
	}

	// Delete the temporary directory after the reconciliation is completed (regardless of success or failure).
	defer sess.CleanupPulumiDir()

	currentCommit, err := revisionAtWorkingDir(sess.workdir)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Step 2. If there are extra environment variables, read them in now and use them for subsequent commands.
	if err = sess.SetEnvs(stack.Envs, request.Namespace); err != nil {
		if err2 := sess.markStackFailed(errors.Wrap(err, "could not find ConfigMap for Envs"), instance, currentCommit, ""); err2 != nil {
			return reconcile.Result{}, err2
		}
		return reconcile.Result{}, err
	}
	if err = sess.SetSecretEnvs(stack.SecretEnvs, request.Namespace); err != nil {
		if err2 := sess.markStackFailed(errors.Wrap(err, "could not find Secret for SecretEnvs"), instance, currentCommit, ""); err2 != nil {
			return reconcile.Result{}, err2
		}
		return reconcile.Result{}, err
	}

	// Check if the Stack instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isStackMarkedToBeDeleted = instance.GetDeletionTimestamp() != nil

	// Finalize the stack, or add a finalizer based on the deletion timestamp.
	if isStackMarkedToBeDeleted {
		if contains(instance.GetFinalizers(), pulumiFinalizer) {
			err := sess.finalize(instance)
			// Manage extra status here
			return reconcile.Result{}, err
		}
	} else {
		if !contains(instance.GetFinalizers(), pulumiFinalizer) {
			// Add finalizer to Stack if not being deleted
			err := sess.addFinalizer(instance)
			if err != nil {
				return reconcile.Result{}, err
			}
			time.Sleep(2 * time.Second) // arbitrary sleep after finalizer add to avoid stale obj for permalink
			// Add default permalink for the stack in the Pulumi Service.
			if err := sess.addDefaultPermalink(instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// If a branch is specified, then track changes to the branch.
	trackBranch := len(sess.stack.Branch) > 0

	if trackBranch {
		reqLogger.Info("Checking current HEAD commit hash", "Current commit", currentCommit)
		if instance.Status.LastUpdate.LastSuccessfulCommit == currentCommit {
			reqLogger.Info("Commit hash unchanged. Will poll again in 60 seconds.")
			// Reconcile every 60 seconds to check for new commits to the branch.
			return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
		}
		reqLogger.Info("New commit hash found", "Current commit", currentCommit,
			"Last commit", instance.Status.LastUpdate.LastSuccessfulCommit)
	}

	// Step 3. If a stack refresh is requested, run it now.
	if sess.stack.Refresh {
		permalink, err := sess.RefreshStack(sess.stack.ExpectNoRefreshChanges)
		if err != nil {
			if err2 := sess.markStackFailed(errors.Wrap(err, "refreshing stack"), instance, currentCommit, permalink); err2 != nil {
				return reconcile.Result{}, err2
			}
			return reconcile.Result{}, err
		}
		err = sess.getLatestResource(instance, request.NamespacedName)
		if err != nil {
			sess.logger.Error(err, "Failed to get latest Stack to update refresh status", "Stack.Name", instance.Spec.Stack)
			return reconcile.Result{}, err
		}
		if instance.Status.LastUpdate == nil {
			instance.Status.LastUpdate = &pulumiv1alpha1.StackUpdateState{}
		}
		instance.Status.LastUpdate.Permalink = permalink

		err = sess.updateResourceStatus(instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update Stack status for refresh", "Stack.Name", stack.Stack)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Successfully refreshed Stack", "Stack.Name", stack.Stack)
	}

	// Step 4. Run a `pulumi up --skip-preview`.
	// TODO: is it possible to support a --dry-run with a preview?
	status, permalink, result, err := sess.UpdateStack()
	switch status {
	case pulumiv1alpha1.StackUpdateConflict:
		if sess.stack.RetryOnUpdateConflict {
			reqLogger.Error(err, "Conflict with another concurrent update -- will retry shortly", "Stack.Name", stack.Stack)
			return reconcile.Result{RequeueAfter: time.Second * 5}, nil
		}
		reqLogger.Error(err, "Conflict with another concurrent update -- NOT retrying", "Stack.Name", stack.Stack)
		return reconcile.Result{}, nil
	case pulumiv1alpha1.StackNotFound:
		reqLogger.Error(err, "Stack not found -- will retry shortly", "Stack.Name", stack.Stack, "Err:")
		return reconcile.Result{RequeueAfter: time.Second * 5}, nil
	default:
		if err != nil {
			if err2 := sess.markStackFailed(err, instance, currentCommit, permalink); err2 != nil {
				return reconcile.Result{}, err2
			}
			return reconcile.Result{}, err
		}
	}

	// Step 5. Capture outputs onto the resulting status object.
	outs, err := sess.GetStackOutputs(result.Outputs)
	if err != nil {
		reqLogger.Error(err, "Failed to get Stack outputs", "Stack.Name", stack.Stack)
		return reconcile.Result{}, err
	}
	if outs == nil {
		reqLogger.Info("Stack outputs are empty. Skipping status update", "Stack.Name", stack.Stack)
		return reconcile.Result{}, nil
	}
	err = sess.getLatestResource(instance, request.NamespacedName)
	if err != nil {
		sess.logger.Error(err, "Failed to get latest Stack to update successful Stack status", "Stack.Name", instance.Spec.Stack)
		return reconcile.Result{}, err
	}
	instance.Status.Outputs = outs
	instance.Status.LastUpdate = &pulumiv1alpha1.StackUpdateState{
		State:                pulumiv1alpha1.SucceededStackStateMessage,
		LastAttemptedCommit:  currentCommit,
		LastSuccessfulCommit: currentCommit,
		Permalink:            permalink,
	}
	err = sess.updateResourceStatus(instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update Stack status", "Stack.Name", stack.Stack)
		return reconcile.Result{}, err
	}
	reqLogger.Info("Successfully updated status for Stack", "Stack.Name", stack.Stack)

	if trackBranch {
		// Reconcile every 60 seconds to check for new commits to the branch.
		return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
	}

	return reconcile.Result{}, nil
}

func (sess *reconcileStackSession) markStackFailed(err error, instance *pulumiv1alpha1.Stack, currentCommit string, permalink pulumiv1alpha1.Permalink) error {
	sess.logger.Error(err, "Failed to update Stack", "Stack.Name", sess.stack.Stack)
	// Update Stack status with failed state
	instance.Status.LastUpdate.LastAttemptedCommit = currentCommit
	instance.Status.LastUpdate.State = pulumiv1alpha1.FailedStackStateMessage
	instance.Status.LastUpdate.Permalink = permalink

	if err2 := sess.updateResourceStatus(instance); err2 != nil {
		msg := "Failed to update status for a failed Stack update"
		err3 := errors.Wrapf(err, err2.Error())
		sess.logger.Error(err3, msg)
		return err3
	}
	return nil
}

func (sess *reconcileStackSession) finalize(stack *pulumiv1alpha1.Stack) error {
	sess.logger.Info("Finalizing the stack")
	// Run finalization logic for pulumiFinalizer. If the
	// finalization logic fails, don't remove the finalizer so
	// that we can retry during the next reconciliation.
	if err := sess.finalizeStack(); err != nil {
		sess.logger.Error(err, "Failed to run Pulumi finalizer", "Stack.Name", stack.Spec.Stack)
		return err
	}

	// Remove pulumiFinalizer. Once all finalizers have been
	// removed, the object will be deleted.
	controllerutil.RemoveFinalizer(stack, pulumiFinalizer)
	if err := sess.updateResource(stack); err != nil {
		sess.logger.Error(err, "Failed to delete Pulumi finalizer", "Stack.Name", stack.Spec.Stack)
		return err
	}

	// Since the client is hitting a cache, waiting for the deletion here will guarantee that the next
	// reconciliation will see that the CR has been deleted and
	// that there's nothing left to do.
	if err := sess.waitForDeletion(stack); err != nil {
		log.Info("Failed waiting for Stack deletion")
		return err
	}

	return nil
}

func (sess *reconcileStackSession) finalizeStack() error {
	// Destroy the stack resources and stack.
	if sess.stack.DestroyOnFinalize {
		if err := sess.DestroyStack(); err != nil {
			return err
		}
	}
	sess.logger.Info("Successfully finalized stack")
	return nil
}

//addFinalizer will add this attribute to the Stack CR
func (sess *reconcileStackSession) addFinalizer(stack *pulumiv1alpha1.Stack) error {
	sess.logger.Debug("Adding Finalizer for the Stack", "Stack.Name", stack.Name)
	namespacedName := types.NamespacedName{Name: stack.Name, Namespace: stack.Namespace}
	err := sess.getLatestResource(stack, namespacedName)
	if err != nil {
		sess.logger.Error(err, "Failed to get latest Stack to add Pulumi finalizer", "Stack.Name", stack.Spec.Stack)
		return err
	}
	controllerutil.AddFinalizer(stack, pulumiFinalizer)
	err = sess.updateResource(stack)
	if err != nil {
		sess.logger.Error(err, "Failed to add Pulumi finalizer", "Stack.Name", stack.Spec.Stack)
		return err
	}
	return nil
}

type reconcileStackSession struct {
	logger     logging.Logger
	kubeClient client.Client
	stack      pulumiv1alpha1.StackSpec
	autoStack  *auto.Stack
	namespace  string
	workdir    string
	rootDir    string
}

// blank assignment to verify that reconcileStackSession implements pulumiv1alpha1.StackController.
var _ pulumiv1alpha1.StackController = &reconcileStackSession{}

func newReconcileStackSession(
	logger logging.Logger,
	stack pulumiv1alpha1.StackSpec,
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
func (sess *reconcileStackSession) SetEnvs(configMapNames []string, namespace string) error {
	for _, env := range configMapNames {
		config := &corev1.ConfigMap{}
		if err := sess.getLatestResource(config, types.NamespacedName{Name: env, Namespace: namespace}); err != nil {
			return errors.Wrapf(err, "Namespace=%s Name=%s", namespace, env)
		}
		if err := sess.autoStack.Workspace().SetEnvVars(config.Data); err != nil {
			return errors.Wrapf(err, "Namespace=%s Name=%s", namespace, env)
		}
	}
	return nil
}

// SetSecretEnvs populates the environment of the stack run with values
// from an array of Kubernetes Secrets in a Namespace.
func (sess *reconcileStackSession) SetSecretEnvs(secrets []string, namespace string) error {
	for _, env := range secrets {
		config := &corev1.Secret{}
		if err := sess.getLatestResource(config, types.NamespacedName{Name: env, Namespace: namespace}); err != nil {
			return errors.Wrapf(err, "Namespace=%s Name=%s", namespace, env)
		}
		envvars := map[string]string{}
		for k, v := range config.Data {
			envvars[k] = string(v)
		}
		if err := sess.autoStack.Workspace().SetEnvVars(envvars); err != nil {
			return errors.Wrapf(err, "Namespace=%s Name=%s", namespace, env)
		}
	}
	return nil
}

// SetEnvRefsForWorkspace populates environment variables for workspace using items in
// the EnvRefs field in the stack specification.
func (sess *reconcileStackSession) SetEnvRefsForWorkspace(w auto.Workspace) error {
	envRefs := sess.stack.EnvRefs
	for envVar, ref := range envRefs {
		val, err := sess.resolveResourceRef(&ref)
		if err != nil {
			return errors.Wrapf(err, "resolving env variable reference for: %q", envVar)
		}
		w.SetEnvVar(envVar, val)
	}
	return nil
}

func (sess *reconcileStackSession) resolveResourceRef(ref *pulumiv1alpha1.ResourceRef) (string, error) {
	switch ref.SelectorType {
	case pulumiv1alpha1.ResourceSelectorEnv:
		if ref.Env != nil {
			resolved := os.Getenv(ref.Env.Name)
			if resolved == "" {
				return "", fmt.Errorf("missing value for environment variable: %s", ref.Env.Name)
			}
			return resolved, nil
		}
		return "", errors.New("missing env reference in ResourceRef")
	case pulumiv1alpha1.ResourceSelectorLiteral:
		if ref.LiteralRef != nil {
			return ref.LiteralRef.Value, nil
		}
		return "", errors.New("missing literal reference in ResourceRef")
	case pulumiv1alpha1.ResourceSelectorFS:
		if ref.FileSystem != nil {
			contents, err := os.ReadFile(ref.FileSystem.Path)
			if err != nil {
				return "", errors.Wrapf(err, "reading path: %q", ref.FileSystem.Path)
			}
			return string(contents), nil
		}
		return "", errors.New("Missing filesystem reference in ResourceRef")
	case pulumiv1alpha1.ResourceSelectorSecret:
		if ref.SecretRef != nil {
			config := &corev1.Secret{}
			namespace := ref.SecretRef.Namespace
			if namespace == "" {
				namespace = "default"
			}
			if err := sess.getLatestResource(config, types.NamespacedName{Name: ref.SecretRef.Name, Namespace: namespace}); err != nil {
				return "", errors.Wrapf(err, "Namespace=%s Name=%s", ref.SecretRef.Namespace, ref.SecretRef.Name)
			}
			secretVal, ok := config.Data[ref.SecretRef.Key]
			if !ok {
				return "", errors.Errorf("No key %s found in secret %s/%s", ref.SecretRef.Key, ref.SecretRef.Namespace, ref.SecretRef.Name)
			}
			return string(secretVal), nil
		}
		return "", errors.New("Mising secret reference in ResourceRef")
	default:
		return "", errors.Errorf("Unsupported selector type: %v", ref.SelectorType)
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
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	// Start the command asynchronously.
	err := cmd.Start()
	if err != nil {
		return "", "", err
	}

	// Kick off some goroutines to stream the output asynchronously. Since Pulumi can take
	// a while to run, this helps to debug issues that might be ongoing before a command completes.
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	go func() {
		outs := bufio.NewScanner(stdoutR)
		for outs.Scan() {
			text := outs.Text()
			sess.logger.Debug(title, "Dir", cmd.Dir, "Path", cmd.Path, "Args", cmd.Args, "Stdout", text)
			stdout.WriteString(text + "\n")
		}
	}()
	go func() {
		errs := bufio.NewScanner(stderrR)
		for errs.Scan() {
			text := errs.Text()
			sess.logger.Debug(title, "Dir", cmd.Dir, "Path", cmd.Path, "Args", cmd.Args, "Text", text)
			stderr.WriteString(text + "\n")
		}
	}()

	// Now wait for the command to finish. No matter what, return everything written to stdout and
	// stderr, in addition to the resulting error, if any.
	err = cmd.Wait()
	return stdout.String(), stderr.String(), err
}

func (sess *reconcileStackSession) lookupPulumiAccessToken() (string, bool) {
	if sess.stack.AccessTokenSecret != "" {
		// Fetch the API token from the named secret.
		secret := &corev1.Secret{}
		if err := sess.kubeClient.Get(context.TODO(),
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

func (sess *reconcileStackSession) SetupPulumiWorkdir(gitAuth *auto.GitAuth) error {
	repo := auto.GitRepo{
		URL:         sess.stack.ProjectRepo,
		ProjectPath: sess.stack.RepoDir,
		CommitHash:  sess.stack.Commit,
		Branch:      sess.stack.Branch,
		Setup:       sess.InstallProjectDependencies,
		Auth:        gitAuth,
	}

	sess.logger.Debug("Setting up pulumi workdir for stack", "stack", sess.stack)
	// Create a new workspace.
	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)

	// Create the temporary workdir
	dir, err := os.MkdirTemp("", "pulumi_auto")
	if err != nil {
		return errors.Wrap(err, "unable to create tmp directory for workspace")
	}
	sess.rootDir = dir

	// Cleanup the rootdir on failure setting up the workspace.
	defer func() {
		if err != nil {
			_ = os.RemoveAll(sess.rootDir)
		}
	}()

	var w auto.Workspace
	w, err = auto.NewLocalWorkspace(context.Background(), auto.WorkDir(dir), auto.Repo(repo), secretsProvider)
	if err != nil {
		return errors.Wrap(err, "failed to create local workspace")
	}

	sess.workdir = w.WorkDir()

	if sess.stack.Backend != "" {
		w.SetEnvVar("PULUMI_BACKEND_URL", sess.stack.Backend)
	}
	if accessToken, found := sess.lookupPulumiAccessToken(); found {
		w.SetEnvVar("PULUMI_ACCESS_TOKEN", accessToken)
	}

	if err = sess.SetEnvRefsForWorkspace(w); err != nil {
		return err
	}

	ctx := context.Background()
	var a auto.Stack

	if sess.stack.UseLocalStackOnly {
		sess.logger.Info("Using local stack", "stack", sess.stack.Stack)
		a, err = auto.SelectStack(ctx, sess.stack.Stack, w)
	} else {
		sess.logger.Info("Upserting stack", "stack", sess.stack.Stack, "workspace", w)
		a, err = auto.UpsertStack(ctx, sess.stack.Stack, w)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to create and/or select stack: %s", sess.stack.Stack)
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
	if err = sess.ensureStackSettings(context.Background(), w); err != nil {
		return err
	}

	// Update the stack config and secret config values.
	err = sess.UpdateConfig(ctx)
	if err != nil {
		sess.logger.Error(err, "failed to set stack config", "Stack.Name", sess.stack.Stack)
		return errors.Wrap(err, "failed to set stack config")
	}

	// Install project dependencies
	if err = sess.InstallProjectDependencies(context.Background(), sess.autoStack.Workspace()); err != nil {
		return errors.Wrap(err, "installing project dependencies")
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
	if err := w.SaveStackSettings(context.Background(), sess.stack.Stack, stackConfig); err != nil {
		return errors.Wrap(err, "failed to save stack settings.")
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
		return "", errors.Wrapf(err, "failed to resolve git repository from working directory: %s", workingDir)
	}
	headRef, err := gitRepo.Head()
	if err != nil {
		return "", errors.Wrapf(err, "failed to determine revision for git repository at %s", workingDir)
	}
	return headRef.Hash().String(), nil
}

func (sess *reconcileStackSession) InstallProjectDependencies(ctx context.Context, workspace auto.Workspace) error {
	project, err := workspace.ProjectSettings(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to get project runtime")
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
		resolved, err := sess.resolveResourceRef(&ref)
		if err != nil {
			return errors.Wrapf(err, "updating secretRef for: %q", k)
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

func (sess *reconcileStackSession) RefreshStack(expectNoChanges bool) (pulumiv1alpha1.Permalink, error) {
	writer := sess.logger.LogWriterDebug("Pulumi Refresh")
	defer contract.IgnoreClose(writer)
	opts := []optrefresh.Option{optrefresh.ProgressStreams(writer), optrefresh.UserAgent(execAgent)}
	if expectNoChanges {
		opts = append(opts, optrefresh.ExpectNoChanges())
	}
	result, err := sess.autoStack.Refresh(
		context.Background(),
		opts...)
	if err != nil {
		return "", errors.Wrapf(err, "refreshing stack %q", sess.stack.Stack)
	}
	p, err := auto.GetPermalink(result.StdOut)
	if err != nil {
		// Successful update but no permalink suggests a backend which doesn't support permalinks. Ignore.
		sess.logger.Error(err, "No permalink found.", "Namespace", sess.namespace)
	}
	permalink := pulumiv1alpha1.Permalink(p)
	return permalink, nil
}

// UpdateStack runs the update on the stack and returns an update status code
// and error. In certain cases, an update may be unabled to proceed due to locking,
// in which case the operator will requeue itself to retry later.
func (sess *reconcileStackSession) UpdateStack() (pulumiv1alpha1.StackUpdateStatus, pulumiv1alpha1.Permalink, *auto.UpResult, error) {
	writer := sess.logger.LogWriterDebug("Pulumi Update")
	defer contract.IgnoreClose(writer)

	result, err := sess.autoStack.Up(context.Background(), optup.ProgressStreams(writer), optup.UserAgent(execAgent))
	if err != nil {
		// If this is the "conflict" error message, we will want to gracefully quit and retry.
		if auto.IsConcurrentUpdateError(err) {
			return pulumiv1alpha1.StackUpdateConflict, pulumiv1alpha1.Permalink(""), nil, err
		}
		// If this is the "not found" error message, we will want to gracefully quit and retry.
		if strings.Contains(result.StdErr, "error: [404] Not found") {
			return pulumiv1alpha1.StackNotFound, pulumiv1alpha1.Permalink(""), nil, err
		}
		return pulumiv1alpha1.StackUpdateFailed, pulumiv1alpha1.Permalink(""), nil, err
	}
	p, err := auto.GetPermalink(result.StdOut)
	if err != nil {
		// Successful update but no permalink suggests a backend which doesn't support permalinks. Ignore.
		sess.logger.Error(err, "No permalink found.", "Namespace", sess.namespace)
	}
	permalink := pulumiv1alpha1.Permalink(p)
	return pulumiv1alpha1.StackUpdateSucceeded, permalink, &result, nil
}

// GetStackOutputs gets the stack outputs and parses them into a map.
func (sess *reconcileStackSession) GetStackOutputs(outs auto.OutputMap) (pulumiv1alpha1.StackOutputs, error) {
	o := make(pulumiv1alpha1.StackOutputs)
	for k, v := range outs {
		var value apiextensionsv1.JSON
		if v.Secret {
			value = apiextensionsv1.JSON{Raw: []byte(`"[secret]"`)}
		} else {
			// Marshal the OutputMap value only, to use in unmarshaling to StackOutputs
			valueBytes, err := json.Marshal(v.Value)
			if err != nil {
				return nil, errors.Wrap(err, "marshaling stack output value interface")
			}
			if err := json.Unmarshal(valueBytes, &value); err != nil {
				return nil, errors.Wrap(err, "unmarshaling stack output value")
			}
		}

		o[k] = value
	}
	return o, nil
}

func (sess *reconcileStackSession) DestroyStack() error {
	writer := sess.logger.LogWriterInfo("Pulumi Destroy")
	defer contract.IgnoreClose(writer)

	_, err := sess.autoStack.Destroy(context.Background(),
		optdestroy.ProgressStreams(writer),
		optdestroy.UserAgent(execAgent),
	)
	if err != nil {
		return errors.Wrapf(err, "destroying resources for stack '%s'", sess.stack.Stack)
	}

	err = sess.autoStack.Workspace().RemoveStack(context.Background(), sess.stack.Stack)
	if err != nil {
		return errors.Wrapf(err, "removing stack '%s'", sess.stack.Stack)
	}
	return nil
}

// SetupGitAuth sets up the authentication option to use for the git source
// repository of the stack. If neither gitAuth or gitAuthSecret are set,
// a pointer to a zero value of GitAuth is returned — representing
// unauthenticated git access.
func (sess *reconcileStackSession) SetupGitAuth() (*auto.GitAuth, error) {
	gitAuth := &auto.GitAuth{}

	if sess.stack.GitAuth != nil {
		if sess.stack.GitAuth.SSHAuth != nil {
			privateKey, err := sess.resolveResourceRef(&sess.stack.GitAuth.SSHAuth.SSHPrivateKey)
			if err != nil {
				return nil, errors.Wrap(err, "resolving gitAuth SSH private key")
			}
			gitAuth.SSHPrivateKey = privateKey

			if sess.stack.GitAuth.SSHAuth.Password != nil {
				password, err := sess.resolveResourceRef(sess.stack.GitAuth.SSHAuth.Password)
				if err != nil {
					return nil, errors.Wrap(err, "resolving gitAuth SSH password")
				}
				gitAuth.Password = password
			}

			return gitAuth, nil
		}

		if sess.stack.GitAuth.PersonalAccessToken != nil {
			accessToken, err := sess.resolveResourceRef(sess.stack.GitAuth.PersonalAccessToken)
			if err != nil {
				return nil, errors.Wrap(err, "resolving gitAuth personal access token")
			}
			gitAuth.PersonalAccessToken = accessToken
			return gitAuth, nil
		}

		if sess.stack.GitAuth.BasicAuth == nil {
			return nil, errors.New("gitAuth config must specify exactly one of " +
				"'personalAccessToken', 'sshPrivateKey' or 'basicAuth'")
		}

		userName, err := sess.resolveResourceRef(&sess.stack.GitAuth.BasicAuth.UserName)
		if err != nil {
			return nil, errors.Wrap(err, "resolving gitAuth username")
		}

		password, err := sess.resolveResourceRef(&sess.stack.GitAuth.BasicAuth.Password)
		if err != nil {
			return nil, errors.Wrap(err, "resolving gitAuth password")
		}

		gitAuth.Username = userName
		gitAuth.Password = password
	} else if sess.stack.GitAuthSecret != "" {
		namespacedName := types.NamespacedName{Name: sess.stack.GitAuthSecret, Namespace: sess.namespace}

		// Fetch the named secret.
		secret := &corev1.Secret{}
		if err := sess.kubeClient.Get(context.TODO(), namespacedName, secret); err != nil {
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
func (sess *reconcileStackSession) addDefaultPermalink(stack *pulumiv1alpha1.Stack) error {
	namespacedName := types.NamespacedName{Name: stack.Name, Namespace: stack.Namespace}
	err := sess.getLatestResource(stack, namespacedName)
	if err != nil {
		sess.logger.Error(err, "Failed to get latest Stack to update Stack Permalink URL", "Stack.Name", stack.Spec.Stack)
		return err
	}
	// Get stack URL.
	info, err := sess.autoStack.Info(context.Background())
	if err != nil {
		sess.logger.Error(err, "Failed to update Stack status with default permalink", "Stack.Name", stack.Spec.Stack)
		return err
	}
	// Set stack URL.
	if stack.Status.LastUpdate == nil {
		stack.Status.LastUpdate = &pulumiv1alpha1.StackUpdateState{}
	}
	stack.Status.LastUpdate.Permalink = pulumiv1alpha1.Permalink(info.URL)
	err = sess.updateResourceStatus(stack)
	if err != nil {
		sess.logger.Error(err, "Failed to update Stack status with default permalink", "Stack.Name", stack.Spec.Stack)
		return err
	}
	sess.logger.Debug("Successfully updated Stack with default permalink", "Stack.Name", stack.Spec.Stack)
	return nil
}

func (sess *reconcileStackSession) getLatestResource(o client.Object, namespacedName types.NamespacedName) error {
	return sess.kubeClient.Get(context.TODO(), namespacedName, o)
}

func (sess *reconcileStackSession) updateResource(o client.Object) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return sess.kubeClient.Update(context.TODO(), o)
	})
}

func (sess *reconcileStackSession) updateResourceStatus(o client.Object) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return sess.kubeClient.Status().Update(context.TODO(), o)
	})
}

func (sess *reconcileStackSession) waitForDeletion(o client.Object) error {
	key := client.ObjectKeyFromObject(o)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return wait.PollImmediateUntil(time.Millisecond*10, func() (bool, error) {
		err := sess.getLatestResource(o, key)
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	}, ctx.Done())
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
		return errors.Wrap(err, "error parsing project repo URL to use with ssh-keyscan")
	}
	hostPort := strings.Split(u.Host, ":")
	if len(hostPort) == 0 || len(hostPort) > 2 {
		return errors.Wrap(err, "error parsing project repo URL to use with ssh-keyscan")
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
		return errors.Wrap(err, "error running ssh-keyscan")
	}

	// Add the repo public keys to the SSH known hosts to enforce key checking.
	filename := fmt.Sprintf("%s/%s", os.Getenv("HOME"), ".ssh/known_hosts")
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return errors.Wrap(err, "error running ssh-keyscan")
	}
	defer f.Close()
	if _, err = f.WriteString(stdout); err != nil {
		return errors.Wrap(err, "error running ssh-keyscan")
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
	kubeFp := os.ExpandEnv("$HOME/.kube")
	kubeconfigFp := fmt.Sprintf("%s/config", kubeFp)

	cert, err := waitForFile(certFp)
	if err != nil {
		return errors.Wrap(err, "failed to open in-cluster ServiceAccount CA certificate")
	}
	token, err := waitForFile(tokenFp)
	if err != nil {
		return errors.Wrap(err, "failed to open in-cluster ServiceAccount token")
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
  name: local
current-context: local
kind: Config
users:
- name: local
  user:
    token: %s
`, string(base64.StdEncoding.EncodeToString(cert)), os.ExpandEnv("$KUBERNETES_PORT_443_TCP_ADDR"), string(token))

	err = os.Mkdir(os.ExpandEnv(kubeFp), 0755)
	if err != nil {
		return errors.Wrap(err, "failed to create .kube directory")
	}
	file, err := os.Create(os.ExpandEnv(kubeconfigFp))
	if err != nil {
		return errors.Wrap(err, "failed to create kubeconfig file")
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
		return nil, errors.Wrapf(err, "failed to open file: %s", fp)
	}
	return file, err
}
