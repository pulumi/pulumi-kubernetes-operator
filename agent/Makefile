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

GOLANGCI_LINT = $(shell pwd)/../bin/golangci-lint

.PHONY: lint
lint:  ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run --config ../.golangci.yml

.PHONY: lint-fix
lint-fix: ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix --config ../.golangci.yml
	
.PHONY: all agent protoc test

