/*
Copyright 2024 The Kubernetes Authors.

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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// The actual test suite.
var _ = t.Describe("inspecti", func() {
	const (
		imageSuccessText = "Image is up to date"
		image1           = registry + "test-image-2"
		image2           = registry + "test-image-3"
	)

	BeforeEach(func() {
		t.CrictlExpectSuccess("pull "+image1, imageSuccessText)
		t.CrictlExpectSuccess("pull "+image2, imageSuccessText)
	})

	AfterEach(func() {
		// Remove images by tag first
		t.Crictl(fmt.Sprintf("rmi %s %s", image1, image2))

		// Remove any remaining digest references for these images
		res := t.Crictl("images --filter reference=" + image1 + " -q")
		contents := res.Out.Contents()
		if len(contents) > 0 {
			output := strings.Split(string(contents), "\n")
			t.Crictl("rmi " + strings.TrimSpace(strings.Join(output, " ")))
		}
		res = t.Crictl("images --filter reference=" + image2 + " -q")
		contents = res.Out.Contents()
		if len(contents) > 0 {
			output := strings.Split(string(contents), "\n")
			t.Crictl("rmi " + strings.TrimSpace(strings.Join(output, " ")))
		}
	})

	It("should succeed", func() {
		// Single response
		res := t.Crictl("inspecti " + image1)
		Expect(res).To(Exit(0))
		contents := res.Out.Contents()

		// Should be no slice
		singleResponse := map[string]any{}
		Expect(json.Unmarshal(contents, &singleResponse)).NotTo(HaveOccurred())
		Expect(singleResponse).To(HaveKey("info"))
		Expect(singleResponse).To(HaveKey("status"))

		// Multi response
		res = t.Crictl(fmt.Sprintf("inspecti %s %s", image1, image2))
		Expect(res).To(Exit(0))
		contents = res.Out.Contents()

		// Should be a slice
		multiResponse := []map[string]any{}
		Expect(json.Unmarshal(contents, &multiResponse)).NotTo(HaveOccurred())
		const length = 2
		Expect(multiResponse).To(HaveLen(length))
		for i := range length {
			Expect(multiResponse[i]).To(HaveKey("info"))
			Expect(multiResponse[i]).To(HaveKey("status"))
		}
	})
})
