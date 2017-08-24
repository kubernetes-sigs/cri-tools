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
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

var runPodSandboxCommand = cli.Command{
	Name:      "runs",
	Usage:     "Run a new sandbox",
	ArgsUsage: "sandbox-config.[json|yaml]",
	Action: func(context *cli.Context) error {
		sandboxSpec := context.Args().First()
		if sandboxSpec == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

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
	Name:      "stops",
	Usage:     "Stop a running sandbox",
	ArgsUsage: "SANDBOX",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		err := StopPodSandbox(runtimeClient, id)
		if err != nil {
			return fmt.Errorf("stopping the pod sandbox failed: %v", err)
		}
		return nil
	},
}

var removePodSandboxCommand = cli.Command{
	Name:      "rms",
	Usage:     "Remove a sandbox",
	ArgsUsage: "SANDBOX",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		err := RemovePodSandbox(runtimeClient, id)
		if err != nil {
			return fmt.Errorf("removing the pod sandbox failed: %v", err)
		}
		return nil
	},
}

var podSandboxStatusCommand = cli.Command{
	Name:      "inspects",
	Usage:     "Display the status of a sandbox",
	ArgsUsage: "SANDBOX",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Output format, One of: json|yaml|table",
		},
	},
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		err := PodSandboxStatus(runtimeClient, id, context.String("output"))
		if err != nil {
			return fmt.Errorf("getting the pod sandbox status failed: %v", err)
		}
		return nil
	},
}

var listPodSandboxCommand = cli.Command{
	Name:  "sandboxes",
	Usage: "List sandboxes",
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
			Name:  "verbose, v",
			Usage: "show verbose info for sandboxes",
		},
		cli.BoolFlag{
			Name:  "quiet",
			Usage: "list only sandbox IDs",
		},
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Output format, One of: json|yaml|table",
		},
	},
	Action: func(context *cli.Context) error {
		if err := getRuntimeClient(context); err != nil {
			return err
		}

		opts := listOptions{
			id:      context.String("id"),
			state:   context.String("state"),
			verbose: context.Bool("verbose"),
			labels:  make(map[string]string),
			quiet:   context.Bool("quiet"),
			output:  context.String("output"),
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
	request := &pb.RunPodSandboxRequest{Config: config}
	logrus.Debugf("RunPodSandboxRequest: %v", request)
	r, err := client.RunPodSandbox(context.Background(), request)
	logrus.Debugf("RunPodSandboxResponse: %v", r)
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
	request := &pb.StopPodSandboxRequest{PodSandboxId: ID}
	logrus.Debugf("StopPodSandboxRequest: %v", request)
	r, err := client.StopPodSandbox(context.Background(), request)
	logrus.Debugf("StopPodSandboxResponse: %v", r)
	if err != nil {
		return err
	}

	fmt.Printf("Stopped sandbox %s", ID)
	return nil
}

// RemovePodSandbox sends a RemovePodSandboxRequest to the server, and parses
// the returned RemovePodSandboxResponse.
func RemovePodSandbox(client pb.RuntimeServiceClient, ID string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	request := &pb.RemovePodSandboxRequest{PodSandboxId: ID}
	logrus.Debugf("RemovePodSandboxRequest: %v", request)
	r, err := client.RemovePodSandbox(context.Background(), request)
	logrus.Debugf("RemovePodSandboxResponse: %v", r)
	if err != nil {
		return err
	}
	fmt.Printf("Removed sandbox %s\n", ID)
	return nil
}

// PodSandboxStatus sends a PodSandboxStatusRequest to the server, and parses
// the returned PodSandboxStatusResponse.
func PodSandboxStatus(client pb.RuntimeServiceClient, ID, output string) error {
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	request := &pb.PodSandboxStatusRequest{PodSandboxId: ID}
	logrus.Debugf("PodSandboxStatusRequest: %v", request)
	r, err := client.PodSandboxStatus(context.Background(), &pb.PodSandboxStatusRequest{PodSandboxId: ID})
	logrus.Debugf("PodSandboxStatusResponse: %v", r)
	if err != nil {
		return err
	}

	switch output {
	case "json":
		return outputJSON(r.Status)

	case "yaml":
		return outputYAML(r.Status)
	}

	// output in table format by default.
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
	request := &pb.ListPodSandboxRequest{
		Filter: filter,
	}
	logrus.Debugf("ListPodSandboxRequest: %v", request)
	r, err := client.ListPodSandbox(context.Background(), request)
	logrus.Debugf("ListPodSandboxResponse: %v", r)
	if err != nil {
		return err
	}

	switch opts.output {
	case "json":
		return outputJSON(r.Items)

	case "yaml":
		return outputYAML(r.Items)
	}

	// output in table format by default.
	printHeader := true
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	for _, pod := range r.Items {
		if opts.quiet {
			fmt.Printf("%s\n", pod.Id)
			continue
		}
		if !opts.verbose {
			if printHeader {
				printHeader = false
				fmt.Fprintln(w, "SANDBOX ID\tNAME\tSTATE")
			}

			fmt.Fprintf(w, "%s\t%s\t%s\n", pod.Id, pod.Metadata.Name, pod.State)
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
			if pod.Metadata.Attempt != 0 {
				fmt.Printf("Attempt: %v\n", pod.Metadata.Attempt)
			}
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

	w.Flush()
	return nil
}
