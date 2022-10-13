import * as k8s from "@pulumi/kubernetes";


// A git repository source for our Pulumi program
const gitRepo = new k8s.apiextensions.CustomResource("pko-dev", {
    apiVersion: 'source.toolkit.fluxcd.io/v1beta2',
    kind: 'GitRepository',
    metadata: {
        'namespace': 'default',
    },
    spec: {
        interval: '5m0s',
        url: 'https://github.com/squaremo/pko-dev',
        ref: { branch: 'main' },
    },
});

// A secret with the Pulumi access token, taken from the environment.
const tokenSecret = new k8s.core.v1.Secret("pulumi-token", {
    stringData: {
        'PULUMI_ACCESS_TOKEN': process.env['PULUMI_ACCESS_TOKEN'] || 'token',
    },
});

const stack = new k8s.apiextensions.CustomResource("basic", {
    apiVersion: 'pulumi.com/v1',
    kind: 'Stack',
    metadata: {
        'namespace': 'default',
    },
    spec: {
        stack: 'squaremo/basic/docker-desktop',
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
    },
});
