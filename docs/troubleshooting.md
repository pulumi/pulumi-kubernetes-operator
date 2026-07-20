# Troubleshooting

## General Approach

For an overview of the general architecture of the Pulumi Kubernetes Operator, see
["What’s New in Pulumi Kubernetes Operator 2.0?"](https://www.pulumi.com/blog/pulumi-kubernetes-operator-2-0/#whats-new-in-pulumi-kubernetes-operator-20).

Here's some tips for troubleshooting issues with stack deployments.

### Watch the Kubernetes events

Use `kubectl` to watch for Kubernetes events in a given namespace:

```bash
kubectl get events -n default --watch
```

Key events to watch for:

- `StackUpdateDetected` - the controller intends to perform a Pulumi stack update (`pulumi up`).
- `ConnectionFailure` - the operator is unable to connect to the workspace pod.
- `InstallationFailure` - the `pulumi install` command is failing within the workspace pod.
- `Initialized` - a workspace pod was successfully initialized with your program source code and is ready to perform stack updates.
- `UpdateSucceeded` - a stack update succeeeded.
- `UpdateFailed` - a stack update failed.
- `StackCreated` - the stack is now up-to-date

### Check the Status information

The Stack object has a rich status field with information about the latest update activity and about conditions
affecting the stack deployment. Use `kubectl describe stack $STACK_NAME` to see the status block.

Look for:
- `.status.lastUpdate` - key information about the last stack update that was applied.
- `.status.currentUpdate` - information about a planned or ongoing stack update.
- `.status.conditions` - conditions affecting the stack, including important messages.

### Monitor the workspace pod logs

The workspace pod is where Pulumi deployment activity happens. Use `kubectl logs` (or [stern](https://github.com/stern/stern))
to see log output from the Pulumi CLI.

### Force a Update

When a stack fails to deploy for any reason, the system applies a back-off retry strategy. After a few tries,
the system waits for an hour or longer before trying again.

It is possible to force the operator to make another attempt, simply by annotating your stack object as shown below. 

```bash
kubectl annotate stack $STACK_NAME "pulumi.com/reconciliation-request=$(date)" --overwrite  
```

### Get a Shell to a Workspace Pod

By default, the operator leaves the workspace pod in-place after running a stack update. Feel free to use the workspace
pod for interactive troubleshooting and to use the Pulumi CLI. For example, to get a shell to the workspace pod for
a Stack named `random-yaml`, use the following command:

```bash
kubectl exec -i -t random-yaml-git-workspace-0 --container pulumi -- bash
```

The default directory is the location of your program source code.  You should be able to run Pulumi CLI commands from there.

### Enable Pulumi Verbose Logging

The Stack API provides a convenient option for increasing Pulumi's log verbosity:

```diff
apiVersion: pulumi.com/v1
kind: Stack
spec:
+  pulumiLogLevel: 10
```

## Specific Issues

### Fetching from Flux Source

The `fetch` init container is responsible for fetching your program source code. When using a Flux source,
the pod connects to the `source-controller` pod in the `flux-system` namespace.

If your cluster has [Network Policy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) enabled,
a policy must be configured to allow the above traffic. Apply this manifest: [operator/config/flux/network_policy.yaml](./operator/config/flux/network_policy.yaml).

#### Flux artifact exceeds the extraction size limit

The `fetch` init container extracts the Flux artifact with a default cap of 100 MiB on the *uncompressed*
contents. A larger artifact fails init with an error such as `bigger than max archive size`.

Raise the cap by setting the `FLUX_MAX_UNTAR_SIZE_BYTES` environment variable on the operator (bytes; `-1`
disables the limit):

- **Helm**: set `flux.maxUntarSizeBytes`, e.g. `--set flux.maxUntarSizeBytes=524288000`.
- **Kustomize / raw manifests**: add the env var to the manager container.
- **Local (`make run`)**: `export FLUX_MAX_UNTAR_SIZE_BYTES=524288000` before running.

### Stack Conflicts

On rare occasion, your Pulumi stack may become locked in the state backend. To unlock your stack:
1. Get a shell to your workspace pod (see above).
2. Run `pulumi cancel`.

### Stack Deletion Hangs When the Namespace Is Terminating

A Stack with `destroyOnFinalize: true` runs `pulumi destroy` in a workspace pod before its finalizer is
removed. Kubernetes will not create new objects — including that workspace pod — in a namespace that is
`Terminating`.

The Stack reports a `Stalled` condition with reason `NamespaceTerminating`, whose message includes the recovery steps 
below.

```bash
kubectl describe stack $STACK_NAME -n $NAMESPACE
# or, just the message:
kubectl get stack $STACK_NAME -n $NAMESPACE \
  -o jsonpath='{.status.conditions[?(@.type=="Stalled")].message}'
```

**Recommended pattern:** do not bundle the Namespace with the Stacks that live in it, so that deleting the workload 
removes the Stack first and lets `pulumi destroy` complete before the namespace is torn down.

**Recovering a stuck Stack:** the cloud resources were never destroyed, so you must clean them up yourself and then
release the Stack so the namespace can finish terminating:

1. Destroy the resources manually — for example, run `pulumi destroy` against the same backend stack from your
   workstation, or delete them in the cloud provider. The permalink in the Stack's stalled message (also at
   `.status.lastUpdate.permalink`) points at the backend stack.
2. Remove the finalizer so Kubernetes can delete the Stack:

```bash
kubectl patch stack $STACK_NAME -n $NAMESPACE --type=merge -p '{"metadata":{"finalizers":[]}}'
```

