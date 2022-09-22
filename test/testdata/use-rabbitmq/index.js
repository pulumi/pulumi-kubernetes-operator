"use strict";
const k8s = require("@pulumi/kubernetes");
const rabbitmq = require("@pulumi/rabbitmq");

const creds = k8s.core.v1.Secret.get('rabbitmq-user-creds', 'default/rabbitmq-creds');

const user = creds.data['user'].apply(x => Buffer.from(x, 'base64').toString('ascii'))
const pass = creds.data['password'].apply(x => Buffer.from(x, 'base64').toString('ascii'))

const provider = new rabbitmq.Provider('local-rabbitmq', {
    endpoint: 'http://localhost:32001', // Port mapped in ../run-mino/index.js
    username: user,
    password: pass,
});

const exchange = new rabbitmq.Exchange('foo', {
    settings: {
        type: "fanout",
    },
}, { provider });

exports.providerURN = provider.urn;
