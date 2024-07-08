#!/usr/bin/env bash
set -e

cwd=$(dirname "$0")
apis_dir="$cwd/../pkg/apis"
deploy_dir="$cwd/../deploy/crds"

echo "Generating CRD API types..."

controller-gen paths="$apis_dir/..." crd:crdVersions=v1 output:crd:dir="$deploy_dir"
cp "${deploy_dir}/pulumi.com_programs.yaml" "${deploy_dir}/../helm/pulumi-operator/crds/program-crd.yaml"
cp "${deploy_dir}/pulumi.com_stacks.yaml" "${deploy_dir}/../helm/pulumi-operator/crds/stack-crd.yaml"
