#!/bin/bash
set -e

cwd=$(dirname "$0")
apis_dir="$cwd/../pkg/apis"
deploy_dir="$cwd/../deploy/crds"

function pased() {
    if [ "$(uname)" = 'Darwin' ]; then
        sed -i '' -e "$1" "$2"
    else
        sed -i'' -e "$1" "$2"
    fi
}

echo "Generating CRD API types..."

controller-gen crd paths="$apis_dir/..." crd:crdVersions=v1 output:crd:dir="$deploy_dir"

# Manually overwrite until issue is resolved in controller-tools:
# https://git.io/JJsjs
pased "s#conditions: null#conditions: []#g" "$deploy_dir/pulumi.com_stacks.yaml"
pased "s#storedVersions: null#storedVersions: []#g" "$deploy_dir/pulumi.com_stacks.yaml"
