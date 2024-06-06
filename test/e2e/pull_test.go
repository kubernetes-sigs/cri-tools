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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// The actual test suite
var _ = t.Describe("pull", func() {
	const (
		imageSuccessText = "Image is up to date"
		registry         = "gcr.io/k8s-staging-cri-tools/"
	)

	AfterEach(func() {
		Expect(t.Crictl("rmi -a")).To(Exit(0))
	})

	It("should succeed without tag or digest", func() {
		t.CrictlExpectSuccess(
			"pull "+registry+"test-image-1",
			imageSuccessText)
	})

	It("should succeed with tag", func() {
		t.CrictlExpectSuccess(
			"pull "+registry+"test-image-1:latest",
			imageSuccessText)
	})

	It("should succeed with digest", func() {
		t.CrictlExpectSuccess(
			"pull "+registry+"test-image-digest"+
				"@sha256:9700f9a2f5bf2c45f2f605a0bd3bce7cf37420ec9d3ed50ac2758413308766bf",
			imageSuccessText)
	})

	It("should succeed to show the help", func() {
		t.CrictlExpectSuccess("pull -h", "Pull an image")
	})

	It("should fail on not existing image", func() {
		t.CrictlExpectFailure("pull localhost/wrong", "", "pulling image")
	})
})
