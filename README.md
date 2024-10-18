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
    - [Using kubectl](#using-kubectl)
    - [Using Pulumi](#using-pulumi)
    - [Using Helm](#using-helm)
  - [Create Pulumi Stack CustomResources](#create-pulumi-stack-customresources)
    - [Using kubectl](#using-kubectl-1)
    - [Using Pulumi](#using-pulumi-1)
    - [Extended Examples](#extended-examples)
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

### Using kubectl

First, download the [latest release](https://github.com/pulumi/pulumi-kubernetes-operator/releases) `source code` tar ball and expand it locally.

Install the operator:

```bash
kubectl apply -f deploy/yaml
```

This will deploy the operator to the `pulumi-kubernetes-operator` namespace.

### Using Pulumi

First, make sure you have installed Pulumi as described in ["Download & install Pulumi"](https://www.pulumi.com/docs/iac/download-install/).

Use the Pulumi program located in `deploy/deploy-operator-yaml` to install the Operator cluster-wide with default settings.

```bash
cd deploy/deploy-operator-yaml
pulumi up
```

### Using Helm

A Helm chart is provided in `deploy/helm/pulumi-operator`, offering more customization options.

```bash
cd deploy/helm/pulumi-operator
helm install pulumi-kubernetes-operator -n pulumi-kubernetes-operator .
```

## Create Pulumi Stack CustomResources

The following are examples to create Pulumi Stacks in Kubernetes that are managed and run by the operator.

### Using kubectl

Check out [Create Pulumi Stacks using `kubectl` ](./docs/create-stacks-using-kubectl.md) for YAML examples.

### Using Pulumi

Check out [Create Pulumi Stacks using Pulumi](./docs/create-stacks-using-pulumi.md) for Typescript, Python, Go, and .NET examples.

### Extended Examples

- [Managing a Kubernetes Blue/Green Deployment](./examples/blue-green)
- [AWS S3 Buckets](./examples/aws-s3)

If you'd like to use your own Pulumi Stack, ensure that you have an existing Pulumi program in a git repo,
and update the CR with:
  - An existing github `project` and/or `commit`,
  - A Pulumi `stack` name that exists and will be selected, or a new stack that will be created and selected.
  - A Kubernetes Secret for your Pulumi API `accessToken`,
  - A Kubernetes Secret for other sensitive settings like cloud provider credentials, and
  - Environment variables and stack config as needed.

## Stack CR Documentation

Detailed documentation on Stack Custom Resource is available [here](./docs/stacks.md).

## Prometheus Metrics Integration

Details on metrics emitted by the Pulumi Kubernetes Operator as instructions on getting them to flow to Prometheus are available [here](./docs/metrics.md).

## Development

Check out [docs/build.md](./docs/build.md) for more details on building and
working with the operator locally.
