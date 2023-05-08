"use strict";
const pulumi = require("@pulumi/pulumi");
const docker = require("@pulumi/docker");
const k8s = require("@pulumi/kubernetes");
const random = require("@pulumi/random");

const config = new pulumi.Config();
const secretKey = config.require('password');
const secretName = config.require('secretName');
const port = config.requireNumber('port');

const accessKey = new random.RandomString("rabbitmq-user", {
    length: 8,
    special: false,
});

const creds = new k8s.core.v1.Secret('rabbitmq-creds', {
    metadata: {
        name: secretName,
    },
    stringData: {
        user: accessKey.result,
        password: secretKey,
    },
});

const container = new docker.Container("rabbitmq", {
    image: 'rabbitmq:3-management',
    hostname: 'my-rabbit',
    envs: [pulumi.interpolate `RABBITMQ_DEFAULT_USER=${accessKey.result}`,
           pulumi.interpolate `RABBITMQ_DEFAULT_PASS=${secretKey}`],
    ports: [{internal: 15672, external: port}],
    mustRun: true,
});

exports.containerId = container.id;
