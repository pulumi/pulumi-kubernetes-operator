#!/bin/sh
set -e

cwd=$(dirname "$0")
apis_dir="$cwd/../pkg/apis"
deploy_dir="$cwd/../deploy/crds"

echo "Generating CRD API types..."

controller-gen crd paths="$apis_dir/..." crd:crdVersions=v1 output:crd:dir="$deploy_dir"

# Manually overwrite until issue is resolved in controller-tools:
# https://git.io/JJsjs
# Requires gnu-sed. On Macs you might have to do the following: 
# export PATH="/usr/local/opt/gnu-sed/libexec/gnubin:$PATH"
sed -i "s#conditions: null#conditions: []#g" "$deploy_dir/pulumi.com_stacks.yaml"
sed -i "s#storedVersions: null#storedVersions: []#g" "$deploy_dir/pulumi.com_stacks.yaml"
