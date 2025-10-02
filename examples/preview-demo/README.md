# Preview Mode Example

This example demonstrates the `preview` field on Stack resources, which allows you to run preview-only operations without applying changes. This is useful for:

- Validating proposed infrastructure changes before deployment
- Running "what-if" analysis on configuration changes
- Comparing current and proposed states side-by-side

## Architecture

This example creates two Stack resources pointing to the **same Pulumi stack** (`nginx-preview-demo`) but with different configurations. This demonstrates how the `preview` flag allows you to see what changes would be made to the same stack before applying them.

1. **nginx-current**: The "production" Stack resource with `preview: false` (default)
   - Pulumi stack: `nginx-preview-demo`
   - Configuration: 2 replicas
   - Behavior: Performs actual deployments (pulumi up)

2. **nginx-next**: The preview Stack resource with `preview: true`
   - Pulumi stack: `nginx-preview-demo` (same as above)
   - Configuration: 3 replicas (proposed change)
   - Behavior: Only runs previews, never applies changes (pulumi preview)
   - **Prerequisite**: `nginx-current` - ensures the current stack is deployed and successful before running preview

Both Stack resources point to the same Pulumi stack in the backend, making it easy to compare the current state (maintained by nginx-current) with proposed changes (previewed by nginx-next). They use an inline Program resource (`nginx-deployment`) which deploys a simple Nginx deployment (nginx:1.27-alpine) with configurable replica count.

The `prerequisites` field on nginx-next creates an explicit dependency, ensuring that:
- nginx-current must be in a successful state before nginx-next runs its preview
- nginx-next automatically reruns its preview when nginx-current updates
- The preview always compares against the latest deployed state

This approach is helpful for previewing configuration changes, and can also be used to preview program changes by comparing across different Git refs (branches, tags, or commits), and to preview changes to the Pulumi CLI and other dependencies.

## Prerequisites

- Kubernetes cluster with the Pulumi Kubernetes Operator installed
- Pulumi Cloud access token stored in a Secret named `pulumi-api-secret`

```bash
kubectl create secret generic pulumi-api-secret \
  --from-literal=accessToken=$PULUMI_ACCESS_TOKEN
```

## Usage

Deploy both stacks:

```bash
kubectl apply -f stack.yaml
```

Watch the stacks reconcile:

```bash
kubectl get stacks -w
```

Note: nginx-current will reconcile first. Once it reaches a successful state, nginx-next will automatically run its preview due to the prerequisite dependency.

## Observing the Difference

### Current Stack (nginx-current)

The current stack will perform actual deployments:

```bash
# View the stack status
kubectl get stack nginx-current -o yaml

# Check the deployed resources
kubectl get deployments -l app=nginx
```

The stack status will show:
- `lastUpdate.kind: up` - indicating an actual deployment
- Deployed resources in the cluster
- Stack outputs from the deployment

### Preview Stack (nginx-next)

The preview stack will only show what *would* happen:

```bash
# View the preview stack status
kubectl get stack nginx-next -o yaml
```

The stack status will show:
- `lastUpdate.kind: preview` - indicating a preview operation
- Changes that would be made (in `lastUpdate.result`)
- No actual resources deployed

## Comparing Current vs. Next

Both Stack resources operate on the same Pulumi stack (`nginx-preview-demo`), which serves as the basis for comparison:

- **nginx-current** maintains the actual state in the stack with 2 replicas
- **nginx-next** previews what would happen if we changed to 3 replicas

To see the proposed changes:

```bash
# View the current deployment
kubectl describe deployment nginx

# View the preview results to see what would change
kubectl get stack nginx-next -o jsonpath='{.status.lastUpdate.result}' | jq
```

The preview compares against the current state maintained by nginx-current and shows that changing replicas from 2 to 3 would:
- Update the deployment's replica count
- Preserve other configuration unchanged

Since both Stack resources point to the same Pulumi stack, the preview accurately reflects the diff between the current deployed state and the proposed configuration.

## Accepting the Previewed Changes

After reviewing the preview and deciding to accept the changes,
update the `nginx-current` Stack resource to match the configuration that was previewed:

```yaml
# In stack.yaml, change nginx-current's config
spec:
  config:
    replicas: "3"  # Changed from "2"
```

Then apply:

```bash
kubectl apply -f stack.yaml
```

This will trigger nginx-current to perform an actual deployment with 3 replicas. Since both Stack resources now have the same configuration, nginx-next will show no changes in its next preview.

## Use Cases

1. **Configuration Testing**: Change the `config.replicas` value in nginx-next to test different scaling scenarios
2. **Program Validation**: Update the `branch` or `commit` fields to preview changes from a different version
3. **Safe Experimentation**: Modify nginx-next's configuration freely without affecting the running deployment
4. **Change Approval Workflow**: Use nginx-next for automated preview in CI/CD, then promote changes to nginx-current after approval

## Cleanup

```bash
kubectl delete -f stack.yaml
```

The `destroyOnFinalize: true` setting ensures that the nginx-current stack's resources are cleaned up when deleted.
