package stack

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	libpredicate "github.com/operator-framework/operator-lib/predicate"
	"github.com/pkg/errors"
	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1alpha1"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v2/go/x/auto/optup"
	giturls "github.com/whilp/git-urls"
	git "gopkg.in/src-d/go-git.v4"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_stack")

const pulumiFinalizer = "finalizer.stack.pulumi.com"
const maxConcurrentReconciles = 10 // arbitrary value greater than default of 1

// Add creates a new Stack Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	// Use the ServiceAccount CA cert and token to setup $HOME/.kube/config.
	// This is used to deploy Pulumi Stacks of k8s resources
	// in-cluster that use the default, ambient kubeconfig.
	if err := setupInClusterKubeconfig(); err != nil {
		log.Info("skipping in-cluster kubeconfig setup due to non-existent ServiceAccount", "error", err)
	}
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileStack{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
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

	// Watch for changes to primary resource Stack
	err = c.Watch(&source.Kind{Type: &pulumiv1alpha1.Stack{}}, &handler.EnqueueRequestForObject{}, predicates...)
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
func (r *ReconcileStack) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
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

	var accessToken string
	if stack.AccessTokenSecret != "" {
		// Fetch the API token from the named secret.
		secret := &corev1.Secret{}
		if err = r.client.Get(context.TODO(),
			types.NamespacedName{Name: stack.AccessTokenSecret, Namespace: request.Namespace}, secret); err != nil {
			reqLogger.Error(err, "Could not find secret for Pulumi API access",
				"Namespace", request.Namespace, "Stack.AccessTokenSecret", stack.AccessTokenSecret)
			return reconcile.Result{}, err
		}

		accessToken = string(secret.Data["accessToken"])
		if accessToken == "" {
			err = errors.New("Secret accessToken data is empty")
			reqLogger.Error(err, "Illegal empty secret accessToken data for Pulumi API access",
				"Namespace", request.Namespace, "Stack.AccessTokenSecret", stack.AccessTokenSecret)
			return reconcile.Result{}, err
		}
	}

	// Create a new reconciliation session.
	sess := newReconcileStackSession(reqLogger, accessToken, stack, r.client)

	// Step 1. Set up the workdir, select the right stack and populate config if supplied.
	var gitAuth *auto.GitAuth
	if sess.stack.GitAuthSecret != "" {
		gitAuth, err = sess.SetupGitAuth(request.Namespace)
		if err != nil {
			reqLogger.Error(err, "Failed to setup git authentication", "Stack.Name", stack.Stack)
			return reconcile.Result{}, err
		}
	}
	if err = sess.SetupPulumiWorkdir(gitAuth); err != nil {
		reqLogger.Error(err, "Failed to setup Pulumi workdir", "Stack.Name", stack.Stack)
		return reconcile.Result{}, err
	}

	currentCommit, err := revisionAtWorkingDir(sess.workdir)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Delete the working directory after the reconciliation is completed (regardless of success or failure).
	defer sess.CleanupPulumiWorkdir()

	// Step 2. If there are extra environment variables, read them in now and use them for subsequent commands.
	err = sess.SetEnvs(stack.Envs, request.Namespace)
	if err != nil {
		reqLogger.Error(err, "Could not find ConfigMap for Envs")
		return reconcile.Result{}, err
	}
	err = sess.SetSecretEnvs(stack.SecretEnvs, request.Namespace)
	if err != nil {
		reqLogger.Error(err, "Could not find Secret for SecretEnvs")
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

	// Step 3. If a stack refresh is requested, run it now.
	if sess.stack.Refresh {
		permalink, err := sess.RefreshStack(sess.stack.ExpectNoRefreshChanges)
		if err != nil {
			sess.logger.Error(err, "Failed to refresh stack", "Stack.Name", instance.Spec.Stack)
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
			reqLogger.Info("Conflict with another concurrent update -- will retry shortly", "Stack.Name", stack.Stack, "Err:", err)
			return reconcile.Result{RequeueAfter: time.Second * 5}, nil
		}
		reqLogger.Info("Conflict with another concurrent update -- NOT retrying", "Stack.Name", stack.Stack, "Err:", err)
		return reconcile.Result{}, nil
	case pulumiv1alpha1.StackNotFound:
		reqLogger.Info("Stack not found -- will retry shortly", "Stack.Name", stack.Stack, "Err:", err)
		return reconcile.Result{RequeueAfter: time.Second * 5}, nil
	default:
		if err != nil {
			reqLogger.Error(err, "Failed to update Stack", "Stack.Name", stack.Stack)
			// Update Stack status with failed state
			instance.Status.LastUpdate.LastAttemptedCommit = currentCommit
			instance.Status.LastUpdate.State = pulumiv1alpha1.FailedStackStateMessage
			instance.Status.LastUpdate.Permalink = permalink

			if err2 := sess.updateResourceStatus(instance); err2 != nil {
				msg := "Failed to update status for a failed Stack update"
				err3 := errors.Wrapf(err, err2.Error())
				reqLogger.Error(err3, msg)
				return reconcile.Result{}, err3
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
	reqLogger.Info("Successfully updated successful status for Stack", "Stack.Name", stack.Stack)

	return reconcile.Result{}, nil
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
	sess.logger.Info("Adding Finalizer for the Stack", "Stack.Name", stack.Name)
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
	logger        logr.Logger
	kubeClient    client.Client
	accessToken   string
	stack         pulumiv1alpha1.StackSpec
	autoStack     *auto.Stack
	workdir       string
}

// blank assignment to verify that reconcileStackSession implements pulumiv1alpha1.StackController.
var _ pulumiv1alpha1.StackController = &reconcileStackSession{}

func newReconcileStackSession(
	logger logr.Logger, accessToken string, stack pulumiv1alpha1.StackSpec,
	kubeClient client.Client) *reconcileStackSession {
	return &reconcileStackSession{
		logger:      logger,
		kubeClient:  kubeClient,
		accessToken: accessToken,
		stack:       stack,
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
			sess.logger.Info(title, "Path", cmd.Path, "Args", cmd.Args, "Stdout", text)
			stdout.WriteString(text + "\n")
		}
	}()
	go func() {
		errs := bufio.NewScanner(stderrR)
		for errs.Scan() {
			text := errs.Text()
			sess.logger.Info(title, "Path", cmd.Path, "Args", cmd.Args, "Text", text)
			stderr.WriteString(text + "\n")
		}
	}()

	// Now wait for the command to finish. No matter what, return everything written to stdout and
	// stderr, in addition to the resulting error, if any.
	err = cmd.Wait()
	return stdout.String(), stderr.String(), err
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

	// Create a new workspace.
	secretsProvider := auto.SecretsProvider(sess.stack.SecretsProvider)
	w, err := auto.NewLocalWorkspace(context.Background(), auto.Repo(repo), secretsProvider)
	if err != nil {
		return errors.Wrap(err, "failed to create local workspace")
	}
	if sess.stack.Backend != "" {
		w.SetEnvVar("PULUMI_BACKEND_URL", sess.stack.Backend)
	}
	if sess.accessToken != "" {
		w.SetEnvVar("PULUMI_ACCESS_TOKEN", sess.accessToken)
	}
	sess.workdir = w.WorkDir()

	// Create a new stack if the stack does not already exist, or fall back to
	// selecting the existing stack. If the stack does not exist, it will be created and selected.
	a, err := auto.UpsertStack(context.Background(), sess.stack.Stack, w)
	if err != nil {
		return errors.Wrap(err, "failed to create and/or select stack")
	}
	sess.autoStack = &a

	// Update the stack config and secret config values.
	err = sess.UpdateConfig()
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

func (sess *reconcileStackSession) CleanupPulumiWorkdir() {
	if sess.workdir != "" {
		if err := os.RemoveAll(sess.workdir); err != nil {
			sess.logger.Error(err, "Failed to delete working dir: %s", sess.workdir)
		}
	}
}

// Determine the actual commit information from the working directory (Spec commit etc. is optional).
func revisionAtWorkingDir(workingDir string) (string, error) {
	gitRepo, err := git.PlainOpen(workingDir)
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
		sess.logger.Info(fmt.Sprintf("Handling unknown project runtime '%s'", project.Runtime.Name()), "Stack.Name", sess.stack.Stack)
		return nil
	}
}

func (sess *reconcileStackSession) UpdateConfig() error {
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
	return sess.autoStack.SetAllConfig(context.Background(), m)
}

func (sess *reconcileStackSession) RefreshStack(expectNoChanges bool) (pulumiv1alpha1.Permalink, error) {
	writer := logWriter(sess.logger, "Pulumi Refresh")
	result, err := sess.autoStack.Refresh(
		context.Background(),
		optrefresh.ExpectNoChanges(),
		optrefresh.ProgressStreams(writer),
	)
	if err != nil {
		return pulumiv1alpha1.Permalink(""), errors.Wrapf(err, "refreshing stack '%s'", sess.stack.Stack)
	}
	p, err := auto.GetPermalink(result.StdOut)
	if err != nil {
		return pulumiv1alpha1.Permalink(""), err
	}
	permalink := pulumiv1alpha1.Permalink(p)
	return permalink, nil
}

// UpdateStack runs the update on the stack and returns an update status code
// and error. In certain cases, an update may be unabled to proceed due to locking,
// in which case the operator will requeue itself to retry later.
func (sess *reconcileStackSession) UpdateStack() (pulumiv1alpha1.StackUpdateStatus, pulumiv1alpha1.Permalink, *auto.UpResult, error) {
	writer := logWriter(sess.logger, "Pulumi Update")
	result, err := sess.autoStack.Up(context.Background(), optup.ProgressStreams(writer))
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
		return pulumiv1alpha1.StackUpdateFailed, pulumiv1alpha1.Permalink(""), nil, err
	}
	permalink := pulumiv1alpha1.Permalink(p)
	return pulumiv1alpha1.StackUpdateSucceeded, permalink, &result, nil
}

// GetPulumiOutputs gets the stack outputs and parses them into a map.
func (sess *reconcileStackSession) GetStackOutputs(outs auto.OutputMap) (pulumiv1alpha1.StackOutputs, error) {
	o := make(pulumiv1alpha1.StackOutputs)
	for k, v := range outs {
		// Marshal the OutputMap value only, to use in unmarshaling to StackOutputs
		valueBytes, err := json.Marshal(v.Value)
		if err != nil {
			return nil, errors.Wrap(err, "marshaling stack output value interface")
		}
		var value apiextensionsv1.JSON
		if err := json.Unmarshal(valueBytes, &value); err != nil {
			return nil, errors.Wrap(err, "unmarshaling stack output value")
		}
		o[k] = value
	}
	return o, nil
}

func (sess *reconcileStackSession) DestroyStack() error {
	writer := logWriter(sess.logger, "Pulumi Destroy")
	_, err := sess.autoStack.Destroy(context.Background(), optdestroy.ProgressStreams(writer))
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
// repository of the stack.
func (sess *reconcileStackSession) SetupGitAuth(namespace string) (*auto.GitAuth, error) {
	var err error
	namespacedName := types.NamespacedName{Name: sess.stack.GitAuthSecret, Namespace: namespace}

	// Fetch the named secret.
	secret := &corev1.Secret{}
	if err = sess.kubeClient.Get(context.TODO(), namespacedName, secret); err != nil {
		sess.logger.Error(err, "Could not find secret for access to the git repositoriy",
			"Namespace", namespace, "Stack.GitAuthSecret", sess.stack.GitAuthSecret)
		return nil, err
	}

	var gitAuth *auto.GitAuth

	// First check if an SSH private key has been specified.
	if sshPrivateKey, exists := secret.Data["sshPrivateKey"]; exists {
		// Create a temp file
		tmpfile, err := ioutil.TempFile(os.TempDir(), "gitauth-")
		if err != nil {
			return nil, errors.Wrap(err, "setting up git authentication")
		}
		// TODO: cleanup temp ssh key file. what we need is for GitAuth to
		// accept SshPrivateKey in addition to SshPrivateKeyFile s.t  we don't
		// need to rely on a temp file.
		// See: https://github.com/pulumi/pulumi/issues/5383
		// defer os.Remove(tmpfile.Name())

		// Write to the file
		if _, err = tmpfile.Write(sshPrivateKey); err != nil {
			return nil, errors.Wrap(err, "setting up git authentication")
		}

		// Close the file
		if err := tmpfile.Close(); err != nil {
			return nil, errors.Wrap(err, "setting up git authentication")
		}

		gitAuth = &auto.GitAuth{
			SSHPrivateKeyPath: tmpfile.Name(),
		}

		if password, exists := secret.Data["password"]; exists {
			gitAuth.Password = string(password)
		}

		// Add the project repo's public SSH keys to the SSH known hosts
		// to perform the necessary key checking during SSH git cloning.
		sess.addSSHKeysToKnownHosts(sess.stack.ProjectRepo)
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
			return nil, errors.Wrap(err, "setting up git authentication")
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
	sess.logger.Info("Successfully updated Stack with default permalink", "Stack.Name", stack.Spec.Stack)
	return nil
}

func (sess *reconcileStackSession) getLatestResource(o runtime.Object, namespacedName types.NamespacedName) error {
	return sess.kubeClient.Get(context.TODO(), namespacedName, o)
}

func (sess *reconcileStackSession) updateResource(o runtime.Object) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return sess.kubeClient.Update(context.TODO(), o)
	})
}

func (sess *reconcileStackSession) updateResourceStatus(o runtime.Object) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return sess.kubeClient.Status().Update(context.TODO(), o)
	})
}

func (sess *reconcileStackSession) waitForDeletion(o runtime.Object) error {
	key, err := client.ObjectKeyFromObject(o)
	if err != nil {
		return err
	}

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
	return ioutil.WriteFile(file.Name(), []byte(s), 0644)
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

	file, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file: %s", fp)
	}
	return file, err
}

// logWriter constructs an io.Writer that logs to the provided logr.Logger
func logWriter(logger logr.Logger, msg string, keysAndValues ...interface{}) io.Writer {
	stdoutR, stdoutW := io.Pipe()
	go func() {
		outs := bufio.NewScanner(stdoutR)
		for outs.Scan() {
			text := outs.Text()
			logger.Info(msg, append([]interface{}{"Stdout", text}, keysAndValues...)...)
		}
		err := outs.Err()
		if err != nil {
			logger.Error(err, msg, keysAndValues...)
		}
	}()
	return stdoutW
}
