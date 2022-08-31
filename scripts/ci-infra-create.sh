#!/usr/bin/env bash
set -o nounset -o errexit -o pipefail

echo Creating S3 backend and KMS key

pushd test/s3backend
yarn install --json --verbose >out
tail -10 out
pulumi stack init "${STACK}"
pulumi up --skip-preview --yes
touch ~/.envfile
echo export PULUMI_S3_BACKEND_BUCKET="`pulumi stack output bucketName`" > ~/.envfile
echo export PULUMI_KMS_KEY="`pulumi stack output kmsKey`" >> ~/.envfile
popd
