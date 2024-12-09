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

MAKEFLAGS += --no-print-directory
GO ?= go

GOARCH ?= $(shell $(GO) env GOARCH)
GOOS ?= $(shell $(GO) env GOOS)
ifeq ($(GOOS),windows)
	BIN_EXT := .exe
endif

# test for go module support
ifeq ($(shell go help mod >/dev/null 2>&1 && echo true), true)
export GO_BUILD=GO111MODULE=on GOARCH=$(GOARCH) GOOS=$(GOOS) $(GO) build -mod=vendor
export GO_TEST=GO111MODULE=on GOARCH=$(GOARCH) GOOS=$(GOOS) $(GO) test -mod=vendor
else
export GO_BUILD=GOARCH=$(GOARCH) GOOS=$(GOOS) $(GO) build
export GO_TEST=GOARCH=$(GOARCH) GOOS=$(GOOS) $(GO) test
endif

PROJECT := sigs.k8s.io/cri-tools
BINDIR ?= /usr/local/bin

VERSION ?= $(shell git describe --tags --dirty --always | sed 's/^v//')
CGO_ENABLED ?= 0
GOFLAGS ?= -trimpath
GO_LDFLAGS := $(GO_LDFLAGS) -X $(PROJECT)/pkg/version.Version=$(VERSION)

BUILD_PATH := $(shell pwd)/build
BUILD_BIN_PATH := $(BUILD_PATH)/bin/$(GOOS)/$(GOARCH)

define go-build
	$(shell cd `pwd` && $(GO_BUILD) -o $(BUILD_BIN_PATH)/$(shell basename $(1)) $(1))
	@echo > /dev/null
endef

define curl_to
	curl -sSfL --retry 5 --retry-delay 3 "$(1)" -o $(2)
	chmod +x $(2)
endef

ZEITGEIST_VERSION = v0.5.3
GOLANGCI_LINT_VERSION := v1.62.2
REPO_INFRA_VERSION = v0.2.5

GINKGO := $(BUILD_BIN_PATH)/ginkgo
GOLANGCI_LINT := $(BUILD_BIN_PATH)/golangci-lint
ZEITGEIST := $(BUILD_BIN_PATH)/zeitgeist
VERIFY_BOILERPLATE := $(BUILD_BIN_PATH)/verify_boilerplate.py

CRITEST := $(BUILD_BIN_PATH)/critest$(BIN_EXT)
CRICTL := $(BUILD_BIN_PATH)/crictl$(BIN_EXT)

all: binaries

help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'install' - Install binaries to system locations."
	@echo " * 'binaries' - Build critest and crictl."
	@echo " * 'clean' - Clean artifacts."

critest:
	@$(MAKE) -B $(CRITEST)

$(CRITEST):
	CGO_ENABLED=$(CGO_ENABLED) $(GO_TEST) -c -o $@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(GOFLAGS) \
	     $(PROJECT)/cmd/critest

crictl:
	@$(MAKE) -B $(CRICTL)

$(CRICTL):
	CGO_ENABLED=$(CGO_ENABLED) $(GO_BUILD) -o $@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(GOFLAGS) \
		$(PROJECT)/cmd/crictl

clean:
	find . -name \*~ -delete
	find . -name \#\* -delete
	rm -rf $(BUILD_PATH)

binaries: critest crictl

install: $(CRITEST) $(CRICTL)
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $^ $(DESTDIR)$(BINDIR)/

verify-lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

verify-boilerplate: $(VERIFY_BOILERPLATE)
	$(VERIFY_BOILERPLATE) --boilerplate-dir hack/boilerplate

$(BUILD_BIN_PATH):
	mkdir -p $(BUILD_BIN_PATH)

$(VERIFY_BOILERPLATE): $(BUILD_BIN_PATH)
	$(call curl_to,https://raw.githubusercontent.com/kubernetes/repo-infra/$(REPO_INFRA_VERSION)/hack/verify_boilerplate.py,$(VERIFY_BOILERPLATE))

install.tools: $(GINKGO) $(GOLANGCI_LINT)

install.ginkgo: $(GINKGO)
install.lint: $(GOLANGCI_LINT)

$(GOLANGCI_LINT):
	export \
		VERSION=$(GOLANGCI_LINT_VERSION) \
		URL=https://raw.githubusercontent.com/golangci/golangci-lint \
		BINDIR=${BUILD_BIN_PATH} && \
	curl -sfL $$URL/$$VERSION/install.sh | sh -s $$VERSION


$(GINKGO):
	$(call go-build,./vendor/github.com/onsi/ginkgo/v2/ginkgo)

release:
	hack/release.sh

release-notes:
	hack/release-notes.sh

# needs to run as root to work
test-e2e: $(GINKGO)
	$(GINKGO) \
		-r \
		--randomize-all \
		--randomize-suites \
		--race \
		--vv \
		test \
		-- \
		$(TESTFLAGS)

test-crictl: $(GINKGO)
	# Run go test for templates_test.go and util_test.go
	CGO_ENABLED=$(CGO_ENABLED) $(GO_TEST) \
		-ldflags '$(GO_LDFLAGS)' \
		$(GOFLAGS) \
		$(PROJECT)/cmd/crictl
	$(GINKGO) $(TESTFLAGS) \
		-r -p \
		--randomize-all \
		--randomize-suites \
		--poll-progress-after 60s \
		cmd/crictl

vendor:
	export GO111MODULE=on GOSUMDB= && \
		$(GO) mod tidy && \
		$(GO) mod vendor && \
		$(GO) mod verify

$(ZEITGEIST): $(BUILD_BIN_PATH)
	$(call curl_to,https://storage.googleapis.com/k8s-artifacts-sig-release/kubernetes-sigs/zeitgeist/$(ZEITGEIST_VERSION)/zeitgeist-amd64-linux,$(ZEITGEIST))

verify-docs:
	hack/verify-docs.sh

verify-dependencies: $(BUILD_BIN_PATH)/zeitgeist
	$(ZEITGEIST) validate --local-only --base-path . --config dependencies.yaml

verify-go-modules:
	hack/verify-go-modules.sh

.PHONY: \
	help \
	critest \
	crictl \
	clean \
	binaries \
	install \
	install.tools \
	install.ginkgo \
	install.lint \
	release \
	release-notes \
	test-e2e \
	vendor \
	verify-boilerplate \
	verify-dependencies \
	verify-docs \
	verify-go-modules \
	verify-lint
