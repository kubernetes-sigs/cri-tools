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
GOBINDIR := $(word 1,$(subst :, ,$(GOPATH)))
PATH := $(GOBINDIR)/bin:$(PATH)

all: binaries

help:
	@echo "Usage: make <target>"
	@echo
	@echo " * 'install' - Install binaries to system locations."
	@echo " * 'binaries' - Build critest."
	@echo " * 'clean' - Clean artifacts."

check-gopath:
ifndef GOPATH
	$(error GOPATH is not set)
endif

critest: check-gopath
		$(GO) install \
		$(PROJECT)/cmd/critest

clean:
	find . -name \*~ -delete
	find . -name \#\* -delete

binaries: critest

install: check-gopath
	install -D -m 755 $(GOBINDIR)/bin/critest $(BINDIR)/critest

uninstall:
	rm -f $(BINDIR)/critest

lint:
	./hack/repo-infra/verify/go-tools/verify-gometalinter.sh

install.tools:
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install


.PHONY: \
	help \
	check-gopath \
	critest \
	clean \
	binaries \
	install \
	uninstall \
	lint \
	install.tools
