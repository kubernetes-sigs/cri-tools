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
	"errors"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var eventsCommand = &cli.Command{
	Name:                   "events",
	Usage:                  "Stream the events of containers",
	Aliases:                []string{"event"},
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   outputTypeJSON,
			Usage:   "Output format, One of: json|yaml|go-template",
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

		switch format := c.String("output"); format {
		case outputTypeJSON, outputTypeYAML:
			if c.String("template") != "" {
				return fmt.Errorf("template can't be used with %q format", format)
			}
		case outputTypeGoTemplate:
			if err := validateTemplate(c.String(("template"))); err != nil {
				return fmt.Errorf("failed to parse go-template: %w", err)
			}
		default:
			return fmt.Errorf("don't support %q format", format)
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		if err = Events(c, runtimeClient); err != nil {
			return fmt.Errorf("getting container events: %w", err)
		}

		return nil
	},
}

func Events(cliContext *cli.Context, client internalapi.RuntimeService) error {
	errCh := make(chan error, 1)

	containerEventsCh := make(chan *pb.ContainerEventResponse)

	go func() {
		logrus.Debug("getting container events")

		_, err := InterruptableRPC(cliContext.Context, func(ctx context.Context) (any, error) {
			return nil, client.GetContainerEvents(ctx, containerEventsCh, nil)
		})
		if errors.Is(err, io.EOF) {
			errCh <- nil

			return
		}

		errCh <- err
	}()

	for {
		select {
		case err := <-errCh:
			return err
		case e := <-containerEventsCh:
			err := outputEvent(e, cliContext.String("output"), cliContext.String("template"))
			if err != nil {
				fmt.Printf("failed to format container event with the error: %s\n", err)
			}
		}
	}
}
