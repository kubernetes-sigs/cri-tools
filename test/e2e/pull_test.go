/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
)

// The actual test suite.
var _ = t.Describe("pull", func() {
	const (
		imageSuccessText = "Image is up to date"
		registry         = "gcr.io/k8s-staging-cri-tools/"

		image1 = registry + "test-image-1"
		image2 = registry + "test-image-1:latest"
		image3 = registry + "test-image-digest@sha256:9700f9a2f5bf2c45f2f605a0bd3bce7cf37420ec9d3ed50ac2758413308766bf"
	)

	AfterEach(func() {
		// Remove images by tag/digest first
		t.Crictl("rmi " + strings.Join([]string{image1, image2, image3}, " "))

		// Remove any remaining digest references for test-image-1
		// Note: image1 and image2 both refer to test-image-1
		res := t.Crictl("images --filter reference=" + image1 + " -q")
		contents := res.Out.Contents()
		if len(contents) > 0 {
			output := strings.Split(string(contents), "\n")
			t.Crictl("rmi " + strings.TrimSpace(strings.Join(output, " ")))
		}
	})

	It("should succeed without tag or digest", func() {
		t.CrictlExpectSuccess("pull "+image1, imageSuccessText)
	})

	It("should succeed with tag", func() {
		t.CrictlExpectSuccess("pull "+image2, imageSuccessText)
	})

	It("should succeed with digest", func() {
		t.CrictlExpectSuccess("pull "+image3, imageSuccessText)
	})

	It("should succeed to show the help", func() {
		t.CrictlExpectSuccess("pull -h", "Pull an image")
	})

	It("should succeed to generate the JSON schema", func() {
		t.CrictlExpectSuccess("pull jsonschema", "PodSandboxConfig")
	})

	It("should fail on not existing image", func() {
		t.CrictlExpectFailure("pull localhost/wrong", "", "pulling image")
	})
})
