VERSION ?= $(shell git describe --tags --always --dirty)

all: protoc agent

ensure:
	go mod tidy
	
protoc:
	@echo "Generating Go files"
	cd pkg/proto && protoc --go_out=. --go_opt=paths=source_relative \
    	--go-grpc_out=. --go-grpc_opt=paths=source_relative *.proto

agent: protoc
	@echo "Building agent"
	go build -o pulumi-kubernetes-agent \
		-ldflags "-X github.com/pulumi/pulumi-kubernetes-operator/agent/version.Version=${VERSION}" \
		github.com/pulumi/pulumi-kubernetes-operator/agent

image: agent
	docker build --build-arg="VERSION=$(VERSION)" -t pulumi/pulumi-kubernetes-agent:latest .

.PHONY: agent protoc image

