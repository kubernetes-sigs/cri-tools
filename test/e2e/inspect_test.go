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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

// The actual test suite.
var _ = t.Describe("inspect", func() {
	const imageLatest = registry + "test-image-latest"
	sandbox := ""

	BeforeEach(func() {
		sb, err := os.CreateTemp("", "sandbox-")
		Expect(err).NotTo(HaveOccurred())
		_, err = fmt.Fprintf(sb, `{ "metadata": { "name": "sb", "uid": "uid", "namespace": "ns" }}`)
		Expect(err).NotTo(HaveOccurred())

		ctr, err := os.CreateTemp("", "container-")
		Expect(err).NotTo(HaveOccurred())
		_, err = fmt.Fprintf(ctr, `{ "metadata": { "name": "ctr" }, "image": { "image": "`+imageLatest+`" }, "args": [] }`)
		Expect(err).NotTo(HaveOccurred())

		res := t.Crictl(fmt.Sprintf("run %s %s", ctr.Name(), sb.Name()))
		Expect(res).To(Exit(0))
		Expect(os.RemoveAll(sb.Name())).NotTo(HaveOccurred())
		Expect(os.RemoveAll(ctr.Name())).NotTo(HaveOccurred())

		res = t.Crictl("pods --name sb -q")
		Expect(res).To(Exit(0))
		sandbox = string(bytes.TrimSpace(res.Out.Contents()))
	})

	AfterEach(func() {
		Expect(t.Crictl("rmp -f " + sandbox)).To(Exit(0))

		// Remove image by tag first
		t.Crictl("rmi " + imageLatest)

		// Remove any remaining digest references for this image
		res := t.Crictl("images --filter reference=" + imageLatest + " -q")
		contents := res.Out.Contents()
		if len(contents) > 0 {
			output := strings.Split(string(contents), "\n")
			t.Crictl("rmi " + strings.TrimSpace(strings.Join(output, " ")))
		}

		t.CrictlRemovePauseImages()
	})

	validateSingleResponse := func(contents []byte) {
		singleResponse := map[string]any{}
		Expect(json.Unmarshal(contents, &singleResponse)).NotTo(HaveOccurred())
		Expect(singleResponse).To(HaveKey("info"))
		Expect(singleResponse).To(HaveKey("status"))
	}

	expectNothingFound := func(contents []byte) {
		Expect(string(contents)).To(ContainSubstring("nothing found per filter"))
	}

	It("should succeed", func() {
		res := t.Crictl("inspect -a")
		Expect(res).To(Exit(0))
		validateSingleResponse(res.Out.Contents())

		// Not output without `--all` since container is exited
		res = t.Crictl("inspect")
		Expect(res).To(Exit(0))
		expectNothingFound(res.Err.Contents())

		// Should allow filter per image name
		res = t.Crictl("inspect -a --image " + imageLatest)
		Expect(res).To(Exit(0))
		validateSingleResponse(res.Out.Contents())

		// Should filter nothing if image does not match
		res = t.Crictl("inspect -a --image wrong")
		Expect(res).To(Exit(0))
		expectNothingFound(res.Err.Contents())
	})
})
