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
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/tools/portforward"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
)

var runtimePortForwardCommand = &cli.Command{
	Name:      "port-forward",
	Usage:     "Forward local port to a pod",
	ArgsUsage: "POD-ID [LOCAL_PORT:]REMOTE_PORT",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    transportFlag,
			Aliases: []string{"r"},
			Value:   common.TransportSpdy,
			Usage:   fmt.Sprintf("Transport protocol to use, one of: %s|%s", common.TransportSpdy, common.TransportWebsocket),
		},
		&cli.StringFlag{
			Name:    flagTLSSNI,
			Usage:   "Server name used in the TLS client to check server certificates against",
			Aliases: []string{"tls-server-name"},
			Value:   "localhost",
		},
		&cli.StringFlag{
			Name:  flagTLSCA,
			Usage: "Path to the streaming TLS CA certificate",
		},
		&cli.StringFlag{
			Name:  flagTLSCert,
			Usage: "Path to the streaming TLS certificate",
		},
		&cli.StringFlag{
			Name:  flagTLSKey,
			Usage: "Path to the streaming TLS key",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() < 2 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		opts := portforwardOptions{
			id:        c.Args().Get(0),
			ports:     c.Args().Tail(),
			transport: c.String(transportFlag),
		}

		opts.tlsConfig, err = tlsConfigFromFlags(c)
		if err != nil {
			return fmt.Errorf("get TLS config from flags: %w", err)
		}

		if err = PortForward(c.Context, runtimeClient, opts); err != nil {
			return fmt.Errorf("port forward: %w", err)
		}

		return nil
	},
}

// PortForward sends an PortForwardRequest to server, and parses the returned PortForwardResponse.
func PortForward(ctx context.Context, client internalapi.RuntimeService, opts portforwardOptions) error {
	if opts.id == "" {
		return errors.New("ID cannot be empty")
	}

	request := &pb.PortForwardRequest{
		PodSandboxId: opts.id,
	}
	logrus.Debugf("PortForwardRequest: %v", request)

	r, err := InterruptableRPC(ctx, func(ctx context.Context) (*pb.PortForwardResponse, error) {
		return client.PortForward(ctx, request)
	})
	logrus.Debugf("PortForwardResponse; %v", r)

	if err != nil {
		return err
	}

	parsedURL, err := url.Parse(r.GetUrl())
	if err != nil {
		return err
	}

	if parsedURL.Host == "" {
		parsedURL.Host = kubeletURLHost
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = kubeletURLSchema
	}

	logrus.Debugf("PortForward URL: %v", parsedURL)

	dialer, err := common.GetDialer(opts.transport, parsedURL, opts.tlsConfig)
	if err != nil {
		return fmt.Errorf("get dialer: %w", err)
	}

	readyChan := make(chan struct{})

	logrus.Debugf("Ports to forward: %v", opts.ports)

	pf, err := portforward.New(dialer, opts.ports, SetupInterruptSignalHandler(), readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	return pf.ForwardPorts()
}
