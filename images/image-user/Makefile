# Copyright 2020 The Kubernetes Authors.
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

# Build multi-arch test images for non-x86 architecture

.PHONY: all manifest-tool push-manifest

MANIFEST_TOOL_DIR := $(shell mktemp -d)
export PATH := $(MANIFEST_TOOL_DIR):$(PATH)

MANIFEST_TOOL_VERSION := v0.7.0
USERNAME ?= oauth2accesstoken
PASSWORD ?= $(shell gcloud auth print-access-token)
MANIFEST_TOOL = manifest-tool --username=$(USERNAME) --password=$(PASSWORD)

space :=
space +=
comma := ,
prefix_linux = $(addprefix linux/,$(strip $1))
join_platforms = $(subst $(space),$(comma),$(call prefix_linux,$(strip $1)))

REGISTRY = gcr.io/$(PROJ_ID)
ALL_ARCH = amd64 arm arm64 ppc64le s390x mips64le
TAG = latest
IMAGES_LIST = test-image-user-uid test-image-user-username test-image-user-uid-group test-image-user-username-group

all: manifest-tool push-manifest

manifest-tool:
	curl -sSL https://github.com/estesp/manifest-tool/releases/download/$(MANIFEST_TOOL_VERSION)/manifest-tool-linux-amd64 > $(MANIFEST_TOOL_DIR)/manifest-tool
	chmod +x $(MANIFEST_TOOL_DIR)/manifest-tool

push-manifest:
	$(foreach image,$(IMAGES_LIST),$(MANIFEST_TOOL) push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/$(image)-ARCH:$(TAG) --target $(REGISTRY)/$(image):$(TAG);)
