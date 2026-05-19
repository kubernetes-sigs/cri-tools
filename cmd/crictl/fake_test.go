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

package main

import (
	"context"
	"testing"
	"time"

	internalapi "k8s.io/cri-api/pkg/apis"
)

func TestConfigOverride(t *testing.T) {
	t.Parallel()

	cfg := &CrictlConfig{
		Timeout:                2 * time.Second,
		runtimeServiceOverride: fakeRuntimeSvc{},
		imageServiceOverride:   fakeImageSvc{},
	}

	rs, err := cfg.GetRuntimeService(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := rs.(fakeRuntimeSvc); !ok {
		t.Fatal("expected fake runtime service override to be returned")
	}

	is, err := cfg.GetImageService(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := is.(fakeImageSvc); !ok {
		t.Fatal("expected fake image service override to be returned")
	}
}

type fakeRuntimeSvc struct {
	internalapi.RuntimeService
}

type fakeImageSvc struct {
	internalapi.ImageManagerService
}
