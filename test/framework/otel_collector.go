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

package framework

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
)

// OtelCollector is a simple OTLP gRPC collector for testing.
type OtelCollector struct {
	coltracepb.UnimplementedTraceServiceServer

	mu    sync.Mutex
	spans []*v1.Span
	port  int
	srv   *grpc.Server
}

// NewOtelCollector creates a new OtelCollector.
func NewOtelCollector() *OtelCollector {
	return &OtelCollector{
		spans: []*v1.Span{},
	}
}

// Start starts the collector on a random port.
func (c *OtelCollector) Start() error {
	lc := net.ListenConfig{}

	lis, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	addr, ok := lis.Addr().(*net.TCPAddr)
	if !ok {
		return errors.New("failed to get TCP address")
	}

	c.port = addr.Port

	c.srv = grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(c.srv, c)

	go func() {
		if err := c.srv.Serve(lis); err != nil {
			fmt.Printf("Collector stopped: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the collector.
func (c *OtelCollector) Stop() {
	if c.srv != nil {
		c.srv.Stop()
	}
}

// Export implements the OTLP trace service.
func (c *OtelCollector) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			c.spans = append(c.spans, ss.GetSpans()...)
		}
	}

	return &coltracepb.ExportTraceServiceResponse{}, nil
}

// GetSpans returns the collected spans.
func (c *OtelCollector) GetSpans() []*v1.Span {
	c.mu.Lock()
	defer c.mu.Unlock()

	res := make([]*v1.Span, len(c.spans))
	copy(res, c.spans)

	return res
}

// ClearSpans clears the collected spans.
func (c *OtelCollector) ClearSpans() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.spans = []*v1.Span{}
}

// GetEndpoint returns the collector endpoint.
func (c *OtelCollector) GetEndpoint() string {
	return fmt.Sprintf("127.0.0.1:%d", c.port)
}
