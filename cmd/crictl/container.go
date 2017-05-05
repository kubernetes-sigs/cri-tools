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

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

var runtimeContainerCommand = cli.Command{
	Name:    "container",
	Usage:   "manage containers",
	Aliases: []string{"ctr"},
	Subcommands: []cli.Command{
		createContainerCommand,
		startContainerCommand,
		stopContainerCommand,
		removeContainerCommand,
		containerStatusCommand,
		listContainersCommand,
	},
	After: closeConnection,
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
	Name:      "create",
	Usage:     "create a container",
	ArgsUsage: "sandboxID container-config.json sandbox-config.json",
	Flags:     []cli.Flag{},
	Action: func(context *cli.Context) error {
		if len(context.Args()) != 3 {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		opts := createOptions{
			podID:      context.Args().Get(0),
			configPath: context.Args().Get(1),
			podConfig:  context.Args().Get(2),
		}

		err := CreateContainer(runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("Creating container failed: %v", err)
		}
		return nil
	},
}

var startContainerCommand = cli.Command{
	Name:      "start",
	Usage:     "start a container",
	ArgsUsage: "containerID",
	Action: func(context *cli.Context) error {
		containerID := context.Args().First()
		if containerID == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		err := StartContainer(runtimeClient, containerID)
		if err != nil {
			return fmt.Errorf("Starting the container failed: %v", err)
		}
		return nil
	},
}

var stopContainerCommand = cli.Command{
	Name:      "stop",
	Usage:     "stop the container",
	ArgsUsage: "containerID",
	Action: func(context *cli.Context) error {
		containerID := context.Args().First()
		if containerID == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		err := StopContainer(runtimeClient, containerID)
		if err != nil {
			return fmt.Errorf("Stopping the container failed: %v", err)
		}
		return nil
	},
}

var removeContainerCommand = cli.Command{
	Name:      "rm",
	Usage:     "remove the container",
	ArgsUsage: "containerID",
	Action: func(context *cli.Context) error {
		containerID := context.Args().First()
		if containerID == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		err := RemoveContainer(runtimeClient, containerID)
		if err != nil {
			return fmt.Errorf("Removing the container failed: %v", err)
		}
		return nil
	},
}

var containerStatusCommand = cli.Command{
	Name:      "status",
	Usage:     "get status of the container",
	ArgsUsage: "containerID",
	Action: func(context *cli.Context) error {
		containerID := context.Args().First()
		if containerID == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		err := ContainerStatus(runtimeClient, containerID)
		if err != nil {
			return fmt.Errorf("Getting the status of the container failed: %v", err)
		}
		return nil
	},
}

var listContainersCommand = cli.Command{
	Name:  "ls",
	Usage: "list containers",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "show verbos information for containers",
		},
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "filter by container id",
		},
		cli.StringFlag{
			Name:  "sandbox",
			Value: "",
			Usage: "filter by sandbox id",
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
		if err := getRuntimeClient(context); err != nil {
			return err
		}

		opts := listOptions{
			id:      context.String("id"),
			podID:   context.String("sandbox"),
			state:   context.String("state"),
			verbose: context.Bool("verbose"),
			labels:  make(map[string]string),
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

	request := &pb.CreateContainerRequest{
		PodSandboxId:  opts.podID,
		Config:        config,
		SandboxConfig: podConfig,
	}
	logrus.Debugf("CreateContainerRequest: %v", request)
	r, err := client.CreateContainer(context.Background(), request)
	logrus.Debugf("CreateContainerResponse: %v", r)
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
	request := &pb.StartContainerRequest{
		ContainerId: ID,
	}
	logrus.Debugf("StartContainerRequest: %v", request)
	r, err := client.StartContainer(context.Background(), request)
	logrus.Debugf("StartContainerResponse: %v", r)
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
	request := &pb.StopContainerRequest{
		ContainerId: ID,
	}
	logrus.Debugf("StopContainerRequest: %v", request)
	r, err := client.StopContainer(context.Background(), request)
	logrus.Debugf("StopContainerResponse: %v", r)
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
	request := &pb.RemoveContainerRequest{
		ContainerId: ID,
	}
	logrus.Debugf("RemoveContainerRequest: %v", request)
	r, err := client.RemoveContainer(context.Background(), request)
	logrus.Debugf("RemoveContainerResponse: %v", r)
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
	request := &pb.ContainerStatusRequest{
		ContainerId: ID,
	}
	logrus.Debugf("ContainerStatusRequest: %v", request)
	r, err := client.ContainerStatus(context.Background(), request)
	logrus.Debugf("ContainerStatusResponse: %v", r)
	if err != nil {
		return err
	}
	fmt.Printf("ID: %s\n", r.Status.Id)
	if r.Status.Metadata != nil {
		if r.Status.Metadata.Name != "" {
			fmt.Printf("Name: %s\n", r.Status.Metadata.Name)
		}
		if r.Status.Metadata.Attempt != 0 {
			fmt.Printf("Attempt: %v\n", r.Status.Metadata.Attempt)
		}
	}
	fmt.Printf("State: %s\n", r.Status.State)
	ctm := time.Unix(0, r.Status.CreatedAt)
	fmt.Printf("Created: %v\n", ctm)
	if r.Status.State != pb.ContainerState_CONTAINER_CREATED {
		stm := time.Unix(0, r.Status.StartedAt)
		fmt.Printf("Started: %v\n", stm)
	}
	if r.Status.FinishedAt != 0 {
		ftm := time.Unix(0, r.Status.FinishedAt)
		fmt.Printf("Finished: %v\n", ftm)
	}
	if r.Status.State == pb.ContainerState_CONTAINER_EXITED {
		fmt.Printf("Exit Code: %v\n", r.Status.ExitCode)
	}

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
	request := &pb.ListContainersRequest{
		Filter: filter,
	}
	logrus.Debugf("ListContainerRequest: %v", request)
	r, err := client.ListContainers(context.Background(), &pb.ListContainersRequest{
		Filter: filter,
	})
	logrus.Debugf("ListContainerResponse: %v", r)
	if err != nil {
		return err
	}
	printHeader := true
	for _, c := range r.GetContainers() {
		ctm := time.Unix(0, c.CreatedAt)
		if !opts.verbose {
			if printHeader {
				printHeader = false
				fmt.Println("CONTAINER ID\tCREATED\tSTATE\tNAME")
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", c.Id, ctm, c.State, c.GetMetadata().GetName())
			continue
		}
		fmt.Printf("ID: %s\n", c.Id)
		fmt.Printf("SandboxID: %s\n", c.PodSandboxId)
		if c.Metadata != nil {
			if c.Metadata.Name != "" {
				fmt.Printf("Name: %s\n", c.Metadata.Name)
			}
			fmt.Printf("Attempt: %v\n", c.Metadata.Attempt)
		}
		fmt.Printf("State: %s\n", c.State)
		if c.Image != nil {
			fmt.Printf("Image: %s\n", c.Image.Image)
		}
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
