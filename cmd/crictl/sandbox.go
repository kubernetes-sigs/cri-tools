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

var runtimePodSandboxCommand = cli.Command{
	Name:  "sandbox",
	Usage: "Manage sandbox",
	Subcommands: []cli.Command{
		runPodSandboxCommand,
		stopPodSandboxCommand,
		removePodSandboxCommand,
		podSandboxStatusCommand,
		listPodSandboxCommand,
	},
	Before: getRuntimeClient,
	After:  closeConnection,
}

var runPodSandboxCommand = cli.Command{
	Name:  "run",
	Usage: "run a pod",
	Action: func(context *cli.Context) error {
		sandboxSpec := context.Args().First()
		podSandboxConfig, err := loadPodSandboxConfig(sandboxSpec)
		if err != nil {
			return fmt.Errorf("load podSandboxConfig failed: %v", err)
		}

		// Test RuntimeServiceClient.RunPodSandbox
		err = RunPodSandbox(runtimeClient, podSandboxConfig)
		if err != nil {
			return fmt.Errorf("Run pod sandbox failed: %v", err)
		}
		return nil
	},
}

var stopPodSandboxCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a pod sandbox",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		err := StopPodSandbox(runtimeClient, id)
		if err != nil {
			return fmt.Errorf("stopping the pod sandbox failed: %v", err)
		}
		return nil
	},
}

var removePodSandboxCommand = cli.Command{
	Name:  "remove",
	Usage: "remove a pod sandbox",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		err := RemovePodSandbox(runtimeClient, id)
		if err != nil {
			return fmt.Errorf("removing the pod sandbox failed: %v", err)
		}
		return nil
	},
}

var podSandboxStatusCommand = cli.Command{
	Name:  "status",
	Usage: "return the status of a pod",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		err := PodSandboxStatus(runtimeClient, id)
		if err != nil {
			return fmt.Errorf("getting the pod sandbox status failed: %v", err)
		}
		return nil
	},
}

var listPodSandboxCommand = cli.Command{
	Name:  "list",
	Usage: "list pod sandboxes",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "filter by pod sandbox id",
		},
		cli.StringFlag{
			Name:  "state,s",
			Value: "",
			Usage: "filter by pod sandbox state",
		},
		cli.StringSliceFlag{
			Name:  "label,l",
			Usage: "filter by key=value label",
		},
		cli.BoolFlag{
			Name:  "quiet,q",
			Usage: "list only pod IDs",
		},
	},
	Action: func(context *cli.Context) error {
		opts := listOptions{
			id:     context.String("id"),
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

		err := ListPodSandboxes(runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("listing pod sandboxes failed: %v", err)
		}
		return nil
	},
}

// RunPodSandbox sends a RunPodSandboxRequest to the server, and parses
// the returned RunPodSandboxResponse.
func RunPodSandbox(client pb.RuntimeServiceClient, config *pb.PodSandboxConfig) error {
	r, err := client.RunPodSandbox(context.Background(), &pb.RunPodSandboxRequest{Config: config})
	if err != nil {
		return err
	}
	fmt.Println(r.PodSandboxId)
	return nil
}

// StopPodSandbox sends a StopPodSandboxRequest to the server, and parses
// the returned StopPodSandboxResponse.
func StopPodSandbox(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := client.StopPodSandbox(context.Background(), &pb.StopPodSandboxRequest{PodSandboxId: ID})
	if err != nil {
		return err
	}
	fmt.Printf("Stop sandbox success ID: %s\n", ID)
	return nil
}

// RemovePodSandbox sends a RemovePodSandboxRequest to the server, and parses
// the returned RemovePodSandboxResponse.
func RemovePodSandbox(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := client.RemovePodSandbox(context.Background(), &pb.RemovePodSandboxRequest{PodSandboxId: ID})
	if err != nil {
		return err
	}
	fmt.Printf("Remove sandbox success ID: %s\n", ID)
	return nil
}

// PodSandboxStatus sends a PodSandboxStatusRequest to the server, and parses
// the returned PodSandboxStatusResponse.
func PodSandboxStatus(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	r, err := client.PodSandboxStatus(context.Background(), &pb.PodSandboxStatusRequest{PodSandboxId: ID})
	if err != nil {
		return err
	}
	fmt.Printf("ID: %s\n", r.Status.Id)
	if r.Status.Metadata != nil {
		if r.Status.Metadata.Name != "" {
			fmt.Printf("Name: %s\n", r.Status.Metadata.Name)
		}
		if r.Status.Metadata.Uid != "" {
			fmt.Printf("UID: %s\n", r.Status.Metadata.Uid)
		}
		if r.Status.Metadata.Namespace != "" {
			fmt.Printf("Namespace: %s\n", r.Status.Metadata.Namespace)
		}
		fmt.Printf("Attempt: %v\n", r.Status.Metadata.Attempt)
	}
	fmt.Printf("Status: %s\n", r.Status.State)
	ctm := time.Unix(0, r.Status.CreatedAt)
	fmt.Printf("Created: %v\n", ctm)
	fmt.Printf("Network namespace: %s\n", r.Status.Linux.Namespaces.Network)
	if r.Status.Network != nil {
		fmt.Printf("IP Address: %v\n", r.Status.Network.Ip)
	}
	if r.Status.Labels != nil {
		fmt.Println("Labels:")
		for _, k := range getSortedKeys(r.Status.Labels) {
			fmt.Printf("\t%s -> %s\n", k, r.Status.Labels[k])
		}
	}
	if r.Status.Annotations != nil {
		fmt.Println("Annotations:")
		for _, k := range getSortedKeys(r.Status.Annotations) {
			fmt.Printf("\t%s -> %s\n", k, r.Status.Annotations[k])
		}
	}
	return nil
}

// ListPodSandboxes sends a ListPodSandboxRequest to the server, and parses
// the returned ListPodSandboxResponse.
func ListPodSandboxes(client pb.RuntimeServiceClient, opts listOptions) error {
	filter := &pb.PodSandboxFilter{}
	if opts.id != "" {
		filter.Id = opts.id
	}
	if opts.state != "" {
		st := &pb.PodSandboxStateValue{}
		st.State = pb.PodSandboxState_SANDBOX_NOTREADY
		switch opts.state {
		case "ready":
			st.State = pb.PodSandboxState_SANDBOX_READY
			filter.State = st
		case "notready":
			st.State = pb.PodSandboxState_SANDBOX_NOTREADY
			filter.State = st
		default:
			log.Fatalf("--state should be ready or notready")
		}
	}
	if opts.labels != nil {
		filter.LabelSelector = opts.labels
	}
	r, err := client.ListPodSandbox(context.Background(), &pb.ListPodSandboxRequest{
		Filter: filter,
	})
	if err != nil {
		return err
	}
	for _, pod := range r.Items {
		if opts.quiet {
			fmt.Println(pod.Id)
			continue
		}
		fmt.Printf("ID: %s\n", pod.Id)
		if pod.Metadata != nil {
			if pod.Metadata.Name != "" {
				fmt.Printf("Name: %s\n", pod.Metadata.Name)
			}
			if pod.Metadata.Uid != "" {
				fmt.Printf("UID: %s\n", pod.Metadata.Uid)
			}
			if pod.Metadata.Namespace != "" {
				fmt.Printf("Namespace: %s\n", pod.Metadata.Namespace)
			}
			fmt.Printf("Attempt: %v\n", pod.Metadata.Attempt)
		}
		fmt.Printf("Status: %s\n", pod.State)
		ctm := time.Unix(0, pod.CreatedAt)
		fmt.Printf("Created: %v\n", ctm)
		if pod.Labels != nil {
			fmt.Println("Labels:")
			for _, k := range getSortedKeys(pod.Labels) {
				fmt.Printf("\t%s -> %s\n", k, pod.Labels[k])
			}
		}
		if pod.Annotations != nil {
			fmt.Println("Annotations:")
			for _, k := range getSortedKeys(pod.Annotations) {
				fmt.Printf("\t%s -> %s\n", k, pod.Annotations[k])
			}
		}
		fmt.Println()
	}
	return nil
}
