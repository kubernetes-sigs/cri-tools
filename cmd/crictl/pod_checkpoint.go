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
	"io"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"sigs.k8s.io/yaml"
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

		_, err = fmt.Fprintf(c.App.Writer, "Pod %s checkpointed to %s\n", podID, path)

		return err
	},
}

var restorePodCommand = &cli.Command{
	Name:                   "restorep",
	Usage:                  "Prepare a pod sandbox and containers from a checkpoint; start containers separately",
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
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml; includes the restored container IDs",
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

		format := c.String("output")
		switch format {
		case "", outputTypeJSON, outputTypeYAML:
		default:
			return fmt.Errorf("unsupported output format %q", format)
		}

		request, err := loadPodRestoreRequest(c)
		if err != nil {
			return err
		}

		runtimeClient, err := configFromContext(c).GetRuntimeService(c.Context, 0)
		if err != nil {
			return err
		}

		logrus.Debugf(
			"Restoring pod from %q with %d containers using runtime handler %q",
			request.GetCheckpointPath(),
			len(request.GetContainerConfigs()),
			request.GetRuntimeHandler(),
		)

		response, err := InterruptableRPC(c.Context, func(
			ctx context.Context,
		) (*pb.RestorePodResponse, error) {
			return runtimeClient.RestorePod(ctx, request)
		})
		if err != nil {
			return fmt.Errorf("restoring pod from %q failed: %w", request.GetCheckpointPath(), err)
		}

		if err := validatePodRestoreResponse(request, response); err != nil {
			return fmt.Errorf(
				"invalid RestorePod response for sandbox %q: %w",
				response.GetPodSandboxId(),
				err,
			)
		}

		return printPodRestoreResponse(c.App.Writer, response, format)
	},
}

func loadPodRestoreRequest(c *cli.Context) (*pb.RestorePodRequest, error) {
	config, err := readPodSandboxConfig(c.String("pod-config"))
	if err != nil {
		return nil, fmt.Errorf("failed to load pod sandbox config: %w", err)
	}

	if config.GetMetadata().GetUid() == "" {
		return nil, errors.New("metadata.uid is required for restore")
	}

	request := &pb.RestorePodRequest{
		CheckpointPath: c.Args().First(),
		RuntimeHandler: c.String("runtime"),
		Config:         config,
	}
	seenNames := make(map[string]struct{})

	for _, configPath := range c.StringSlice("container-config") {
		config, err := loadContainerConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load container config %q: %w", configPath, err)
		}

		name := config.GetMetadata().GetName()
		if _, exists := seenNames[name]; exists {
			return nil, fmt.Errorf("container name %q is duplicated in restore configs", name)
		}

		seenNames[name] = struct{}{}

		request.ContainerConfigs = append(request.ContainerConfigs, config)
	}

	return request, nil
}

func validatePodRestoreResponse(
	request *pb.RestorePodRequest,
	response *pb.RestorePodResponse,
) error {
	if response.GetPodSandboxId() == "" {
		return errors.New("pod sandbox ID is empty")
	}

	if len(response.GetRestoredContainers()) != len(request.GetContainerConfigs()) {
		return fmt.Errorf("runtime returned %d containers, expected %d",
			len(response.GetRestoredContainers()), len(request.GetContainerConfigs()))
	}

	expectedNames := make(map[string]struct{}, len(request.GetContainerConfigs()))
	for _, config := range request.GetContainerConfigs() {
		expectedNames[config.GetMetadata().GetName()] = struct{}{}
	}

	seenIDs := make(map[string]struct{}, len(response.GetRestoredContainers()))
	for _, container := range response.GetRestoredContainers() {
		name := container.GetName()
		if _, exists := expectedNames[name]; !exists {
			return fmt.Errorf("runtime returned unexpected or duplicate container name %q", name)
		}

		delete(expectedNames, name)

		id := container.GetContainerId()
		if id == "" {
			return fmt.Errorf("runtime returned an empty ID for container %q", name)
		}

		if _, exists := seenIDs[id]; exists {
			return fmt.Errorf("runtime returned duplicate container ID %q", id)
		}

		seenIDs[id] = struct{}{}
	}

	return nil
}

func printPodRestoreResponse(
	writer io.Writer,
	response *pb.RestorePodResponse,
	format string,
) error {
	if format == "" {
		_, err := fmt.Fprintf(writer, "Pod restored: %s\n", response.GetPodSandboxId())

		return err
	}

	output, err := protobufObjectToJSON(response)
	if err != nil {
		return err
	}

	if format == outputTypeYAML {
		data, err := yaml.JSONToYAML([]byte(output))
		if err != nil {
			return err
		}

		output = string(data)
	}

	_, err = fmt.Fprintln(writer, output)

	return err
}
