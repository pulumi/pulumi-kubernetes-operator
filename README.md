# Pulumi Kubernetes Operator

A simple operator that deploys Pulumi updates by cloning Git repos and running Pulumi programs. This is
a hacky prototype to explore the idea, experimental, and not ready for prime time.

## Overview

- [Operator Design Doc](https://docs.google.com/document/d/1cXsamgIbiF7QDXz4mQ7tBpowUt6vgdXY1ZpRcCwF9Pk/edit#)
- Kubernetes Custom Resource Definitions (CRD): A custom Kubernetes API type.
- Kubernetes Custom Resources (CR) - an instance of a Custom Resource Definition.

## Project Layout

### Stack CRD and CR

A custom Kubernetes API type to implement a Pulumi Stack, it's configuration,
and job run settings.

- [CRD API type](./pkg/apis/pulumi/v1alpha1/stack_types.go)
- [Generated CRD YAML Manifest](./deploy/crds/pulumi.com_stacks_crd.yaml)
- [Generated CR YAML Manifest](./deploy/crds/pulumi.com_v1alpha1_stack_cr.yaml)

### Stack Controller

A controller that registers the Stack CRD, and manages user-created CRs by
creating a Kubernetes Job for each Stack CR that is attempted to run until
completion of the Pulumi update execution run.

- [Stack Controller](./pkg/controller/stack/stack_controller.go)

### Operator

A managed Kubernetes application that uses the Stack Controller to operate the
Stack CRD and user-created Stack CRs.

- [Deployment - Generated YAML Manifest](./deploy/operator.yaml)
- [Role - Generated YAML Manifest](./deploy/role.yaml)
- [RoleBinding - Generated YAML Manifest](./deploy/role_binding.yaml)
- [ServiceAccount - Generated YAML Manifest](./deploy/service_account.yaml)

## Official Operator SDK Docs

- [Quickstart](https://sdk.operatorframework.io/docs/golang/quickstart/)
- [Project Scaffolding](https://sdk.operatorframework.io/docs/golang/references/project-layout/)

## Walkthrough

Install the [`operator-sdk`][operator-sdk] to build and run locally.  

Ensure generated CRDs and controller logic is up to date by running:

```
$ make build
```

Install the CRD in your cluster.  

```
$ make install-crds
```

To build and install together, simply run `make`.

Push the built image to DockerHub.

```bash
$ make push-image
```

Currently, the docker image is private, so create an imagePullSecret named
`pulumi-kubernetes-operator` in the default namespace for the operator to use.

```bash
kubectl create secret generic pulumi-kubernetes-operator --from-file=.dockerconfigjson=$HOME/.docker/config.json --type=kubernetes.io/dockerconfigjson
```

Deploy the controller locally.

```bash
$ make deploy
```

Ensure that you have an existing GitHub repo, and update `examples/s3_bucket_stack.yaml` to refer to the appropriate github `project` and `commit`, Pulumi `stack` name, and appropriate Pulumi `accessToken` and environment variables needed for the deployment controller.
Once these are ready - deploy the `pulumi.com/v1alpha1.Stack`:

```
$ kubectl apply -f examples/s3_bucket_stack.yaml
```

Get the stack details.

```bash
$ kubectl get stack s3-bucket-stack -o json
```

<details>
<summary>Click to expand stack details</summary>

```bash
{
    "apiVersion": "pulumi.com/v1alpha1",
    "kind": "Stack",
    "metadata": {
        "annotations": {
            "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"pulumi.com/v1alpha1\",\"kind\":\"Stack\",\"metadata\":{\"annotations\":{},\"name\":\"s3-bucket-stack-02\",\"namespace\":\"default\"},\"spec\":{\"accessTokenSecret\":\"pulumi-api-secret\",\"commit\":\"bd1edfac28577d62068b7ace0586df595bda33be\",\"config\":{\"aws:region\":\"us-east-2
\"},\"envSecrets\":[\"pulumi-aws-secrets\"],\"initOnCreate\":true,\"projectRepo\":\"https://github.com/metral/test-s3-op-project\",\"stack\":\"metral/s3-op-project/dev\"}}\n"
        },
        "creationTimestamp": "2020-07-15T23:38:19Z",
        "finalizers": [
            "finalizer.pulumi.example.com"
        ],
        "generation": 1,
        "name": "s3-bucket-stack",
        "namespace": "default",
        "resourceVersion": "4925362",
        "selfLink": "/apis/pulumi.com/v1alpha1/namespaces/default/stacks/s3-bucket-stack",
        "uid": "65a832bf-9b1d-439f-9323-f7c13ceb99d4"
    },
    "spec": {
        "accessTokenSecret": "pulumi-api-secret",
        "commit": "bd1edfac28577d62068b7ace0586df595bda33be",
        "config": {
            "aws:region": "us-east-2"
        },
        "envSecrets": [
            "pulumi-aws-secrets"
        ],
        "initOnCreate": true,
        "projectRepo": "https://github.com/metral/test-s3-op-project",
        "stack": "metral/s3-op-project/dev"
    },
    "status": {
        "lastUpdate": {
            "state": "succeeded"
        },
        "outputs": {
            "bucketNames": [
                "my-bucket-0-c5f59e1",
                "my-bucket-1-941a57c"
            ]
        }
    }
}
```
</details>

Now, you can make a change to the CR - like changing the `commit` to redeploy to a different commit.  Applying this to the cluster will drive a Pulumi deployment to update the stack.

```
$ kubectl apply -f examples/s3_bucket_stack.yaml
```

[operator-sdk]: https://sdk.operatorframework.io/docs/install-operator-sdk/
