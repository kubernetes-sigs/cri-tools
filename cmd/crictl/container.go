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

package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

var runtimeContainerCommand = cli.Command{
	Name:    "container",
	Usage:   "Manage container",
	Aliases: []string{"ctr"},
	Subcommands: []cli.Command{
		createContainerCommand,
		startContainerCommand,
		stopContainerCommand,
		removeContainerCommand,
		containerStatusCommand,
		listContainersCommand,
	},
	Before: getRuntimeClient,
	After:  closeConnection,
}

type createOptions struct {
	// configPath is path to the config for container
	configPath string
	// podID of the container
	podID string
	// podConfig is path to the config for sandbox
	podConfig string
}

var createContainerCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "pod",
			Usage: "the id of the pod sandbox to which the container belongs",
		},
		cli.StringFlag{
			Name:  "config",
			Value: "config.json",
			Usage: "the path of a container config file",
		},
		cli.StringFlag{
			Name:  "podconfig",
			Usage: "the path of a sandbox config file",
		},
	},
	Action: func(context *cli.Context) error {
		if !context.IsSet("pod") {
			return fmt.Errorf("Please specify the id of the pod sandbox to which the container belongs via the --pod option")
		}

		opts := createOptions{
			configPath: context.String("config"),
			podID:      context.String("pod"),
			podConfig:  context.String("podconfig"),
		}

		err := CreateContainer(runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("Creating container failed: %v", err)
		}
		return nil
	},
}

var startContainerCommand = cli.Command{
	Name:  "start",
	Usage: "start a container",
	Action: func(context *cli.Context) error {
		containerID := context.Args().First()
		err := StartContainer(runtimeClient, containerID)
		if err != nil {
			return fmt.Errorf("Starting the container failed: %v", err)
		}
		return nil
	},
}

var stopContainerCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a container",
	Action: func(context *cli.Context) error {
		containerID := context.Args().First()
		err := StopContainer(runtimeClient, containerID)
		if err != nil {
			return fmt.Errorf("Stopping the container failed: %v", err)
		}
		return nil
	},
}

var removeContainerCommand = cli.Command{
	Name:  "remove",
	Usage: "remove a container",
	Action: func(context *cli.Context) error {
		containerID := context.Args().First()
		err := RemoveContainer(runtimeClient, containerID)
		if err != nil {
			return fmt.Errorf("Removing the container failed: %v", err)
		}
		return nil
	},
}

var containerStatusCommand = cli.Command{
	Name:  "status",
	Usage: "get the status of a container",
	Action: func(context *cli.Context) error {
		containerID := context.Args().First()
		err := ContainerStatus(runtimeClient, containerID)
		if err != nil {
			return fmt.Errorf("Getting the status of the container failed: %v", err)
		}
		return nil
	},
}

var listContainersCommand = cli.Command{
	Name:  "list",
	Usage: "list containers",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet",
			Usage: "list only container IDs",
		},
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "filter by container id",
		},
		cli.StringFlag{
			Name:  "pod",
			Value: "",
			Usage: "filter by container pod id",
		},
		cli.StringFlag{
			Name:  "state",
			Value: "",
			Usage: "filter by container state",
		},
		cli.StringSliceFlag{
			Name:  "label",
			Usage: "filter by key=value label",
		},
	},
	Action: func(context *cli.Context) error {
		opts := listOptions{
			id:     context.String("id"),
			podID:  context.String("pod"),
			state:  context.String("state"),
			quiet:  context.Bool("quiet"),
			labels: make(map[string]string),
		}

		for _, l := range context.StringSlice("label") {
			pair := strings.Split(l, "=")
			if len(pair) != 2 {
				return fmt.Errorf("incorrectly specified label: %v", l)
			}
			opts.labels[pair[0]] = pair[1]
		}

		err := ListContainers(runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("listing containers failed: %v", err)
		}
		return nil
	},
}

// CreateContainer sends a CreateContainerRequest to the server, and parses
// the returned CreateContainerResponse.
func CreateContainer(client pb.RuntimeServiceClient, opts createOptions) error {
	config, err := loadContainerConfig(opts.configPath)
	if err != nil {
		return err
	}
	var podConfig *pb.PodSandboxConfig
	if opts.podConfig != "" {
		podConfig, err = loadPodSandboxConfig(opts.podConfig)
		if err != nil {
			return err
		}
	}

	r, err := client.CreateContainer(context.Background(), &pb.CreateContainerRequest{
		PodSandboxId:  opts.podID,
		Config:        config,
		SandboxConfig: podConfig,
	})
	if err != nil {
		return err
	}
	fmt.Println(r.ContainerId)
	return nil
}

// StartContainer sends a StartContainerRequest to the server, and parses
// the returned StartContainerResponse.
func StartContainer(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := client.StartContainer(context.Background(), &pb.StartContainerRequest{
		ContainerId: ID,
	})
	if err != nil {
		return err
	}
	fmt.Println(ID)
	return nil
}

// StopContainer sends a StopContainerRequest to the server, and parses
// the returned StopContainerResponse.
func StopContainer(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := client.StopContainer(context.Background(), &pb.StopContainerRequest{
		ContainerId: ID,
	})
	if err != nil {
		return err
	}
	fmt.Println(ID)
	return nil
}

// RemoveContainer sends a RemoveContainerRequest to the server, and parses
// the returned RemoveContainerResponse.
func RemoveContainer(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := client.RemoveContainer(context.Background(), &pb.RemoveContainerRequest{
		ContainerId: ID,
	})
	if err != nil {
		return err
	}
	fmt.Println(ID)
	return nil
}

// ContainerStatus sends a ContainerStatusRequest to the server, and parses
// the returned ContainerStatusResponse.
func ContainerStatus(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	r, err := client.ContainerStatus(context.Background(), &pb.ContainerStatusRequest{
		ContainerId: ID})
	if err != nil {
		return err
	}
	fmt.Printf("ID: %s\n", r.Status.Id)
	if r.Status.Metadata != nil {
		if r.Status.Metadata.Name != "" {
			fmt.Printf("Name: %s\n", r.Status.Metadata.Name)
		}
		fmt.Printf("Attempt: %v\n", r.Status.Metadata.Attempt)
	}
	fmt.Printf("Status: %s\n", r.Status.State)
	ctm := time.Unix(0, r.Status.CreatedAt)
	fmt.Printf("Created: %v\n", ctm)
	stm := time.Unix(0, r.Status.StartedAt)
	fmt.Printf("Started: %v\n", stm)
	ftm := time.Unix(0, r.Status.FinishedAt)
	fmt.Printf("Finished: %v\n", ftm)
	fmt.Printf("Exit Code: %v\n", r.Status.ExitCode)

	return nil
}

// ListContainers sends a ListContainerRequest to the server, and parses
// the returned ListContainerResponse.
func ListContainers(client pb.RuntimeServiceClient, opts listOptions) error {
	filter := &pb.ContainerFilter{}
	if opts.id != "" {
		filter.Id = opts.id
	}
	if opts.podID != "" {
		filter.PodSandboxId = opts.podID
	}
	if opts.state != "" {
		st := &pb.ContainerStateValue{}
		st.State = pb.ContainerState_CONTAINER_UNKNOWN
		switch opts.state {
		case "created":
			st.State = pb.ContainerState_CONTAINER_CREATED
			filter.State = st
		case "running":
			st.State = pb.ContainerState_CONTAINER_RUNNING
			filter.State = st
		case "stopped":
			st.State = pb.ContainerState_CONTAINER_EXITED
			filter.State = st
		default:
			log.Fatalf("--state should be one of created, running or stopped")
		}
	}
	if opts.labels != nil {
		filter.LabelSelector = opts.labels
	}
	r, err := client.ListContainers(context.Background(), &pb.ListContainersRequest{
		Filter: filter,
	})
	if err != nil {
		return err
	}
	for _, c := range r.GetContainers() {
		if opts.quiet {
			fmt.Println(c.Id)
			continue
		}
		fmt.Printf("ID: %s\n", c.Id)
		fmt.Printf("Pod: %s\n", c.PodSandboxId)
		if c.Metadata != nil {
			if c.Metadata.Name != "" {
				fmt.Printf("Name: %s\n", c.Metadata.Name)
			}
			fmt.Printf("Attempt: %v\n", c.Metadata.Attempt)
		}
		fmt.Printf("Status: %s\n", c.State)
		if c.Image != nil {
			fmt.Printf("Image: %s\n", c.Image.Image)
		}
		ctm := time.Unix(0, c.CreatedAt)
		fmt.Printf("Created: %v\n", ctm)
		if c.Labels != nil {
			fmt.Println("Labels:")
			for _, k := range getSortedKeys(c.Labels) {
				fmt.Printf("\t%s -> %s\n", k, c.Labels[k])
			}
		}
		if c.Annotations != nil {
			fmt.Println("Annotations:")
			for _, k := range getSortedKeys(c.Annotations) {
				fmt.Printf("\t%s -> %s\n", k, c.Annotations[k])
			}
		}
		fmt.Println()
	}
	return nil
}
