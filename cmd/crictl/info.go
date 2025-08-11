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
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var runtimeStatusCommand = &cli.Command{
	Name:                   "info",
	Usage:                  "Display information of the container runtime",
	ArgsUsage:              "",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   outputTypeJSON,
			Usage:   "Output format, One of: json|yaml|go-template",
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
	},
	Action: func(c *cli.Context) error {
		if c.NArg() != 0 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		if err = Info(c, runtimeClient); err != nil {
			return fmt.Errorf("getting status of runtime: %w", err)
		}

		return nil
	},
}

// Info sends a StatusRequest to the server, and parses the returned StatusResponse.
func Info(cliContext *cli.Context, client internalapi.RuntimeService) error {
	request := &pb.StatusRequest{Verbose: !cliContext.Bool("quiet")}
	logrus.Debugf("StatusRequest: %v", request)

	r, err := InterruptableRPC(cliContext.Context, func(ctx context.Context) (*pb.StatusResponse, error) {
		return client.Status(ctx, request.GetVerbose())
	})
	logrus.Debugf("StatusResponse: %v", r)

	if err != nil {
		return err
	}

	statusJSON, err := protobufObjectToJSON(r.GetStatus())
	if err != nil {
		return fmt.Errorf("create status JSON: %w", err)
	}

	handlers, err := json.Marshal(r.GetRuntimeHandlers()) // protobufObjectToJSON cannot be used
	if err != nil {
		return err
	}

	features, err := json.Marshal(r.GetFeatures()) // protobufObjectToJSON cannot be used
	if err != nil {
		return err
	}

	data := []statusData{{json: statusJSON, runtimeHandlers: string(handlers), features: string(features), info: r.GetInfo()}}

	return outputStatusData(data, cliContext.String("output"), cliContext.String("template"))
}
