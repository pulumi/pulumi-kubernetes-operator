import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";
import * as random from "@pulumi/random";

const config = new pulumi.Config();
export const gitURL = config.require('git-url');
export const stackName = config.require('stack-name');

// A git repository source for our Pulumi program
const gitRepo = new k8s.apiextensions.CustomResource("pko-dev", {
    apiVersion: 'source.toolkit.fluxcd.io/v1beta2',
    kind: 'GitRepository',
    metadata: {
        'namespace': 'default',
    },
    spec: {
        interval: '5m0s',
        url: gitURL,
        ref: { branch: 'main' },
    },
});

const pulumiToken = process.env['PULUMI_ACCESS_TOKEN'];
if (!pulumiToken) {
    throw new Error('This stack needs a Pulumi access token in the environment variable PULUMI_ACCESS_TOKEN')
}

// A secret with the Pulumi access token, taken from the environment.
const tokenSecret = new k8s.core.v1.Secret("pulumi-token", {
    stringData: {
        'PULUMI_ACCESS_TOKEN': pulumiToken,
    },
});

const stack = new k8s.apiextensions.CustomResource("basic", {
    apiVersion: 'pulumi.com/v1',
    kind: 'Stack',
    metadata: {
        'namespace': 'default',
    },
    spec: {
        stack: stackName,
        envRefs: {
            'PULUMI_ACCESS_TOKEN': {
                'type': 'Secret',
                'secret': {
                    key: 'PULUMI_ACCESS_TOKEN',
                    name: tokenSecret.metadata.name,
                },
            },
        },
        fluxSource: {
            sourceRef: {
                apiVersion: gitRepo.apiVersion,
                kind: gitRepo.kind,
                name: gitRepo.metadata.name,
            },
            dir: 'basic',
        },
        refresh: true,
    },
});

// Using webhooks: this follows the guide at https://fluxcd.io/flux/guides/webhook-receivers/.

// This program will go as far as creating the receiver in the cluster. You will need to

// - expose that to the internet, either by creating an Ingress or LoadBalancer (explained in the
// Flux documentation linked above), or using something like ngrok
// (https://hub.docker.com/r/ngrok/ngrok).
//
// - install the webhook in the GitHub repository you're syncing from.

// GitHub webhooks need a shared secret.

export const token = pulumi.secret(new random.RandomString("webhook-token", {
    length: 40,
    special: false,
}).result);

const webhookSecret = new k8s.core.v1.Secret("webhook-token", {
    metadata: {
        namespace: 'default',
    },
    stringData: { token },
});

const receiver = new k8s.apiextensions.CustomResource("app-hook", {
    apiVersion: 'notification.toolkit.fluxcd.io/v1beta1',
    kind: 'Receiver',
    metadata: {
        namespace: 'default',
    },
    spec: {
        type: 'github',
        events: ["ping", "push"],
        secretRef: {
            name: webhookSecret.metadata.name,
        },
        resources: [{
            kind: 'GitRepository',
            name: gitRepo.metadata.name,
        }],
    },
});

export const readme = pulumi.unsecret(pulumi.interpolate `
The GitRepository, Stack and webhook receiver are set up. Get the webhook path with

    kubectl get receiver/${receiver.metadata.name}

The token output has the secret for using as a webhook secret. Install the webhook at
${gitURL}/settings/hooks, using the public host you've set up, the path from the receiver status,
and the token.
`);
