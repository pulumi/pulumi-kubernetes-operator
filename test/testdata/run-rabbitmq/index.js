"use strict";
const pulumi = require("@pulumi/pulumi");
const docker = require("@pulumi/docker");
const k8s = require("@pulumi/kubernetes");
const random = require("@pulumi/random");

const config = new pulumi.Config();

const secretKey = new random.RandomString("rabbitmq-pass", {
    length: config.require('passwordLength'),
    special: false,
});

const accessKey = new random.RandomString("rabbitmq-user", {
    length: config.require('passwordLength'),
    special: false,
});


const creds = new k8s.core.v1.Secret('rabbitmq-creds', {
    metadata: {
        name: 'rabbitmq-creds',
    },
    stringData: {
        user: accessKey.result,
        password: secretKey.result,
    },
});

const container = new docker.Container("rabbitmq", {
    image: 'rabbitmq:3-management',
    hostname: 'my-rabbit',
    envs: [pulumi.interpolate `RABBITMQ_DEFAULT_USER=${accessKey.result}`,
           pulumi.interpolate `RABBITMQ_DEFAULT_PASS=${secretKey.result}`],
    ports: [{internal: 5672, external: 32000},
            {internal: 15672, external: 32001}],
    mustRun: true,
});

exports.containerId = container.id;
