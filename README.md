![Pulumi Kubernetes Operator](https://github.com/pulumi/pulumi-kubernetes-operator/workflows/Pulumi%20Kubernetes%20Operator/badge.svg?branch=master)
# Pulumi Kubernetes Operator

A Kubernetes operator that provides a CI/CD workflow for Pulumi stacks using Kubernetes primitives.
To learn more about the Pulumi Kubernetes Operator visit the [Pulumi documentation](https://www.pulumi.com/docs/guides/continuous-delivery/pulumi-kubernetes-operator/).

## Overview

- [Pulumi Kubernetes Operator](#pulumi-kubernetes-operator)
  - [Overview](#overview)
    - [What is Pulumi?](#what-is-pulumi)
    - [When To Use the Pulumi Kubernetes Operator?](#when-to-use-the-pulumi-kubernetes-operator)
    - [Prerequisites](#prerequisites)
      - [Install Pulumi CLI](#install-pulumi-cli)
      - [Login to Your Chosen State Backend](#login-to-your-chosen-state-backend)
  - [Deploy the Operator](#deploy-the-operator)
    - [Using kubectl](#using-kubectl)
    - [Using Pulumi](#using-pulumi)
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

The following steps should be completed before starting on Pulumi:

#### Install Pulumi CLI

Follow the [Pulumi installation instructions](https://www.pulumi.com/docs/get-started/install/) for your OS. For instance, on Mac OS, the easiest way to install Pulumi CLI is from Homebrew:

```shell
$ brew install pulumi
```

#### Login to Your Chosen State Backend

The operator stores additional metadata about provisioned resources. By default, Pulumi (and the Pulumi Kubernetes Operator) uses the [Pulumi managed SaaS backend](https://app.pulumi.com/) to store this state and manage concurrency. 
However, in addition to the managed backend, Pulumi also readily integrates with a variety of state backends, like [S3](https://www.pulumi.com/docs/intro/concepts/state/#logging-into-the-aws-s3-backend), [Azure Blob Storage](https://www.pulumi.com/docs/intro/concepts/state/#logging-into-the-azure-blob-storage-backend), [Google Cloud Storage](https://www.pulumi.com/docs/intro/concepts/state/#logging-into-the-google-cloud-storage-backend), etc. See [here](https://www.pulumi.com/docs/intro/concepts/state/#deciding-on-a-backend) for a detailed discussion on choosing a state backend.

Login to Pulumi using your chosen state backend. For simplicity we will only cover the Pulumi managed SaaS state backend and AWS S3 here:

<details>
<summary> Pulumi SaaS Backend </summary>

```bash
$ pulumi login
```

This will display a prompt that asks for you to provide an access token or automatically request an access token:
```bash
Manage your Pulumi stacks by logging in.
Run `pulumi login --help` for alternative login options.
Enter your access token from https://app.pulumi.com/account/tokens
    or hit <ENTER> to log in using your browser                   :
```

In order to configure the Pulumi Kubernetes Operator to use Stacks with state stored on the SaaS backend, you will also need to manually generate access tokens.
This can be done by accessing the [Access Tokens page](https://app.pulumi.com/account/tokens). Setting the environment variable `PULUMI_ACCESS_TOKEN` to the manually generated token will obviate the need for a `pulumi login`.

At this point your `pulumi` CLI is configured to work with the Pulumi SaaS backend.
</details>

<details>
<summary> AWS S3 Backend </summary>

1. First, you will need to create an S3 bucket manually, either through the [AWS CLI](https://aws.amazon.com/cli/) or the [AWS Console](https://console.aws.amazon.com/).
1. If you have already [configured](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html) the AWS CLI to use credential files, single sign-on etc., Pulumi will automatically respect and use these settings. Alternatively you can set `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables to the access key and secret access key respectively.
1. To use the AWS S3 backend, pass the `s3://<bucket-name>` as your `<backend-url>` to `pulumi login`, i.e.:
   ```
   $ pulumi login s3://<bucket-name>
   ```
   For additional options, refer to the [Pulumi documentation](https://www.pulumi.com/docs/intro/concepts/state/#logging-into-the-aws-s3-backend).
1. You will need the AWS credentials when configuring Stack CRs for stacks you wish to be backed by the S3 bucket.
1. Lastly you will need to [create an AWS Key Management Service (KMS) key](https://docs.aws.amazon.com/kms/latest/developerguide/create-keys.html#create-symmetric-cmk). This key will be used by Pulumi to encrypt secret configuration values or outputs associated with stacks. Pulumi ensures all secrets are stored encrypted in transit and at rest. By default, the SaaS backend creates per-stack encryption keys to do this, however, Pulumi can leverage KMS as one of [several supported encryption providers](https://www.pulumi.com/docs/intro/concepts/secrets/#available-encryption-providers) instead, thus allowing users to self-manage their encryption keys. 
</details>

## Deploy the Operator

Deploy the operator to a Kubernetes cluster.

You can use an existing cluster, or [get started](https://www.pulumi.com/docs/get-started/kubernetes/) by creating a new [managed Kubernetes cluster](https://www.pulumi.com/docs/tutorials/kubernetes/#clusters). We will assume that your target Kubernetes cluster is already created and you have configured `kubectl` to point to it. Note that Pulumi doesn't actually use `kubectl` but for convenience can use the same mechanism to authenticate against clusters.

### Using kubectl

First, download the [latest release](https://github.com/pulumi/pulumi-kubernetes-operator/releases) `source code` tar ball and expand it locally.

Deploy the CustomResourceDefinitions (CRDs) for the operator.

```bash
kubectl apply -f deploy/crds/
```

Deploy the API resources for the operator.

```bash
kubectl apply -f deploy/yaml
```

This will deploy the operator to the default namespace (which depends on your configuration, and is
usually `"default"`). To deploy to a different namespace:

```bash
kubectl apply -n <namespace> -f deploy/yaml
```

You can deploy to several namespaces by repeating the above command.

### Using Pulumi

First, make sure you have reviewed and performed the tasks identified in the [prerequisite section](#prerequisites).

We will create a Pulumi project to deploy the operator by using a template, then customize it if
necessary, then use `pulumi up` to run it. There is a choice of template, related to the programming
language and environment you wish to use:

 - deploy/deploy-operator-cs (.NET)
 - deploy/deploy-operator-go (Go)
 - deploy/deploy-operator-py (Python)
 - deploy/deploy-operator-ts (TypeScript/NodeJS)

Pick one of those, then create a new project in a fresh directory:

```bash
TEMPLATE=deploy/deploy-operator-ts # for example
mkdir deploy-operator
cd deploy-operator
pulumi new https://github.com/pulumi/pulumi-kubernetes-operator/$TEMPLATE
# If using the S3 state backend, you may wish to set the secrets provider here
pulumi stack change-secrets-provider "awskms:///arn:aws:kms:...?region=<region>"
```

You can then set the namespace, or namespaces, in which to deploy the operator:

```bash
pulumi config set namespace ns1
# OR deploy to multiple namespaces
pulumi set config --path namespaces[0] ns1
pulumi set config --path namespaces[1] ns2
```

And finally, run the program:

```bash
pulumi up
```

### Upgrading the operator

For patch and minor version releases, you can just bump the version in your stack config and rerun
`pulumi up`. For example, if the new version is v1.10.2, you would do this:

```bash
pulumi config set operator-version v1.10.2
pulumi up
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
