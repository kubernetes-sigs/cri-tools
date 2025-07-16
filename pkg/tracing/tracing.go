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

package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// Init initializes OpenTelemetry tracing.
func Init(ctx context.Context, collectorAddress string, samplingRate int) (*trace.TracerProvider, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	const serviceName = "cri-tools"

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.HostNameKey.String(hostname),
		semconv.ProcessPIDKey.Int64(int64(os.Getpid())),
	)

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(collectorAddress),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create tracing exporter: %w", err)
	}

	// Only emit spans when the kubelet sends a request with a sampled trace.
	sampler := trace.NeverSample()

	// Or, emit spans for a fraction of transactions.
	if samplingRate > 0 {
		sampler = trace.TraceIDRatioBased(float64(samplingRate) / float64(1000000))
	} else if samplingRate < 0 {
		sampler = trace.AlwaysSample()
	}

	// Batch span processor to aggregate spans before export.
	bsp := trace.NewBatchSpanProcessor(exporter)

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.ParentBased(sampler)),
		trace.WithSpanProcessor(bsp),
		trace.WithResource(res),
	)

	tmp := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(tmp)

	return tp, nil
}
