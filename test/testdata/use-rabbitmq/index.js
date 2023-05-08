"use strict";
const pulumi = require("@pulumi/pulumi");
const k8s = require("@pulumi/kubernetes");
const rabbitmq = require("@pulumi/rabbitmq");

const config = new pulumi.Config();
const port = config.requireNumber('port');
const secretName = config.require('secretName');

const creds = k8s.core.v1.Secret.get('rabbitmq-user-creds', `default/${secretName}`);

const user = creds.data['user'].apply(x => Buffer.from(x, 'base64').toString('ascii'))
const pass = creds.data['password'].apply(x => Buffer.from(x, 'base64').toString('ascii'))

const provider = new rabbitmq.Provider('local-rabbitmq', {
    endpoint: `http://localhost:${port}`,
    username: user,
    password: pass,
});

const exchange = new rabbitmq.Exchange('foo', {
    settings: {
        type: "fanout",
    },
}, { provider });

exports.providerURN = provider.urn;
