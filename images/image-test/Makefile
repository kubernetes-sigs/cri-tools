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

.PHONY: all-push all-push-images test-image-tags test-image-tag push-manifest

REGISTRY ?= gcr.io/cri-tools
TAG = latest
ALL_ARCH = amd64 arm64 ppc64le s390x
IMAGES_LIST = test-image-1 test-image-2 test-image-3 test-image-latest test-image-digest

all-push: all-push-images test-image-tags test-image-tag push-manifest

all-push-images: $(addprefix sub-push-,$(ALL_ARCH))

sub-push-%:
	$(foreach name,$(IMAGES_LIST),touch $(name) && docker build . -t $(REGISTRY)/$(name)-$*:$(TAG) --build-arg TEST=$(name) --build-arg ARCH=$($*) && rm -f $(name);)
	$(foreach name,$(IMAGES_LIST),docker push $(REGISTRY)/$(name)-$*:$(TAG);)

test-image-tags:
	$(foreach arch,$(ALL_ARCH),touch $@ && docker build . -t $(REGISTRY)/$@-$(arch):1 --build-arg TEST=$@ --build-arg ARCH=$($(arch)) && rm -f $@;)
	$(foreach arch,$(ALL_ARCH),docker tag $(REGISTRY)/$@-$(arch):1 $(REGISTRY)/$@-$(arch):2;)
	$(foreach arch,$(ALL_ARCH),docker tag $(REGISTRY)/$@-$(arch):1 $(REGISTRY)/$@-$(arch):3;)
	$(foreach arch,$(ALL_ARCH),docker push $(REGISTRY)/$@-$(arch):1;)
	$(foreach arch,$(ALL_ARCH),docker push $(REGISTRY)/$@-$(arch):2;)
	$(foreach arch,$(ALL_ARCH),docker push $(REGISTRY)/$@-$(arch):3;)

test-image-tag:
	$(foreach arch,$(ALL_ARCH),touch $@ && docker build . -t $(REGISTRY)/$@-$(arch):test --build-arg TEST=$@ --build-arg ARCH=$($(arch)) && rm -f $@;)
	$(foreach arch,$(ALL_ARCH),docker push $(REGISTRY)/$@-$(arch):test;)

push-manifest: manifest-tool
	$(foreach image,$(IMAGES_LIST),manifest-tool push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/$(image)-ARCH:$(TAG) --target $(REGISTRY)/$(image):$(TAG);)
	$(foreach arch,$(ALL_ARCH),manifest-tool push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/test-image-tags-ARCH:1 --target $(REGISTRY)/test-image-tags:1;)
	$(foreach arch,$(ALL_ARCH),manifest-tool push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/test-image-tags-ARCH:2 --target $(REGISTRY)/test-image-tags:2;)
	$(foreach arch,$(ALL_ARCH),manifest-tool push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/test-image-tags-ARCH:3 --target $(REGISTRY)/test-image-tags:3;)
	$(foreach arch,$(ALL_ARCH),manifest-tool push from-args --platforms $(call join_platforms,$(ALL_ARCH)) --template $(REGISTRY)/test-image-tag-ARCH:test --target $(REGISTRY)/test-image-tag:test;)
