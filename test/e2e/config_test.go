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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = t.Describe("config", func() {
	var configFile *os.File

	BeforeEach(func() {
		var err error
		configFile, err = os.CreateTemp("", "crictl-*.yaml")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(configFile.Name())).NotTo(HaveOccurred())
	})

	listConfig := func() string {
		res := t.Crictl("--config " + configFile.Name() + " config --list")
		Expect(res).To(Exit(0))

		return string(res.Out.Contents())
	}

	It("should succeed to list with empty config", func() {
		cfg := listConfig()
		Expect(cfg).To(ContainSubstring("runtime-endpoint"))
		Expect(cfg).To(ContainSubstring("image-endpoint"))
		Expect(cfg).To(MatchRegexp("timeout .* 0"))
		Expect(cfg).To(MatchRegexp("debug .* false"))
		Expect(cfg).To(MatchRegexp("pull-image-on-create .* false"))
		Expect(cfg).To(MatchRegexp("disable-pull-on-run .* false"))
	})

	It("should succeed to set config values", func() {
		t.CrictlExpectSuccess("--config "+configFile.Name()+" config --set runtime-endpoint=foo,timeout=10", "")

		cfg := listConfig()
		Expect(cfg).To(MatchRegexp("runtime-endpoint .* foo"))
		Expect(cfg).To(MatchRegexp("timeout .* 10"))
	})

	It("should succeed to set config values and preserve comments", func() {
		_, err := configFile.WriteString(`
runtime-endpoint: "foo"
image-endpoint: "bar" # an inline comment
timeout: 5

# comment below a newline
debug: true
`)
		Expect(err).NotTo(HaveOccurred())

		t.CrictlExpectSuccess("--config "+configFile.Name()+" config --set runtime-endpoint=bar,image-endpoint=baz,timeout=10,debug=false", "")

		cfgContent, err := os.ReadFile(configFile.Name())
		Expect(err).NotTo(HaveOccurred())

		Expect(string(cfgContent)).To(Equal(
			`runtime-endpoint: "bar"
image-endpoint: "baz" # an inline comment
timeout: 10
# comment below a newline
debug: false
pull-image-on-create: false
disable-pull-on-run: false
`))
	})

	It("should succeed to get the right value if duplicate entries are defined", func() {
		_, err := configFile.WriteString(`
timeout: 20
timeout: 5
timeout: 10
`)
		Expect(err).NotTo(HaveOccurred())

		t.CrictlExpectSuccess("--config "+configFile.Name()+" config --get timeout", "10")
	})

	It("should succeed to set duplicate entries", func() {
		_, err := configFile.WriteString(`
timeout: 20
timeout: 5
timeout: 10
`)
		Expect(err).NotTo(HaveOccurred())

		t.CrictlExpectSuccess("--config "+configFile.Name()+" config --set timeout=30", "")

		cfgContent, err := os.ReadFile(configFile.Name())
		Expect(err).NotTo(HaveOccurred())

		Expect(string(cfgContent)).To(Equal(
			`timeout: 30
timeout: 30
timeout: 30
runtime-endpoint: ""
image-endpoint: ""
debug: false
pull-image-on-create: false
disable-pull-on-run: false
`))

		t.CrictlExpectSuccess("--config "+configFile.Name()+" config --get timeout", "30")
	})
})
