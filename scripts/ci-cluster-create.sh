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

echo Creating S3 backend and KMS key

pushd test/s3backend
yarn install --json --verbose >out
tail -10 out
pulumi stack init "${STACK}"
pulumi up --skip-preview --yes
touch ~/.envfile
echo PULUMI_S3_BACKEND_BUCKET="`pulumi stack output bucketName`" > ~/.envfile
echo PULUMI_KMS_KEY="`pulumi stack output kmsKey`" >> ~/.envfile
popd
