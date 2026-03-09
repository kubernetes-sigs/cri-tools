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
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var checkpointPodCommand = &cli.Command{
	Name:                   "checkpointp",
	Usage:                  "Checkpoint a running pod sandbox",
	ArgsUsage:              "POD-ID",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "path",
			Aliases: []string{"p"},
			Usage:   "Path to the directory where the checkpoint will be stored",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return errors.New("POD-ID cannot be empty")
		}

		if c.NArg() > 1 {
			return errors.New("only one pod sandbox can be checkpointed at a time")
		}

		path := c.String("path")
		if path == "" {
			return errors.New(
				"cannot checkpoint a pod without specifying the checkpoint path. " +
					"Use --path=/path/to/checkpoint/dir",
			)
		}

		runtimeClient, err := configFromContext(c).GetRuntimeService(c.Context, 0)
		if err != nil {
			return err
		}

		podID := c.Args().First()

		containers, err := runtimeClient.ListContainers(c.Context, &pb.ContainerFilter{
			PodSandboxId: podID,
			State: &pb.ContainerStateValue{
				State: pb.ContainerState_CONTAINER_RUNNING,
			},
		})
		if err != nil {
			return fmt.Errorf("listing running containers for pod %q failed: %w", podID, err)
		}

		if len(containers) == 0 {
			return fmt.Errorf("pod %q has no running containers to checkpoint", podID)
		}

		containerIDs := make([]string, 0, len(containers))
		for _, container := range containers {
			containerIDs = append(containerIDs, container.GetId())
		}

		request := &pb.CheckpointPodRequest{
			PodSandboxId: podID,
			OutputPath:   path,
			ContainerIds: containerIDs,
		}
		logrus.Debugf("CheckpointPodRequest: %v", request)

		_, err = InterruptableRPC(c.Context, func(
			ctx context.Context,
		) (*pb.CheckpointPodResponse, error) {
			return nil, runtimeClient.CheckpointPod(ctx, request)
		})
		if err != nil {
			return fmt.Errorf("checkpointing pod %q failed: %w", podID, err)
		}

		fmt.Printf("Pod %s checkpointed to %s\n", podID, path)

		return nil
	},
}

var restorePodCommand = &cli.Command{
	Name:                   "restorep",
	Usage:                  "Restore a pod sandbox from a checkpoint",
	ArgsUsage:              "CHECKPOINT-PATH",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "pod-config",
			Aliases: []string{"c"},
			Usage:   "Path to the PodSandboxConfig JSON or YAML file for the restored pod",
		},
		&cli.StringSliceFlag{
			Name:    "container-config",
			Aliases: []string{"C"},
			Usage:   "Path to a ContainerConfig JSON or YAML file; specify once per restored container",
		},
		&cli.StringFlag{
			Name:    "runtime",
			Aliases: []string{"r"},
			Usage:   "Runtime handler to use for restore",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return errors.New("CHECKPOINT-PATH cannot be empty")
		}

		if c.NArg() > 1 {
			return errors.New("only one checkpoint path can be restored at a time")
		}

		if c.String("pod-config") == "" {
			return errors.New("--pod-config is required")
		}

		if len(c.StringSlice("container-config")) == 0 {
			return errors.New("at least one --container-config is required")
		}

		runtimeClient, err := configFromContext(c).GetRuntimeService(c.Context, 0)
		if err != nil {
			return err
		}

		checkpointPath := c.Args().First()
		request := &pb.RestorePodRequest{
			CheckpointPath: checkpointPath,
			RuntimeHandler: c.String("runtime"),
		}

		request.Config, err = loadPodSandboxConfig(c.String("pod-config"))
		if err != nil {
			return fmt.Errorf("failed to load pod sandbox config: %w", err)
		}

		for _, configPath := range c.StringSlice("container-config") {
			config, err := loadContainerConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load container config %q: %w", configPath, err)
			}

			request.ContainerConfigs = append(request.ContainerConfigs, config)
		}

		logrus.Debugf("RestorePodRequest: %v", request)

		response, err := InterruptableRPC(c.Context, func(
			ctx context.Context,
		) (*pb.RestorePodResponse, error) {
			return runtimeClient.RestorePod(ctx, request)
		})
		if err != nil {
			return fmt.Errorf("restoring pod from %q failed: %w", checkpointPath, err)
		}

		fmt.Printf("Pod restored: %s\n", response.GetPodSandboxId())

		return nil
	},
}
