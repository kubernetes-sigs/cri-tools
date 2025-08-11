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

package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var runtimeConfigCommand = &cli.Command{
	Name:                   "runtime-config",
	Usage:                  "Retrieve the container runtime configuration",
	UseShortOptionHandling: true,
	Action: func(c *cli.Context) error {
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return fmt.Errorf("get runtime client: %w", err)
		}

		if err := runtimeConfig(c.Context, runtimeClient); err != nil {
			return fmt.Errorf("get runtime config: %w", err)
		}

		return nil
	},
}

// Attach sends an AttachRequest to server, and parses the returned AttachResponse.
func runtimeConfig(ctx context.Context, client internalapi.RuntimeService) error {
	resp, err := InterruptableRPC(ctx, func(ctx context.Context) (*pb.RuntimeConfigResponse, error) {
		return client.RuntimeConfig(ctx)
	})
	if err != nil {
		return fmt.Errorf("call RuntimeConfig RPC: %w", err)
	}

	display := newTableDisplay(10, 1, 3, ' ', 0)
	if resp.GetLinux() != nil {
		display.AddRow([]string{"cgroup driver:", resp.GetLinux().GetCgroupDriver().String()})
	}

	if err := display.Flush(); err != nil {
		return fmt.Errorf("flush display: %w", err)
	}

	return nil
}
