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
)

// The actual test suite.
var _ = t.Describe("help", func() {
	const helpMessageIdentifier = "crictl - client for CRI"

	It("should succeed with `help` subcommand", func() {
		t.CrictlExpectSuccess("help", helpMessageIdentifier)
	})

	It("should succeed with `--help` flag", func() {
		t.CrictlExpectSuccess("--help", helpMessageIdentifier)
	})

	It("should succeed with `-h` flag", func() {
		t.CrictlExpectSuccess("-h", helpMessageIdentifier)
	})

	It("should show help on invalid flag", func() {
		t.CrictlExpectFailure("--invalid", helpMessageIdentifier,
			"flag provided but not defined")
	})
})

// The actual test suite.
var _ = t.Describe("help subcommand", func() {
	It("should show help running rm without params", func() {
		t.CrictlExpectSuccess("rm", "crictl rm")
	})

	It("should show help running rmi without params", func() {
		t.CrictlExpectSuccess("rmi", "crictl rmi")
	})

	It("should show help running rmp without params", func() {
		t.CrictlExpectSuccess("rmp", "crictl rmp")
	})

	// The runtime may still hold objects (e.g. images cached by a previous
	// critest run), so stdout must be either empty (nothing to remove) or
	// consist only of lines reporting removed objects. The subcommand help
	// is also printed to stdout, but it fails both checks: it is not empty and
	// its lines ("NAME:", "USAGE:", ...) do not match the removal line format.
	noErrors := Not(ContainSubstring("level=error"))

	It("should not show help running rm -a", func() {
		t.CrictlExpectMatch("rm -a", 0,
			Or(BeEmpty(), MatchRegexp(`^(\S+\n)+$`)), noErrors)
	})

	It("should not show help running rmi -a", func() {
		t.CrictlExpectMatch("rmi -a", 0,
			Or(BeEmpty(), MatchRegexp(`^(Deleted: \S+\n)+$`)), noErrors)
	})

	It("should not show help running rmp -a", func() {
		t.CrictlExpectMatch("rmp -a", 0,
			Or(BeEmpty(), MatchRegexp(`^(Removed sandbox \S+\n)+$`)), noErrors)
	})
})
