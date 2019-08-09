# Copyright 2019 The Kubernetes Authors.
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

$Registry = "gcr.io/cri-tools"
$Tag = "latest"
$ImagesList = @(
	"win-test-image-1:$Tag", "win-test-image-2:$Tag", "win-test-image-3:$Tag",
	"win-test-image-latest:$Tag", "win-test-image-digest:$Tag",
	"win-test-image-tags:1", "win-test-image-tags:2", "win-test-image-tags:3",
	"win-test-image-tag:test", "win-test-image-tag:all")

Foreach ($image in $ImagesList) {
	$imageName = $image.Substring(0, $image.IndexOf(":"))
	New-Item -ItemType File -Path . -Name $imageName
	docker build . -t "$Registry/${image}" --build-arg TEST=$imageName
	docker push "$Registry/${image}"
	Remove-Item -Force $imageName
}
