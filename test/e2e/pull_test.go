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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega/gexec"
)

// The actual test suite
var _ = t.Describe("pull", func() {
	var (
		endpoint, testDir string
		crio              *Session
	)
	BeforeEach(func() {
		endpoint, testDir, crio = t.StartCrio()
	})

	AfterEach(func() {
		t.StopCrio(testDir, crio)
	})

	const imageSuccessText = "Image is up to date"

	It("should succeed without tag or digest", func() {
		t.CrictlExpectSuccessWithEndpoint(endpoint,
			"pull gcr.io/cri-tools/test-image-1",
			imageSuccessText)
	})

	It("should succeed with tag", func() {
		t.CrictlExpectSuccessWithEndpoint(endpoint,
			"pull gcr.io/cri-tools/test-image-1:latest",
			imageSuccessText)
	})

	It("should succeed with digest", func() {
		t.CrictlExpectSuccessWithEndpoint(endpoint,
			"pull gcr.io/cri-tools/test-image-digest"+
				"@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343",
			imageSuccessText)
	})

	It("should succeed to show the help", func() {
		t.CrictlExpectSuccess("pull -h", "Pull an image")
	})

	It("should fail on not existing image", func() {
		t.CrictlExpectFailureWithEndpoint(endpoint, "pull localhost/wrong",
			"", "pulling image failed")
	})
})
