import * as pulumi from "@pulumi/pulumi";
import * as kubernetes from "@pulumi/kubernetes";

const defaultCRDVersion = "v1.10.1";
const defaultOperatorVersion = "v1.10.1";

const config = new pulumi.Config();
const deployNamespace = config.get("namespace") || 'default';
const deployNamespaceList = config.getObject<string[]>("namespaces") || [deployNamespace];
const crdVersion = config.get("crd-version") || defaultCRDVersion;
const operatorVersion = config.get("operator-version") || defaultOperatorVersion;

const stackCRD = new kubernetes.yaml.ConfigFile("stackcrd", {
    file: `https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/${crdVersion}/deploy/crds/pulumi.com_stacks.yaml`
});
const programCRD = new kubernetes.yaml.ConfigFile("programcrd", {
    file: `https://raw.githubusercontent.com/pulumi/pulumi-kubernetes-operator/${crdVersion}/deploy/crds/pulumi.com_programs.yaml`
});

const deploymentOptions = { dependsOn: [stackCRD, programCRD] };

for (let ns of deployNamespaceList) {
    const operatorServiceAccount = new kubernetes.core.v1.ServiceAccount(`operator-service-account-${ns}`, {
        metadata: {
            "namespace": ns,
        },
    });

    const operatorRole = new kubernetes.rbac.v1.Role(`operator-role-${ns}`, {
        metadata: {
            "namespace": ns,
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

    const operatorRoleBinding = new kubernetes.rbac.v1.RoleBinding(`operator-role-binding-${ns}`, {
        metadata: {
            "namespace": ns,
        },
        subjects: [{
            kind: "ServiceAccount",
            name: operatorServiceAccount.metadata.name,
        }],
        roleRef: {
            kind: "Role",
            name: operatorRole.metadata.name,
            apiGroup: "rbac.authorization.k8s.io",
        },
    });

    const operatorDeployment = new kubernetes.apps.v1.Deployment(`pulumi-kubernetes-operator-${ns}`, {
        metadata: {
            "namespace": ns,
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
                    serviceAccountName: operatorServiceAccount.metadata.name,
                    containers: [{
                        name: "pulumi-kubernetes-operator",
                        image: `pulumi/pulumi-kubernetes-operator:${operatorVersion}`,
                        args: ["--zap-level=error", "--zap-time-encoding=iso8601"],
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
                            {
                                name: "GRACEFUL_SHUTDOWN_TIMEOUT_DURATION",
                                value: "5m",
                            },
                            {
                                name: "MAX_CONCURRENT_RECONCILES",
                                value: "10",
                            },


                        ],
                    }],
                    // Should be same or larger than GRACEFUL_SHUTDOWN_TIMEOUT_DURATION
                    terminationGracePeriodSeconds: 300,
                },
            },
        },
    }, deploymentOptions);
}
