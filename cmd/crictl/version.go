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

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	remote "k8s.io/cri-client/pkg"
)

var runtimeVersionCommand = &cli.Command{
	Name:  "version",
	Usage: "Display runtime version information",
	Action: func(c *cli.Context) error {
		if c.NArg() != 0 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}
		if err := Version(c.Context, runtimeClient, string(remote.CRIVersionV1)); err != nil {
			return fmt.Errorf("getting the runtime version: %w", err)
		}

		return nil
	},
}

// Version sends a VersionRequest to the server, and parses the returned VersionResponse.
func Version(ctx context.Context, client internalapi.RuntimeService, version string) error {
	request := &pb.VersionRequest{Version: version}
	logrus.Debugf("VersionRequest: %v", request)

	r, err := InterruptableRPC(ctx, func(ctx context.Context) (*pb.VersionResponse, error) {
		return client.Version(ctx, version)
	})
	logrus.Debugf("VersionResponse: %v", r)

	if err != nil {
		return err
	}

	fmt.Println("Version: ", r.GetVersion())
	fmt.Println("RuntimeName: ", r.GetRuntimeName())
	fmt.Println("RuntimeVersion: ", r.GetRuntimeVersion())
	fmt.Println("RuntimeApiVersion: ", r.GetRuntimeApiVersion())

	return nil
}
