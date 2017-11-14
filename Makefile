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
PROJECT := github.com/kubernetes-incubator/cri-tools
BINDIR := /usr/local/bin
ifeq ($(GOPATH),)
export GOPATH := $(CURDIR)/_output
unexport GOBIN
endif
GOBINDIR := $(word 1,$(subst :, ,$(GOPATH)))
PATH := $(GOBINDIR)/bin:$(PATH)
GOPKGDIR := $(GOPATH)/src/$(PROJECT)
GOPKGBASEDIR := $(shell dirname "$(GOPKGDIR)")


all: binaries

help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'install' - Install binaries to system locations."
	@echo " * 'binaries' - Build critest and crictl."
	@echo " * 'clean' - Clean artifacts."

check-gopath:
ifeq ("$(wildcard $(GOPKGDIR))","")
	mkdir -p "$(GOPKGBASEDIR)"
	ln -s "$(CURDIR)" "$(GOPKGBASEDIR)/cri-tools"
endif
ifndef GOPATH
	$(error GOPATH is not set)
endif

critest: check-gopath
		$(GO) install \
		$(PROJECT)/cmd/critest

crictl: check-gopath
		$(GO) install \
		$(PROJECT)/cmd/crictl

clean:
	find . -name \*~ -delete
	find . -name \#\* -delete

binaries: critest crictl

install: check-gopath
	install -D -m 755 $(GOBINDIR)/bin/critest $(BINDIR)/critest
	install -D -m 755 $(GOBINDIR)/bin/crictl $(BINDIR)/crictl

uninstall:
	rm -f $(BINDIR)/critest
	rm -f $(BINDIR)/crictl

lint:
	./hack/repo-infra/verify/go-tools/verify-gometalinter.sh
	./hack/repo-infra/verify/verify-go-src.sh -r $(shell pwd) -v
	./hack/repo-infra/verify/verify-boilerplate.sh

gofmt:
	./hack/repo-infra/verify/go-tools/verify-gofmt.sh

install.tools:
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install


.PHONY: \
	help \
	check-gopath \
	critest \
	crictl \
	clean \
	binaries \
	install \
	uninstall \
	lint \
	gofmt \
	install.tools
