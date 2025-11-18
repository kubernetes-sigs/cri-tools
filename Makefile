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
GOLANGCI_LINT_VERSION := v2.6.2
REPO_INFRA_VERSION = v0.2.5

GINKGO := $(BUILD_BIN_PATH)/ginkgo
GOLANGCI_LINT := $(BUILD_BIN_PATH)/golangci-lint
ZEITGEIST := $(BUILD_BIN_PATH)/zeitgeist
VERIFY_BOILERPLATE := $(BUILD_BIN_PATH)/verify_boilerplate.py

CRITEST := $(BUILD_BIN_PATH)/critest$(BIN_EXT)
CRICTL := $(BUILD_BIN_PATH)/crictl$(BIN_EXT)

all: binaries

COLOR:=\\033[36m
NOCOLOR:=\\033[0m
WIDTH:=30

.PHONY: helpIn
help: ## Display this help.
	@awk \
		-v "col=${COLOR}" -v "nocol=${NOCOLOR}" \
		' \
			BEGIN { \
				FS = ":.*##" ; \
				printf "Usage:\n  make %s<target>%s\n", col, nocol \
			} \
			/^[./a-zA-Z0-9_-]+:.*?##/ { \
				printf "  %s%-${WIDTH}s%s %s\n", col, $$1, nocol, $$2 \
			} \
			/^##@/ { \
				printf "\n%s\n", substr($$0, 5) \
			} \
		' $(MAKEFILE_LIST)

##@ Build targets:

.PHONY: binaries
binaries: critest crictl ## Build the critest and crictl binaries.

.PHONY: critest
critest: ## Build the critest binary.
	@$(MAKE) -B $(CRITEST)

$(CRITEST):
	CGO_ENABLED=$(CGO_ENABLED) $(GO_TEST) -c -o $@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(GOFLAGS) \
	     $(PROJECT)/cmd/critest

.PHONY: crictl
crictl: ## Build the crictl binary.
	@$(MAKE) -B $(CRICTL)

$(CRICTL):
	CGO_ENABLED=$(CGO_ENABLED) $(GO_BUILD) -o $@ \
		-ldflags '$(GO_LDFLAGS)' \
		$(GOFLAGS) \
		$(PROJECT)/cmd/crictl

.PHONY: clean
clean: ## Clean the repository.
	find . -name \*~ -delete
	find . -name \#\* -delete
	rm -rf $(BUILD_PATH)


.PHONY: install
install: $(CRITEST) $(CRICTL) ## Install critest and crictl.
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $^ $(DESTDIR)$(BINDIR)/

.PHONY: release
release: ## Build a release.
	hack/release.sh

##@ Verify targets:

.PHONY: verify-lint
verify-lint: $(GOLANGCI_LINT) ## Run golangci-lint.
	$(GOLANGCI_LINT) run

.PHONY: verify-boilerplate
verify-boilerplate: $(VERIFY_BOILERPLATE) ## Verify the boilerplate headers.
	$(VERIFY_BOILERPLATE) --boilerplate-dir hack/boilerplate

$(BUILD_BIN_PATH):
	mkdir -p $(BUILD_BIN_PATH)

$(VERIFY_BOILERPLATE): $(BUILD_BIN_PATH)
	$(call curl_to,https://raw.githubusercontent.com/kubernetes/repo-infra/$(REPO_INFRA_VERSION)/hack/verify_boilerplate.py,$(VERIFY_BOILERPLATE))

.PHONY: verify-docs
verify-docs: ## Verify the documentation.
	hack/verify-docs.sh

.PHONY: verify-dependencies
verify-dependencies: $(BUILD_BIN_PATH)/zeitgeist ## Verify third party dependencies.
	$(ZEITGEIST) validate --local-only --base-path . --config dependencies.yaml

$(ZEITGEIST): $(BUILD_BIN_PATH)
	$(call curl_to,https://storage.googleapis.com/k8s-artifacts-sig-release/kubernetes-sigs/zeitgeist/$(ZEITGEIST_VERSION)/zeitgeist-amd64-linux,$(ZEITGEIST))

.PHONY: verify-go-modules
verify-go-modules: ## Verify vendored golang modules.
	hack/verify-go-modules.sh

##@ Test targets:

.PHONY: test-e2e
test-e2e: $(GINKGO) ## Run the e2e test suite.
	$(GINKGO) \
		-r \
		--randomize-all \
		--randomize-suites \
		--race \
		--vv \
		test \
		-- \
		$(TESTFLAGS)

.PHONY: test-crictl
test-crictl: $(GINKGO) ## Run the crictl test suite.
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

##@ Utility targets:

.PHONY: install.tools
install.tools: $(GINKGO) $(GOLANGCI_LINT) ## Install all required verification tools.

.PHONY: install.ginkgo
install.ginkgo: $(GINKGO) ## Install ginkgo.

$(GINKGO):
	$(call go-build,./vendor/github.com/onsi/ginkgo/v2/ginkgo)

.PHONY: install.lint
install.lint: $(GOLANGCI_LINT) ## Install golangci-lint.

$(GOLANGCI_LINT):
	export \
		VERSION=$(GOLANGCI_LINT_VERSION) \
		URL=https://raw.githubusercontent.com/golangci/golangci-lint \
		BINDIR=${BUILD_BIN_PATH} && \
	curl -sfL $$URL/$$VERSION/install.sh | sh -s $$VERSION

.PHONY: vendor
vendor: ## Update vendored golang modules.
	export GO111MODULE=on GOSUMDB= && \
		$(GO) mod tidy && \
		$(GO) mod vendor && \
		$(GO) mod verify

.PHONY: release-notes
release-notes: ## Build the release notes.
	hack/release-notes.sh
