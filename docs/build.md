# Build

## Project Layout

### Stack CRD and CR

A custom Kubernetes API type to implement a Pulumi Stack, it's configuration,
and job run settings.

- [CRD API type](../pkg/apis/pulumi/v1alpha1/stack_types.go)
- [Generated CRD YAML Manifest](../deploy/crds/pulumi.com_stacks.yaml)

### Stack Controller

A controller that manages user-created Stack CRs by running a Pulumi program until
completion of the update execution run.

- [Stack Controller](./pkg/controller/stack/stack_controller.go)

### The Operator

A managed Kubernetes application that uses the Stack Controller to process
Stack CRs.

- [Deployment - Generated YAML Manifest](./deploy/yaml/operator.yaml)
- [Role - Generated YAML Manifest](./deploy/yaml/role.yaml)
- [RoleBinding - Generated YAML Manifest](./deploy/yaml/role_binding.yaml)
- [ServiceAccount - Generated YAML Manifest](./deploy/yaml/service_account.yaml)

## Requirements

Install the following binaries.

- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl)
- [ginkgo (for testing only)](https://onsi.github.io/ginkgo/)

### Quickly Build

Quickly build the operator to check that it compiles.

This runs a fast build that is dynamically-linked.

```
make build
```

### Install CRD

Codegen and Install the CRD in your existing Kubernetes cluster.

```
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
