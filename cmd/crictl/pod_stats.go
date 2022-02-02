/*
Copyright 2022 The Kubernetes Authors.

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
	"sort"
	"time"

	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/context"
	cri "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type podStatsOptions struct {
	// id of the pod sandbox
	id string

	// sample is the duration for sampling cpu usage.
	sample time.Duration

	// labels are selectors for the sandbox
	labels map[string]string

	// output format
	output string

	// live watch
	watch bool
}

var podStatsCommand = &cli.Command{
	Name:                   "statsp",
	Usage:                  "List pod resource usage statistics",
	UseShortOptionHandling: true,
	ArgsUsage:              "[ID]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "Filter by pod id",
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
	Action: func(ctx *cli.Context) error {
		client, err := getRuntimeService(ctx)
		if err != nil {
			return errors.Wrap(err, "get runtime service")
		}

		id := ctx.String("id")
		if id == "" && ctx.NArg() > 0 {
			id = ctx.Args().Get(0)
		}

		opts := podStatsOptions{
			id:     id,
			sample: time.Duration(ctx.Int("seconds")) * time.Second,
			output: ctx.String("output"),
			watch:  ctx.Bool("watch"),
		}
		opts.labels, err = parseLabelStringSlice(ctx.StringSlice("label"))
		if err != nil {
			return errors.Wrap(err, "parse pod labels")
		}

		return errors.Wrap(podStats(ctx.Context, client, opts), "get pod stats")
	},
}

type podStatsByID []*pb.PodSandboxStats

func (c podStatsByID) Len() int      { return len(c) }
func (c podStatsByID) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c podStatsByID) Less(i, j int) bool {
	return c[i].Attributes.Id < c[j].Attributes.Id
}

func podStats(
	ctx context.Context,
	client cri.RuntimeService,
	opts podStatsOptions,
) error {
	filter := &pb.PodSandboxStatsFilter{}
	if opts.id != "" {
		filter.Id = opts.id
	}
	if opts.labels != nil {
		filter.LabelSelector = opts.labels
	}

	display := newTableDisplay(20, 1, 3, ' ', 0)
	if opts.watch {
		displayErrCh := make(chan error, 1)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		watchCtx, cancelFn := context.WithCancel(context.Background())
		defer cancelFn()

		// Put the displayPodStats in another goroutine, because it might be
		// time consuming with lots of pods and we want to cancel it
		// ASAP when user hit CtrlC
		go func() {
			for range ticker.C {
				if err := displayPodStats(watchCtx, client, filter, display, opts); err != nil {
					displayErrCh <- err
					break
				}
			}
		}()

		// listen for CtrlC or error
		select {
		case <-SetupInterruptSignalHandler():
			cancelFn()
			return nil
		case err := <-displayErrCh:
			return err
		}
	}

	if err := displayPodStats(ctx, client, filter, display, opts); err != nil {
		return errors.Wrap(err, "display pod stats")
	}

	return nil
}

func displayPodStats(
	ctx context.Context,
	client cri.RuntimeService,
	filter *pb.PodSandboxStatsFilter,
	display *display,
	opts podStatsOptions,
) error {
	stats, err := getPodSandboxStats(client, filter)
	if err != nil {
		return err
	}

	response := &pb.ListPodSandboxStatsResponse{Stats: stats}
	switch opts.output {
	case "json":
		return outputProtobufObjAsJSON(response)
	case "yaml":
		return outputProtobufObjAsYAML(response)
	}

	oldStats := make(map[string]*pb.PodSandboxStats)
	for _, s := range stats {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		oldStats[s.Attributes.Id] = s
	}

	time.Sleep(opts.sample)

	stats, err = getPodSandboxStats(client, filter)
	if err != nil {
		return err
	}

	display.AddRow([]string{columnPodName, columnPodID, columnCPU, columnMemory})
	for _, s := range stats {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		id := getTruncatedID(s.Attributes.Id, "")

		linux := s.GetLinux()
		var cpu, mem uint64
		if linux != nil {
			cpu = linux.GetCpu().GetUsageCoreNanoSeconds().GetValue()
			mem = linux.GetMemory().GetWorkingSetBytes().GetValue()
		}

		if cpu == 0 && mem == 0 {
			// Skip without data, maybe windows pod
			continue
		}

		old, ok := oldStats[s.Attributes.Id]
		if !ok {
			// Skip new pod
			continue
		}
		oldLinux := old.GetLinux()

		var cpuPerc float64
		if cpu != 0 {
			// Only generate cpuPerc for running sandbox
			duration := linux.GetCpu().GetTimestamp() - oldLinux.GetCpu().GetTimestamp()
			if duration == 0 {
				return errors.Errorf("cpu stat is not updated during sample")
			}
			cpuPerc = float64(cpu-oldLinux.GetCpu().GetUsageCoreNanoSeconds().GetValue()) / float64(duration) * 100
		}
		display.AddRow([]string{
			s.Attributes.GetMetadata().GetName(),
			id,
			fmt.Sprintf("%.2f", cpuPerc),
			units.HumanSize(float64(mem)),
		})

	}
	display.ClearScreen()
	display.Flush()

	return nil
}

func getPodSandboxStats(
	client cri.RuntimeService,
	filter *pb.PodSandboxStatsFilter,
) ([]*pb.PodSandboxStats, error) {
	logrus.Debugf("PodSandboxStatsFilter: %v", filter)

	stats, err := client.ListPodSandboxStats(filter)
	if err != nil {
		return nil, errors.Wrap(err, "list pod sandbox stats")
	}
	logrus.Debugf("Stats: %v", stats)

	sort.Sort(podStatsByID(stats))

	return stats, nil
}
