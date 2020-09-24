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

.PHONY: all docker-all oci-all hostnet-nginx push-docker-manifest oci-hostnet-nginx push-oci-manifest

REGISTRY = gcr.io/cri-tools
ALL_ARCH = amd64 arm64 ppc64le s390x
TAG = latest
DOCKER_IMAGES_LIST = hostnet-nginx
OCI_IMAGES_LIST = oci-hostnet-nginx
PORT=12003

all: docker-all oci-all
docker-all: hostnet-nginx push-docker-manifest

# TODO: Add push-oci-manifest target when 
# https://github.com/estesp/manifest-tool v2 is released
# because it will provide OCI support
oci-all: oci-hostnet-nginx

hostnet-nginx:
	sed -i "s/listen\s*80;/listen ${PORT};/g" default.conf
	$(foreach arch,$(ALL_ARCH),docker build . -t $(REGISTRY)/$@-$(arch) --build-arg ARCH=$($(arch));)
	$(foreach arch,$(ALL_ARCH),docker push $(REGISTRY)/$@-$(arch);)

oci-hostnet-nginx:
	../../hack/install-buildah.sh
	sed -i "s/listen\s*80;/listen ${PORT};/g" default.conf
	$(foreach arch,$(ALL_ARCH),buildah build-using-dockerfile -t $(REGISTRY)/$@-$(arch) --build-arg ARCH=$($(arch));)
	$(foreach arch,$(ALL_ARCH),buildah push $(REGISTRY)/$@-$(arch);)

push-docker-manifest: manifest-tool
	$(foreach image,$(DOCKER_IMAGES_LIST),$(MANIFEST_TOOL) push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/$(image)-ARCH:$(TAG) --target $(REGISTRY)/$(image):$(TAG);)

# TODO: Dependant on OCI support which will be available
# when https://github.com/estesp/manifest-tool v2 is released
push-oci-manifest: manifest-tool
	$(foreach image,$(OCI_IMAGES_LIST),$(MANIFEST_TOOL) push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/$(image)-ARCH:$(TAG) --target $(REGISTRY)/$(image):$(TAG);)
