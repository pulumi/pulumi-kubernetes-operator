#!/bin/bash
set -o nounset -o errexit -o pipefail

echo Deleting ephemeral Kubernetes cluster...

pushd test/ci-cluster
pulumi stack select "${STACK}" && \
  pulumi destroy --skip-preview --yes && \
  pulumi stack rm --yes
popd

echo Deleting S3 backend and KMS Key...
pushd test/s3backend

pulumi stack select "${STACK}"
bucket=`pulumi stack output bucketName`
echo Deleting contents in S3 bucket $bucket...
aws s3 rm s3://${bucket} --recursive

echo Destroying stack
pulumi destroy --skip-preview --yes && \
  pulumi stack rm --yes
popd

