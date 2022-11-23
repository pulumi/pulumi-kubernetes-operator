"use strict";
const k8s = require("@pulumi/kubernetes");

const configMap = new k8s.core.v1.ConfigMap("config", {
    data: {
        "foo": "bar",
    }
});
