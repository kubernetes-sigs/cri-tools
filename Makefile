# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO ?= go

# test for go module support
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
export GO_BUILD=GO111MODULE=on $(GO) build -mod=vendor
export GO_TEST=GO111MODULE=on $(GO) test -mod=vendor
else
export GO_BUILD=$(GO) build
export GO_TEST=$(GO) test
endif

GOOS := $(shell $(GO) env GOOS)
ifeq ($(GOOS),windows)
	BIN_EXT := .exe
endif

PROJECT := github.com/kubernetes-sigs/cri-tools
BINDIR ?= /usr/local/bin

VERSION := $(shell git describe --tags --dirty --always)
VERSION := $(VERSION:v%=%)
GO_LDFLAGS := -X $(PROJECT)/pkg/version.Version=$(VERSION)
BUILDTAGS := selinux

BUILD_PATH := $(shell pwd)/build
BUILD_BIN_PATH := $(BUILD_PATH)/bin

define go-build
	$(shell cd `pwd` && $(GO_BUILD) -o $(BUILD_BIN_PATH)/$(shell basename $(1)) $(1))
	@echo > /dev/null
endef

GINKGO := $(BUILD_BIN_PATH)/ginkgo
GOLANGCI_LINT := $(BUILD_BIN_PATH)/golangci-lint

all: binaries

help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'install' - Install binaries to system locations."
	@echo " * 'binaries' - Build critest and crictl."
	@echo " * 'clean' - Clean artifacts."

critest:
	CGO_ENABLED=0 $(GO_TEST) -c -o $(CURDIR)/_output/critest$(BIN_EXT) \
		-ldflags '$(GO_LDFLAGS)' \
		-tags '$(BUILDTAGS)' \
	     $(PROJECT)/cmd/critest

crictl:
	CGO_ENABLED=0 $(GO_BUILD) -o $(CURDIR)/_output/crictl$(BIN_EXT) \
		-ldflags '$(GO_LDFLAGS)' \
		-tags '$(BUILDTAGS)' \
		$(PROJECT)/cmd/crictl

clean:
	find . -name \*~ -delete
	find . -name \#\* -delete
	rm -rf _output/*

binaries: critest crictl

install-critest:
	install -D -m 755 $(CURDIR)/_output/critest$(BIN_EXT) $(BINDIR)/critest$(BIN_EXT)

install-crictl:
	install -D -m 755 $(CURDIR)/_output/crictl$(BIN_EXT) $(BINDIR)/crictl$(BIN_EXT)

install: install-critest install-crictl

uninstall-critest:
	rm -f $(BINDIR)/critest

uninstall-crictl:
	rm -f $(BINDIR)/crictl

uninstall: uninstall-critest uninstall-crictl

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run
	./hack/repo-infra/verify/verify-boilerplate.sh

install.tools: install.lint install.ginkgo

install.ginkgo: $(GINKGO)
install.lint: $(GOLANGCI_LINT)

$(GOLANGCI_LINT):
	export \
		VERSION=v1.21.0 \
		URL=https://raw.githubusercontent.com/golangci/golangci-lint \
		BINDIR=${BUILD_BIN_PATH} && \
	curl -sfL $$URL/$$VERSION/install.sh | sh -s $$VERSION


$(GINKGO):
	$(call go-build,./vendor/github.com/onsi/ginkgo/ginkgo)

release:
	hack/release.sh

# needs to run as root to work
test-e2e: $(GINKGO)
	$(GINKGO) $(TESTFLAGS) \
		-r -p \
		--randomizeAllSpecs \
		--randomizeSuites \
		--succinct \
		--slowSpecThreshold 60 \
		test

vendor:
	export GO111MODULE=on GOSUMDB= && \
		$(GO) mod tidy && \
		$(GO) mod vendor && \
		$(GO) mod verify

.PHONY: \
	help \
	critest \
	crictl \
	clean \
	binaries \
	install \
	install-critest \
	install-crictl \
	uninstall \
	uninstall-critest \
	uninstall-crictl \
	lint \
	install.tools \
	release \
	test-e2e \
	vendor
