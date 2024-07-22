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
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type statsOptions struct {
	// all containers
	all bool
	// id of container
	id string
	// podID of container
	podID string
	// sample is the duration for sampling cpu usage.
	sample time.Duration
	// labels are selectors for the sandbox
	labels map[string]string
	// output format
	output string
	// live watch
	watch bool
}

var statsCommand = &cli.Command{
	Name:                   "stats",
	Usage:                  "List container(s) resource usage statistics",
	UseShortOptionHandling: true,
	ArgsUsage:              "[ID]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Show all containers (default shows just running)",
		},
		&cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "Filter by container id",
		},
		&cli.StringFlag{
			Name:    "pod",
			Aliases: []string{"p"},
			Value:   "",
			Usage:   "Filter by pod id",
		},
		&cli.StringSliceFlag{
			Name:  "label",
			Usage: "Filter by key=value label",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|table",
		},
		&cli.IntFlag{
			Name:    "seconds",
			Aliases: []string{"s"},
			Value:   1,
			Usage:   "Sample duration for CPU usage in seconds",
		},
		&cli.BoolFlag{
			Name:    "watch",
			Aliases: []string{"w"},
			Usage:   "Watch pod resources",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() > 1 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		id := c.String("id")
		if id == "" && c.NArg() > 0 {
			id = c.Args().First()
		}

		opts := statsOptions{
			all:    c.Bool("all"),
			id:     id,
			podID:  c.String("pod"),
			sample: time.Duration(c.Int("seconds")) * time.Second,
			output: c.String("output"),
			watch:  c.Bool("watch"),
		}
		opts.labels, err = parseLabelStringSlice(c.StringSlice("label"))
		if err != nil {
			return err
		}

		if err = ContainerStats(runtimeClient, opts); err != nil {
			return fmt.Errorf("get container stats: %w", err)
		}
		return nil
	},
}

type containerStatsByID []*pb.ContainerStats

func (c containerStatsByID) Len() int      { return len(c) }
func (c containerStatsByID) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c containerStatsByID) Less(i, j int) bool {
	return c[i].Attributes.Id < c[j].Attributes.Id
}

type containerStatsDisplayer struct {
	opts    statsOptions
	request *pb.ListContainerStatsRequest
	*display
}

// ContainerStats sends a ListContainerStatsRequest to the server, and
// parses the returned ListContainerStatsResponse.
func ContainerStats(client internalapi.RuntimeService, opts statsOptions) error {
	d := containerStatsDisplayer{
		opts: opts,
		request: &pb.ListContainerStatsRequest{
			Filter: &pb.ContainerStatsFilter{
				Id:            opts.id,
				PodSandboxId:  opts.podID,
				LabelSelector: opts.labels,
			},
		},
		display: newTableDisplay(20, 1, 3, ' ', 0),
	}

	return handleDisplay(context.TODO(), client, opts.watch, d.displayStats)
}

func (d containerStatsDisplayer) displayStats(ctx context.Context, client internalapi.RuntimeService) error {
	r, err := getContainerStats(ctx, client, d.request)
	if err != nil {
		return err
	}
	switch d.opts.output {
	case "json":
		return outputProtobufObjAsJSON(r)
	case "yaml":
		return outputProtobufObjAsYAML(r)
	}
	oldStats := make(map[string]*pb.ContainerStats)
	for _, s := range r.GetStats() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		oldStats[s.Attributes.Id] = s
	}

	time.Sleep(d.opts.sample)

	r, err = getContainerStats(ctx, client, d.request)
	if err != nil {
		return err
	}

	d.display.AddRow([]string{columnContainer, columnName, columnCPU, columnMemory, columnDisk, columnInodes})
	for _, s := range r.GetStats() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		id := getTruncatedID(s.Attributes.Id, "")
		name := s.GetAttributes().GetMetadata().GetName()
		cpu := s.GetCpu().GetUsageCoreNanoSeconds().GetValue()
		mem := s.GetMemory().GetWorkingSetBytes().GetValue()
		disk := s.GetWritableLayer().GetUsedBytes().GetValue()
		inodes := s.GetWritableLayer().GetInodesUsed().GetValue()
		if !d.opts.all && cpu == 0 && mem == 0 {
			// Skip non-running container
			continue
		}
		old, ok := oldStats[s.Attributes.Id]
		if !ok {
			// Skip new container
			continue
		}
		var cpuPerc float64
		if cpu != 0 {
			// Only generate cpuPerc for running container
			duration := s.GetCpu().GetTimestamp() - old.GetCpu().GetTimestamp()
			if duration == 0 {
				return errors.New("cpu stat is not updated during sample")
			}
			cpuPerc = float64(cpu-old.GetCpu().GetUsageCoreNanoSeconds().GetValue()) / float64(duration) * 100
		}
		d.display.AddRow([]string{id, name, fmt.Sprintf("%.2f", cpuPerc), units.HumanSize(float64(mem)),
			units.HumanSize(float64(disk)), strconv.FormatUint(inodes, 10)})

	}
	d.display.ClearScreen()
	d.display.Flush()

	return nil
}

func getContainerStats(ctx context.Context, client internalapi.RuntimeService, request *pb.ListContainerStatsRequest) (*pb.ListContainerStatsResponse, error) {
	logrus.Debugf("ListContainerStatsRequest: %v", request)
	r, err := InterruptableRPC(ctx, func(ctx context.Context) ([]*pb.ContainerStats, error) {
		return client.ListContainerStats(ctx, request.Filter)
	})
	logrus.Debugf("ListContainerResponse: %v", r)
	if err != nil {
		return nil, err
	}
	sort.Sort(containerStatsByID(r))
	return &pb.ListContainerStatsResponse{Stats: r}, nil
}
