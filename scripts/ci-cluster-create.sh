#!/bin/bash
set -o nounset -o errexit -o pipefail

echo Creating ephemeral Kubernetes cluster for CI testing...

pushd test/ci-cluster
yarn install --json --verbose >out
tail -10 out
pulumi stack init "${STACK}"
pulumi up --skip-preview --yes

mkdir -p "$HOME/.kube/"
pulumi stack output kubeconfig --show-secrets >~/.kube/config

popd
