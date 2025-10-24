![Pulumi Kubernetes Operator](https://github.com/pulumi/pulumi-kubernetes-operator/actions/workflows/run-acceptance-tests.yaml/badge.svg?branch=master)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/pulumi-kubernetes-operator)](https://artifacthub.io/packages/search?repo=pulumi-kubernetes-operator)
# Pulumi Kubernetes Operator

A Kubernetes operator that provides a CI/CD workflow for Pulumi stacks using Kubernetes primitives.
To learn more about the Pulumi Kubernetes Operator visit the [Pulumi documentation](https://www.pulumi.com/docs/guides/continuous-delivery/pulumi-kubernetes-operator/).

## Overview

- [Pulumi Kubernetes Operator](#pulumi-kubernetes-operator)
  - [Overview](#overview)
    - [What is Pulumi?](#what-is-pulumi)
    - [When To Use the Pulumi Kubernetes Operator?](#when-to-use-the-pulumi-kubernetes-operator)
    - [Prerequisites](#prerequisites)
  - [Deploy the Operator](#deploy-the-operator)
    - [Using Helm](#using-helm)
    - [Using Pulumi](#using-pulumi)
    - [Dev Install](#dev-install)
    - [From Source](#from-source)
  - [Create Pulumi Stack Resources](#create-pulumi-stack-resources)
    - [Examples](#examples)
  - [Stack CR Documentation](#stack-cr-documentation)
  - [Prometheus Metrics Integration](#prometheus-metrics-integration)
  - [Development](#development)

### What is Pulumi?

Pulumi is an open source infrastructure-as-code tool for creating, deploying, and managing cloud infrastructure in the programming language of your choice. If you are new to Pulumi, please consider visiting the [getting started](https://www.pulumi.com/docs/get-started/) first to familiarize yourself with Pulumi and concepts such as [Pulumi stacks](https://www.pulumi.com/docs/intro/concepts/stack/) and [backends](https://www.pulumi.com/docs/intro/concepts/state/).

### When To Use the Pulumi Kubernetes Operator?

The Pulumi Kubernetes Operator enables Kubernetes users to create a Pulumi Stack as a first-class Kubernetes API resource, and use the StackController to drive the updates. It allows users to adopt a GitOps workflow for managing their cloud infrastructure using Pulumi. This infrastructure includes Kubernetes resources in addition to over 60 cloud providers including AWS, Azure, and Google Cloud. The operator provides an alternative to Pulumi's other CI/CD integrations such as [Github Actions](https://www.pulumi.com/docs/guides/continuous-delivery/github-actions/), [Gitlab CI](https://www.pulumi.com/docs/guides/continuous-delivery/gitlab-ci/), [Jenkins](https://www.pulumi.com/docs/guides/continuous-delivery/jenkins/) etc. See the full list of Pulumi's CI/CD integrations [here](https://www.pulumi.com/docs/guides/continuous-delivery/). Since the Pulumi Kubernetes Operator can be deployed on any Kubernetes cluster, it provides turnkey GitOps functionality for Pulumi users running in self-hosted or restricted settings. The [Kubernetes Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/), lends itself nicely to automation scenarios by driving to the specified state and automatically retrying if transient failures are encountered.

### Prerequisites

The following steps should be completed before using the operator:

## Deploy the Operator

Deploy the operator to a Kubernetes cluster.

You can use an existing cluster, or [get started](https://www.pulumi.com/docs/get-started/kubernetes/) by creating a new [managed Kubernetes cluster](https://www.pulumi.com/docs/tutorials/kubernetes/#clusters). We will assume that your target Kubernetes cluster is already created and you have configured `kubectl` to point to it.

### Using Helm

A Helm chart is provided in `deploy/helm/pulumi-operator` and is also published to [Artifact Hub](https://artifacthub.io/packages/helm/pulumi-kubernetes-operator/pulumi-kubernetes-operator).

```bash
helm install --create-namespace -n pulumi-kubernetes-operator pulumi-kubernetes-operator oci://ghcr.io/pulumi/helm-charts/pulumi-kubernetes-operator
```

### Using Pulumi

First, make sure you have installed Pulumi as described in ["Download & install Pulumi"](https://www.pulumi.com/docs/iac/download-install/).

Use the Pulumi program located in `deploy/deploy-operator-yaml` to install the Operator cluster-wide with default settings.

```bash
cd deploy/deploy-operator-yaml
pulumi up
```

### Dev Install

A simple "quickstart" installation manifest is provided for non-production environments.

Install with `kubectl`:

```
kubectl apply -f https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/refs/tags/v2.3.0/deploy/quickstart/install.yaml
```

### From Source

To build and install the operator from this repository:

1. Build the operator image: `make build-image` (produces `pulumi/pulumi-kubernetes-operator:v2.3.0`).
2. Push or load the image into your cluster's registry.
3. Deploy to your current cluster context: `make deploy`.

This approach deploys a Kustomization directory located at `./operator/config/default`.

## Create Pulumi Stack Resources

The following are examples to create Pulumi Stacks in Kubernetes that are managed and run by the operator.

Some of the examples use Pulumi Cloud as a state backend, and require that a Pulumi access token
be stored into a Kubernetes Secret. For example, here's now to create a secret from your `PULUMI_ACCESS_TOKEN` environment variable.

```bash
kubectl create secret generic -n default pulumi-api-secret --from-literal=accessToken=$PULUMI_ACCESS_TOKEN
```

### Examples

Working with sources:

- [Git repositories](./examples/git-source)
- [Flux sources](./examples/flux-source)
- [Program resources](./examples/program-source)
- [Custom sources](./examples/custom-source)

Better together with Pulumi IAC:

- [Pulumi (TypeScript)](./examples/pulumi-ts)

Advanced configurations:

- [Workspace customization](./examples/custom-workspace)

## Stack CR Documentation

Detailed documentation on Stack Custom Resource is available [here](./docs/stacks.md).

## Prometheus Metrics Integration

Details on metrics emitted by the Pulumi Kubernetes Operator as instructions on getting them to flow to Prometheus are available [here](./docs/metrics.md).

## Development

Check out [docs/build.md](./docs/build.md) for more details on building and
working with the operator locally.
