![Pulumi Kubernetes Operator](https://github.com/pulumi/pulumi-kubernetes-operator/workflows/Pulumi%20Kubernetes%20Operator/badge.svg?branch=master)
# Pulumi Kubernetes Operator

A Kubernetes operator that deploys Pulumi updates by cloning Git repos and running Pulumi programs.

## Overview

* [Deploy the Operator](#deploy-the-operator)
  - [Using kubectl](#using-kubectl)
  - [Using Pulumi](#using-pulumi)
* [Create a Pulumi Stack CustomResource](#create-a-pulumi-stack-customresource)
  - [Using kubectl](#using-kubectl-1)
  - [Using Pulumi](#using-pulumi-1)
* [Development](#development)

## Deploy the Operator

Deploy the operator to a Kubernetes cluster.

You can use an existing cluster, or [get started](https://www.pulumi.com/docs/get-started/kubernetes/) by creating a new [managed Kubernetes cluster](https://www.pulumi.com/docs/tutorials/kubernetes/#clusters).

### Using kubectl

Deploy the API resources for the operator.

```bash
kubectl apply -f deploy/yaml
```

### Using Pulumi

- Typescript - [TODO](https://github.com/pulumi/pulumi-kubernetes-operator/issues/43)
- Python - [TODO](https://github.com/pulumi/pulumi-kubernetes-operator/issues/44)
- Go - [TODO](https://github.com/pulumi/pulumi-kubernetes-operator/issues/45)
- .NET - [TODO](https://github.com/pulumi/pulumi-kubernetes-operator/issues/46)

## Create Pulumi Stack CustomResources

The following are examples to create Pulumi Stacks in Kubernetes that are managed and run by the operator.

If you'd like to use your own Pulumi Stack, ensure that you have an existing Pulumi program in a git repo,
and update the CR with:
  - An existing github `project` and `commit`,
  - A Pulumi `stack` name that exists, or must be created if `initOnCreate: true` is set,
  - A Kubernetes Secret for your Pulumi API `accessToken`,
  - A Kubernetes Secret for other sensitive settings like cloud provider credentials, and
  - Environment variables and stack config needed.

### Using kubectl

Check out [Create Pulumi Stacks using `kubectl` ](./docs/create-stacks-using-kubectl.md).

### Using Pulumi

Check out [Create Pulumi Stacks using Pulumi](./docs/create-stacks-using-pulumi.md).

## Development

Check out [docs/build.md](./docs/build.md) for more details on building and
working with the operator locally.
