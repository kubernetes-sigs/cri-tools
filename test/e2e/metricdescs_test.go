/*
Copyright 2025 The Kubernetes Authors.

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
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// The actual test suite.
var _ = t.Describe("metricdescs", func() {
	It("should list metric descriptors", func() {
		if t.IsContainerd() {
			Skip("ListMetricDescriptors is not supported by containerd")
		}
		// Run the command with JSON output format
		res := t.Crictl("metricdescs")
		Expect(res).To(Exit(0))
		contents := res.Out.Contents()

		// Verify JSON output is valid
		var response map[string]any
		Expect(json.Unmarshal(contents, &response)).NotTo(HaveOccurred())

		// Verify response has expected structure
		Expect(response).To(HaveKey("descriptors"))

		// Validate descriptors are an array (even if empty)
		_, ok := response["descriptors"].([]any)
		Expect(ok).To(BeTrue())
	})
})
