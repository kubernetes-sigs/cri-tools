/*
Copyright 2026 The Kubernetes Authors.

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

var _ = t.Describe("tracing", func() {
	It("should generate spans for version command", func() {
		t.CrictlWithTracing("version")
		// It might fail with permission denied if not run as root,
		// but it should still generate a span if tracing is enabled.

		// Wait a bit for spans to be exported (crictl flushes on exit)
		Eventually(func() int {
			return len(t.OtelCollector.GetSpans())
		}, "5s", "100ms").Should(BeNumerically(">", 0))

		spans := t.OtelCollector.GetSpans()

		foundRoot := false
		for _, span := range spans {
			if span.GetName() == "version" {
				foundRoot = true

				break
			}
		}
		Expect(foundRoot).To(BeTrue(), "Root span 'version' not found")
	})

	It("should generate child spans for pods command", func() {
		// This requires sudo to actually talk to the socket and generate child spans
		res := t.CrictlWithTracing("pods")

		Eventually(func() int {
			return len(t.OtelCollector.GetSpans())
		}, "5s", "100ms").Should(BeNumerically(">", 0))

		spans := t.OtelCollector.GetSpans()

		var rootSpan *v1.Span
		var childSpan *v1.Span
		for _, span := range spans {
			if span.GetName() == "pods" {
				rootSpan = span
			}
			// Look for CRI gRPC child spans
			if strings.Contains(span.GetName(), "ListPodSandbox") {
				childSpan = span
			}
		}
		Expect(rootSpan).NotTo(BeNil(), "Root span 'pods' not found")

		if res.ExitCode() == 0 {
			Expect(childSpan).NotTo(BeNil(), "Child span 'ListPodSandbox' not found")
			Expect(bytes.Equal(rootSpan.GetTraceId(), childSpan.GetTraceId())).To(BeTrue(),
				"Root span and child span should have the same trace ID")
			Expect(bytes.Equal(childSpan.GetParentSpanId(), rootSpan.GetSpanId())).To(BeTrue(),
				"Child span should have root span as parent")
		}
	})

	It("should generate Version gRPC span as a child of the root span", func() {
		res := t.CrictlWithTracing("version")

		Eventually(func() int {
			return len(t.OtelCollector.GetSpans())
		}, "5s", "100ms").Should(BeNumerically(">", 0))

		spans := t.OtelCollector.GetSpans()

		var rootSpan *v1.Span
		for _, span := range spans {
			if span.GetName() == "version" {
				rootSpan = span

				break
			}
		}

		Expect(rootSpan).NotTo(BeNil(), "Root span 'version' not found")

		// If sudo was used and connection succeeded, we should have at least one child span for Version
		if res.ExitCode() == 0 {
			foundVersionChild := false
			for _, span := range spans {
				if strings.Contains(span.GetName(), "runtime.v1.RuntimeService/Version") {
					if bytes.Equal(span.GetParentSpanId(), rootSpan.GetSpanId()) && bytes.Equal(span.GetTraceId(), rootSpan.GetTraceId()) {
						foundVersionChild = true

						break
					}
				}
			}
			Expect(foundVersionChild).To(BeTrue(), "Child span for Version API not found as a child of the root span")
		}
	})
})
