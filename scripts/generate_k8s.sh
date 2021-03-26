#!/bin/sh
set -e

cwd=$(dirname "$0")
apis_dir="$cwd/../pkg/apis"

echo "Updating the CRD k8s deepcopy code..."
go run sigs.k8s.io/controller-tools/cmd/controller-gen object crd:crdVersions=v1 paths="$apis_dir/..."
