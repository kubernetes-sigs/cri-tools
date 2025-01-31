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
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// The actual test suite.
var _ = t.Describe("inspectp", func() {
	const sandboxesLength = 2
	sandboxes := []string{}

	BeforeEach(func() {
		sandboxes = []string{}

		for i := range sandboxesLength {
			f, err := os.CreateTemp("", "sandbox-")
			Expect(err).NotTo(HaveOccurred())
			_, err = fmt.Fprintf(f, `{ "metadata": { "name": "sb-%d", "uid": "uid-%d", "namespace": "ns" }}`, i, i)
			Expect(err).NotTo(HaveOccurred())

			res := t.Crictl("runp " + f.Name())
			Expect(res).To(Exit(0))
			sandboxes = append(sandboxes, string(bytes.TrimSpace(res.Out.Contents())))

			Expect(os.RemoveAll(f.Name())).NotTo(HaveOccurred())
		}
	})

	AfterEach(func() {
		for _, sb := range sandboxes {
			Expect(os.RemoveAll(sb)).NotTo(HaveOccurred())
			res := t.Crictl("rmp -f " + sb)
			Expect(res).To(Exit(0))
		}
		t.CrictlRemovePauseImages()
	})

	It("should succeed", func() {
		// Single entry
		res := t.Crictl("inspectp " + sandboxes[0])
		Expect(res).To(Exit(0))
		contents := res.Out.Contents()

		// Should be no slice
		singleResponse := map[string]any{}
		Expect(json.Unmarshal(contents, &singleResponse)).NotTo(HaveOccurred())
		Expect(singleResponse).To(HaveKey("info"))
		Expect(singleResponse).To(HaveKey("status"))

		// Multiple entries
		res = t.Crictl(fmt.Sprintf("inspectp %s %s", sandboxes[0], sandboxes[1]))
		Expect(res).To(Exit(0))
		contents = res.Out.Contents()

		// Should be a slice
		multiResponse := []map[string]any{}
		Expect(json.Unmarshal(contents, &multiResponse)).NotTo(HaveOccurred())
		Expect(multiResponse).To(HaveLen(sandboxesLength))
		for i := range sandboxesLength {
			Expect(multiResponse[i]).To(HaveKey("info"))
			Expect(multiResponse[i]).To(HaveKey("status"))
		}
	})
})
