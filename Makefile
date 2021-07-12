# Build Information
build_date := $(shell date +%Y-%m-%dT%H:%M:%SZ)
git_commit := $(shell git rev-parse --short HEAD)
OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)
UNAME := $(shell uname -s)

# versions
BUF_VERSION := v0.43.2

# Directories
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
TOOLS_SHARE_DIR := $(TOOLS_DIR)/share
TEST_E2E_DIR := test/e2e

PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

$(TOOLS_BIN_DIR):
	mkdir -p $@

$(TOOLS_SHARE_DIR):
	mkdir -p $@

# Binaries
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
GINKGO := $(TOOLS_BIN_DIR)/ginkgo
BUF := $(TOOLS_BIN_DIR)/buf
MOCKGEN:= $(TOOLS_BIN_DIR)/mockgen

.DEFAULT_GOAL := help

##@ Generate

.PHONY: generate
generate: $(BUF) $(MOCKGEN) ## Generate code
	go generate ./...
	
##@ Linting

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint
	$(GOLANGCI_LINT) run -v --fast=false

##@ Testing

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: test-e2e
test-e2e: ## Run e2e tests
	go test -timeout 30m -p 1 -v -tags=e2e ./test/e2e/...

.PHONY: compile-e2e
compile-e2e: # Test e2e compilation
	go test -c -o /dev/null -tags=e2e ./test/e2e

##@ Tools binaries

$(GOLANGCI_LINT): hack/tools/go.mod # Get and build golangci-lint
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/golangci/golangci-lint/cmd/golangci-lint

$(GINKGO): hack/tools/go.mod  # Get and build gginkgo
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/onsi/ginkgo/ginkgo

$(MOCKGEN): hack/tools/go.mod  # Get and build mockgen
	cd $(TOOLS_DIR); go build -tags=tools -o $(subst hack/tools/,,$@) github.com/golang/mock/mockgen

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


.PHONY: help
help:  ## Display this help. Thanks to https://suva.sh/posts/well-documented-makefiles/
ifeq ($(OS),Windows_NT)
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-40s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
else
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-40s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
endif
