import * as pulumi from '@pulumi/pulumi';
import * as kubernetes from '@pulumi/kubernetes';

class Operator extends pulumi.ComponentResource {
    constructor(name, opts) {
        super("pulumi:v1alpha1:Operator", name, {}, opts);

        const operatorServiceAccount = new kubernetes.core.v1.ServiceAccount("operatorServiceAccount", {
                metadata: {
                    name: "pulumi-kubernetes-operator",
                }
            },
            {parent: this}
        );

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
                ],
            },
            {parent: this}
        );

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
            },
            {parent: this}
        );

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
                                image: "pulumi/pulumi-kubernetes-operator:v0.0.7",
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
            },
            {parent: this}
        );
    }
}
