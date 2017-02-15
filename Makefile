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

.PHONY: \
	help \
	check-gopath \
	critest \
	clean \
	binaries \
	install \
	uninstall
