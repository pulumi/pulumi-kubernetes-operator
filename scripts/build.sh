#!/bin/bash
set -o nounset -o errexit -o pipefail

export GO111MODULE=on
export GOARCH=amd64
export GOOS=linux

name="pulumi-kubernetes-operator"
build_static="${1:-}"

# Extra build args for a static binary if requested.
if [ "$build_static" == "static" ]; then
		export CGO_ENABLED=0
		extra_args="$(cat <<-EOF
-ldflags '-w -extldflags "-static"' \
-x -a -tags netgo
EOF
)"
fi

# Build the operator.
/bin/bash -c "go build -o $name ${extra_args:-} ./cmd/manager/main.go"
chmod +x "$name"
