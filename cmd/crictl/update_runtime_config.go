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

package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	podCIDRFlag = "pod-cidr"
)

var updateRuntimeConfigCommand = &cli.Command{
	Name:                   "update-runtime-config",
	Usage:                  "Update the runtime configuration",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    podCIDRFlag,
			Aliases: []string{"p"},
			Usage:   "The new Classless Inter-Domain Routing (CIDR) value to be used for pod IP addresses. If the CIDR is empty, runtimes should omit it.",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() != 0 || c.NumFlags() == 0 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeConfig := &pb.RuntimeConfig{}
		if c.IsSet(podCIDRFlag) {
			runtimeConfig.NetworkConfig = &pb.NetworkConfig{PodCidr: c.String(podCIDRFlag)}
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		if _, err := InterruptableRPC(c.Context, func(ctx context.Context) (any, error) {
			return nil, runtimeClient.UpdateRuntimeConfig(ctx, runtimeConfig)
		}); err != nil {
			return fmt.Errorf("update runtime config: %w", err)
		}

		logrus.Info("Runtime config successfully updated")

		return nil
	},
}
