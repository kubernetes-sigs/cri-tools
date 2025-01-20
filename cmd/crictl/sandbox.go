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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	errorUtils "k8s.io/apimachinery/pkg/util/errors"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type sandboxByCreated []*pb.PodSandbox

func (a sandboxByCreated) Len() int      { return len(a) }
func (a sandboxByCreated) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sandboxByCreated) Less(i, j int) bool {
	return a[i].CreatedAt > a[j].CreatedAt
}

var runPodCommand = &cli.Command{
	Name:      "runp",
	Usage:     "Run a new pod",
	ArgsUsage: "pod-config.[json|yaml]",
	Subcommands: []*cli.Command{{
		Name:      "jsonschema",
		Aliases:   []string{"js"},
		Usage:     "Display the JSON schema for the pod-config.json",
		UsageText: "The schema will be generated from the PodSandboxConfig of the CRI API compiled with this version of crictl",
		Action: func(*cli.Context) error {
			return printJSONSchema(&pb.PodSandboxConfig{})
		},
	}},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "runtime",
			Aliases: []string{"r"},
			Usage:   "Runtime handler to use. Available options are defined by the container runtime.",
		},
		&cli.DurationFlag{
			Name:    "cancel-timeout",
			Aliases: []string{"T"},
			Usage:   "Seconds to wait for a run pod sandbox request to complete before cancelling the request",
		},
	},

	Action: func(c *cli.Context) error {
		sandboxSpec := c.Args().First()
		if c.NArg() != 1 || sandboxSpec == "" {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeClient, err := getRuntimeService(c, c.Duration("cancel-timeout"))
		if err != nil {
			return err
		}

		podSandboxConfig, err := loadPodSandboxConfig(sandboxSpec)
		if err != nil {
			return fmt.Errorf("load podSandboxConfig: %w", err)
		}

		// Test RuntimeServiceClient.RunPodSandbox
		podID, err := RunPodSandbox(runtimeClient, podSandboxConfig, c.String("runtime"))
		if err != nil {
			return fmt.Errorf("run pod sandbox: %w", err)
		}
		fmt.Println(podID)
		return nil
	},
}

var stopPodCommand = &cli.Command{
	Name:      "stopp",
	Usage:     "Stop one or more running pods",
	ArgsUsage: "POD-ID [POD-ID...]",
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return cli.ShowSubcommandHelp(c)
		}
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}
		for i := range c.NArg() {
			id := c.Args().Get(i)
			err := StopPodSandbox(runtimeClient, id)
			if err != nil {
				return fmt.Errorf("stopping the pod sandbox %q: %w", id, err)
			}
		}
		return nil
	},
}

var removePodCommand = &cli.Command{
	Name:                   "rmp",
	Usage:                  "Remove one or more pods",
	ArgsUsage:              "POD-ID [POD-ID...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Force removal of the pod sandbox, disregarding if running",
		},
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Remove all pods",
		},
	},
	Action: func(ctx *cli.Context) error {
		runtimeClient, err := getRuntimeService(ctx, 0)
		if err != nil {
			return err
		}

		ids := ctx.Args().Slice()
		if ctx.Bool("all") {
			r, err := InterruptableRPC(nil, func(ctx context.Context) ([]*pb.PodSandbox, error) {
				return runtimeClient.ListPodSandbox(ctx, nil)
			})
			if err != nil {
				return err
			}
			ids = nil
			for _, sb := range r {
				ids = append(ids, sb.GetId())
			}
		}

		if len(ids) == 0 {
			if ctx.Bool("all") {
				logrus.Info("No pods to remove")
				return nil
			}
			return cli.ShowSubcommandHelp(ctx)
		}

		funcs := []func() error{}
		// set max concurrency to 10, to avoid too many goroutines
		maxConcurrency := 10
		sem := make(chan struct{}, maxConcurrency)
		for _, id := range ids {
			funcs = append(funcs, func() error {
				sem <- struct{}{}
				defer func() { <-sem }()
				resp, err := InterruptableRPC(nil, func(ctx context.Context) (*pb.PodSandboxStatusResponse, error) {
					return runtimeClient.PodSandboxStatus(ctx, id, false)
				})
				if err != nil {
					return fmt.Errorf("getting sandbox status of pod %q: %w", id, err)
				}
				if resp.Status.State == pb.PodSandboxState_SANDBOX_READY {
					if ctx.Bool("force") {
						if err := StopPodSandbox(runtimeClient, id); err != nil {
							return fmt.Errorf("stopping the pod sandbox %q failed: %w", id, err)
						}
					} else {
						return fmt.Errorf("pod sandbox %q is running, please stop it first", id)
					}
				}

				err = RemovePodSandbox(runtimeClient, id)
				if err != nil {
					return fmt.Errorf("removing the pod sandbox %q: %w", id, err)
				}

				return nil
			})
		}

		return errorUtils.AggregateGoroutines(funcs...)
	},
}

var podStatusCommand = &cli.Command{
	Name:                   "inspectp",
	Usage:                  "Display the status of one or more pods",
	ArgsUsage:              "POD-ID [POD-ID...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|go-template|table",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Do not show verbose information",
		},
		&cli.StringFlag{
			Name:  "template",
			Usage: "The template string is only used when output is go-template; The Template format is golang template",
		},
		&cli.StringFlag{
			Name:  "name",
			Usage: "Filter by pod name regular expression pattern",
		},
		&cli.StringFlag{
			Name:  "namespace",
			Usage: "Filter by pod namespace regular expression pattern",
		},
		&cli.StringFlag{
			Name:    "state",
			Aliases: []string{"s"},
			Usage:   "Filter by pod state",
		},
		&cli.StringSliceFlag{
			Name:  "label",
			Usage: "Filter by key=value label",
		},
		&cli.BoolFlag{
			Name:    "latest",
			Aliases: []string{"l"},
			Usage:   "Show the most recently created pod",
		},
		&cli.IntFlag{
			Name:    "last",
			Aliases: []string{"n"},
			Usage:   "Show last n recently created pods. Set 0 for unlimited",
		},
	},
	Action: func(c *cli.Context) error {
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		ids := c.Args().Slice()

		if len(ids) == 0 {
			opts := &listOptions{
				nameRegexp:         c.String("name"),
				podNamespaceRegexp: c.String("namespace"),
				state:              c.String("state"),
				latest:             c.Bool("latest"),
				last:               c.Int("last"),
			}
			opts.labels, err = parseLabelStringSlice(c.StringSlice("label"))
			if err != nil {
				return fmt.Errorf("parse label string slice: %w", err)
			}
			sbs, err := ListPodSandboxes(runtimeClient, opts)
			if err != nil {
				return fmt.Errorf("listing pod sandboxes: %w", err)
			}
			for _, sb := range sbs {
				ids = append(ids, sb.GetId())
			}
		}

		if len(ids) == 0 {
			logrus.Error("No IDs provided or nothing found per filter")
			return cli.ShowSubcommandHelp(c)
		}

		if err := podSandboxStatus(
			runtimeClient,
			ids,
			c.String("output"),
			c.Bool("quiet"),
			c.String("template"),
		); err != nil {
			return fmt.Errorf("get the status of pod sandboxes: %w", err)
		}

		return nil
	},
}

var listPodCommand = &cli.Command{
	Name:                   "pods",
	Usage:                  "List pods",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "id",
			Usage: "filter by pod id",
		},
		&cli.StringFlag{
			Name:  "name",
			Usage: "Filter by pod name regular expression pattern",
		},
		&cli.StringFlag{
			Name:  "namespace",
			Usage: "Filter by pod namespace regular expression pattern",
		},
		&cli.StringFlag{
			Name:    "state",
			Aliases: []string{"s"},
			Usage:   "Filter by pod state",
		},
		&cli.StringSliceFlag{
			Name:  "label",
			Usage: "Filter by key=value label",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Show verbose info for pods",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "List only pod IDs",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|table",
			Value:   outputTypeTable,
		},
		&cli.BoolFlag{
			Name:    "latest",
			Aliases: []string{"l"},
			Usage:   "Show the most recently created pod",
		},
		&cli.IntFlag{
			Name:    "last",
			Aliases: []string{"n"},
			Usage:   "Show last n recently created pods. Set 0 for unlimited",
		},
		&cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Show output without truncating the ID",
		},
	},
	Action: func(c *cli.Context) error {
		var err error
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		opts := &listOptions{
			id:                 c.String("id"),
			state:              c.String("state"),
			verbose:            c.Bool("verbose"),
			quiet:              c.Bool("quiet"),
			output:             c.String("output"),
			latest:             c.Bool("latest"),
			last:               c.Int("last"),
			noTrunc:            c.Bool("no-trunc"),
			nameRegexp:         c.String("name"),
			podNamespaceRegexp: c.String("namespace"),
		}
		opts.labels, err = parseLabelStringSlice(c.StringSlice("label"))
		if err != nil {
			return err
		}
		if err = OutputPodSandboxes(runtimeClient, opts); err != nil {
			return fmt.Errorf("listing pod sandboxes: %w", err)
		}
		return nil
	},
}

// RunPodSandbox sends a RunPodSandboxRequest to the server, and parses
// the returned RunPodSandboxResponse.
func RunPodSandbox(client internalapi.RuntimeService, config *pb.PodSandboxConfig, runtime string) (string, error) {
	request := &pb.RunPodSandboxRequest{
		Config:         config,
		RuntimeHandler: runtime,
	}
	logrus.Debugf("RunPodSandboxRequest: %v", request)
	r, err := InterruptableRPC(nil, func(ctx context.Context) (string, error) {
		return client.RunPodSandbox(ctx, config, runtime)
	})
	logrus.Debugf("RunPodSandboxResponse: %v", r)
	if err != nil {
		return "", err
	}
	return r, nil
}

// StopPodSandbox sends a StopPodSandboxRequest to the server, and parses
// the returned StopPodSandboxResponse.
func StopPodSandbox(client internalapi.RuntimeService, id string) error {
	if id == "" {
		return errors.New("ID cannot be empty")
	}
	logrus.Debugf("Stopping pod sandbox: %s", id)
	if _, err := InterruptableRPC(nil, func(ctx context.Context) (any, error) {
		return nil, client.StopPodSandbox(ctx, id)
	}); err != nil {
		return err
	}

	fmt.Printf("Stopped sandbox %s\n", id)
	return nil
}

// RemovePodSandbox sends a RemovePodSandboxRequest to the server, and parses
// the returned RemovePodSandboxResponse.
func RemovePodSandbox(client internalapi.RuntimeService, id string) error {
	if id == "" {
		return errors.New("ID cannot be empty")
	}
	logrus.Debugf("Removing pod sandbox: %s", id)
	if _, err := InterruptableRPC(nil, func(ctx context.Context) (any, error) {
		return nil, client.RemovePodSandbox(ctx, id)
	}); err != nil {
		return err
	}
	fmt.Printf("Removed sandbox %s\n", id)
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

// podSandboxStatus sends a PodSandboxStatusRequest to the server, and parses
// the returned PodSandboxStatusResponse.
//
//nolint:dupl // pods and containers are similar, but still different
func podSandboxStatus(client internalapi.RuntimeService, ids []string, output string, quiet bool, tmplStr string) error {
	verbose := !(quiet)
	if output == "" { // default to json output
		output = outputTypeJSON
	}
	if len(ids) == 0 {
		return errors.New("ID cannot be empty")
	}

	statuses := []statusData{}
	for _, id := range ids {
		request := &pb.PodSandboxStatusRequest{
			PodSandboxId: id,
			Verbose:      verbose,
		}
		logrus.Debugf("PodSandboxStatusRequest: %v", request)
		r, err := InterruptableRPC(nil, func(ctx context.Context) (*pb.PodSandboxStatusResponse, error) {
			return client.PodSandboxStatus(ctx, id, verbose)
		})

		logrus.Debugf("PodSandboxStatusResponse: %v", r)
		if err != nil {
			return fmt.Errorf("get pod sandbox status: %w", err)
		}

		statusJSON, err := marshalPodSandboxStatus(r.Status)
		if err != nil {
			return fmt.Errorf("marshal pod sandbox status: %w", err)
		}

		if output == outputTypeTable {
			outputPodSandboxStatusTable(r, verbose)
		} else {
			statuses = append(statuses, statusData{json: statusJSON, info: r.Info})
		}
	}

	return outputStatusData(statuses, output, tmplStr)
}

func outputPodSandboxStatusTable(r *pb.PodSandboxStatusResponse, verbose bool) {
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
		fmt.Printf("IP Addresses: %v\n", r.Status.Network.Ip)
		for _, ip := range r.Status.Network.AdditionalIps {
			fmt.Printf("Additional IP: %v\n", ip.Ip)
		}
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
}

// ListPodSandboxes sends a ListPodSandboxRequest to the server, and parses
// the returned ListPodSandboxResponse.
func ListPodSandboxes(client internalapi.RuntimeService, opts *listOptions) ([]*pb.PodSandbox, error) {
	filter := &pb.PodSandboxFilter{}
	if opts.id != "" {
		filter.Id = opts.id
	}
	if opts.state != "" {
		st := &pb.PodSandboxStateValue{}
		st.State = pb.PodSandboxState_SANDBOX_NOTREADY
		switch strings.ToLower(opts.state) {
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
	r, err := InterruptableRPC(nil, func(ctx context.Context) ([]*pb.PodSandbox, error) {
		return client.ListPodSandbox(ctx, filter)
	})
	logrus.Debugf("ListPodSandboxResponse: %v", r)
	if err != nil {
		return nil, fmt.Errorf("call list sandboxes RPC: %w", err)
	}
	return getSandboxesList(r, opts), nil
}

// OutputPodSandboxes sends a ListPodSandboxRequest to the server, and parses
// the returned ListPodSandboxResponse for output.
func OutputPodSandboxes(client internalapi.RuntimeService, opts *listOptions) error {
	r, err := ListPodSandboxes(client, opts)
	if err != nil {
		return fmt.Errorf("list pod sandboxes: %w", err)
	}

	switch opts.output {
	case outputTypeJSON:
		return outputProtobufObjAsJSON(&pb.ListPodSandboxResponse{Items: r})
	case outputTypeYAML:
		return outputProtobufObjAsYAML(&pb.ListPodSandboxResponse{Items: r})
	case outputTypeTable:
	// continue; output will be generated after the switch block ends.
	default:
		return fmt.Errorf("unsupported output format %q", opts.output)
	}

	display := newDefaultTableDisplay()
	if !opts.verbose && !opts.quiet {
		display.AddRow([]string{
			columnPodID,
			columnCreated,
			columnState,
			columnName,
			columnNamespace,
			columnAttempt,
			columnPodRuntime,
		})
	}
	c := cases.Title(language.Und)
	for _, pod := range r {
		if opts.quiet {
			fmt.Printf("%s\n", pod.Id)
			continue
		}
		if !opts.verbose {
			createdAt := time.Unix(0, pod.CreatedAt)
			ctm := units.HumanDuration(time.Now().UTC().Sub(createdAt)) + " ago"
			id := pod.Id
			if !opts.noTrunc {
				id = getTruncatedID(id, "")
			}
			display.AddRow([]string{
				id,
				ctm,
				convertPodState(pod.State),
				pod.Metadata.Name,
				pod.Metadata.Namespace,
				strconv.FormatUint(uint64(pod.Metadata.Attempt), 10),
				getSandboxesRuntimeHandler(pod),
			})
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
		fmt.Printf("Status: %s\n", convertPodState(pod.State))
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
		fmt.Printf("%s: %s\n",
			c.String(columnPodRuntime),
			getSandboxesRuntimeHandler(pod))

		fmt.Println()
	}

	display.Flush()
	return nil
}

func convertPodState(state pb.PodSandboxState) string {
	switch state {
	case pb.PodSandboxState_SANDBOX_READY:
		return "Ready"
	case pb.PodSandboxState_SANDBOX_NOTREADY:
		return "NotReady"
	default:
		log.Fatalf("Unknown pod state %q", state)
		return ""
	}
}

func getSandboxesRuntimeHandler(sandbox *pb.PodSandbox) string {
	if sandbox.RuntimeHandler == "" {
		return "(default)"
	}
	return sandbox.RuntimeHandler
}

func getSandboxesList(sandboxesList []*pb.PodSandbox, opts *listOptions) []*pb.PodSandbox {
	filtered := []*pb.PodSandbox{}
	for _, p := range sandboxesList {
		// Filter by pod name/namespace regular expressions.
		if p.Metadata != nil && matchesRegex(opts.nameRegexp, p.Metadata.Name) &&
			matchesRegex(opts.podNamespaceRegexp, p.Metadata.Namespace) {
			filtered = append(filtered, p)
		}
	}

	sort.Sort(sandboxByCreated(filtered))
	n := len(filtered)
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
	}(n, len(filtered))

	return filtered[:n]
}
