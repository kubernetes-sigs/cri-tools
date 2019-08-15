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

.PHONY: all-push all-push-images push-manifest

REGISTRY ?= gcr.io/cri-tools
TAG = latest
ALL_ARCH = amd64 arm64 ppc64le s390x
IMAGES_LIST = test-image-1 test-image-2 test-image-3 test-image-latest test-image-digest
IMAGE_TAGS_LIST = test all
SAME_IMAGE_TAGS_LIST = 1 2 3

all-push: all-push-images push-manifest

all-push-images: $(addprefix sub-push-,$(ALL_ARCH))

sub-push-%:
	$(foreach name,$(IMAGES_LIST),touch $(name) && docker build . -t $(REGISTRY)/$(name)-$*:$(TAG) --build-arg TEST=$(name) --build-arg ARCH=$($*) && rm -f $(name);)
	$(foreach name,$(IMAGES_LIST),docker push $(REGISTRY)/$(name)-$*:$(TAG);)
	$(foreach tag,$(IMAGE_TAGS_LIST),touch $(tag) && docker build . -t $(REGISTRY)/test-image-tag-$*:$(tag) --build-arg TEST=$(tag) --build-arg ARCH=$($*) && rm -f $(tag);)
	$(foreach tag,$(IMAGE_TAGS_LIST),docker push $(REGISTRY)/test-image-tag-$*:$(tag);)
	$(foreach tag,$(SAME_IMAGE_TAGS_LIST),touch same-image && docker build . -t $(REGISTRY)/test-image-tags-$*:$(tag) --build-arg TEST=same-image --build-arg ARCH=$($*) && rm -f same-image;)
	$(foreach tag,$(SAME_IMAGE_TAGS_LIST),docker push $(REGISTRY)/test-image-tag-$*:$(tag);)

push-manifest: manifest-tool
	$(foreach image,$(IMAGES_LIST),$(MANIFEST_TOOL) push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/$(image)-ARCH:$(TAG) --target $(REGISTRY)/$(image):$(TAG);)
	$(foreach tag,$(IMAGE_TAGS_LIST),$(MANIFEST_TOOL) push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/test-image-tag-ARCH:$(tag) --target $(REGISTRY)/test-image-tag:$(tag);)
	$(foreach tag,$(SAME_IMAGE_TAGS_LIST),$(MANIFEST_TOOL) push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/test-image-tag-ARCH:$(tag) --target $(REGISTRY)/test-image-tag:$(tag);)
