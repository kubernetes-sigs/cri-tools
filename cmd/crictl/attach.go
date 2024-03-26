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
	"fmt"
	"net/url"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var runtimeAttachCommand = &cli.Command{
	Name:                   "attach",
	Usage:                  "Attach to a running container",
	ArgsUsage:              "CONTAINER-ID",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "tty",
			Aliases: []string{"t"},
			Usage:   "Allocate a pseudo-TTY",
		},
		&cli.BoolFlag{
			Name:    "stdin",
			Aliases: []string{"i"},
			Usage:   "Keep STDIN open",
		},
		&cli.StringFlag{
			Name:    transportFlag,
			Aliases: []string{"r"},
			Value:   transportSpdy,
			Usage:   fmt.Sprintf("Transport protocol to use, one of: %s|%s", transportSpdy, transportWebsocket),
		},
	},
	Action: func(c *cli.Context) error {
		id := c.Args().First()
		if id == "" {
			return fmt.Errorf("ID cannot be empty")
		}

		if c.NArg() != 1 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(c.Context)
		defer cancel()

		var opts = attachOptions{
			id:    id,
			tty:   c.Bool("tty"),
			stdin: c.Bool("stdin"),
		}
		if err = Attach(ctx, runtimeClient, opts); err != nil {
			return fmt.Errorf("attaching running container failed: %w", err)

		}
		return nil
	},
}

// Attach sends an AttachRequest to server, and parses the returned AttachResponse
func Attach(ctx context.Context, client internalapi.RuntimeService, opts attachOptions) error {
	if opts.id == "" {
		return fmt.Errorf("ID cannot be empty")

	}
	request := &pb.AttachRequest{
		ContainerId: opts.id,
		Tty:         opts.tty,
		Stdin:       opts.stdin,
		Stdout:      true,
		Stderr:      !opts.tty,
	}
	logrus.Debugf("AttachRequest: %v", request)
	r, err := client.Attach(ctx, request)
	logrus.Debugf("AttachResponse: %v", r)
	if err != nil {
		return err
	}
	attachURL := r.Url

	URL, err := url.Parse(attachURL)
	if err != nil {
		return err
	}

	if URL.Host == "" {
		URL.Host = kubeletURLHost
	}
	if URL.Scheme == "" {
		URL.Scheme = kubeletURLSchema
	}

	logrus.Debugf("Attach URL: %v", URL)
	return stream(ctx, opts.stdin, opts.tty, opts.transport, URL)
}
