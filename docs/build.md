# Build

## Project Layout

### Stack Resource

A custom Kubernetes resource (CR) to deploy and maintain a Pulumi stack, based on 
a program (from a Git repository, a `Program` resource, or a Flux source),
a stack configuration, and other options.

- [Type Definition](../operator/api/pulumi/v1/stack_types.go)
- [CRD Manifest](../deploy/crds/pulumi.com_stacks.yaml)
- [Controller](../operator/internal/controller/pulumi/stack_controller.go)
- [Controller Tests](../operator/internal/controller/pulumi/stack_controller_tests.go)

### Program Resource

A custom Kubernetes resource to define a Pulumi YAML program.

- [Type Definition](../operator/api/pulumi/v1/program_types.go)
- [CRD Manifest](../deploy/crds/pulumi.com_programs.yaml)
- [Controller](../operator/internal/controller/pulumi/program_controller.go)
- [Controller Tests](../operator/internal/controller/pulumi/program_controller_tests.go)

### Workspace Resource

A custom Kubernetes resource (CR) to make workspace pods to serve as an execution environment
for Pulumi deployment operation(s), applied to the given program.

- [Type Definition](../operator/api/auto/v1alpha1/workspace_types.go)
- [CRD Manifest](../deploy/crds/auto.pulumi.com_workspaces.yaml)
- [Controller](../operator/internal/controller/auto/workspace_controller.go)
- [Controller Tests](../operator/internal/controller/auto/workspace_controller_tests.go)

### Update Resource

A custom Kubernetes resource (CR) to execute a Pulumi deployment operation (e.g `pulumi up`, `pulumi destroy`)
in the given workspace pod.

- [Type Definition](../operator/api/auto/v1alpha1/update_types.go)
- [CRD Manifest](../deploy/crds/auto.pulumi.com_updates.yaml)
- [Controller](../operator/internal/controller/auto/update_controller.go)
- [Controller Tests](../operator/internal/controller/auto/update_controller_tests.go)

### The Operator

A managed Kubernetes application that uses controllers to manage Pulumi workloads.

- [Main Entrypoint](../operator/cmd/main.go)
- [Dockerfile](../operator/Dockerfile)
- [CRD Manifests](./deploy/crds/)
- [Quickstart Manifests](./deploy/quickstart/)
- [Helm Chart](./deploy/helm/pulumi-operator/)
- [Pulumi App](./deploy/deploy-operator-yaml/)

## Requirements

Install the following binaries.

- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl)
- [kubebuilder v4.2.0](https://github.com/kubernetes-sigs/kubebuilder/releases/tag/v4.2.0)
- [ginkgo (for testing only)](https://onsi.github.io/ginkgo/)

### Quickly Build

Quickly build the operator to check that it compiles.

This runs a fast build that is dynamically-linked.

```bash
make build
```

### Install CRD

Codegen and Install the CRD in your existing Kubernetes cluster.

```bash
make codegen install-crds
```

### Build & push to DockerHub and Deploy

Builds a Docker image with a statically-linked binary.

```bash
make build-image
```

Push the built image to DockerHub.

```bash
make push-image
```

If the DockerHub repo is private, create an imagePullSecret named
`pulumi-kubernetes-operator` in the namespace for the operator to use.

This will create a Secret based on your default Docker credentials in `$HOME/.docker/config.json`.

```bash
kubectl create secret generic pulumi-kubernetes-operator --from-file=.dockerconfigjson=$HOME/.docker/config.json --type=kubernetes.io/dockerconfigjson
```

Deploy the operator into the cluster via `KUBECONFIG`.

```bash
make deploy
```

### Integration Testing

To execute the test suite of Pulumi Stacks against the operator, run the following:

```bash
make test
```

## Official Operator SDK Docs

- [Quickstart](https://sdk.operatorframework.io/docs/golang/quickstart/)
- [Project Scaffolding](https://sdk.operatorframework.io/docs/golang/references/project-layout/)
