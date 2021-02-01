# Copyright 2018 The Kubernetes Authors.
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

include ../../hack/make-rules/Makefile.manifest
include ../../hack/make-rules/BASEIMAGES

.PHONY: all push-manifest

REGISTRY = gcr.io/cri-tools
ALL_ARCH = amd64 arm64 ppc64le s390x mips64le
TAG = latest
IMAGES_LIST = hostnet-nginx
PORT=12003

all: hostnet-nginx push-manifest

hostnet-nginx:
	sed -i "s/listen\s*80;/listen ${PORT};/g" default.conf
	$(foreach arch,$(ALL_ARCH),docker build . -t $(REGISTRY)/$@-$(arch) --build-arg ARCH=$($(arch));)
	$(foreach arch,$(ALL_ARCH),docker push $(REGISTRY)/$@-$(arch);)

push-manifest: manifest-tool
	$(foreach image,$(IMAGES_LIST),$(MANIFEST_TOOL) push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/$(image)-ARCH:$(TAG) --target $(REGISTRY)/$(image):$(TAG);)
