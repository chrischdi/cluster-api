SHELL:=/usr/bin/env bash

GO_VERSION ?= 1.18.3
GO_CONTAINER_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

TAG ?= dev
ARCH ?= $(shell go env GOARCH)

EXTENSION_IMG ?= chrischdi/skip-worker-minor

.PHONY: docker-build
docker-build: ## Build the docker image for core controller manager
	DOCKER_BUILDKIT=1 docker build --build-arg builder_image=$(GO_CONTAINER_IMAGE) --build-arg goproxy=$(GOPROXY) --build-arg ARCH=$(ARCH) . -t $(EXTENSION_IMG)-$(ARCH):$(TAG) --file ./Dockerfile
	# $(MAKE) set-manifest-image MANIFEST_IMG=$(TEST_EXTENSION_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./test/extension/config/default/extension_image_patch.yaml"
	# $(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./test/extension/config/default/extension_pull_policy.yaml"
