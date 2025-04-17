/*
Copyright 2025 The Kubernetes Authors.

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

type metricDescriptorsOptions struct {
	// output format
	output string
}

var metricDescriptorsCommand = &cli.Command{
	Name:                   "metricdescs",
	Usage:                  "List metric descriptors. Returns information about the metrics available through the CRI.",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml",
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

		opts := metricDescriptorsOptions{
			output: c.String("output"),
		}

		switch opts.output {
		case outputTypeJSON, outputTypeYAML, "":
		default:
			return cli.ShowSubcommandHelp(c)
		}

		if err := metricDescriptors(c.Context, client, opts); err != nil {
			return fmt.Errorf("get metric descriptors: %w", err)
		}

		return nil
	},
}

func metricDescriptors(
	c context.Context,
	client cri.RuntimeService,
	opts metricDescriptorsOptions,
) error {
	d := metricDescriptorsDisplayer{opts}

	return d.displayMetricDescriptors(c, client)
}

type metricDescriptorsDisplayer struct {
	opts metricDescriptorsOptions
}

func (m *metricDescriptorsDisplayer) displayMetricDescriptors(
	c context.Context,
	client cri.RuntimeService,
) error {
	descriptors, err := listMetricDescriptors(c, client)
	if err != nil {
		return err
	}

	response := &pb.ListMetricDescriptorsResponse{Descriptors: descriptors}

	switch m.opts.output {
	case outputTypeJSON, "":
		return outputProtobufObjAsJSON(response)
	case outputTypeYAML:
		return outputProtobufObjAsYAML(response)
	}

	return nil
}

func listMetricDescriptors(ctx context.Context, client cri.RuntimeService) ([]*pb.MetricDescriptor, error) {
	descriptors, err := InterruptableRPC(ctx, func(ctx context.Context) ([]*pb.MetricDescriptor, error) {
		return client.ListMetricDescriptors(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("list metric descriptors: %w", err)
	}

	logrus.Debugf("MetricDescriptors: %v", descriptors)

	return descriptors, nil
}
