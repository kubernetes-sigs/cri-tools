/*
Copyright 2023 The Kubernetes Authors.

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
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	cri "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type podMetricsOptions struct {
	// output format
	output string

	// live watch
	watch bool
}

var podMetricsCommand = &cli.Command{
	Name:                   "metricsp",
	Usage:                  "List pod metrics. Metrics are unstructured key/value pairs gathered by CRI meant to replace cAdvisor's /metrics/cadvisor endpoint.",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml",
		},
		&cli.BoolFlag{
			Name:    "watch",
			Aliases: []string{"w"},
			Usage:   "Watch pod metrics",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() > 0 {
			return cli.ShowSubcommandHelp(c)
		}

		client, err := getRuntimeService(c, 0)
		if err != nil {
			return fmt.Errorf("get runtime service: %w", err)
		}

		opts := podMetricsOptions{
			output: c.String("output"),
			watch:  c.Bool("watch"),
		}

		switch opts.output {
		case outputTypeJSON, outputTypeYAML, "":
		default:
			return cli.ShowSubcommandHelp(c)
		}

		if err := podMetrics(c.Context, client, opts); err != nil {
			return fmt.Errorf("get pod metrics: %w", err)
		}

		return nil
	},
}

func podMetrics(
	c context.Context,
	client cri.RuntimeService,
	opts podMetricsOptions,
) error {
	d := podMetricsDisplayer{opts}

	return handleDisplay(c, client, opts.watch, d.displayPodMetrics)
}

type podMetricsDisplayer struct {
	opts podMetricsOptions
}

func (p *podMetricsDisplayer) displayPodMetrics(
	c context.Context,
	client cri.RuntimeService,
) error {
	metrics, err := podSandboxMetrics(c, client)
	if err != nil {
		return err
	}

	response := &pb.ListPodSandboxMetricsResponse{PodMetrics: metrics}

	switch p.opts.output {
	case outputTypeJSON, "":
		return outputProtobufObjAsJSON(response)
	case outputTypeYAML:
		return outputProtobufObjAsYAML(response)
	}

	return nil
}

func podSandboxMetrics(ctx context.Context, client cri.RuntimeService) ([]*pb.PodSandboxMetrics, error) {
	metrics, err := InterruptableRPC(ctx, func(ctx context.Context) ([]*pb.PodSandboxMetrics, error) {
		return client.ListPodSandboxMetrics(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("list pod sandbox metrics: %w", err)
	}

	logrus.Debugf("PodMetrics: %v", metrics)

	return metrics, nil
}
