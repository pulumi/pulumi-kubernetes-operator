# Build the operator binary
FROM --platform=${BUILDPLATFORM} golang:1.23 AS op-builder
ARG TARGETOS
ARG TARGETARCH

# Copy the Go Modules manifests
COPY /go.mod go.mod
COPY /go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY / .

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.

ARG VERSION
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o /manager -ldflags="-X github.com/pulumi/pulumi-kubernetes-operator/v2/operator/version.Version=${VERSION}" ./operator/cmd/main.go

# Build the agent binary
FROM --platform=${BUILDPLATFORM} golang:1.23 AS agent-builder
ARG TARGETOS
ARG TARGETARCH

# Copy the Go Modules manifests
COPY /go.mod go.mod
COPY /go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY / .

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o /agent -ldflags "-X github.com/pulumi/pulumi-kubernetes-operator/v2/agent/version.Version=${VERSION}" ./agent/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static-debian12:debug-nonroot
COPY --from=op-builder /manager /manager
COPY --from=agent-builder /agent /agent
USER 65532:65532

ENTRYPOINT ["/manager"]
