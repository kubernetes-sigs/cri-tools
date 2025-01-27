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

package common_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/cri-tools/pkg/common"
)

var _ = DescribeTable("ReadConfig",
	func(content string, expectedConfig *common.Config, shouldFail bool) {
		f, err := os.CreateTemp("", "crictl-read-config-")
		defer os.RemoveAll(f.Name())
		Expect(err).NotTo(HaveOccurred())

		_, err = f.WriteString(content)
		Expect(err).NotTo(HaveOccurred())

		readConfig, err := common.ReadConfig(f.Name())
		if shouldFail {
			Expect(err).To(HaveOccurred())

			return
		} else {
			Expect(err).NotTo(HaveOccurred())
		}

		Expect(readConfig.RuntimeEndpoint).To(Equal(expectedConfig.RuntimeEndpoint))
		Expect(readConfig.ImageEndpoint).To(Equal(expectedConfig.ImageEndpoint))
		Expect(readConfig.Timeout).To(Equal(expectedConfig.Timeout))
		Expect(readConfig.Debug).To(Equal(expectedConfig.Debug))
		Expect(readConfig.PullImageOnCreate).To(Equal(expectedConfig.PullImageOnCreate))
		Expect(readConfig.DisablePullOnRun).To(Equal(expectedConfig.DisablePullOnRun))
	},

	Entry("should succeed with valid config", `
runtime-endpoint: "foo"
image-endpoint: "bar"
timeout: 10
debug: true
pull-image-on-create: true
disable-pull-on-run: true
`, &common.Config{
		RuntimeEndpoint:   "foo",
		ImageEndpoint:     "bar",
		Timeout:           10,
		Debug:             true,
		PullImageOnCreate: true,
		DisablePullOnRun:  true,
	}, false),

	Entry("should succeed with comments", `
# This is a comment
runtime-endpoint: "foo" # Comment
# This is another comment
`, &common.Config{
		RuntimeEndpoint:   "foo",
		ImageEndpoint:     "",
		Timeout:           0,
		Debug:             false,
		PullImageOnCreate: false,
		DisablePullOnRun:  false,
	}, false),

	Entry("should succeed with empty lines between entries", `
runtime-endpoint: "foo"


image-endpoint: "bar"

`, &common.Config{
		RuntimeEndpoint:   "foo",
		ImageEndpoint:     "bar",
		Timeout:           0,
		Debug:             false,
		PullImageOnCreate: false,
		DisablePullOnRun:  false,
	}, false),

	Entry("should succeed with duplicate entries", `
runtime-endpoint: "foo"
runtime-endpoint: "bar"
image-endpoint: "bar"
timeout: 10
timeout: 20

`, &common.Config{
		RuntimeEndpoint:   "bar",
		ImageEndpoint:     "bar",
		Timeout:           20,
		Debug:             false,
		PullImageOnCreate: false,
		DisablePullOnRun:  false,
	}, false),

	Entry("should succeed with an empty file", "", &common.Config{
		RuntimeEndpoint:   "",
		ImageEndpoint:     "",
		Timeout:           0,
		Debug:             false,
		PullImageOnCreate: false,
		DisablePullOnRun:  false,
	}, false),

	Entry("should fail with invalid config option", `runtime-endpoint-wrong: "foo"`, nil, true),
	Entry("should fail with invalid 'timeout' value", `timeout: "foo"`, nil, true),
	Entry("should fail with invalid 'debug' value", `debug: "foo"`, nil, true),
	Entry("should fail with invalid 'pull-image-on-create' value", `pull-image-on-create: "foo"`, nil, true),
	Entry("should fail with invalid 'disable-pull-on-run' value", `disable-pull-on-run: "foo"`, nil, true),
)

var _ = DescribeTable("WriteConfig",
	func(config *common.Config) {
		f, err := os.CreateTemp("", "crictl-write-config-")
		defer os.RemoveAll(f.Name())
		Expect(err).NotTo(HaveOccurred())

		err = common.WriteConfig(config, f.Name())
		Expect(err).NotTo(HaveOccurred())

		readConfig, err := common.ReadConfig(f.Name())
		Expect(err).NotTo(HaveOccurred())

		if config == nil {
			config = &common.Config{}
		}

		Expect(readConfig.RuntimeEndpoint).To(Equal(config.RuntimeEndpoint))
		Expect(readConfig.ImageEndpoint).To(Equal(config.ImageEndpoint))
		Expect(readConfig.Timeout).To(Equal(config.Timeout))
		Expect(readConfig.Debug).To(Equal(config.Debug))
		Expect(readConfig.PullImageOnCreate).To(Equal(config.PullImageOnCreate))
		Expect(readConfig.DisablePullOnRun).To(Equal(config.DisablePullOnRun))
	},

	Entry("should succeed with config", &common.Config{
		RuntimeEndpoint:   "foo",
		ImageEndpoint:     "bar",
		Timeout:           10,
		Debug:             true,
		PullImageOnCreate: true,
		DisablePullOnRun:  true,
	}),

	Entry("should succeed with nil config", nil),
)
