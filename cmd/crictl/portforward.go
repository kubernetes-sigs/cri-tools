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
	"net/http"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var runtimePortForwardCommand = &cli.Command{
	Name:      "port-forward",
	Usage:     "Forward local port to a pod",
	ArgsUsage: "POD-ID [LOCAL_PORT:]REMOTE_PORT",
	Action: func(context *cli.Context) error {
		args := context.Args().Slice()
		if len(args) < 2 {
			return cli.ShowSubcommandHelp(context)
		}

		runtimeClient, err := getRuntimeService(context, 0)
		if err != nil {
			return err
		}

		var opts = portforwardOptions{
			id:    args[0],
			ports: args[1:],
		}
		err = PortForward(runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("port forward: %w", err)

		}
		return nil

	},
}

// PortForward sends an PortForwardRequest to server, and parses the returned PortForwardResponse
func PortForward(client internalapi.RuntimeService, opts portforwardOptions) error {
	if opts.id == "" {
		return fmt.Errorf("ID cannot be empty")

	}
	request := &pb.PortForwardRequest{
		PodSandboxId: opts.id,
	}
	logrus.Debugf("PortForwardRequest: %v", request)
	r, err := client.PortForward(context.TODO(), request)
	logrus.Debugf("PortForwardResponse; %v", r)
	if err != nil {
		return err
	}
	portforwardURL := r.Url

	URL, err := url.Parse(portforwardURL)
	if err != nil {
		return err
	}

	if URL.Host == "" {
		URL.Host = kubeletURLHost
	}

	if URL.Scheme == "" {
		URL.Scheme = kubeletURLSchema
	}

	logrus.Debugf("PortForward URL: %v", URL)
	transport, upgrader, err := spdy.RoundTripperFor(&restclient.Config{})
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", URL)

	readyChan := make(chan struct{})

	logrus.Debugf("Ports to forword: %v", opts.ports)
	pf, err := portforward.New(dialer, opts.ports, SetupInterruptSignalHandler(), readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	return pf.ForwardPorts()
}
