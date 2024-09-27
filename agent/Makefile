VERSION ?= $(shell git describe --tags --always --dirty)

all: protoc agent

protoc:
	cd pkg/proto && protoc --go_out=. --go_opt=paths=source_relative \
    	--go-grpc_out=. --go-grpc_opt=paths=source_relative *.proto

test:
	go test -covermode=atomic -coverprofile=coverage.out -coverpkg=./... -v ./...
	
agent: protoc
	go build -o pulumi-kubernetes-agent \
		-ldflags "-X github.com/pulumi/pulumi-kubernetes-operator/v2/agent/version.Version=${VERSION}" \
		main.go

.PHONY: all agent protoc test

