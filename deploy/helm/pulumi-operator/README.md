# node-red ⚙

![Version: 0.5.0](https://img.shields.io/badge/Version-0.5.0-informational?style=for-the-badge) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=for-the-badge) ![AppVersion: 1.14.0](https://img.shields.io/badge/AppVersion-1.14.0-informational?style=for-the-badge) 

## Description 📜

A Helm chart for the Pulumi Kubernetes Operator

## Usage (via OCI Registry)

To install the chart using the OCI artifact, run:

```bash
helm install pulumi-kubernetes-operator oci://ghcr.io/pulumi/helm-charts/pulumi-kubernetes-operator --version 0.5.0
```

## Usage
Adding `pulumi-kubernetes-operator` repository
Before installing any chart provided by this repository, add the `pulumi-kubernetes-operator` Charts Repository:

```bash
helm repo add pulumi-kubernetes-operator https://pulumi.github.io/pulumi-kubernetes-operator/
helm repo update
```

### Installing the Chart 📦
To install the chart with the release name `pulumi-kubernetes-operator` run:

```bash
helm install pulumi-kubernetes-operator pulumi-kubernetes-operator/pulumi-kubernetes-operator --version 0.5.0
```

After a few seconds, the `pulumi-kubernetes-operator` should be running.

To install the chart in a specific namespace use following commands:

```bash
kubectl create ns pulumi-kubernetes-operator
helm install pulumi-kubernetes-operator pulumi-kubernetes-operator/pulumi-kubernetes-operator --namespace pulumi-kubernetes-operator
```

> **Tip**: List all releases using `helm list`, a release is a name used to track a specific deployment

### Uninstalling the Chart 🗑️

To uninstall the `pulumi-kubernetes-operator` deployment:

```bash
helm uninstall pulumi-kubernetes-operator
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | The affinity constraint |
| controller.args | list | `["--zap-level=error","--zap-time-encoding=iso8601"]` | List of arguments to pass to the operator |
| controller.gracefulShutdownTimeoutDuration | string | `"5m"` | Graceful shutdown timeout duration, default: `5m` |
| controller.kubernetesClusterDomain | string | `"cluster.local"` | Kubernetes Cluster Domain, default: `cluster.local` |
| controller.maxConcurrentReconciles | string | `"10"` | Max concurrent reconciles, default: `10` |
| controller.pulumiInferNamespace | string | `"1"` | Pulumi infer namespace, default: `1` |
| deploymentAnnotations | object | `{}` | Deployment annotations |
| deploymentStrategy | string | `""` | Specifies the strategy used to replace old Pods by new ones, default: `RollingUpdate` |
| extraEnv | list | `[]` | Extra Environments to be passed to the operator |
| extraSidecars | list | `[]` | You can configure extra sidecars containers to run alongside the pulumi-kubernetes-operator pod. default: [] |
| extraVolumeMounts | string | `nil` | Extra Volume Mounts for the pulumi-kubernetes-operator pod |
| extraVolumes | string | `nil` | Extra Volumes for the pod |
| fullnameOverride | string | `""` | String to fully override "pulumi-kubernetes-operator.fullname" |
| image.pullPolicy | string | `"IfNotPresent"` | The image pull policy |
| image.registry | string | `"docker.io"` | The image registry to pull from |
| image.repository | string | `"pulumi/pulumi-kubernetes-operator"` | The image repository to pull from |
| image.tag | string | `""` | The image tag to pull, default: `Chart.appVersion` |
| imagePullSecrets | string | `""` | The image pull secrets |
| initContainers | list | `[]` | containers which are run before the app containers are started |
| nameOverride | string | `""` | Provide a name in place of pulumi-kubernetes-operator |
| nodeSelector | object | `{}` | Node selector |
| podAnnotations | object | `{}` | Pod annotations |
| podLabels | object | `{}` | Labels to add to the pulumi-kubernetes-operator pod. default: {} |
| podSecurityContext | object | `{"fsGroup":1000,"runAsUser":1000}` | Pod Security Context see [values.yaml](values.yaml) |
| podSecurityContext.fsGroup | int | `1000` | pulumi-kubernetes-operator group is 1000 |
| podSecurityContext.runAsUser | int | `1000` | pulumi-kubernetes-operator user is 1000 |
| replicaCount | int | `1` | Specifies the replica count for the deployment |
| resources | object | `{"limits":{"cpu":"500m","memory":"5123Mi"},"requests":{"cpu":"100m","memory":"128Mi"}}` | CPU/Memory resource requests/limits |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"privileged":false,"runAsGroup":10003,"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Security Context see [values.yaml](values.yaml) |
| serviceAccount.annotations | object | `{}` | Additional ServiceAccount annotations |
| serviceAccount.create | bool | `true` | Create service account |
| serviceAccount.name | string | `""` | Service account name to use, when empty will be set to created account if |
| serviceMonitor.enabled | bool | `false` | When set true then use a ServiceMonitor to configure scraping |
| terminationGracePeriodSeconds | int | `300` | Specifies termination grace period, default: `300` |
| tolerations | list | `[]` | Toleration labels for pod assignment |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```bash
helm install pulumi-kubernetes-operator pulumi-kubernetes-operator/pulumi-kubernetes-operator --set image.tag=latest
```

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart. For example,

```bash
helm install pulumi-kubernetes-operator pulumi-kubernetes-operator/pulumi-kubernetes-operator -f values.yaml
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
