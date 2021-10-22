#!/usr/bin/env bash
set -o nounset -o errexit -o pipefail

export GO111MODULE=on
export GOARCH=amd64
export GOOS=linux

name="pulumi-kubernetes-operator"
build_static="${1:-}"

ldflags="-X github.com/pulumi/pulumi-kubernetes-operator/version.Version=${VERSION}"
extra_args=""

# Extra build args for a static binary if requested.
if [ "$build_static" == "static" ]; then
	export CGO_ENABLED=0
	ldflags="$ldflags -w -extldflags \"-static\""
	extra_args="-x -a -tags netgo"
fi

# Build the operator.
/usr/bin/env bash -c "go build -o $name -ldflags \"${ldflags:-}\" $extra_args ./cmd/manager/main.go"
chmod +x "$name"
