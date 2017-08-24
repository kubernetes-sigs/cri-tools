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
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/ghodss/yaml"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

var runtimeClient pb.RuntimeServiceClient
var imageClient pb.ImageServiceClient
var conn *grpc.ClientConn

type listOptions struct {
	// id of container or sandbox
	id string
	// podID of container
	podID string
	// state of the sandbox
	state string
	// show verbose info for the sandbox
	verbose bool
	// labels are selectors for the sandbox
	labels map[string]string
	// quiet is for listing just container/sandbox/image IDs
	quiet bool
	// output format
	output string
}

type execOptions struct {
	// id of container
	id string
	// timeout to stop command
	timeout int64
	// Whether to exec a command in a tty
	tty bool
	// Whether to stream stdin
	stdin bool
	// Command to exec
	cmd []string
}
type attachOptions struct {
	// id of container
	id string
	// Whether the stdin is TTY
	tty bool
	// Whether pass Stdin to container
	stdin bool
}

type portforwardOptions struct {
	// id of sandbox
	id string
	// ports to forward
	ports []string
}

func getSortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}
func loadContainerConfig(path string) (*pb.ContainerConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pb.ContainerConfig
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadPodSandboxConfig(path string) (*pb.PodSandboxConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pb.PodSandboxConfig
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func openFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config at %s not found", path)
		}
		return nil, err
	}
	return f, nil
}

func getRuntimeClient(context *cli.Context) error {
	// Set up a connection to the server.
	var err error
	conn, err = getRuntimeClientConnection(context)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	runtimeClient = pb.NewRuntimeServiceClient(conn)
	return nil
}

func getImageClient(context *cli.Context) error {
	// Set up a connection to the server.
	var err error
	conn, err = getImageClientConnection(context)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	imageClient = pb.NewImageServiceClient(conn)
	return nil
}

func closeConnection(context *cli.Context) error {
	if conn == nil {
		return nil
	}

	return conn.Close()
}

func outputJson(v interface{}) error {
	marshaledJson, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(marshaledJson))
	return nil
}

func outputYaml(v interface{}) error {
	marshaledyaml, err := yaml.Marshal(v)
	if err != nil {
		return err
	}

	fmt.Println(string(marshaledyaml))
	return nil
}
