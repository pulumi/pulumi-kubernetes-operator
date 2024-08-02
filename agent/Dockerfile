# Build the agent binary
FROM golang:1.22 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY version/ version/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o agent main.go

# runtime image
FROM gcr.io/distroless/static-debian12:debug-nonroot

# RUN apt-get install tini

# install agent binary
WORKDIR /
COPY --from=builder /workspace/agent agent
# COPY init.sh /opt/pulumi-kubernetes-operator/init.sh

# The image is intended as a tool
# ENTRYPOINT ["tini", "--"]

# RUN useradd -m pulumi-kubernetes-agent
# RUN mkdir -p /home/pulumi-kubernetes-agent/.ssh \
#     && touch /home/pulumi-kubernetes-agent/.ssh/known_hosts \
#     && chmod 700 /home/pulumi-kubernetes-agent/.ssh \
#     && chown -R pulumi-kubernetes-agent:pulumi-kubernetes-agent /home/pulumi-kubernetes-agent/.ssh

# USER pulumi-kubernetes-agent

# ENV XDG_CONFIG_HOME=/tmp/.config
# ENV XDG_CACHE_HOME=/tmp/.cache
# ENV XDG_CONFIG_CACHE=/tmp/.cache
# ENV GOCACHE=/tmp/.cache/go-build
# ENV GOPATH=/tmp/.cache/go

