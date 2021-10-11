// Copyright 2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import * as kx from "@pulumi/kubernetesx";
import * as operator from "./operator";

// #############################################################################
// Deploy the Pulumi Kubernetes Operator

// By default, uses $HOME/.kube/config when no kubeconfig is set.
const provider = new k8s.Provider("k8s");

// Create the Pulumi Kubernetes Operator.
// Uses a custom ComponentResource class based on Typescript code in https://git.io/JJ6yj
const name = "pulumi-k8s-operator"
const pulumiOperator = new operator.PulumiKubernetesOperator(name, {
    namespace: "default",
    provider,
});

// #############################################################################
// Deploy AWS S3 Buckets

// Get the Pulumi API token and AWS creds.
const config = new pulumi.Config();

const pulumiAccessToken = config.requireSecret("pulumiAccessToken");

const awsAccessKeyId = config.require("awsAccessKeyId");
const awsSecretAccessKey = config.requireSecret("awsSecretAccessKey");
const awsSessionToken = config.requireSecret("awsSessionToken");

const stackName = config.require("stackName");
const stackProjectRepo = config.require("stackProjectRepo");
const stackCommit = config.require("stackCommit");

// Create the creds as Kubernetes Secrets.
const accessToken = new kx.Secret("accesstoken", {
    stringData: {accessToken: pulumiAccessToken},
});
const awsCreds = new kx.Secret("aws-creds", {
    stringData: {
        "AWS_ACCESS_KEY_ID": awsAccessKeyId,
        "AWS_SECRET_ACCESS_KEY": awsSecretAccessKey,
        "AWS_SESSION_TOKEN": awsSessionToken,
    },
});

// Create an AWS S3 Pulumi Stack in Kubernetes.
const mystack = new k8s.apiextensions.CustomResource("my-stack", {
    apiVersion: 'pulumi.com/v1',
    kind: 'Stack',
    spec: {
        stack: stackName,
        projectRepo: stackProjectRepo,
        commit: stackCommit,
        accessTokenSecret: accessToken.metadata.name,
        config: {
            "aws:region": "us-west-2",
        },
        envSecrets: [awsCreds.metadata.name],
        destroyOnFinalize: true,
    },
}, {dependsOn: pulumiOperator.deployment});
