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
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	restclient "k8s.io/client-go/rest"
	portforward "k8s.io/client-go/tools/portforward"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

var runtimePortForwardCommand = cli.Command{
	Name:  "portforward",
	Usage: "forword ports from a sandbox",
	Action: func(context *cli.Context) error {
		args := context.Args()
		if len(args) < 2 {
			return fmt.Errorf("Please specify sandbox and port to forward")
		}
		var opts = portforwardOptions{
			id:    args[0],
			ports: args[1:],
		}
		err := PortForward(runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("port forward failed: %v", err)

		}
		return nil

	},
	Before: getRuntimeClient,
	After:  closeConnection,
}

// PortForward sends an PortForwardRequest to server, and parses the returned PortForwardResponse
func PortForward(client pb.RuntimeServiceClient, opts portforwardOptions) error {
	if opts.id == "" {
		return fmt.Errorf("ID cannot be empty")

	}
	request := &pb.PortForwardRequest{
		PodSandboxId: opts.id,
	}
	logrus.Debugf("PortForwardRequest: %v", request)
	r, err := client.PortForward(context.Background(), request)
	logrus.Debugf("PortForwardResponse; %v", r)
	if err != nil {
		return err
	}
	portforwardURL := r.Url
	if !strings.HasPrefix(portforwardURL, "http") {
		portforwardURL = kubeletURLPrefix + portforwardURL
	}

	URL, err := url.Parse(portforwardURL)
	if err != nil {
		return err
	}
	logrus.Debugf("PortForward URL: %v", URL)
	exec, err := remotecommand.NewExecutor(&restclient.Config{}, "POST", URL)
	if err != nil {
		return err
	}

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if stopChan != nil {
			close(stopChan)
		}
	}()
	logrus.Debugf("Ports to forword: %v", opts.ports)
	pf, err := portforward.New(exec, opts.ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	return pf.ForwardPorts()
}
