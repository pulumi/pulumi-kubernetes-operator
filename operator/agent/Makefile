all: protoc agent

protoc:
	@echo "Generating Go files"
	cd pkg/proto && protoc --go_out=. --go_opt=paths=source_relative \
    	--go-grpc_out=. --go-grpc_opt=paths=source_relative *.proto

agent: protoc
	@echo "Building agent"
	GOOS=linux GOARCH=amd64 go build -o ../pulumi-kubernetes-agent \
		github.com/pulumi/pulumi-kubernetes-operator/agent/cmd

image: agent
	docker build -t pulumi/pulumi-kubernetes-agent:latest .

.PHONY: agent protoc image

