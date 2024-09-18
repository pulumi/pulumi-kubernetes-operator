# Build the operator binary
FROM --platform=${BUILDPLATFORM} golang:1.22 AS op-builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /operator
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Multiple go.mod tax... the operator can't build without the agent's module and source.
COPY --from=agent / /agent/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . /operator/.

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.

ARG VERSION
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager -ldflags="-X github.com/pulumi/pulumi-kubernetes-operator/operator/version.Version=${VERSION}" cmd/main.go

# Build the agent binary
FROM golang:1.22 AS agent-builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /operator
# Copy the Go Modules manifests
COPY --from=agent /go.mod go.mod
COPY --from=agent /go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY --from=agent / .

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o agent .

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static-debian12:debug-nonroot
WORKDIR /
COPY --from=op-builder /operator/manager .
COPY --from=agent-builder /operator/agent .
USER 65532:65532

ENTRYPOINT ["/manager"]
