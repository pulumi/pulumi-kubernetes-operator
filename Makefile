GIT_COMMIT := $(shell git rev-parse --short HEAD)
VERSION := 0.1.0-$(GIT_COMMIT)
IMAGE_NAME := docker.io/pulumi/pulumi-kubernetes-operator

default: build

install-crds:
	kubectl apply -f deploy/crds/pulumi.com_stacks.yaml

codegen: generate-k8s generate-crds

generate-crds:
	./scripts/generate_crds.sh

generate-k8s:
	./scripts/generate_k8s.sh

build-image: build-static
	docker build --rm -t $(IMAGE_NAME):$(VERSION) -f Dockerfile .

build:
	./scripts/build.sh

build-static:
	./scripts/build.sh static

push-image:
	docker push $(IMAGE_NAME):$(VERSION)

push-image-latest: push-image
	# Tag and push the current version as latest
	docker tag $(IMAGE_NAME):$(VERSION) $(IMAGE_NAME):latest
	docker push $(IMAGE_NAME):latest

test:
	ginkgo -v ./test/...

deploy:
	kubectl apply -f deploy/service_account.yaml
	kubectl apply -f deploy/role.yaml
	kubectl apply -f deploy/role_binding.yaml
	sed -e "s#<IMG_NAME>:<IMG_VERSION>#$(IMAGE_NAME):$(VERSION)#g" deploy/operator.yaml | kubectl apply -f -

version:
	@echo $(VERSION)

dep-tidy:
	go mod tidy

release: test push-image-latest

.PHONY: build build-static codegen generate-crds install-crds generate-k8s test version dep-tidy build-image push-image push-image-latest release deploy
