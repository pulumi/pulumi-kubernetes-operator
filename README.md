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
  - [Extended Examples](#extended-examples)
* [Development](#development)

## Deploy the Operator

Deploy the operator to a Kubernetes cluster.

You can use an existing cluster, or [get started](https://www.pulumi.com/docs/get-started/kubernetes/) by creating a new [managed Kubernetes cluster](https://www.pulumi.com/docs/tutorials/kubernetes/#clusters).

### Using kubectl

Deploy the CustomResourceDefinitions (CRDs) for the operator.

```bash
kubectl apply -f deploy/crds/
```

Deploy the API resources for the operator.

```bash
kubectl apply -f deploy/yaml
```

### Using Pulumi

<details>
<summary>Typescript</summary>

```ts
import * as pulumi from "@pulumi/pulumi";
import * as kubernetes from "@pulumi/kubernetes";

const crds = new kubernetes.yaml.ConfigFile("crds", {file: "https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/master/deploy/crds/pulumi.com_stacks.yaml"});

const operatorServiceAccount = new kubernetes.core.v1.ServiceAccount("operatorServiceAccount", {metadata: {
    name: "pulumi-kubernetes-operator",
}});
const operatorRole = new kubernetes.rbac.v1.Role("operatorRole", {
    metadata: {
        name: "pulumi-kubernetes-operator",
    },
    rules: [
        {
            apiGroups: [""],
            resources: [
                "pods",
                "services",
                "services/finalizers",
                "endpoints",
                "persistentvolumeclaims",
                "events",
                "configmaps",
                "secrets",
            ],
            verbs: [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
        {
            apiGroups: ["apps"],
            resources: [
                "deployments",
                "daemonsets",
                "replicasets",
                "statefulsets",
            ],
            verbs: [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
        {
            apiGroups: ["monitoring.coreos.com"],
            resources: ["servicemonitors"],
            verbs: [
                "create",
                "get",
            ],
        },
        {
            apiGroups: ["apps"],
            resourceNames: ["pulumi-kubernetes-operator"],
            resources: ["deployments/finalizers"],
            verbs: ["update"],
        },
        {
            apiGroups: [""],
            resources: ["pods"],
            verbs: ["get"],
        },
        {
            apiGroups: ["apps"],
            resources: [
                "replicasets",
                "deployments",
            ],
            verbs: ["get"],
        },
        {
            apiGroups: ["pulumi.com"],
            resources: ["*"],
            verbs: [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
        {
            apiGroups: ["coordination.k8s.io"],
            resources: ["leases"],
            verbs: [
                "create",
                "get",
                "list",
                "update",
            ],
        },
    ],
});
const operatorRoleBinding = new kubernetes.rbac.v1.RoleBinding("operatorRoleBinding", {
    metadata: {
        name: "pulumi-kubernetes-operator",
    },
    subjects: [{
        kind: "ServiceAccount",
        name: "pulumi-kubernetes-operator",
    }],
    roleRef: {
        kind: "Role",
        name: "pulumi-kubernetes-operator",
        apiGroup: "rbac.authorization.k8s.io",
    },
});
const operatorDeployment = new kubernetes.apps.v1.Deployment("operatorDeployment", {
    metadata: {
        name: "pulumi-kubernetes-operator",
    },
    spec: {
        replicas: 1,
        selector: {
            matchLabels: {
                name: "pulumi-kubernetes-operator",
            },
        },
        template: {
            metadata: {
                labels: {
                    name: "pulumi-kubernetes-operator",
                },
            },
            spec: {
                serviceAccountName: "pulumi-kubernetes-operator",
                imagePullSecrets: [{
                    name: "pulumi-kubernetes-operator",
                }],
                containers: [{
                    name: "pulumi-kubernetes-operator",
                    image: "pulumi/pulumi-kubernetes-operator:v1.0.0",
                    args: ["--zap-level=debug"],
                    imagePullPolicy: "Always",
                    env: [
                        {
                            name: "WATCH_NAMESPACE",
                            valueFrom: {
                                fieldRef: {
                                    fieldPath: "metadata.namespace",
                                },
                            },
                        },
                        {
                            name: "POD_NAME",
                            valueFrom: {
                                fieldRef: {
                                    fieldPath: "metadata.name",
                                },
                            },
                        },
                        {
                            name: "OPERATOR_NAME",
                            value: "pulumi-kubernetes-operator",
                        },
                    ],
                }],
                terminationGracePeriodSeconds: 300, // Should be same or larger than GRACEFUL_SHUTDOWN_TIMEOUT_DURATION
            },
        },
    },
}, {dependsOn: crds});
```
</details>

<details>
<summary>Python</summary>

```python
import pulumi
from pulumi.resource import ResourceOptions
import pulumi_kubernetes as kubernetes

# Work around https://github.com/pulumi/pulumi-kubernetes/issues/1481
def delete_status():
    def f(o):
        if "status" in o:
            del o["status"]
    return f

crds = kubernetes.yaml.ConfigFile("crds", file="https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/master/deploy/crds/pulumi.com_stacks.yaml",
    transformations=[delete_status()])

operator_service_account = kubernetes.core.v1.ServiceAccount("operatorServiceAccount", metadata={
    "name": "pulumi-kubernetes-operator",
})
operator_role = kubernetes.rbac.v1.Role("operatorRole",
    metadata={
        "name": "pulumi-kubernetes-operator",
    },
    rules=[
        {
            "api_groups": [""],
            "resources": [
                "pods",
                "services",
                "services/finalizers",
                "endpoints",
                "persistentvolumeclaims",
                "events",
                "configmaps",
                "secrets",
            ],
            "verbs": [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
        {
            "api_groups": ["apps"],
            "resources": [
                "deployments",
                "daemonsets",
                "replicasets",
                "statefulsets",
            ],
            "verbs": [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
        {
            "api_groups": ["monitoring.coreos.com"],
            "resources": ["servicemonitors"],
            "verbs": [
                "create",
                "get",
            ],
        },
        {
            "api_groups": ["apps"],
            "resource_names": ["pulumi-kubernetes-operator"],
            "resources": ["deployments/finalizers"],
            "verbs": ["update"],
        },
        {
            "api_groups": [""],
            "resources": ["pods"],
            "verbs": ["get"],
        },
        {
            "api_groups": ["apps"],
            "resources": [
                "replicasets",
                "deployments",
            ],
            "verbs": ["get"],
        },
        {
            "api_groups": ["pulumi.com"],
            "resources": ["*"],
            "verbs": [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
        {
            "api_groups": ["coordination.k8s.io"],
            "resources": ["leases"],
            "verbs": [
                "create",
                "get",
                "list",
                "update",
            ],
        }, 
    ])
operator_role_binding = kubernetes.rbac.v1.RoleBinding("operatorRoleBinding",
    metadata={
        "name": "pulumi-kubernetes-operator",
    },
    subjects=[{
        "kind": "ServiceAccount",
        "name": "pulumi-kubernetes-operator",
    }],
    role_ref={
        "kind": "Role",
        "name": "pulumi-kubernetes-operator",
        "api_group": "rbac.authorization.k8s.io",
    })
operator_deployment = kubernetes.apps.v1.Deployment("operatorDeployment",
    metadata={
        "name": "pulumi-kubernetes-operator",
    },
    spec={
        "replicas": 1,
        "selector": {
            "match_labels": {
                "name": "pulumi-kubernetes-operator",
            },
        },
        "template": {
            "metadata": {
                "labels": {
                    "name": "pulumi-kubernetes-operator",
                },
            },
            "spec": {
                "service_account_name": "pulumi-kubernetes-operator",
                "image_pull_secrets": [{
                    "name": "pulumi-kubernetes-operator",
                }],
                "containers": [{
                    "name": "pulumi-kubernetes-operator",
                    "image": "pulumi/pulumi-kubernetes-operator:v1.0.0",
                    "command": ["pulumi-kubernetes-operator"],
                    "args": ["--zap-level=debug"],
                    "image_pull_policy": "Always",
                    "env": [
                        {
                            "name": "WATCH_NAMESPACE",
                            "value_from": {
                                "field_ref": {
                                    "field_path": "metadata.namespace",
                                },
                            },
                        },
                        {
                            "name": "POD_NAME",
                            "value_from": {
                                "field_ref": {
                                    "field_path": "metadata.name",
                                },
                            },
                        },
                        {
                            "name": "OPERATOR_NAME",
                            "value": "pulumi-kubernetes-operator",
                        },
                    ],
                }],
            },
            "terminationGracePeriodSeconds": 300,
        },
    },
    opts=ResourceOptions(depends_on=crds))
```
</details>

<details>
<summary>C#</summary>

```csharp
using Pulumi;
using Kubernetes = Pulumi.Kubernetes;
using Pulumi.Kubernetes.Types.Inputs.Core.V1;
using Pulumi.Kubernetes.Types.Inputs.Apps.V1;
using Pulumi.Kubernetes.Types.Inputs.Meta.V1;
using Pulumi.Kubernetes.Types.Inputs.Rbac.V1;

class MyStack : Stack
{
    public MyStack()
    {
        var crds = new Kubernetes.Yaml.ConfigFile("crds", new Kubernetes.Yaml.ConfigFileArgs{
            File = "https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/master/deploy/crds/pulumi.com_stacks.yaml"
        });

        var operatorServiceAccount = new Kubernetes.Core.V1.ServiceAccount("operatorServiceAccount", new ServiceAccountArgs
        {
            Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
        });
        var operatorRole = new Kubernetes.Rbac.V1.Role("operatorRole", new RoleArgs
        {
            Metadata = new ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
            Rules = 
            {
                new PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "",
                    },
                    Resources = 
                    {
                        "pods",
                        "services",
                        "services/finalizers",
                        "endpoints",
                        "persistentvolumeclaims",
                        "events",
                        "configmaps",
                        "secrets",
                    },
                    Verbs = 
                    {
                        "create",
                        "delete",
                        "get",
                        "list",
                        "patch",
                        "update",
                        "watch",
                    },
                },
                new PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "apps",
                    },
                    Resources = 
                    {
                        "deployments",
                        "daemonsets",
                        "replicasets",
                        "statefulsets",
                    },
                    Verbs = 
                    {
                        "create",
                        "delete",
                        "get",
                        "list",
                        "patch",
                        "update",
                        "watch",
                    },
                },
                new PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "monitoring.coreos.com",
                    },
                    Resources = 
                    {
                        "servicemonitors",
                    },
                    Verbs = 
                    {
                        "create",
                        "get",
                    },
                },
                new PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "apps",
                    },
                    ResourceNames = 
                    {
                        "pulumi-kubernetes-operator",
                    },
                    Resources = 
                    {
                        "deployments/finalizers",
                    },
                    Verbs = 
                    {
                        "update",
                    },
                },
                new PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "",
                    },
                    Resources = 
                    {
                        "pods",
                    },
                    Verbs = 
                    {
                        "get",
                    },
                },
                new PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "apps",
                    },
                    Resources = 
                    {
                        "replicasets",
                        "deployments",
                    },
                    Verbs = 
                    {
                        "get",
                    },
                },
                new PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "pulumi.com",
                    },
                    Resources = 
                    {
                        "*",
                    },
                    Verbs = 
                    {
                        "create",
                        "delete",
                        "get",
                        "list",
                        "patch",
                        "update",
                        "watch",
                    },
                },
                new PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "coordination.k8s.io",
                    },
                    Resources = 
                    {
                        "leases",
                    },
                    Verbs = 
                    {
                        "create",
                        "get",
                        "list",
                        "update",
                    },
                },
            },
        });
        var operatorRoleBinding = new Kubernetes.Rbac.V1.RoleBinding("operatorRoleBinding", new RoleBindingArgs
        {
            Metadata = new ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
            Subjects = 
            {
                new SubjectArgs
                {
                    Kind = "ServiceAccount",
                    Name = "pulumi-kubernetes-operator",
                },
            },
            RoleRef = new RoleRefArgs
            {
                Kind = "Role",
                Name = "pulumi-kubernetes-operator",
                ApiGroup = "rbac.authorization.k8s.io",
            },
        });
        var operatorDeployment = new Kubernetes.Apps.V1.Deployment("operatorDeployment", new DeploymentArgs
        {
            Metadata = new ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
            Spec = new Kubernetes.Types.Inputs.Apps.V1.DeploymentSpecArgs
            {
                Replicas = 1,
                Selector = new LabelSelectorArgs
                {
                    MatchLabels = 
                    {
                        { "name", "pulumi-kubernetes-operator" },
                    },
                },
                Template = new PodTemplateSpecArgs
                {
                    Metadata = new ObjectMetaArgs
                    {
                        Labels = 
                        {
                            { "name", "pulumi-kubernetes-operator" },
                        },
                    },
                    Spec = new PodSpecArgs
                    {
                        ServiceAccountName = "pulumi-kubernetes-operator",
                        ImagePullSecrets = 
                        {
                            new LocalObjectReferenceArgs
                            {
                                Name = "pulumi-kubernetes-operator",
                            },
                        },
                        Containers = 
                        {
                            new ContainerArgs
                            {
                                Name = "pulumi-kubernetes-operator",
                                Image = "pulumi/pulumi-kubernetes-operator:v1.0.0",
                                Command = 
                                {
                                    "pulumi-kubernetes-operator",
                                },
                                Args = 
                                {
                                    "--zap-level=debug",
                                },
                                ImagePullPolicy = "Always",
                                Env = 
                                {
                                    new EnvVarArgs
                                    {
                                        Name = "WATCH_NAMESPACE",
                                        ValueFrom = new EnvVarSourceArgs
                                        {
                                            FieldRef = new ObjectFieldSelectorArgs
                                            {
                                                FieldPath = "metadata.namespace",
                                            },
                                        },
                                    },
                                    new EnvVarArgs
                                    {
                                        Name = "POD_NAME",
                                        ValueFrom = new EnvVarSourceArgs
                                        {
                                            FieldRef = new ObjectFieldSelectorArgs
                                            {
                                                FieldPath = "metadata.name",
                                            },
                                        },
                                    },
                                    new EnvVarArgs
                                    {
                                        Name = "OPERATOR_NAME",
                                        Value = "pulumi-kubernetes-operator",
                                    },
                                },
                            },
                        },
                        TerminationGracePeriodSeconds = 300,
                    },
                },
            },
        }, new CustomResourceOptions{
            DependsOn = {crds},
        });
    }

}
```
</details>

<details>
<summary>Go</summary>

```go
package main

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		crds, err := yaml.NewConfigFile(ctx, "crds", &yaml.ConfigFileArgs{
			File: "https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/master/deploy/crds/pulumi.com_stacks.yaml",
		})
		if err != nil {
			return err
		}

		_, err = corev1.NewServiceAccount(ctx, "operatorServiceAccount", &corev1.ServiceAccountArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("pulumi-kubernetes-operator"),
			},
		})
		if err != nil {
			return err
		}
		_, err = rbacv1.NewRole(ctx, "operatorRole", &rbacv1.RoleArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("pulumi-kubernetes-operator"),
			},
			Rules: rbacv1.PolicyRuleArray{
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String(""),
					},
					Resources: pulumi.StringArray{
						pulumi.String("pods"),
						pulumi.String("services"),
						pulumi.String("services/finalizers"),
						pulumi.String("endpoints"),
						pulumi.String("persistentvolumeclaims"),
						pulumi.String("events"),
						pulumi.String("configmaps"),
						pulumi.String("secrets"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("create"),
						pulumi.String("delete"),
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("patch"),
						pulumi.String("update"),
						pulumi.String("watch"),
					},
				},
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String("apps"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("deployments"),
						pulumi.String("daemonsets"),
						pulumi.String("replicasets"),
						pulumi.String("statefulsets"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("create"),
						pulumi.String("delete"),
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("patch"),
						pulumi.String("update"),
						pulumi.String("watch"),
					},
				},
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String("monitoring.coreos.com"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("servicemonitors"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("create"),
						pulumi.String("get"),
					},
				},
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String("apps"),
					},
					ResourceNames: pulumi.StringArray{
						pulumi.String("pulumi-kubernetes-operator"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("deployments/finalizers"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("update"),
					},
				},
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String(""),
					},
					Resources: pulumi.StringArray{
						pulumi.String("pods"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("get"),
					},
				},
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String("apps"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("replicasets"),
						pulumi.String("deployments"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("get"),
					},
				},
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String("pulumi.com"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("*"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("create"),
						pulumi.String("delete"),
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("patch"),
						pulumi.String("update"),
						pulumi.String("watch"),
					},
				},
				&rbacv1.PolicyRuleArgs{
					ApiGroups: pulumi.StringArray{
						pulumi.String("coordination.k8s.io"),
					},
					Resources: pulumi.StringArray{
						pulumi.String("leases"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("create"),
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("update"),
					},
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = rbacv1.NewRoleBinding(ctx, "operatorRoleBinding", &rbacv1.RoleBindingArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("pulumi-kubernetes-operator"),
			},
			Subjects: rbacv1.SubjectArray{
				&rbacv1.SubjectArgs{
					Kind: pulumi.String("ServiceAccount"),
					Name: pulumi.String("pulumi-kubernetes-operator"),
				},
			},
			RoleRef: &rbacv1.RoleRefArgs{
				Kind:     pulumi.String("Role"),
				Name:     pulumi.String("pulumi-kubernetes-operator"),
				ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			},
		})
		if err != nil {
			return err
		}
		_, err = appsv1.NewDeployment(ctx, "operatorDeployment", &appsv1.DeploymentArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("pulumi-kubernetes-operator"),
			},
			Spec: &appsv1.DeploymentSpecArgs{
				Replicas: pulumi.Int(1),
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: pulumi.StringMap{
						"name": pulumi.String("pulumi-kubernetes-operator"),
					},
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: pulumi.StringMap{
							"name": pulumi.String("pulumi-kubernetes-operator"),
						},
					},
					Spec: &corev1.PodSpecArgs{
						ServiceAccountName: pulumi.String("pulumi-kubernetes-operator"),
						ImagePullSecrets: corev1.LocalObjectReferenceArray{
							&corev1.LocalObjectReferenceArgs{
								Name: pulumi.String("pulumi-kubernetes-operator"),
							},
						},
						Containers: corev1.ContainerArray{
							&corev1.ContainerArgs{
								Name:  pulumi.String("pulumi-kubernetes-operator"),
								Image: pulumi.String("pulumi/pulumi-kubernetes-operator:v1.0.0"),
								Command: pulumi.StringArray{
									pulumi.String("pulumi-kubernetes-operator"),
								},
								Args: pulumi.StringArray{
									pulumi.String("--zap-level=debug"),
								},
								ImagePullPolicy: pulumi.String("Always"),
								Env: corev1.EnvVarArray{
									&corev1.EnvVarArgs{
										Name: pulumi.String("WATCH_NAMESPACE"),
										ValueFrom: &corev1.EnvVarSourceArgs{
											FieldRef: &corev1.ObjectFieldSelectorArgs{
												FieldPath: pulumi.String("metadata.namespace"),
											},
										},
									},
									&corev1.EnvVarArgs{
										Name: pulumi.String("POD_NAME"),
										ValueFrom: &corev1.EnvVarSourceArgs{
											FieldRef: &corev1.ObjectFieldSelectorArgs{
												FieldPath: pulumi.String("metadata.name"),
											},
										},
									},
									&corev1.EnvVarArgs{
										Name:  pulumi.String("OPERATOR_NAME"),
										Value: pulumi.String("pulumi-kubernetes-operator"),
									},
								},
							},
						},
						TerminationGracePeriodSeconds: pulumi.Int(300),
					},
				},
			},
		}, pulumi.DependsOn([]pulumi.Resource{crds}))
		if err != nil {
			return err
		}
		return nil
	})
}
```
</details>

## Create Pulumi Stack CustomResources

The following are examples to create Pulumi Stacks in Kubernetes that are managed and run by the operator.

If you'd like to use your own Pulumi Stack, ensure that you have an existing Pulumi program in a git repo,
and update the CR with:
  - An existing github `project` and `commit`,
  - A Pulumi `stack` name that exists and will be selected, or a new stack that will be created and selected.
  - A Kubernetes Secret for your Pulumi API `accessToken`,
  - A Kubernetes Secret for other sensitive settings like cloud provider credentials, and
  - Environment variables and stack config needed.

### Using kubectl

Check out [Create Pulumi Stacks using `kubectl` ](./docs/create-stacks-using-kubectl.md) for YAML examples.

### Using Pulumi

Check out [Create Pulumi Stacks using Pulumi](./docs/create-stacks-using-pulumi.md) for Typescript, Python, Go, and .NET examples.

### Extended Examples

- [Managing a Kubernetes Blue/Green Deployment](./examples/blue-green)
- [AWS S3 Buckets](./examples/aws-s3)

### Stack CR Documentation

- Detailed documentation on Stack Custom Resource is available [here](./docs/stacks.md)

## Development

Check out [docs/build.md](./docs/build.md) for more details on building and
working with the operator locally.
