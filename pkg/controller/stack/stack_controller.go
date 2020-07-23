package stack

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	pulumiv1alpha1 "github.com/pulumi/pulumi-kubernetes-operator/pkg/apis/pulumi/v1alpha1"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
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

	// Filter out update events if an object's metadata.generation is unchanged.
	//  - https://github.com/operator-framework/operator-sdk/issues/2795
	//  - https://github.com/kubernetes-sigs/kubebuilder/issues/1103
	//  - https://github.com/kubernetes-sigs/controller-runtime/pull/553
	//  - https://book-v1.book.kubebuilder.io/basics/status_subresource.html
	pred := predicate.GenerationChangedPredicate{}

	// Watch for changes to primary resource Stack
	err = c.Watch(&source.Kind{Type: &pulumiv1alpha1.Stack{}}, &handler.EnqueueRequestForObject{}, pred)
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

	// Don't run this loop if already at desired state, unless marked for deletion.
	if instance.Status.LastUpdate != nil &&
		instance.Status.LastUpdate.State == instance.Spec.Commit &&
		!isStackMarkedToBeDeleted {
		reqLogger.Info("Stack already to desired state", "Stack.Commit", instance.Spec.Commit)
		return reconcile.Result{}, nil
	}

	// Fetch the API token from the named secret.
	secret := &corev1.Secret{}
	if err = r.client.Get(context.TODO(),
		types.NamespacedName{Name: stack.AccessTokenSecret, Namespace: request.Namespace}, secret); err != nil {
		reqLogger.Error(err, "Could not find secret for Pulumi API access",
			"Namespace", request.Namespace, "Stack.AccessTokenSecret", stack.AccessTokenSecret)
		return reconcile.Result{}, err
	}

	accessToken := string(secret.Data["accessToken"])
	if accessToken == "" {
		err = errors.New("Secret accessToken data is empty")
		reqLogger.Error(err, "Illegal empty secret accessToken data for Pulumi API access",
			"Namespace", request.Namespace, "Stack.AccessTokenSecret", stack.AccessTokenSecret)
		return reconcile.Result{}, err
	}

	// Create a new reconciliation session.
	sess := newReconcileStackSession(reqLogger, accessToken, stack, r.client, nil)

	// If there are extra environment variables, read them in now and use them for subsequent commands.
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

	// Step 1. Clone the repo.
	workdir, err := sess.FetchProjectSource(stack.ProjectRepo, &pulumiv1alpha1.ProjectSourceOptions{
		AccessToken: stack.ProjectRepoAccessTokenSecret,
		Commit:      stack.Commit,
		Branch:      stack.Branch,
	})
	if err != nil {
		return reconcile.Result{}, err
	}
	sess.workdir = workdir
	if stack.RepoDir != "" {
		sess.workdir = path.Join(sess.workdir, stack.RepoDir)
	}
	defer os.RemoveAll(workdir)

	// Step 2. Select the right stack and populate config if supplied.
	if err = sess.SetupPulumiWorkdir(); err != nil {
		reqLogger.Error(err, "Failed to change to Pulumi workdir", "Stack.Name", stack.Stack, "Workdir", workdir)
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
			// Requeue after adding finalizer as not doing so can create competing loops.
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	// Step 3. If a stack refresh is requested, run it now.
	if sess.stack.Refresh {
		sess.RefreshStack(sess.stack.ExpectNoRefreshChanges)
	}

	// Step 4. Run a `pulumi up --skip-preview`.
	// TODO: is it possible to support a --dry-run with a preview?
	status, err := sess.UpdateStack()
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
			instance.Status.LastUpdate = &pulumiv1alpha1.StackUpdateState{
				State: "failed",
			}
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
	outs, err := sess.GetStackOutputs()
	if err != nil {
		reqLogger.Error(err, "Failed to get Stack outputs", "Stack.Name", stack.Stack)
		return reconcile.Result{}, err
	}
	err = sess.getLatestResource(instance, request.NamespacedName)
	if err != nil {
		sess.logger.Error(err, "Failed to get latest Stack to update successful Stack status", "Stack.Name", instance.Spec.Stack)
		return reconcile.Result{}, err
	}
	instance.Status.Outputs = outs
	instance.Status.LastUpdate = &pulumiv1alpha1.StackUpdateState{
		State: instance.Spec.Commit,
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
	logger      logr.Logger
	kubeClient  client.Client
	accessToken string
	stack       pulumiv1alpha1.StackSpec
	workdir     string
	extraEnv    map[string]string
}

// blank assignment to verify that reconcileStackSession implements pulumiv1alpha1.StackController.
var _ pulumiv1alpha1.StackController = &reconcileStackSession{}

func newReconcileStackSession(
	logger logr.Logger, accessToken string, stack pulumiv1alpha1.StackSpec,
	kubeClient client.Client, extraEnv map[string]string) *reconcileStackSession {
	return &reconcileStackSession{
		logger:      logger,
		kubeClient:  kubeClient,
		accessToken: accessToken,
		stack:       stack,
		extraEnv:    extraEnv,
	}
}

// FetchProjectSource clones the stack's source repo at the right commit and returns its temporary workdir path.
func (sess *reconcileStackSession) FetchProjectSource(repoURL string, opts *pulumiv1alpha1.ProjectSourceOptions) (string, error) {
	workdir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}

	// TODO: enable use of project source repo accessToken
	sess.logger.Info("Cloning Stack repo",
		"Stack.Name", sess.stack.Stack, "Stack.Repo", repoURL,
		"Stack.Commit", opts.Commit, "Stack.Branch", opts.Branch)

	if err = gitCloneAndCheckoutCommit(repoURL, opts.Commit, opts.Branch, workdir); err != nil {
		return "", err
	}
	return workdir, err
}

// gitCloneAndCheckoutCommit clones the Git repository and checkouts the specified commit hash or branch.
func gitCloneAndCheckoutCommit(url, hash, branch, path string) error {
	repo, err := git.PlainClone(path, false, &git.CloneOptions{URL: url})
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	return w.Checkout(&git.CheckoutOptions{
		Hash:   plumbing.NewHash(hash),
		Branch: plumbing.ReferenceName(branch),
		Force:  true,
	})
}

// SetEnvs populates the environment the stack run with values
// from an array of Kubernetes ConfigMaps in a Namespace.
func (sess *reconcileStackSession) SetEnvs(configMapNames []string, namespace string) error {
	var err error
	if sess.extraEnv == nil {
		sess.extraEnv = make(map[string]string)
	}
	for _, env := range configMapNames {
		config := &corev1.ConfigMap{}
		if err = sess.getLatestResource(config, types.NamespacedName{Name: env, Namespace: namespace}); err != nil {
			return errors.Wrapf(err, "Namespace=%s Name=%s", namespace, env)
		}
		for k, v := range config.Data {
			sess.extraEnv[k] = v
		}
	}
	return nil
}

// SetSecretEnvs populates the environment of the stack run with values
// from an array of Kubernetes Secrets in a Namespace.
func (sess *reconcileStackSession) SetSecretEnvs(secrets []string, namespace string) error {
	var err error
	if sess.extraEnv == nil {
		sess.extraEnv = make(map[string]string)
	}
	for _, env := range secrets {
		config := &corev1.Secret{}
		if err = sess.getLatestResource(config, types.NamespacedName{Name: env, Namespace: namespace}); err != nil {
			return errors.Wrapf(err, "Namespace=%s Name=%s", namespace, env)
		}
		for k, v := range config.Data {
			sess.extraEnv[k] = string(v)
		}
	}
	return nil
}

// runCmd runs the given command with stdout and stderr hooked up to the logger.
func (sess *reconcileStackSession) runCmd(title string, cmd *exec.Cmd) (string, string, error) {
	// If not overridden, set the command to run in the working directory.
	if cmd.Dir == "" {
		cmd.Dir = sess.workdir
	}

	// If there are extra environment variables, set them.
	if sess.extraEnv != nil {
		if len(cmd.Env) == 0 {
			cmd.Env = os.Environ()
		}
		for k, v := range sess.extraEnv {
			cmd.Env = append(cmd.Env, k+"="+v)
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

// pulumi runs a Pulumi CLI command and returns the stdout, stderr, and error, if any.
func (sess *reconcileStackSession) pulumi(args ...string) (string, string, error) {
	sess.logger.Info("Running Pulumi command", "Args", args, "Workdir", sess.workdir)

	// Run the pulumi command in the working directory.
	cmdArgs := []string{"--non-interactive"}
	for _, arg := range args {
		cmdArgs = append(cmdArgs, arg)
	}
	cmd := exec.Command("pulumi", cmdArgs...)
	cmd.Dir = sess.workdir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PULUMI_ACCESS_TOKEN="+sess.accessToken)
	return sess.runCmd("Pulumi CLI", cmd)
}

func (sess *reconcileStackSession) SetupPulumiWorkdir() error {
	// Create the stack if requested.
	if sess.stack.InitOnCreate {
		var secretsProvider *string
		if sess.stack.SecretsProvider != "" {
			secretsProvider = &sess.stack.SecretsProvider
		}
		err := sess.CreateStack(sess.stack.Stack, secretsProvider)
		if err != nil {
			return err
		}
	}
	// Select the desired stack.
	_, _, err := sess.pulumi("stack", "select", sess.stack.Stack)
	if err != nil {
		return errors.Wrap(err, "selecting stack")
	}

	// Update the stack config and secret config values.
	sess.UpdateConfig()
	sess.UpdateSecretConfig()

	// Next we need to install the package manager dependencies for certain languages.
	projbytes, err := ioutil.ReadFile(filepath.Join(sess.workdir, "Pulumi.yaml"))
	if err != nil {
		return errors.Wrap(err, "reading Pulumi.yaml project file")
	}
	var project struct {
		Runtime string `yaml:"runtime"`
	}
	if err = yaml.Unmarshal([]byte(projbytes), &project); err != nil {
		return errors.Wrap(err, "unmarshaling Pulumi.yaml project file")
	}
	if err = sess.InstallProjectDependencies(project.Runtime); err != nil {
		return errors.Wrap(err, "installing project dependencies")
	}

	return nil
}

func (sess *reconcileStackSession) InstallProjectDependencies(runtime string) error {
	switch runtime {
	case "nodejs":
		npm, _ := exec.LookPath("npm")
		if npm == "" {
			npm, _ = exec.LookPath("yarn")
		}
		if npm == "" {
			return errors.New("did not find 'npm' or 'yarn' on the PATH; can't install project dependencies")
		}
		cmd := exec.Command(npm, "install")
		_, _, err := sess.runCmd("NPM/Yarn", cmd)
		return err
	default:
		return errors.Errorf("unsupported project runtime: %s", runtime)
	}
}

func (sess *reconcileStackSession) CreateStack(stack string, secretsProvider *string) error {
	cmdArgs := []string{"stack", "select", "--create"}
	if secretsProvider != nil {
		cmdArgs = append(cmdArgs, "--secrets-provider", *secretsProvider)
	}
	cmdArgs = append(cmdArgs, stack)
	_, stderr, err := sess.pulumi(cmdArgs...)
	if err != nil {
		if strings.Contains(stderr, "already exists") {
			return nil
		}
		return errors.Wrapf(err, "creating stack '%s'", stack)
	}
	return nil
}

func (sess *reconcileStackSession) UpdateConfig() error {
	// Then, populate config if there is any.
	if sess.stack.Config != nil {
		for k, v := range sess.stack.Config {
			_, _, err := sess.pulumi("config", "set", k, v)
			if err != nil {
				return errors.Wrapf(err, "setting config key '%s' to value '%s'", k, v)
			}
		}
	}
	return nil
}

func (sess *reconcileStackSession) UpdateSecretConfig() error {
	// Then, populate secret config if there is any.
	if sess.stack.Secrets != nil {
		for k, v := range sess.stack.Secrets {
			_, _, err := sess.pulumi("config", "set", "--secret", k, v)
			if err != nil {
				return errors.Wrapf(err, "setting secret config key '%s' to value '%s'", k, v)
			}
		}
	}
	return nil
}

func (sess *reconcileStackSession) RefreshStack(expectNoChanges bool) error {
	cmdArgs := []string{"refresh", "--yes"}
	if expectNoChanges {
		cmdArgs = append(cmdArgs, "--expect-no-changes")
	}
	_, _, err := sess.pulumi(cmdArgs...)
	if err != nil {
		return errors.Wrapf(err, "refreshing stack '%s'", sess.stack.Stack)
	}
	return nil
}

// UpdateStack runs the update on the stack and returns an update status code
// and error. In certain cases, an update may be unabled to proceed due to locking,
// in which case the operator will requeue itself to retry later.
func (sess *reconcileStackSession) UpdateStack() (pulumiv1alpha1.StackUpdateStatus, error) {
	_, stderr, err := sess.pulumi("up", "--skip-preview", "--yes")
	if err != nil {
		// If this is the "conflict" error message, we will want to gracefully quit and retry.
		if strings.Contains(stderr, "error: [409] Conflict: Another update is currently in progress") {
			return pulumiv1alpha1.StackUpdateConflict, err
		}
		// If this is the "not found" error message, we will want to gracefully quit and retry.
		if strings.Contains(stderr, "error: [404] Not found") {
			return pulumiv1alpha1.StackNotFound, err
		}
		return pulumiv1alpha1.StackUpdateFailed, err
	}
	return pulumiv1alpha1.StackUpdateSucceeded, nil
}

// GetPulumiOutputs gets the stack outputs and parses them into a map.
func (sess *reconcileStackSession) GetStackOutputs() (pulumiv1alpha1.StackOutputs, error) {
	stdout, _, err := sess.pulumi("stack", "output", "--json")
	if err != nil {
		return nil, errors.Wrap(err, "getting stack outputs")
	}

	// Parse the JSON to ensure it's valid and then encode it as a raw message.
	var outs pulumiv1alpha1.StackOutputs
	if err = json.Unmarshal([]byte(stdout), &outs); err != nil {
		return nil, errors.Wrap(err, "unmarshaling stack outputs (to map)")
	}

	return outs, nil
}

func (sess *reconcileStackSession) DestroyStack() error {
	_, _, err := sess.pulumi("destroy", "--yes")
	if err != nil {
		return errors.Wrapf(err, "destroying resources for stack '%s'", sess.stack.Stack)
	}
	_, _, err = sess.pulumi("stack", "rm", "--yes")
	if err != nil {
		return errors.Wrapf(err, "removing stack '%s'", sess.stack.Stack)
	}
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

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
