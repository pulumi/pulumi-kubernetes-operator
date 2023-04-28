import pulumi
from pulumi.resource import ResourceOptions
import pulumi_kubernetes as kubernetes

DEFAULT_CRD_VERSION = "v1.11.5"
DEFAULT_OPERATOR_VERSION = "v1.11.5"

conf = pulumi.Config()
namespace = conf.get("namespace", "default")
namespaces = conf.get_object("namespaces", [namespace])
crdVersion = conf.get("crd-version", DEFAULT_CRD_VERSION)
operatorVersion = conf.get("operator-version", DEFAULT_OPERATOR_VERSION)

# Work around https://github.com/pulumi/pulumi-kubernetes/issues/1481
def delete_status():
    def f(o):
        if "status" in o:
            del o["status"]
    return f

stack_crd = kubernetes.yaml.ConfigFile(
    "stackcrd",
    file=f"https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/{crdVersion}/deploy/crds/pulumi.com_stacks.yaml",
    transformations=[delete_status()])
program_crd = kubernetes.yaml.ConfigFile(
    "programcrd",
    file=f"https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/{crdVersion}/deploy/crds/pulumi.com_programs.yaml",
    transformations=[delete_status()])
deployment_opts = ResourceOptions(depends_on=[stack_crd, program_crd])

for ns in namespaces:
    
    operator_service_account = kubernetes.core.v1.ServiceAccount(f"operator-service-account-{ns}",
                                                                 metadata={"namespace": ns})
    operator_role = kubernetes.rbac.v1.Role(f"operator-role-{ns}",
                                            metadata={
                                                "namespace": ns,
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

    operator_role_binding = kubernetes.rbac.v1.RoleBinding(f"operator-role-binding-{ns}",
                                                       metadata={
                                                           "namespace": ns,
                                                       },
                                                       subjects=[{
                                                           "kind": "ServiceAccount",
                                                           "name": operator_service_account.metadata.name,
                                                       }],
                                                       role_ref={
                                                           "kind": "Role",
                                                           "name": operator_role.metadata.name,
                                                           "api_group": "rbac.authorization.k8s.io",
                                                       })

    operator_deployment = kubernetes.apps.v1.Deployment(f"pulumi-kubernetes-operator-{ns}",
                                                    metadata={
                                                        "namespace": ns,
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
                                                                "service_account_name": operator_service_account.metadata.name,
                                                                "containers": [{
                                                                    "name": "pulumi-kubernetes-operator",
                                                                    "image": f"pulumi/pulumi-kubernetes-operator:{operatorVersion}",
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
                                                    }, opts=deployment_opts)
