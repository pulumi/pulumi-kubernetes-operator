# Build a base image with modules cached.
FROM --platform=${BUILDPLATFORM} golang:1.23 AS base
ARG TARGETARCH

# Install tini to reap zombie processes.
ENV TINI_VERSION v0.19.0
ADD --chmod=755 https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-static-${TARGETARCH} /tini

COPY /go.mod go.mod
COPY /go.sum go.sum

ENV GOCACHE=/root/.cache/go-build
ENV GOMODCACHE=/go/pkg/mod
RUN --mount=type=cache,target=${GOMODCACHE} \
    go mod download

# Build the operator.
FROM --platform=${BUILDPLATFORM} base AS op-builder
ARG TARGETOS
ARG TARGETARCH

ARG VERSION
RUN --mount=type=cache,target=${GOCACHE} \
    --mount=type=cache,target=${GOMODCACHE} \
    --mount=type=bind,source=/agent,target=./agent \
    --mount=type=bind,source=/operator,target=./operator \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -o /manager -ldflags="-X github.com/pulumi/pulumi-kubernetes-operator/v2/operator/version.Version=${VERSION}" ./operator/cmd/main.go

# Build the agent.
FROM --platform=${BUILDPLATFORM} base AS agent-builder
ARG TARGETOS
ARG TARGETARCH

ARG VERSION
RUN --mount=type=cache,target=${GOCACHE} \
    --mount=type=cache,target=${GOMODCACHE} \
    --mount=type=bind,source=/agent,target=./agent \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -o /agent -ldflags "-X github.com/pulumi/pulumi-kubernetes-operator/v2/agent/version.Version=${VERSION}" ./agent/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static-debian12:debug-nonroot

COPY --from=base /tini /tini
COPY --from=op-builder /manager /manager
COPY --from=agent-builder /agent /agent
USER 65532:65532

ENTRYPOINT ["/tini", "--"]
CMD ["/manager"]
