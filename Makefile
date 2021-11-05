SHELL := /usr/bin/env bash
GIT_COMMIT := $(shell git rev-parse --short HEAD)
VERSION := $(GIT_COMMIT)
PUBLISH_IMAGE_NAME := pulumi/pulumi-kubernetes-operator
IMAGE_NAME := docker.io/$(shell whoami)/pulumi-kubernetes-operator
CURRENT_RELEASE := $(shell git describe --abbrev=0 --tags)
RELEASE ?= $(shell git describe --abbrev=0 --tags)

default: build

install-crds:
	kubectl apply -f deploy/crds/pulumi.com_stacks.yaml

codegen: install-controller-gen install-crdoc generate-k8s generate-crds generate-crdocs

install-controller-gen:
	@echo "Installing controller-gen to GOPATH/bin"; pushd /tmp >& /dev/null && go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.5.0 ; popd >& /dev/null

install-crdoc:
	@echo "Installing crdoc to go GOPATH/bin"; pushd /tmp >& /dev/null && go install fybrik.io/crdoc@v0.5.2; popd >& /dev/null

generate-crds:
	./scripts/generate_crds.sh

generate-k8s:
	./scripts/generate_k8s.sh

generate-crdocs:
	crdoc --resources deploy/crds/pulumi.com_stacks.yaml --output docs/stacks.md

build-image: build-static
	docker build --rm -t $(IMAGE_NAME):$(VERSION) -f Dockerfile .

build:
	VERSION=$(VERSION) ./scripts/build.sh

build-static:
	VERSION=$(VERSION) ./scripts/build.sh static

push-image:
	docker push $(IMAGE_NAME):$(VERSION)

test: install-crds
	ginkgo -v ./test/...

deploy:
	kubectl apply -f deploy/yaml/service_account.yaml
	kubectl apply -f deploy/yaml/role.yaml
	kubectl apply -f deploy/yaml/role_binding.yaml
	sed -e "s#<IMG_NAME>:<IMG_VERSION>#$(IMAGE_NAME):$(VERSION)#g" deploy/operator_template.yaml | kubectl apply -f -

# Run make prep RELEASE=<next-tag> to prep next release
prep: prep-spec prep-docs prep-code

prep-docs:
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" README.md

# Run make prep-spec RELEASE=<next-tag> to prep the spec
prep-spec:
	sed -e "s#<IMG_NAME>:<IMG_VERSION>#$(PUBLISH_IMAGE_NAME):$(RELEASE)#g" deploy/operator_template.yaml > deploy/yaml/operator.yaml

prep-code:
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" deploy/deploy-operator-ts/index.ts deploy/deploy-operator-py/__main__.py deploy/deploy-operator-go/main.go deploy/deploy-operator-cs/MyStack.cs

version:
	@echo $(VERSION)

dep-tidy:
	go mod tidy

.PHONY: build build-static codegen generate-crds install-crds generate-k8s test version dep-tidy build-image push-image push-image-latest deploy prep-spec
