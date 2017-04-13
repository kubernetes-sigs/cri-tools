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

	"github.com/urfave/cli"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

var runtimeStatusCommand = cli.Command{
	Name:  "status",
	Usage: "get the status of runtime",
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
	r, err := client.Status(context.Background(), &pb.StatusRequest{})
	if err != nil {
		return err
	}
	fmt.Println("Runtime Conditions:")
	var runtimeCondition *pb.RuntimeCondition
	for _, runtimeCondition = range r.GetStatus().GetConditions() {
		fmt.Println("Type: ", runtimeCondition.Type)
		fmt.Println("Status: ", runtimeCondition.Status)
		fmt.Println("Reason: ", runtimeCondition.Reason)
		fmt.Println("Message: ", runtimeCondition.Message)
		fmt.Printf("\n")
	}
	return nil
}
