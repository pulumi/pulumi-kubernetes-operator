# RBAC rules for Flux sources

This directory contains manifests for an RBAC Role and RoleBinding that will give the Pulumi
Kubernetes operator access to [Flux sources](). You will need to adjust the namespaces mentioned, if
you deploy the operator in a namespace other than `default`.

See [../../docs/flux.md][] for an explanation of how to deploy the Pulumi Kubernetes operator
alongside Flux.
