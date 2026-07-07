# Manually setting health checks on Argo CD for Pulumi Stacks

To have your Argo setup benefit from the Stack CR's status messages, you can add a small Lua **health check**
to your `argocd-cm` ConfigMap.

> **Heads up:** this manual step is only needed until the health check ships *bundled*
> with Argo CD. Once `resource_customizations/pulumi.com/Stack` lands upstream in
> [`argoproj/argo-cd`](https://github.com/argoproj/argo-cd), every Argo user gets it for
> free and you can skip this page.

## What you get

| Stack condition | Argo health | UI |
|---|---|---|
| `Ready=True` | **Healthy** | green |
| `Reconciling=True` | **Progressing** | blue spinner |
| `Stalled=True` | **Degraded** | red |

## Prerequisites

- **Argo CD** installed (the `argocd-cm` ConfigMap lives in its namespace, usually `argocd`).
- **Pulumi Kubernetes Operator** — a version >v2.7.0, that reports `Stalled` for persistently
  failing updates
- Stacks delivered through Argo (Argo syncs the `Stack`/`Program` CRs; the operator does
  the actual infrastructure work).

## 1. Add the health check

Add this key to the `argocd-cm` ConfigMap under `data:`:

```yaml
data:
  resource.customizations.health.pulumi.com_Stack: |
    local hs = {}
    hs.status = "Progressing"
    hs.message = "Waiting for the stack to be reconciled"
    if obj.status ~= nil and obj.status.conditions ~= nil then
      for _, condition in ipairs(obj.status.conditions) do
        if condition.type == "Stalled" and condition.status == "True" then
          hs.status = "Degraded"
          hs.message = condition.message
          return hs
        end
        if condition.type == "Reconciling" and condition.status == "True" then
          hs.status = "Progressing"
          hs.message = condition.message
          return hs
        end
        if condition.type == "Ready" and condition.status == "True" then
          hs.status = "Healthy"
          hs.message = condition.message
          return hs
        end
      end
    end
    return hs
```

## 2. Apply it

Save the key above into a patch file — for example `argocd-cm-patch.yaml`:

```yaml
data:
  resource.customizations.health.pulumi.com_Stack: |
    # ...the health-check Lua from step 1...
```

Then merge-patch it in:

```bash
kubectl -n argocd patch configmap argocd-cm --type merge --patch-file argocd-cm-patch.yaml
```

Argo watches `argocd-cm` and reloads automatically.

## 3. Verify

With a Stack managed by Argo:

```bash
# via the Argo CLI
argocd app get <app> -o wide        # the Stack row should show Healthy / Progressing / Degraded

# or straight from the cluster
kubectl -n <ns> get stack <name> \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status}/{.reason}  {.message}{"\n"}{end}'
```

In the Argo UI, click the **Stack node** — its health heart reflects the mapping above.

## See also

- Argo CD: [Resource Health](https://argo-cd.readthedocs.io/en/stable/operator-manual/health/)
- Stack status conditions are defined in `operator/api/pulumi/v1/stack_types.go`.
