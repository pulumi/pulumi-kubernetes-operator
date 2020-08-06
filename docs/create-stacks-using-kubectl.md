# Create Pulumi Stacks using kubectl

## Overview

- [Create an NGINX Deployment in-cluster](#nginx-deployment)
- [Create AWS S3 Buckets](#aws-s3-buckets)

## Introduction

To track the progress of Stack CustomResources (CRs) you can:

Tail the operator logs e.g.,

```bash
kubectl logs pulumi-kubernetes-operator-5488b96dcd-dkmm9 -f
```

Or, you can get the stack details.

```bash
kubectl get stack s3-bucket-stack -o json
```

In the details `stack.status` will show:

- A permalink URL for the Stack in the Pulumi Service when available.
- The last commit `state` of the Pulumi program that has been successfully deployed, and
- Any Pulumi stack outputs that exist. 

```bash
{
    ...
    "status": {
        "lastUpdate": {
            "permalink": "https://app.pulumi.com/metral/s3-op-project/dev/updates/1",
            "state": "bd1edfac28577d62068b7ace0586df595bda33be"
        },
        "outputs": {
            "bucketNames": [
                "my-bucket-0-5f38fc3",
                "my-bucket-1-588d2e8"
            ]
        }
    }
}
```

## NGINX Deployment

Create a NGINX Deployment in-cluster to the operator, using its ServiceAccount.

Check out [`../stack-examples/yaml/nginx_k8s_stack.yaml`](../stack-examples/yaml/nginx_k8s_stack.yaml).

Update the Pulumi API token Secret to use your Pulumi credentials.

Also update the `stack` org to match your account, leaving the stack project name as-is to work with the example repo's `Pulumi.yaml`. 

Deploy the Stack CustomResource:

```
kubectl apply -f ../stack-examples/yaml/nginx_k8s_stack.yaml
```

Get the stack details.

```bash
kubectl get stack nginx-k8s-stack -o json
```

<details>
<summary>Click to expand stack details</summary>

```json
{
    "apiVersion": "pulumi.com/v1alpha1",
    "kind": "Stack",
    "metadata": {
        "finalizers": [
            "finalizer.stack.pulumi.com"
        ],
        "generation": 1,
        "name": "nginx-k8s-stack",
        "namespace": "default",
        "resourceVersion": "12091631",
        "selfLink": "/apis/pulumi.com/v1alpha1/namespaces/default/stacks/nginx-k8s-stack",
        "uid": "83d321cd-cef5-4176-97e0-b4579ad702c0"
    },
    "spec": {
        "accessTokenSecret": "pulumi-api-secret",
        "commit": "2b0889718d3e63feeb6079ccd5e4488d8601e353",
        "destroyOnFinalize": true,
        "initOnCreate": true,
        "projectRepo": "https://github.com/metral/pulumi-nginx",
        "stack": "metral/nginx/dev"
    },
    "status": {
        "lastUpdate": {
            "permalink": "https://app.pulumi.com/metral/nginx/dev/updates/1",
            "state": "2b0889718d3e63feeb6079ccd5e4488d8601e353"
        },
        "outputs": {
            "name": "nginx-043u51ml"
        }
    }
}
```
</details>

Delete the Stack CustomResource, and then its secrets.

If `destroyOnFinalize: true` was set on the Stack when created, it will destroy
the stack's resources and the stack before the CR is deleted.

```bash
kubectl delete stack nginx-k8s-stack
kubectl delete secret pulumi-api-secret
```

## AWS S3 Buckets

Deploys an AWS S3 Buckets Stack and its AWS secrets.

Check out [`../stack-examples/yaml/s3_bucket_stack.yaml`](../stack-examples/yaml/s3_bucket_stack.yaml) to start with a simple exmaple.

Update the Pulumi API token Secret, `stack`, and the cloud provider Secret to use
your Pulumi and AWS credentials.

Deploy the Stack CustomResource:

```
kubectl apply -f ../stack-examples/yaml/s3_bucket_stack.yaml
```

Get the stack details.

```bash
kubectl get stack s3-bucket-stack -o json
```

<details>
<summary>Click to expand stack details</summary>

```json
{
    "apiVersion": "pulumi.com/v1alpha1",
    "kind": "Stack",
    "metadata": {
        "finalizers": [
            "finalizer.stack.pulumi.com"
        ],
        "generation": 1,
        "name": "s3-bucket-stack",
        "namespace": "default",
        "resourceVersion": "10967723",
        "selfLink": "/apis/pulumi.com/v1alpha1/namespaces/default/stacks/s3-bucket-stack",
        "uid": "84166e1e-be47-47f8-8b6c-01474c37485b"
    },
    "spec": {
        "accessTokenSecret": "pulumi-api-secret-itolsj",
        "commit": "bd1edfac28577d62068b7ace0586df595bda33be",
        "config": {
            "aws:region": "us-east-2"
        },
        "destroyOnFinalize": true,
        "envSecrets": [
            "pulumi-aws-secrets-ont5hl"
        ],
        "initOnCreate": true,
        "projectRepo": "https://github.com/metral/test-s3-op-project",
        "stack": "metral/s3-op-project/dev"
    },
    "status": {
        "lastUpdate": {
            "permalink": "https://app.pulumi.com/metral/s3-op-project/dev/updates/1",
            "state": "bd1edfac28577d62068b7ace0586df595bda33be"
        },
        "outputs": {
            "bucketNames": [
                "my-bucket-0-5f38fc3",
                "my-bucket-1-588d2e8"
            ]
        }
    }
}
```
</details>

Now, you can make a change to the CR - like changing the `commit` to deploy to a different commit (`cc5442870f1195216d6bc340c14f8ae7d28cf3e2`). Applying this to the cluster will drive a Pulumi deployment to update the stack.


```bash
kubectl apply -f ../stack-examples/yaml/s3_bucket_stack.yaml
```

Delete the Stack CustomResource, and then its secrets.

If `destroyOnFinalize: true` was set on the Stack when created, it will destroy
the stack's resources and the stack before the CR is deleted.

```bash
kubectl delete stack s3-bucket-stack
kubectl delete secret pulumi-api-secret
```

Check out [`../stack-examples/yaml/ext_s3_bucket_stack.yaml`](../stack-examples/yaml/ext_s3_bucket_stack.yaml) for an extended options exmaple.

## Troubleshooting

Check out [troubleshooting](./troubleshooting.md) for more details.
