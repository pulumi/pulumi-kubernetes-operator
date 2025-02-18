
# Migration Guide (v2.x)

## Installation

We recommend using the provided Helm chart to install the Pulumi Kubernetes Operator.

### Upgrading

Please uninstall the previous (v1.x) version of the operator, using the same installation method that you used to install it.

Be prepared to re-deploy any Stack objects that you have, after editing the specification as described below.

### Logging

The chart value to enable verbose operator logging has changed:

```diff
- controller:
-   args: ["--zap-level=debug"]
+ controller:
+   logLevel: debug
```

### Unsupported: Single-Namespace Installation

In v2, we simplified the installation options to support only a cluster-wide
installation. See ["Add single-namespace deployment mode #690"](https://github.com/pulumi/pulumi-kubernetes-operator/issues/690).

## Stack API

### Service Account

New to v2, each stack is processed in a dedicated pod called a _workspace pod_.  You must create and assign a pod service account to each stack:

```diff
apiVersion: pulumi.com/v1
kind: Stack
spec:
+  serviceAccountName: my-stack
```

At minimum, the service account must be granted the `system:auth-delegator` cluster role, to allow the workspace to use the API Server to perform security checks. For example, here's how to do that for an account named `default/my-stack`.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-stack
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-stack:system:auth-delegator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: my-stack
  namespace: default
```

If your stack uses the [Pulumi Kubernetes Provider](https://www.pulumi.com/registry/packages/kubernetes/#pulumi-kubernetes-provider), you should assign extra permissions
to the service account to be able to deploy the resources. For example, here's how to
assign the `cluster-admin` role:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-stack:cluster-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: my-stack
  namespace: default
```

### Workspace Template

The Stack API has a new section called `workspaceTemplate` for customizing the workspace pod. For example:

- pod resources (cpu/memory)
- pod image (default: `pulumi/pulumi:latest-nonroot`)
- security profile (`restricted` or `baseline`)
- additional volumes
- additional init/sidecar containers
- affinity rules and tolerations

### Unsupported: Installation-Wide Files and Environment Variables

In v1, all Pulumi commands were run in the operator's own pod, and had access to the operator's own environment variables. Such variables were often configured via the `extraEnv` setting on the Helm chart. 

In v2, since each stack now has a dedicated pod, any environment variables and/or files that are needed must be configured via the Stack API.

For example, to set the `PULUMI_BACKEND_URL` variable, update the Stack specification:
```diff
apiVersion: pulumi.com/v1
kind: Stack
spec:
+   envRefs:
+     PULUMI_BACKEND_URL:
+       type: Literal
+       literal:
+         value: "<s3-bucket-path>"
+     PULUMI_CONFIG_PASSPHRASE:
+       type: Literal
+       literal:
+         value: "passphrase"
```

### Unsupported: Cross-Namespace References

The Stack API (v1) has a few blocks where a namespace may be specified, e.g.
to reference a `Secret` in another namespace. These blocks do not function as expected in v2.