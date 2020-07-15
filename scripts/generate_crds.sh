#!/bin/sh
set -e

cwd=$(dirname "$0")
apis_dir="$cwd/../pkg/apis"
deploy_dir="$cwd/../deploy/crds"

echo "Generating CRD API types..."

go run sigs.k8s.io/controller-tools/cmd/controller-gen crd paths="$apis_dir/..." output:crd:dir="$deploy_dir"

# Manually overwrite until issue is resolved in controller-tools:
# https://git.io/JJsjs
sed -i "s#conditions: null#conditions: []#g" "$deploy_dir/pulumi.com_stacks.yaml"
sed -i "s#storedVersions: null#storedVersions: []#g" "$deploy_dir/pulumi.com_stacks.yaml"
