import pulumi
import pulumi_kubernetes as kubernetes


class Operator(pulumi.ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__('pulumi:v1alpha1:Operator', name, None, opts)

        self.operator_service_account = kubernetes.core.v1.ServiceAccount("operatorServiceAccount", metadata={
            "name": "pulumi-kubernetes-operator",
        }, opts=pulumi.ResourceOptions(parent=self))

        self.operator_role = kubernetes.rbac.v1.Role("operatorRole",
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
                                                ], opts=pulumi.ResourceOptions(parent=self))

        self.operator_role_binding = kubernetes.rbac.v1.RoleBinding("operatorRoleBinding",
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
                                                               }, opts=pulumi.ResourceOptions(parent=self))

        self.operator_deployment = kubernetes.apps.v1.Deployment("operatorDeployment",
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
                                                                            "image": "pulumi/pulumi-kubernetes-operator:v0.0.7",
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
                                                                },
                                                            }, opts=pulumi.ResourceOptions(parent=self))
