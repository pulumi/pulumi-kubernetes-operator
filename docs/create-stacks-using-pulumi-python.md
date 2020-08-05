# Create Pulumi Stacks using Python

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

Update the Pulumi API token Secret to use your Pulumi credentials.

Also update the `stack` org to match your account, leaving the stack project name as-is to work with the example repo's `Pulumi.yaml`. 

```python
import pulumi
from pulumi_kubernetes import core, apiextensions

# Get the Pulumi API token.
pulumi_config = pulumi.Config()
pulumi_access_token = pulumi_config.require_secret("pulumiAccessToken")

# Create the API token as a Kubernetes Secret.
access_token = core.v1.Secret("accesstoken", string_data={ "access_token": pulumi_access_token })

# Create an NGINX deployment in-cluster.
my_stack = apiextensions.CustomResource("my-stack",
    api_version="pulumi.com/v1alpha1",
    kind="Stack",
    spec={
        "access_token_secret": access_token.metadata["name"],
        "stack": "<YOUR_ORG>/nginx/dev",
        "init_on_create": True,
        "project_repo": "https://github.com/metral/pulumi-nginx",
        "commit": "2b0889718d3e63feeb6079ccd5e4488d8601e353",
        "destroy_on_finalize": True,
    }
)
```

## AWS S3 Buckets

Deploys an AWS S3 Buckets Stack and its AWS secrets.

Update the Pulumi API token Secret, and the cloud provider Secret to use
your Pulumi and AWS credentials.

Also update the `stack` org to match your account, leaving the stack project name as-is to work with the example repo's `Pulumi.yaml`. 

```python
import pulumi
from pulumi_kubernetes import core, apiextensions

# Get the Pulumi API token.
pulumi_config = pulumi.Config()
pulumi_access_token = pulumi_config.require_secret("pulumiAccessToken")
aws_access_key_id = pulumi_config.require("awsAccessKeyId")
aws_secret_access_key = pulumi_config.require_secret("awsSecretAccessKey")
aws_session_token = pulumi_config.require_secret("awsSessionToken")

# Create the creds as Kubernetes Secrets.
access_token = core.v1.Secret("accesstoken", string_data={ "access_token": pulumi_access_token })
aws_creds = core.v1.Secret("aws-creds", string_data={
    "AWS_ACCESS_KEY_ID": aws_access_key_id,
    "AWS_SECRET_ACCESS_KEY": aws_secret_access_key,
    "AWS_SESSION_TOKEN": aws_session_token,
})

# Create an AWS S3 Pulumi Stack in Kubernetes.
my_stack = apiextensions.CustomResource("my-stack",
    api_version="pulumi.com/v1alpha1",
    kind="Stack",
    spec={
        "stack": "<YOUR_ORG>/s3-op-project/dev",
        "project_repo": "https://github.com/metral/test-s3-op-project",
        "commit": "bd1edfac28577d62068b7ace0586df595bda33be",
        "access_token_secret": access_token.metadata["name"],
        "config": {
            "aws:region": "us-west-2",
        },
        "env_secrets": [aws_creds.metadata["name"]],
        "init_on_create": True,
        "destroy_on_finalize": True,
    }
)
```

Deploy the Stack CustomResource by running a `pulumi up`.

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

Now, you can make a change to the CR - like changing the `commit` to deploy to a different commit (`cc5442870f1195216d6bc340c14f8ae7d28cf3e2`). Applying this to the update will drive a Pulumi deployment to update the stack.

After changing the commit, run `pulumi up`

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
        "generation": 2,
        "name": "s3-bucket-stack",
        "namespace": "default",
        "resourceVersion": "10971321",
        "selfLink": "/apis/pulumi.com/v1alpha1/namespaces/default/stacks/s3-bucket-stack",
        "uid": "84166e1e-be47-47f8-8b6c-01474c37485b"
    },
    "spec": {
        "accessTokenSecret": "pulumi-api-secret-itolsj",
        "commit": "cc5442870f1195216d6bc340c14f8ae7d28cf3e2",
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
            "permalink": "https://app.pulumi.com/metral/s3-op-project/dev/updates/2",
            "state": "cc5442870f1195216d6bc340c14f8ae7d28cf3e2"
        },
        "outputs": {
            "bucketNames": [
                "my-bucket-0-5f38fc3",
                "my-bucket-1-588d2e8",
                "my-bucket-2-192f8e9"
            ]
        }
    }
}
```
</details>

Delete the Stack and its secrets by running a `pulumi destroy -y`.

If `destroyOnFinalize: true` was set on the Stack when created, it will destroy the stack's resources and the stack before the CR is deleted.

## Troubleshooting

Check out [troubleshooting](./troubleshooting.md) for more details.
