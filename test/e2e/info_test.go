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
)

// The actual test suite.
var _ = t.Describe("info", func() {
	It("should succeed", func() {
		t.CrictlExpectSuccess("info", "NetworkReady")
	})

	It("should fail with additional argument", func() {
		t.CrictlExpectFailure("--invalid", "crictl - client for CRI", "flag provided but not defined")
	})
})
