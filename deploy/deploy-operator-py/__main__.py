import pulumi
from pulumi.resource import ResourceOptions
import pulumi_kubernetes as kubernetes

# Work around https://github.com/pulumi/pulumi-kubernetes/issues/1481
def delete_status():
    def f(o):
        if "status" in o:
            del o["status"]
    return f

crds = kubernetes.yaml.ConfigFile("crds",
    file="https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/v1.5.0/deploy/crds/pulumi.com_stacks.yaml",
    transformations=[delete_status()])

operator_service_account = kubernetes.core.v1.ServiceAccount("operator-service-account")
operator_role = kubernetes.rbac.v1.Role("operator-role",
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
operator_role_binding = kubernetes.rbac.v1.RoleBinding("operator-role-binding",
    subjects=[{
        "kind": "ServiceAccount",
        "name": operator_service_account.metadata.name,
    }],
    role_ref={
        "kind": "Role",
        "name": operator_role.metadata.name,
        "api_group": "rbac.authorization.k8s.io",
    })
operator_deployment = kubernetes.apps.v1.Deployment("pulumi-kubernetes-operator",
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
                "service_account_name": operator_service_account.metadata.name,
                "containers": [{
                    "name": "pulumi-kubernetes-operator",
                    "image": "pulumi/pulumi-kubernetes-operator:v1.5.0",
                    "command": ["pulumi-kubernetes-operator"],
                    "args": ["--zap-level=error", "--zap-time-encoding=iso8601"],
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
                        {
                            "name": "GRACEFUL_SHUTDOWN_TIMEOUT_DURATION",
                            "value": "5m",
                        },
                        {
                            "name": "MAX_CONCURRENT_RECONCILES",
                            "value": "10",
                        },
                    ],
                }],
            },
            # Should be same or larger than GRACEFUL_SHUTDOWN_TIMEOUT_DURATION
            "terminationGracePeriodSeconds": 300,
        },
    },
    opts=ResourceOptions(depends_on=crds))
