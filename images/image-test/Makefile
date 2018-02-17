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

.PHONY: all test-image-list test-image-tags

all: test-image-list test-image-tags

IMAGES_LIST = test-image-1 test-image-2 test-image-3 test-image-latest test-image-digest test-image-tag:test
test-image-list:
	$(foreach name,$(IMAGES_LIST),docker build . -t gcr.io/cri-tools/$(name) --build-arg TEST=$(name);)
	$(foreach name,$(IMAGES_LIST),gcloud docker -- push gcr.io/cri-tools/$(name);)

test-image-tags:
	docker build . -t gcr.io/cri-tools/$@:1 --build-arg TEST=$@
	docker tag gcr.io/cri-tools/$@:1 gcr.io/cri-tools/$@:2
	docker tag gcr.io/cri-tools/$@:1 gcr.io/cri-tools/$@:3
	gcloud docker -- push gcr.io/cri-tools/$@:1
	gcloud docker -- push gcr.io/cri-tools/$@:2
	gcloud docker -- push gcr.io/cri-tools/$@:3
