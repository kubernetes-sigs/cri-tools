/*
Copyright 2023 The Kubernetes Authors.

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
	. "github.com/onsi/gomega/gbytes"
)

// The actual test suite
var _ = t.Describe("events", func() {
	It("should fail with not supported output format", func() {
		t.CrictlExpectFailure("events --output=ini", "", "don't support .* format")
	})

	It("should fail with template set for non go-template format", func() {
		t.CrictlExpectFailure("events --template=\"{{ .containerID }}\"", "", "template can't be used with .* format")
	})

	It("should succeed", func() {
		// Given
		endpoint, testDir, crio := t.StartCrio()

		// When
		session := t.CrictlWithEndpointNoWait(endpoint, "events")
		Expect(session.Out).ToNot(Say("unknown method GetContainerEvents")) // no errors

		// Then
		session.Terminate()
		t.StopCrio(testDir, crio)
	})
})
