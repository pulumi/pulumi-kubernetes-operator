import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import * as pulumiservice from "@pulumi/pulumiservice";

// Create a Kubernetes ServiceAccount for the Pulumi workspace pod
const sa = new k8s.core.v1.ServiceAccount("random-yaml", {});

// Grant system:auth-delegator to the ServiceAccount
const crb = new k8s.rbac.v1.ClusterRoleBinding("random-yaml:system:auth-delegator", {
    roleRef: {
        apiGroup: "rbac.authorization.k8s.io",
        kind: "ClusterRole",
        name: "system:auth-delegator",
    },
    subjects: [{
        kind: "ServiceAccount",
        name: sa.metadata.name,
        namespace: sa.metadata.namespace,
    }],
});

// Provision a Pulumi Cloud access token and store it in a Kubernetes Secret
const accessToken = new pulumiservice.AccessToken("random-yaml", {
    description: `For stack "${pulumi.runtime.getOrganization()}/${pulumi.runtime.getProject()}/${pulumi.runtime.getStack()}"`,
});
const apiSecret = new k8s.core.v1.Secret("random-yaml", {
    stringData: {
        "accessToken": accessToken.value,
    }
});

// Deploy the "random-yaml" program from the github.com/pulumi/examples repository.
const stack = new k8s.apiextensions.CustomResource("random-yaml", {
    apiVersion: 'pulumi.com/v1',
    kind: 'Stack',
    spec: {
        serviceAccountName: sa.metadata.name,
        projectRepo: "https://github.com/pulumi/examples",
        repoDir: "random-yaml/",
        branch: "master",
        shallow: true,
        stack: "pulumi-ts",
        refresh: true,
        destroyOnFinalize: true,
        envRefs: {
            PULUMI_ACCESS_TOKEN: {
                type: "Secret",
                secret: {
                    name: apiSecret.metadata.name,
                    key: "accessToken",
                },
            },
        },
        workspaceTemplate: {
            spec: {
                image: "pulumi/pulumi:3.198.0-nonroot",
            },
        },
    },
}, {dependsOn: [sa, crb, apiSecret]});

// export const stackName = stack.metadata.name;