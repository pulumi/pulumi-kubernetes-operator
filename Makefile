SHELL := /usr/bin/env bash
GIT_COMMIT := $(shell git rev-parse --short HEAD)
VERSION := $(GIT_COMMIT)
CURRENT_RELEASE := $(shell git describe --abbrev=0 --tags)
RELEASE ?= $(shell git describe --abbrev=0 --tags)

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: codegen
codegen: generate-crds generate-crdocs ## Generate CRDs and documentation

CRD_BASES := operator/config/crd/bases/
CRDS := pulumi.com_stacks.yaml pulumi.com_programs.yaml auto.pulumi.com_workspaces.yaml auto.pulumi.com_updates.yaml
.PHONY: generate-crds
generate-crds:
	cd operator && $(MAKE) manifests
	cp $(addprefix $(CRD_BASES), $(CRDS)) deploy/crds/
	cp $(addprefix $(CRD_BASES), $(CRDS)) deploy/helm/pulumi-operator/crds/

.PHONY: generate-crdocs
generate-crdocs: crdoc ## Generate API Reference documentation into 'docs/crds/'.
	$(CRDOC) --resources deploy/crds/pulumi.com_stacks.yaml --output docs/stacks.md
	$(CRDOC) --resources deploy/crds/pulumi.com_programs.yaml --output docs/programs.md
	$(CRDOC) --resources deploy/crds/auto.pulumi.com_workspaces.yaml --output docs/workspaces.md
	$(CRDOC) --resources deploy/crds/auto.pulumi.com_updates.yaml --output docs/updates.md

.PHONY: test
test:
	cd agent && $(MAKE) test
	cd operator && $(MAKE) test

##@ Linting
GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.64.2
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] && $(GOLANGCI_LINT) version | grep -q $(subst v,,$(GOLANGCI_LINT_VERSION)) || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: lint
lint: golangci-lint ## Lint the code.
	$(MAKE) -C operator lint
	$(MAKE) -C agent lint

##@ Build

.PHONY: build
build: build-agent build-operator build-deploy ## Build the agent and operator binaries and deployment scripts. 	

.PHONY: build-agent
build-agent: ## Build the agent binary.
	@echo "Building agent"
	cd agent && $(MAKE) all

.PHONY: build-operator
build-operator: ## Build the operator manager binary.
	@echo "Building operator"
	cd operator && $(MAKE) all

.PHONY: build-image
build-image: ## Build the operator image.
	@echo "Building operator image"
	cd operator && $(MAKE) docker-build

.PHONY: push-image
push-image: ## Push the operator image.
	cd operator && $(MAKE) docker-push

.PHONY: build-deploy
build-deploy: generate-crds ## Build the deployment scripts.
	cd operator && $(MAKE) build-quickstart
	cp operator/dist/quickstart/install.yaml deploy/quickstart/install.yaml

##@ Deployment

.PHONY: install-crds
install-crds: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	cd operator && $(MAKE) install

.PHONY: deploy
deploy: ## Deploy controller manager to the K8s cluster specified in ~/.kube/config.
	cd operator && $(MAKE) deploy

##@ Release

.PHONY: prep 
prep: ## Prepare the next release (use RELEASE=<next-tag>).
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" README.md
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" agent/version/version.go
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" operator/Makefile
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" operator/version/version.go
	cd operator && $(MAKE) build-quickstart
	cp operator/dist/quickstart/install.yaml deploy/quickstart/install.yaml
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" deploy/deploy-operator-yaml/Pulumi.yaml
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" deploy/helm/pulumi-operator/Chart.yaml
	sed -i '' -e "s|$(CURRENT_RELEASE)|$(RELEASE)|g" deploy/helm/pulumi-operator/README.md

.PHONY: version
version:
	@echo $(VERSION)

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
CRDOC ?= $(LOCALBIN)/crdoc

## Tool Versions
CRDOC_VERSION ?= v0.5.2

.PHONY: crdoc
crdoc: $(CRDOC) ## Download crdoc locally if necessary. No version check.
$(CRDOC): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install fybrik.io/crdoc@$(CRDOC_VERSION)
