import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import * as kx from "@pulumi/kubernetesx";
import * as operator from "./operator";

// By default, uses $HOME/.kube/config when no kubeconfig is set.
const provider = new k8s.Provider("k8s"); 

// Create the Pulumi Kubernetes Operator.
// Uses a custom ComponentResource class based on Typescript code in https://git.io/JJ6yj
const name = "pulumi-k8s-operator"
const pulumiOperator = new operator.PulumiKubernetesOperator(name, {
    namespace: "default",
    provider,
});

// Get the Pulumi API token.
const config = new pulumi.Config();
const pulumiAccessToken = config.requireSecret("pulumiAccessToken")

// Create the API token as a Kubernetes Secret.
const apiAccessToken = new kx.Secret("accesstoken", {
    stringData: { accessToken: pulumiAccessToken},
});

// Create an NGINX deployment in-cluster.
const appStack = new k8s.apiextensions.CustomResource("app-stack", {
    apiVersion: 'pulumi.com/v1alpha1',
    kind: 'Stack',
    spec: {
        accessTokenSecret: apiAccessToken.metadata.name,
        stack: "<YOUR_ORG>/pulumi-blue-green/dev",
        initOnCreate: true,
        projectRepo: config.require("stackProjectRepo"),
        commit: config.require("stackCommit"),
        destroyOnFinalize: true,
    }
}, {dependsOn: pulumiOperator.deployment});
