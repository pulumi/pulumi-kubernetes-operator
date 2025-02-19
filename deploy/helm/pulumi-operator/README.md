# Pulumi Kubernetes Operator - Helm Chart

![Version: 2.0.0-rc.1](https://img.shields.io/badge/Version-2.0.0--rc.1-informational?style=for-the-badge) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=for-the-badge) ![AppVersion: v2.0.0-rc.1](https://img.shields.io/badge/AppVersion-v2.0.0--rc.1-informational?style=for-the-badge)

## Description 📜

A Helm chart for the Pulumi Kubernetes Operator

## Usage (via OCI Registry)

To install the chart using the OCI artifact, run:

```bash
helm install --create-namespace -n pulumi-kubernetes-operator pulumi-kubernetes-operator \
    oci://ghcr.io/pulumi/helm-charts/pulumi-kubernetes-operator --version 2.0.0-rc.1
```

After a few seconds, the `pulumi-kubernetes-operator` release should be deployed and running.

> **Tip**: List all releases using `helm list`, a release is a name used to track a specific deployment

### Uninstalling the Chart 🗑️

To uninstall the `pulumi-kubernetes-operator` deployment:

```bash
helm uninstall -n pulumi-kubernetes-operator pulumi-kubernetes-operator
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | The affinity constraint |
| controller.logLevel | string | `"info"` | Log Level ('debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity) |
| deploymentAnnotations | object | `{}` | Deployment annotations |
| deploymentStrategy | string | `""` | Specifies the strategy used to replace old Pods by new ones, default: `RollingUpdate` |
| extraEnv | list | `[]` | Extra Environments to be passed to the operator |
| extraPorts | string | `nil` |  |
| extraSidecars | list | `[]` | You can configure extra sidecar containers to run within the pulumi-kubernetes-operator pod. default: [] |
| extraVolumeMounts | string | `nil` | Extra Volume Mounts for the pulumi-kubernetes-operator pod |
| extraVolumes | string | `nil` | Extra Volumes for the pod |
| fullnameOverride | string | `""` | String to fully override "pulumi-kubernetes-operator.fullname" |
| image.pullPolicy | string | `"IfNotPresent"` | The image pull policy |
| image.registry | string | `"docker.io"` | The image registry to pull from |
| image.repository | string | `"pulumi/pulumi-kubernetes-operator"` | The image repository to pull from |
| image.tag | string | `""` | The image tag to pull, default: `Chart.appVersion` |
| imagePullSecrets | string | `""` | The image pull secrets |
| initContainers | list | `[]` | You can configure extra init containers to run within the pulumi-kubernetes-operator pod. default: [] |
| nameOverride | string | `""` | Provide a name in place of pulumi-kubernetes-operator |
| nodeSelector | object | `{}` | Node selector |
| podAnnotations | object | `{}` | Pod annotations |
| podLabels | object | `{}` | Labels to add to the pulumi-kubernetes-operator pod. default: {} |
| podSecurityContext | object | `{"runAsGroup":65532,"runAsNonRoot":true,"runAsUser":65532}` | Pod Security Context see [values.yaml](values.yaml) |
| podSecurityContext.runAsGroup | int | `65532` | pulumi-kubernetes-operator group is 65532 |
| podSecurityContext.runAsUser | int | `65532` | pulumi-kubernetes-operator user is 65532 |
| rbac.create | bool | `true` | Specifies whether RBAC resources should be created |
| rbac.createClusterAggregationRoles | bool | `true` | Specifies whether aggregation roles should be created to extend the built-in view and edit roles |
| rbac.createClusterRole | bool | `true` | Specifies whether cluster roles and bindings should be created |
| rbac.createRole | bool | `false` | Specifies whether namespaced roles and bindings should be created |
| rbac.extraRules | list | `[]` | Specifies extra rules for the manager role, e.g. for a 3rd party Flux source |
| replicaCount | int | `1` | Specifies the replica count for the deployment |
| resources | object | `{"limits":{"cpu":"200m","memory":"128Mi"},"requests":{"cpu":"200m","memory":"128Mi"}}` | CPU/Memory resource requests/limits |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | Security Context see [values.yaml](values.yaml) |
| serviceAccount.annotations | object | `{}` | Additional ServiceAccount annotations |
| serviceAccount.create | bool | `true` | Create service account |
| serviceAccount.name | string | `""` | Service account name to use, when empty will be set to created account if |
| serviceAnnotations | object | `{}` | Service annotations |
| serviceMonitor.enabled | bool | `false` | When set true then use a ServiceMonitor to configure scraping |
| terminationGracePeriodSeconds | int | `300` | Specifies termination grace period, default: `300` |
| tolerations | list | `[]` | Toleration labels for pod assignment |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```bash
helm install --create-namespace -n pulumi-kubernetes-operator pulumi-kubernetes-operator \
    oci://ghcr.io/pulumi/helm-charts/pulumi-kubernetes-operator --set image.tag=latest
```

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example,

```bash
helm install --create-namespace -n pulumi-kubernetes-operator pulumi-kubernetes-operator \
    oci://ghcr.io/pulumi/helm-charts/pulumi-kubernetes-operator -f values.yaml
```

> **Tip**: You can use the default [values.yaml](values.yaml)

## Contributing 🤝

### Contributing via GitHub

Feel free to join. Checkout the [contributing guide](CONTRIBUTING.md)

## License ⚖️

Apache License, Version 2.0

## Source Code

* <https://github.com/pulumi/pulumi-kubernetes-operator>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| dirien | <engin@pulumi.com> | <https://pulumi.com> |
| Eron Wright | <eron@pulumi.com> |  |
| Ramon Quitales | <ramon@pulumi.com> |  |
