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
	"os"
	"text/tabwriter"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

var runtimeStatusCommand = cli.Command{
	Name:      "status",
	Usage:     "Display status of the container runtime",
	ArgsUsage: "",
	Action: func(context *cli.Context) error {
		err := Status(runtimeClient)
		if err != nil {
			return fmt.Errorf("Getting status of runtime failed: %v", err)
		}
		return nil
	},
	Before: getRuntimeClient,
	After:  closeConnection,
}

// Status sends a StatusRequest to the server, and parses the returned StatusResponse.
func Status(client pb.RuntimeServiceClient) error {
	request := &pb.StatusRequest{}
	logrus.Debugf("StatusRequest: %v", request)
	r, err := client.Status(context.Background(), request)
	logrus.Debugf("StatusResponse: %v", r)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprintln(w, "CONDITION\tSTATUS\tREASON\tMESSAGE")
	for _, c := range r.GetStatus().GetConditions() {
		fmt.Fprintf(w, "%s\t%v\t%s\t%s\n", c.Type, c.Status, c.Reason, c.Message)
	}
	w.Flush()
	return nil
}
