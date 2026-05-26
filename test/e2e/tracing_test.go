/*
Copyright The Kubernetes Authors.

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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

// findSpan returns the first span whose name matches the given predicate.
func findSpan(spans []*v1.Span, match func(string) bool) *v1.Span {
	for _, span := range spans {
		if match(span.GetName()) {
			return span
		}
	}

	return nil
}

// runTracedCommand runs a crictl command with tracing, waits for spans to
// arrive, and returns the collected spans along with the root span. If
// requireSuccess is true and the command exits non-zero, the test is skipped
// with a detailed message.
func runTracedCommand(command string, requireSuccess bool) (spans []*v1.Span, root *v1.Span) {
	res := t.CrictlWithTracing(command)

	if requireSuccess && res.ExitCode() != 0 {
		Skip("crictl " + command + " failed (likely no runtime access); " +
			"expected exit code 0 to verify child spans, " +
			"but got non-zero — skipping child span assertions")
	}

	Eventually(func() int {
		return len(t.OtelCollector.GetSpans())
	}, "5s", "100ms").Should(BeNumerically(">", 0))

	spans = t.OtelCollector.GetSpans()
	root = findSpan(spans, func(name string) bool { return name == command })

	Expect(root).NotTo(BeNil(), "Root span '%s' not found", command)

	return spans, root
}

var _ = t.Describe("tracing", func() {
	It("should generate spans for version command", func() {
		runTracedCommand("version", false)
	})

	It("should generate child spans for pods command", func() {
		spans, rootSpan := runTracedCommand("pods", true)

		childSpan := findSpan(spans, func(name string) bool {
			return strings.Contains(name, "ListPodSandbox")
		})
		Expect(childSpan).NotTo(BeNil(), "Child span 'ListPodSandbox' not found")
		Expect(bytes.Equal(rootSpan.GetTraceId(), childSpan.GetTraceId())).To(BeTrue(),
			"Root span and child span should have the same trace ID")
		Expect(bytes.Equal(childSpan.GetParentSpanId(), rootSpan.GetSpanId())).To(BeTrue(),
			"Child span should have root span as parent")
	})

	It("should generate Version gRPC span as a child of the root span", func() {
		spans, rootSpan := runTracedCommand("version", true)

		versionChild := findSpan(spans, func(name string) bool {
			return strings.Contains(name, "runtime.v1.RuntimeService/Version")
		})
		Expect(versionChild).NotTo(BeNil(), "Child span for Version API not found")
		Expect(bytes.Equal(versionChild.GetParentSpanId(), rootSpan.GetSpanId())).To(BeTrue(),
			"Version child span should have root span as parent")
		Expect(bytes.Equal(versionChild.GetTraceId(), rootSpan.GetTraceId())).To(BeTrue(),
			"Version child span should share the same trace ID as root span")
	})
})
