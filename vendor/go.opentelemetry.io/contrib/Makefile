TOOLS_MOD_DIR := ./tools

# All source code and documents. Used in spell check.
ALL_DOCS := $(shell find . -name '*.md' -type f | sort)
# All directories with go.mod files related to opentelemetry library. Used for building, testing and linting.
ALL_GO_MOD_DIRS := $(filter-out $(TOOLS_MOD_DIR), $(shell find . -type f -name 'go.mod' -exec dirname {} \; | sort))
ALL_COVERAGE_MOD_DIRS := $(shell find . -type f -name 'go.mod' -exec dirname {} \; | egrep -v '^./example|^$(TOOLS_MOD_DIR)' | sort)

# URLs to check if all contrib entries exist in the registry.
REGISTRY_BASE_URL = https://raw.githubusercontent.com/open-telemetry/opentelemetry.io/main/content/en/registry
CONTRIB_REPO_URL = https://github.com/open-telemetry/opentelemetry-go-contrib/tree/main

GO = go
GOTEST_MIN = $(GO) test -v -timeout 30s
GOTEST = $(GOTEST_MIN) -race
GOTEST_WITH_COVERAGE = $(GOTEST) -coverprofile=coverage.out -covermode=atomic

.DEFAULT_GOAL := precommit

.PHONY: precommit

TOOLS_DIR := $(abspath ./.tools)

$(TOOLS_DIR)/golangci-lint: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	$(GO) build -o $(TOOLS_DIR)/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

$(TOOLS_DIR)/misspell: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	$(GO) build -o $(TOOLS_DIR)/misspell github.com/client9/misspell/cmd/misspell

$(TOOLS_DIR)/gocovmerge: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	$(GO) build -o $(TOOLS_DIR)/gocovmerge github.com/wadey/gocovmerge

$(TOOLS_DIR)/stringer: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	$(GO) build -o $(TOOLS_DIR)/stringer golang.org/x/tools/cmd/stringer

MULTIMOD=$(TOOLS_DIR)/multimod
$(TOOLS_DIR)/multimod: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	$(GO) build -o $(TOOLS_DIR)/multimod go.opentelemetry.io/build-tools/multimod

precommit: dependabot-check license-check generate lint build test

.PHONY: test-with-coverage
test-with-coverage: $(TOOLS_DIR)/gocovmerge
	set -e; \
	printf "" > coverage.txt; \
	for dir in $(ALL_COVERAGE_MOD_DIRS); do \
	  CMD="$(GOTEST_WITH_COVERAGE)"; \
	  echo "$$dir" | \
	    grep -q 'test$$' && \
	    CMD="$$CMD -coverpkg=go.opentelemetry.io/contrib/$$( dirname "$$dir" | sed -e "s/^\.\///g" )/..."; \
	  echo "$$CMD $$dir/..."; \
	  (cd "$$dir" && \
	    $$CMD ./... && \
	    $(GO) tool cover -html=coverage.out -o coverage.html); \
	done; \
	$(TOOLS_DIR)/gocovmerge $$(find . -name coverage.out) > coverage.txt

.PHONY: ci
ci: precommit check-clean-work-tree test-with-coverage

.PHONY: test-gocql
test-gocql:
	@if ./tools/should_build.sh gocql; then \
	  set -e; \
	  docker run --name cass-integ --rm -p 9042:9042 -d cassandra:3; \
	  CMD=cassandra IMG_NAME=cass-integ ./tools/wait.sh; \
	  (cd instrumentation/github.com/gocql/gocql/otelgocql/test/ && \
	    $(GOTEST_WITH_COVERAGE) -coverpkg=go.opentelemetry.io/contrib/instrumentation/github.com/gocql/gocql/otelgocql/...  ./... && \
	    $(GO) tool cover -html=coverage.out -o coverage.html); \
	  cp ./instrumentation/github.com/gocql/gocql/otelgocql/test/coverage.out ./; \
	  docker stop cass-integ; \
	fi

.PHONY: test-mongo-driver
test-mongo-driver:
	@if ./tools/should_build.sh mongo-driver; then \
	  set -e; \
	  docker run --name mongo-integ --rm -p 27017:27017 -d mongo; \
	  CMD=mongo IMG_NAME=mongo-integ ./tools/wait.sh; \
	  (cd instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo/test && \
	    $(GOTEST_WITH_COVERAGE) -coverpkg=go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo/...  ./... && \
	    $(GO) tool cover -html=coverage.out -o coverage.html); \
	  cp ./instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo/test/coverage.out ./; \
	  docker stop mongo-integ; \
	fi

.PHONY: test-gomemcache
test-gomemcache:
	@if ./tools/should_build.sh gomemcache; then \
	  set -e; \
	  docker run --name gomemcache-integ --rm -p 11211:11211 -d memcached; \
	  CMD=gomemcache IMG_NAME=gomemcache-integ  ./tools/wait.sh; \
	  (cd instrumentation/github.com/bradfitz/gomemcache/memcache/otelmemcache/test && \
	    $(GOTEST_WITH_COVERAGE) -coverpkg=go.opentelemetry.io/contrib/instrumentation/github.com/bradfitz/gomemcache/memcache/otelmemcache/...  ./... && \
	    $(GO) tool cover -html=coverage.out -o coverage.html); \
	  docker stop gomemcache-integ ; \
	  cp ./instrumentation/github.com/bradfitz/gomemcache/memcache/otelmemcache/test/coverage.out ./; \
	fi

.PHONY: check-clean-work-tree
check-clean-work-tree:
	@if ! git diff --quiet; then \
	  echo; \
	  echo 'Working tree is not clean, did you forget to run "make precommit"?'; \
	  echo; \
	  git status; \
	  exit 1; \
	fi

.PHONY: build
build:
	# TODO: Fix this on windows.
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "compiling all packages in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GO) build ./... && \
	    $(GO) test -run xxxxxMatchNothingxxxxx ./... >/dev/null); \
	done

.PHONY: test
test:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "$(GO) test ./... + race in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOTEST) ./...); \
	done

.PHONY: test-short
test-short:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "$(GO) test ./... + race in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOTEST_MIN) -short ./...); \
	done

.PHONY: lint
lint: $(TOOLS_DIR)/golangci-lint $(TOOLS_DIR)/misspell lint-modules
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "golangci-lint in $${dir}"; \
	  (cd "$${dir}" && \
	    $(TOOLS_DIR)/golangci-lint run --fix && \
	    $(TOOLS_DIR)/golangci-lint run); \
	done
	$(TOOLS_DIR)/misspell -w $(ALL_DOCS)

.PHONY: lint-modules
lint-modules:
	set -e; for dir in $(ALL_GO_MOD_DIRS) $(TOOLS_MOD_DIR); do \
	  echo "$(GO) mod tidy in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GO) mod tidy); \
	done

.PHONY: generate
generate: $(TOOLS_DIR)/stringer
	PATH="$(TOOLS_DIR):$${PATH}" $(GO) generate ./...

.PHONY: license-check
license-check:
	@licRes=$$(for f in $$(find . -type f \( -iname '*.go' -o -iname '*.sh' \) ! -path './vendor/*' ! -path './exporters/otlp/internal/opentelemetry-proto/*') ; do \
	           awk '/Copyright The OpenTelemetry Authors|generated|GENERATED/ && NR<=3 { found=1; next } END { if (!found) print FILENAME }' $$f; \
	   done); \
	   if [ -n "$${licRes}" ]; then \
	           echo "license header checking failed:"; echo "$${licRes}"; \
	           exit 1; \
	   fi

.PHONY: registry-links-check
registry-links-check:
	@checkRes=$$( \
		for f in $$( find ./instrumentation ./exporters ./detectors ! -path './instrumentation/net/*' -type f -name 'go.mod' -exec dirname {} \; | egrep -v '/example|/utils' | sort ) \
			./instrumentation/net/http; do \
			TYPE="instrumentation"; \
			if $$(echo "$$f" | grep -q "exporters"); then \
				TYPE="exporter"; \
			fi; \
			if $$(echo "$$f" | grep -q "detectors"); then \
				TYPE="detector"; \
			fi; \
			NAME=$$(echo "$$f" | sed -e 's/.*\///' -e 's/.*otel//'); \
			LINK=$(CONTRIB_REPO_URL)/$$(echo "$$f" | sed -e 's/..//' -e 's/\/otel.*$$//'); \
			if ! $$(curl -s $(REGISTRY_BASE_URL)/$${TYPE}-go-$${NAME}.md | grep -q "$${LINK}"); then \
				echo "$$f"; \
			fi \
		done; \
	); \
	if [ -n "$$checkRes" ]; then \
		echo "WARNING: registry link check failed for the following packages:"; echo "$${checkRes}"; \
	fi

.PHONY: dependabot-check
dependabot-check:
	@result=$$( \
		for f in $$( find . -type f -name go.mod -exec dirname {} \; | sed 's/^.\/\?/\//' ); \
			do grep -q "$$f" .github/dependabot.yml \
			|| echo "$$f"; \
		done; \
	); \
	if [ -n "$$result" ]; then \
		echo "missing go.mod dependabot check:"; echo "$$result"; \
		exit 1; \
	fi

COREPATH ?= "../opentelemetry-go"
.PHONY: sync-core
sync-core: | $(MULTIMOD)
	@[ ! -d $COREPATH ] || ( echo ">> Path to core repository must be set in COREPATH and must exist"; exit 1 )
	$(MULTIMOD) verify && $(MULTIMOD) sync -a -o ${COREPATH}


.PHONY: prerelease
prerelease: | $(MULTIMOD)
	@[ "${MODSET}" ] || ( echo ">> env var MODSET is not set"; exit 1 )
	$(MULTIMOD) verify && $(MULTIMOD) prerelease -m ${MODSET}

COMMIT ?= "HEAD"
.PHONY: add-tags
add-tags: | $(MULTIMOD)
	@[ "${MODSET}" ] || ( echo ">> env var MODSET is not set"; exit 1 )
	$(MULTIMOD) verify && $(MULTIMOD) tag -m ${MODSET} -c ${COMMIT}
