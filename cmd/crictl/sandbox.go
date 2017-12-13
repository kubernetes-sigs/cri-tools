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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Sirupsen/logrus"
	units "github.com/docker/go-units"
	"github.com/urfave/cli"
	"golang.org/x/net/context"

	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

type sandboxByCreated []*pb.PodSandbox

func (a sandboxByCreated) Len() int      { return len(a) }
func (a sandboxByCreated) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sandboxByCreated) Less(i, j int) bool {
	return a[i].CreatedAt > a[j].CreatedAt
}

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
			return fmt.Errorf("run pod sandbox failed: %v", err)
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
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Do not show verbose information",
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

		err := PodSandboxStatus(runtimeClient, id, context.String("output"), context.Bool("quiet"))
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
			Name:  "name",
			Value: "",
			Usage: "filter by pod sandbox name",
		},
		cli.StringFlag{
			Name:  "namespace",
			Value: "",
			Usage: "filter by pod sandbox namespace",
		},
		cli.StringFlag{
			Name:  "state, s",
			Value: "",
			Usage: "filter by pod sandbox state",
		},
		cli.StringSliceFlag{
			Name:  "label",
			Usage: "filter by key=value label",
		},
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "show verbose info for sandboxes",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "list only sandbox IDs",
		},
		cli.StringFlag{
			Name:  "output, o",
			Usage: "Output format, One of: json|yaml|table",
		},
		cli.BoolFlag{
			Name:  "latest, l",
			Usage: "Show recently created sandboxes",
		},
		cli.IntFlag{
			Name:  "last, n",
			Usage: "Show last n recently created sandboxes",
		},
		cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Show output without truncating the ID",
		},
	},
	Action: func(context *cli.Context) error {
		var err error
		if err = getRuntimeClient(context); err != nil {
			return err
		}

		opts := listOptions{
			id:      context.String("id"),
			state:   context.String("state"),
			verbose: context.Bool("verbose"),
			quiet:   context.Bool("quiet"),
			output:  context.String("output"),
			latest:  context.Bool("latest"),
			last:    context.Int("last"),
			noTrunc: context.Bool("no-trunc"),
		}
		opts.labels, err = parseLabelStringSlice(context.StringSlice("label"))
		if err != nil {
			return err
		}
		if context.String("name") != "" {
			opts.labels["io.kubernetes.pod.name"] = context.String("name")
		}
		if context.String("namespace") != "" {
			opts.labels["io.kubernetes.pod.namespace"] = context.String("namespace")
		}

		if err = ListPodSandboxes(runtimeClient, opts); err != nil {
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

	fmt.Printf("Stopped sandbox %s\n", ID)
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

// marshalPodSandboxStatus converts pod sandbox status into string and converts
// the timestamps into readable format.
func marshalPodSandboxStatus(ps *pb.PodSandboxStatus) (string, error) {
	statusStr, err := protobufObjectToJSON(ps)
	if err != nil {
		return "", err
	}
	jsonMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(statusStr), &jsonMap)
	if err != nil {
		return "", err
	}
	jsonMap["createdAt"] = time.Unix(0, ps.CreatedAt).Format(time.RFC3339Nano)
	return marshalMapInOrder(jsonMap, *ps)
}

// PodSandboxStatus sends a PodSandboxStatusRequest to the server, and parses
// the returned PodSandboxStatusResponse.
func PodSandboxStatus(client pb.RuntimeServiceClient, ID, output string, quiet bool) error {
	verbose := !(quiet)
	if output == "" { // default to json output
		output = "json"
	}
	if ID == "" {
		return fmt.Errorf("ID cannot be empty")
	}

	request := &pb.PodSandboxStatusRequest{
		PodSandboxId: ID,
		Verbose:      verbose,
	}
	logrus.Debugf("PodSandboxStatusRequest: %v", request)
	r, err := client.PodSandboxStatus(context.Background(), request)
	logrus.Debugf("PodSandboxStatusResponse: %v", r)
	if err != nil {
		return err
	}

	status, err := marshalPodSandboxStatus(r.Status)
	if err != nil {
		return err
	}
	switch output {
	case "json", "yaml":
		return outputStatusInfo(status, r.Info, output)
	case "table": // table output is after this switch block
	default:
		return fmt.Errorf("output option cannot be %s", output)
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
	if verbose {
		fmt.Printf("Info: %v\n", r.GetInfo())
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
	r.Items = getSandboxesList(r.GetItems(), opts)

	switch opts.output {
	case "json":
		return outputProtobufObjAsJSON(r)
	case "yaml":
		return outputProtobufObjAsJSON(r)
	}

	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	if !opts.verbose && !opts.quiet {
		fmt.Fprintln(w, "SANDBOX ID\tCREATED\tSTATE\tNAME\tNAMESPACE\tATTEMPT")
	}
	for _, pod := range r.Items {
		if opts.quiet {
			fmt.Printf("%s\n", pod.Id)
			continue
		}
		if !opts.verbose {
			createdAt := time.Unix(0, pod.CreatedAt)
			ctm := units.HumanDuration(time.Now().UTC().Sub(createdAt)) + " ago"
			id := pod.Id
			if !opts.noTrunc {
				id = strings.TrimPrefix(pod.Id, "")[:truncatedIDLen]
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
				id, ctm, pod.State, pod.Metadata.Name, pod.Metadata.Namespace, pod.Metadata.Attempt)
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

func getSandboxesList(sandboxesList []*pb.PodSandbox, opts listOptions) []*pb.PodSandbox {
	sort.Sort(sandboxByCreated(sandboxesList))
	n := len(sandboxesList)
	if opts.latest {
		n = 1
	}
	if opts.last > 0 {
		n = opts.last
	}
	n = func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}(n, len(sandboxesList))

	return sandboxesList[:n]
}
