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
	"net/http"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var runtimePortForwardCommand = &cli.Command{
	Name:      "port-forward",
	Usage:     "Forward local port to a pod",
	ArgsUsage: "POD-ID [LOCAL_PORT:]REMOTE_PORT",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    transportFlag,
			Aliases: []string{"r"},
			Value:   transportSpdy,
			Usage:   fmt.Sprintf("Transport protocol to use, one of: %s|%s", transportSpdy, transportWebsocket),
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
		if err = PortForward(runtimeClient, opts); err != nil {
			return fmt.Errorf("port forward: %w", err)
		}
		return nil
	},
}

// PortForward sends an PortForwardRequest to server, and parses the returned PortForwardResponse.
func PortForward(client internalapi.RuntimeService, opts portforwardOptions) error {
	if opts.id == "" {
		return errors.New("ID cannot be empty")
	}
	request := &pb.PortForwardRequest{
		PodSandboxId: opts.id,
	}
	logrus.Debugf("PortForwardRequest: %v", request)
	r, err := InterruptableRPC(nil, func(ctx context.Context) (*pb.PortForwardResponse, error) {
		return client.PortForward(ctx, request)
	})
	logrus.Debugf("PortForwardResponse; %v", r)
	if err != nil {
		return err
	}

	parsedURL, err := url.Parse(r.Url)
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
	dialer, err := getDialer(opts.transport, parsedURL)
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

func getDialer(transport string, parsedURL *url.URL) (exec httpstream.Dialer, err error) {
	config := &rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: true}}

	switch transport {
	case transportSpdy:
		tr, upgrader, err := spdy.RoundTripperFor(config)
		if err != nil {
			return nil, fmt.Errorf("get SPDY round tripper: %w", err)
		}
		return spdy.NewDialer(upgrader, &http.Client{Transport: tr}, "POST", parsedURL), nil

	case transportWebsocket:
		return portforward.NewSPDYOverWebsocketDialer(parsedURL, config)

	default:
		return nil, fmt.Errorf("unknown transport: %s", transport)
	}
}
