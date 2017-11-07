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
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	restclient "k8s.io/client-go/rest"
	remoteclient "k8s.io/client-go/tools/remotecommand"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

var runtimeAttachCommand = cli.Command{
	Name:      "attach",
	Usage:     "Attach to a running container",
	ArgsUsage: "CONTAINER",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "tty,t",
			Usage: "Allocate a pseudo-TTY",
		},
		cli.BoolFlag{
			Name:  "stdin,i",
			Usage: "Keep STDIN open",
		},
	},
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return cli.ShowSubcommandHelp(context)
		}

		if err := getRuntimeClient(context); err != nil {
			return err
		}

		var opts = attachOptions{
			id:    id,
			tty:   context.Bool("tty"),
			stdin: context.Bool("stdin"),
		}
		err := Attach(runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("attaching running container failed: %v", err)

		}
		return nil

	},
	After: closeConnection,
}

// Attach sends an AttachRequest to server, and parses the returned AttachResponse
func Attach(client pb.RuntimeServiceClient, opts attachOptions) error {
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
	r, err := client.Attach(context.Background(), request)
	logrus.Debugf("AttachResponse: %v", r)
	if err != nil {
		return err
	}
	attachURL := r.Url
	if !strings.HasPrefix(attachURL, "http") {
		attachURL = kubeletURLPrefix + attachURL
	}

	URL, err := url.Parse(attachURL)
	if err != nil {
		return err
	}
	logrus.Debugf("Attach URL: %v", URL)
	attach, err := remoteclient.NewSPDYExecutor(&restclient.Config{}, "POST", URL)
	if err != nil {
		return err
	}

	streamOptions := remoteclient.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    opts.tty,
	}
	if opts.stdin {
		streamOptions.Stdin = os.Stdin
	}
	logrus.Debugf("StreamOptions: %v", streamOptions)
	return attach.Stream(streamOptions)
}
