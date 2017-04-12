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
	"net"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

const (
	defaultTimeout = 10 * time.Second
)

func getRuntimeClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	runtimeEndpoint := context.GlobalString("runtime-endpoint")
	if runtimeEndpoint == "" {
		return nil, fmt.Errorf("--runtime-endpoint is not set")
	}
	conn, err := grpc.Dial(context.GlobalString("runtime-endpoint"), grpc.WithInsecure(), grpc.WithTimeout(context.GlobalDuration("timeout")),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return conn, nil
}

func main() {
	app := cli.NewApp()
	app.Name = "crictl"
	app.Usage = "client for CRI"
	app.Version = "0.0.1"

	app.Commands = []cli.Command{
		runtimeVersionCommand,
		runtimePodSandboxCommand,
		runtimeContainerCommand,
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "runtime-endpoint",
			Value: "/var/run/dockershim.sock",
			Usage: "CRI server runtime endpoint",
		},
		cli.StringFlag{
			Name:  "image-endpoint",
			Usage: "CRI server image endpoint",
		},
		cli.DurationFlag{
			Name:  "timeout",
			Value: defaultTimeout,
			Usage: "Timeout of connecting to server",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug output",
		},
	}

	if err := app.Run(os.Args); err != nil {
		glog.Fatal(err)
	}
}
