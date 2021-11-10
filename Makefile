# Build Information
BUILD_DATE := $(shell date +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short HEAD)
VERSION := $(shell git describe --always --match "v*")
VERSION_PKG := github.com/weaveworks/flintlock/internal/version
OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)
UNAME := $(shell uname -s)

# Versions
BUF_VERSION := v0.43.2

# Directories
REPO_ROOT := $(shell git rev-parse --show-toplevel)
BIN_DIR := bin
OUT_DIR := out
FLINTLOCKD_CMD := cmd/flintlockd
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
TOOLS_SHARE_DIR := $(TOOLS_DIR)/share
TEST_E2E_DIR := test/e2e

PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

$(TOOLS_BIN_DIR):
	mkdir -p $@

$(TOOLS_SHARE_DIR):
	mkdir -p $@

$(BIN_DIR):
	mkdir -p $@

$(OUT_DIR):
	mkdir -p $@

# Binaries
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
GINKGO := $(TOOLS_BIN_DIR)/ginkgo
BUF := $(TOOLS_BIN_DIR)/buf
MOCKGEN:= $(TOOLS_BIN_DIR)/mockgen
PROTOC_GEN_DOC := $(TOOLS_BIN_DIR)/protoc-gen-doc
PROTOC_GEN_GO := $(TOOLS_BIN_DIR)/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(TOOLS_BIN_DIR)/protoc-gen-go-grpc
PROTO_GEN_GRPC_GW := $(TOOLS_BIN_DIR)/protoc-gen-grpc-gateway
PROTO_GEN_GRPC_OAPI := $(TOOLS_BIN_DIR)/protoc-gen-openapiv2
WIRE := $(TOOLS_BIN_DIR)/wire

# Useful things
test_image = weaveworks/flintlock-e2e

.DEFAULT_GOAL := help

##@ Build

.PHONY: build
build: $(BIN_DIR) ## Build the binaries
	go build -o $(BIN_DIR)/flintlockd ./cmd/flintlockd

.PHONY: build-release
build-release: $(BIN_DIR) ## Build the release binaries
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o $(BIN_DIR)/flintlockd_amd64 -ldflags "-X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).BuildDate=$(BUILD_DATE) -X $(VERSION_PKG).CommitHash=$(GIT_COMMIT)" ./cmd/flintlockd
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BIN_DIR)/flintlockd_arm64 -ldflags "-X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).BuildDate=$(BUILD_DATE) -X $(VERSION_PKG).CommitHash=$(GIT_COMMIT)" ./cmd/flintlockd

##@ Generate

.PHONY: generate
generate: $(BUF) $(MOCKGEN)
generate: ## Generate code
	$(MAKE) generate-go
	$(MAKE) generate-proto
	$(MAKE) generate-di

.PHONY: generate-go
generate-go: $(MOCKGEN) ## Generate Go Code
	go generate ./...

.PHONY: generate-proto ## Generate protobuf/grpc code
generate-proto: $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC) $(PROTO_GEN_GRPC_GW) $(PROTO_GEN_GRPC_OAPI) $(PROTOC_GEN_DOC)
	$(BUF) generate

.PHONY: generate-di ## Generate the dependency injection code
generate-di: $(WIRE)
	$(WIRE) gen github.com/weaveworks/flintlock/internal/inject

##@ Linting

.PHONY: lint
lint: $(GOLANGCI_LINT) $(BUF) ## Lint
	$(GOLANGCI_LINT) run -v --fast=false
	$(BUF) lint

##@ Testing

.PHONY: test
test: ## Run unit tests
	go test -v -race ./...

.PHONY: test-with-cov
test-with-cov: ## Run unit tests with coverage
	go test -v -race -timeout 2m -p 1 -covermode=atomic -coverprofile=coverage.txt -exec sudo ./...

.PHONY: test-e2e
test-e2e: compile-e2e ## Run e2e tests locally
		go test -timeout 30m -p 1 -v -tags=e2e ./test/e2e/...

.PHONY: test-e2e-docker
test-e2e-docker: compile-e2e ## Run e2e tests locally in a container
	docker run --rm -it \
		--privileged \
		--volume /dev:/dev \
		--volume /run/udev/control:/run/udev/control \
		--volume $(REPO_ROOT):/src/flintlock \
		--ipc=host \
		--workdir=/src/flintlock \
		$(test_image):latest \
		/bin/bash -c "make test-e2e"

.PHONY: test-e2e-metal
test-e2e-metal: ## Run e2e tests in Equinix
	echo "must set EQUINIX_ORG_ID"
	./test/tools/run.py run-e2e -o $(EQUINIX_ORG_ID)

.PHONY: compile-e2e
compile-e2e: ## Test e2e compilation
	go test -c -o /dev/null -tags=e2e ./test/e2e

##@ Docker

.PHONY: docker-build
docker-build: ## Build the e2e docker image
	docker build -t $(test_image):latest -f test/docker/Dockerfile.e2e .

.PHONY: docker-push
docker-push: docker-build ## Push the e2e docker image to weaveworks/fl-e2e
	docker push $(test_image):latest

##@ Tools binaries

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod # Get and build golangci-lint
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/golangci/golangci-lint/cmd/golangci-lint

$(GINKGO): $(TOOLS_DIR)/go.mod  # Get and build gginkgo
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/onsi/ginkgo/ginkgo

$(MOCKGEN): $(TOOLS_DIR)/go.mod  # Get and build mockgen
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/golang/mock/mockgen

$(PROTOC_GEN_GO): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) google.golang.org/protobuf/cmd/protoc-gen-go

$(PROTOC_GEN_DOC): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc

$(PROTOC_GEN_GO_GRPC): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) google.golang.org/grpc/cmd/protoc-gen-go-grpc

$(PROTO_GEN_GRPC_GW): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway

$(PROTO_GEN_GRPC_OAPI): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2

$(WIRE): $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/google/wire/cmd/wire

BUF_TARGET := buf-Linux-x86_64.tar.gz

ifeq ($(OS), darwin)
BUF_TARGET := buf-Darwin-x86_64.tar.gz
endif

BUF_SHARE := $(TOOLS_SHARE_DIR)/buf.tar.gz
$(BUF_SHARE): $(TOOLS_SHARE_DIR)
	curl -sL -o $(BUF_SHARE) "https://github.com/bufbuild/buf/releases/download/$(BUF_VERSION)/$(BUF_TARGET)"

$(BUF): $(TOOLS_BIN_DIR) $(BUF_SHARE)
	tar xfvz $(TOOLS_SHARE_DIR)/buf.tar.gz  -C $(TOOLS_SHARE_DIR) buf/bin
	cp $(TOOLS_SHARE_DIR)/buf/bin/* $(TOOLS_BIN_DIR)
	rm -rf $(TOOLS_SHARE_DIR)/buf

##@ Docs
.PHONY: docs-install
docs-install:
	@if [ ! -d "userdocs/node_modules" ]; then \
		echo " >>> npm install"; \
		cd ./userdocs && npm install; \
	fi

.PHONY: docs-build
docs-build: ## Build userdocs site
docs-build: generate-proto docs-install
	cd ./userdocs && yarn build

.PHONY: docs-deploy
docs-deploy: docs-build
	cd ./userdocs && \
		DEPLOYMENT_BRANCH=gh-pages \
		USE_SSH=true \
		yarn deploy

##@ Utility

.PHONY: help
help:  ## Display this help. Thanks to https://www.thapaliya.com/en/writings/well-documented-makefiles/
ifeq ($(OS),Windows_NT)
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-40s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
else
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-40s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
endif
