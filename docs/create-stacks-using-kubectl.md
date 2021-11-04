# Create Pulumi Stacks using kubectl

## Overview

- [Create Pulumi Stacks using kubectl](#create-pulumi-stacks-using-kubectl)
  - [Overview](#overview)
  - [Introduction](#introduction)
  - [NGINX Deployment](#nginx-deployment)
    - [Create Stack](#create-stack)
      - [With Pulumi SaaS Backend](#with-pulumi-saas-backend)
      - [With S3 State Backend](#with-s3-state-backend)
    - [Get the stack details](#get-the-stack-details)
    - [Delete the Stack and Cleanup](#delete-the-stack-and-cleanup)
  - [AWS S3 Buckets](#aws-s3-buckets)
    - [Create Stack](#create-stack-1)
      - [With Pulumi SaaS Backend](#with-pulumi-saas-backend-1)
      - [With S3 Backend](#with-s3-backend)
    - [Get the stack details](#get-the-stack-details-1)
    - [Delete the Stack and Cleanup](#delete-the-stack-and-cleanup-1)
  - [Troubleshooting](#troubleshooting)

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

Create an NGINX Deployment in the same cluster as the operator, using its ServiceAccount.

### Create Stack
Based on what backend you have chosen, choose from one of the following set of instructions:

#### With Pulumi SaaS Backend

When using the Pulumi SaaS backend:
1. Download  [`nginx_k8s_stack.yaml`](../stack-examples/yaml/nginx_k8s_stack.yaml).
1. Update the Pulumi API token Secret to use your Pulumi credentials.
1. Update the `stack` org to match your account, leaving the stack project name as-is to work with the example repo's `Pulumi.yaml`. 
1. Deploy the Stack CustomResource:
   ```
   kubectl apply -f nginx_k8s_stack.yaml
   ```

#### With S3 State Backend

When using the S3 Bucket backed state backend:
1. Download [`s3backend/nginx_k8s_stack.yaml`](../stack-examples/yaml/s3backend/nginx_k8s_stack.yaml).
1. Update `backend` reference in the spec to refer to the S3 bucket where state should be stored.
1. Update the `aws-creds-secret` secret to refer to AWS credentials necessary to access the state backend bucket.
1. Update the `KMS Key ARN` and region to refer to the KMS key to use as a secrets encryption provider.
1. Deploy the Stack CustomResource:
   ```
   kubectl apply -f s3backend/nginx_k8s_stack.yaml
   ```

### Get the stack details

```bash
kubectl get stack nginx-k8s-stack -o json
```

<details>
<summary>Click to expand stack details</summary>

```json
{
    "apiVersion": "pulumi.com/v1",
    "kind": "Stack",
    "metadata": {
        "finalizers": [
            "finalizer.stack.pulumi.com"
        ],
        "generation": 1,
        "name": "nginx-k8s-stack",
        "namespace": "default",
        "resourceVersion": "12091631",
        "selfLink": "/apis/pulumi.com/v1/namespaces/default/stacks/nginx-k8s-stack",
        "uid": "83d321cd-cef5-4176-97e0-b4579ad702c0"
    },
    "spec": {
        "accessTokenSecret": "pulumi-api-secret",
        "commit": "2b0889718d3e63feeb6079ccd5e4488d8601e353",
        "destroyOnFinalize": true,
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

### Delete the Stack and Cleanup

If `destroyOnFinalize: true` was set on the Stack when created, it will destroy
the stack's resources and the stack before the CR is deleted.

```bash
kubectl delete stack nginx-k8s-stack

# For SaaS backend
kubectl delete secret pulumi-api-secret

# For S3 backend
kubectl delete secret aws-creds-secret 
```

## AWS S3 Buckets

Deploys an AWS S3 Buckets Stack and its AWS secrets.

### Create Stack
Based on what backend you have chosen, choose from one of the following set of instructions:

#### With Pulumi SaaS Backend

1. Download [`s3_bucket_stack.yaml`](../stack-examples/yaml/s3_bucket_stack.yaml) to start with a simple example.
1. Update the Pulumi API token Secret, `stack`, and the cloud provider Secret to use your Pulumi and AWS credentials.
1. Deploy the Stack CustomResource:
   ```
   kubectl apply -f s3_bucket_stack.yaml
   ```

#### With S3 Backend

1. Download [`s3backend/s3_bucket_stack.yaml`](../stack-examples/yaml/s3backend/s3_bucket_stack.yaml) to start with a simple example.
1. Update `backend` reference in the spec to refer to the S3 bucket where state should be stored.
1. Update the `pulumi-aws-secrets` secret to refer to AWS credentials necessary to access the state backend bucket.
1. Update the `KMS Key ARN` and `region` to refer to the KMS key to use as a secrets encryption provider.
1. Deploy the Stack CustomResource:
   ```
   kubectl apply -f s3backend/s3_bucket_stack.yaml
   ```

### Get the stack details

```bash
kubectl get stack s3-bucket-stack -o json
```

<details>
<summary>Click to expand stack details</summary>

```json
{
    "apiVersion": "pulumi.com/v1",
    "kind": "Stack",
    "metadata": {
        "finalizers": [
            "finalizer.stack.pulumi.com"
        ],
        "generation": 1,
        "name": "s3-bucket-stack",
        "namespace": "default",
        "resourceVersion": "10967723",
        "selfLink": "/apis/pulumi.com/v1/namespaces/default/stacks/s3-bucket-stack",
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

Now, you can make a change to the CR - like changing the `commit` to deploy to a different commit (e.g. `cc5442870f1195216d6bc340c14f8ae7d28cf3e2` which adds another S3 bucket). Applying this to the cluster will drive a Pulumi deployment to update the stack.


```bash
kubectl apply -f ../stack-examples/yaml/s3_bucket_stack.yaml
```

### Delete the Stack and Cleanup

If `destroyOnFinalize: true` was set on the Stack when created, it will destroy
the stack's resources and the stack before the CR is deleted.

```bash
kubectl delete stack s3-bucket-stack

# For SaaS backend
kubectl delete secret pulumi-api-secret

# For S3 backend
kubectl delete secret pulumi-aws-secrets
```

Check out [`ext_s3_bucket_stack.yaml`](../stack-examples/yaml/ext_s3_bucket_stack.yaml) for an extended options exmaple.

## Troubleshooting

Check out [troubleshooting](./troubleshooting.md) for more details.
