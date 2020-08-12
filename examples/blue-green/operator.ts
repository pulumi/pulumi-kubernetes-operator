import * as k8s from "@pulumi/kubernetes";
import * as pulumi from "@pulumi/pulumi";

// Create a ServiceAccount.
interface ServiceAccountArgs {
    namespace: pulumi.Input<string>;
    provider: k8s.Provider;
}
export function createServiceAccount(
    name: string,
    args: ServiceAccountArgs,
): k8s.core.v1.ServiceAccount {
    return new k8s.core.v1.ServiceAccount(name, {
        metadata: {namespace: args.namespace},
    },{provider: args.provider});
}

// Create a Role.
interface RoleArgs {
    namespace: pulumi.Input<string>;
    provider: k8s.Provider;
}
export function createRole(
    name: string,
    args: RoleArgs,
): k8s.rbac.v1.Role {
    return new k8s.rbac.v1.Role(name, {
        metadata: {namespace: args.namespace},
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
        ],
    },{provider: args.provider});
}

// Create a RoleBinding.
interface RoleBindingArgs {
    serviceAccountName: pulumi.Input<string>;
    roleName: pulumi.Input<string>;
    namespace: pulumi.Input<string>;
    provider: k8s.Provider;
}
export function createRoleBinding(
    name: string,
    args: RoleBindingArgs,
): k8s.rbac.v1.RoleBinding {
    return new k8s.rbac.v1.RoleBinding(name, {
        metadata: {namespace: args.namespace},
        subjects: [{
            kind: "ServiceAccount",
            name: args.serviceAccountName,
            namespace: args.namespace,
        }],
        roleRef: {
            kind: "Role",
            name: args.roleName,
            apiGroup: "rbac.authorization.k8s.io",
        },
    },{provider: args.provider});
}

export interface PulumiKubernetesOperatorArgs {
    namespace: pulumi.Input<string>;
    provider: k8s.Provider,
}
export class PulumiKubernetesOperator extends pulumi.ComponentResource {
    public readonly deployment: k8s.apps.v1.Deployment;
    constructor(name: string,
        args: PulumiKubernetesOperatorArgs,
        opts: pulumi.ComponentResourceOptions = {}) {
        super("pulumi-kubernetes-operator", name, args, opts);

        // Create the service account.
        const serviceAccount = createServiceAccount(name, {
            namespace: args.namespace,
            provider: args.provider,
        });
        const serviceAccountName = serviceAccount.metadata.name;

        // Create RBAC role and binding.
        const role = createRole(name, {
            namespace: args.namespace,
            provider: args.provider,
        });
        const roleName = role.metadata.name;
        const roleBinding = createRoleBinding(name, {
            serviceAccountName: serviceAccountName,
            roleName: roleName,
            namespace: args.namespace,
            provider: args.provider,
        });

        // Create the deployment.
        this.deployment = new k8s.apps.v1.Deployment(name, {
            metadata: {namespace: args.namespace},
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
                        serviceAccountName: serviceAccountName,
                        imagePullSecrets: [{
                            name: "pulumi-kubernetes-operator",
                        }],
                        containers: [{
                            name: "pulumi-kubernetes-operator",
                            image: "pulumi/pulumi-kubernetes-operator:v0.0.5",
                            command: ["pulumi-kubernetes-operator"],
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
                    },
                },
            },
        }, {dependsOn: roleBinding});
    }
}
