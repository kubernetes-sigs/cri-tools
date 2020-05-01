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
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"google.golang.org/grpc"

	internalapi "k8s.io/cri-api/pkg/apis"
	"k8s.io/kubernetes/pkg/kubelet/remote"
	"k8s.io/kubernetes/pkg/kubelet/util"

	"github.com/kubernetes-sigs/cri-tools/pkg/common"
	"github.com/kubernetes-sigs/cri-tools/pkg/version"
)

const (
	defaultTimeout = 2 * time.Second
)

var (
	// RuntimeEndpoint is CRI server runtime endpoint
	RuntimeEndpoint string
	// ImageEndpoint is CRI server image endpoint, default same as runtime endpoint
	ImageEndpoint string
	// Timeout  of connecting to server (default: 10s)
	Timeout time.Duration
	// Debug enable debug output
	Debug bool
)

func getRuntimeClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	if RuntimeEndpoint == "" {
		return nil, fmt.Errorf("--runtime-endpoint is not set")
	}
	logrus.Debugf("get runtime connection")
	// As dockershim is deprecated as a default CRI server, it still needs to
	// be supported. This allows other default endpoint types to be
	// checked if cannot connect to dockershim
	if strings.Contains(RuntimeEndpoint, "dockershim") {
		return getConnection(defaultRuntimeEndpoints)
	}
	return getConnection([]string{RuntimeEndpoint})
}

func getImageClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	if ImageEndpoint == "" {
		if RuntimeEndpoint == "" {
			return nil, fmt.Errorf("--image-endpoint is not set")
		}
		ImageEndpoint = RuntimeEndpoint
	}
	logrus.Debugf("get image connection")
	// As dockershim is deprecated as a default CRI server, it still needs to
	// be supported. This allows other default endpoint types to be
	// checked if cannot connect to dockershim
	if strings.Contains(ImageEndpoint, "dockershim") {
		return getConnection(defaultRuntimeEndpoints)
	}
	return getConnection([]string{ImageEndpoint})
}

func getConnection(endPoints []string) (*grpc.ClientConn, error) {
	if endPoints == nil || len(endPoints) == 0 {
		return nil, fmt.Errorf("endpoint is not set")
	}
	endPointsLen := len(endPoints)
	if endPointsLen > 1 {
		logrus.Warningf("using default endpoints: %s", endPoints)
	}

	var conn *grpc.ClientConn
	for indx, endPoint := range endPoints {
		if endPointsLen > 1 {
			if strings.Contains(endPoint, "dockershim") {
				logrus.Warningf("connect using default endpoint: %s. Note: Dockershim endpoint is now deprecated.", endPoint)
			} else {
				logrus.Warningf("connect using default endpoint: %s", endPoint)
			}
		} else {
			logrus.Debugf("connect using endpoint: %s", endPoint)
		}
		addr, dialer, err := util.GetAddressAndDialer(endPoint)
		if err != nil {
			if indx == endPointsLen-1 {
				return nil, err
			}
			logrus.Error(err)
			continue
		}

		conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(Timeout), grpc.WithContextDialer(dialer))
		if err != nil {
			errMsg := errors.Wrapf(err, "connect endpoint '%s', make sure you are running as root and the endpoint has been started", endPoint)
			if indx == endPointsLen-1 {
				return nil, errMsg
			}
			logrus.Error(errMsg)
		} else {
			break
		}
	}
	return conn, nil
}

func getRuntimeService(context *cli.Context) (internalapi.RuntimeService, error) {
	return remote.NewRemoteRuntimeService(RuntimeEndpoint, Timeout)
}

func main() {
	app := cli.NewApp()
	app.Name = "crictl"
	app.Usage = "client for CRI"
	app.Version = version.Version

	app.Commands = []*cli.Command{
		runtimeAttachCommand,
		createContainerCommand,
		runtimeExecCommand,
		runtimeVersionCommand,
		listImageCommand,
		containerStatusCommand,
		imageStatusCommand,
		imageFsInfoCommand,
		podStatusCommand,
		logsCommand,
		runtimePortForwardCommand,
		listContainersCommand,
		pullImageCommand,
		runContainerCommand,
		runPodCommand,
		removeContainerCommand,
		removeImageCommand,
		removePodCommand,
		listPodCommand,
		startContainerCommand,
		runtimeStatusCommand,
		stopContainerCommand,
		stopPodCommand,
		updateContainerCommand,
		configCommand,
		statsCommand,
		completionCommand,
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			EnvVars: []string{"CRI_CONFIG_FILE"},
			Value:   defaultConfigPath,
			Usage:   "Location of the client config file. If not specified and the default does not exist, the program's directory is searched as well",
		},
		&cli.StringFlag{
			Name:    "runtime-endpoint",
			Aliases: []string{"r"},
			EnvVars: []string{"CONTAINER_RUNTIME_ENDPOINT"},
			Value:   defaultRuntimeEndpoint,
			Usage:   "Endpoint of CRI container runtime service",
		},
		&cli.StringFlag{
			Name:    "image-endpoint",
			Aliases: []string{"i"},
			EnvVars: []string{"IMAGE_SERVICE_ENDPOINT"},
			Usage:   "Endpoint of CRI image manager service",
		},
		&cli.DurationFlag{
			Name:    "timeout",
			Aliases: []string{"t"},
			Value:   defaultTimeout,
			Usage:   "Timeout of connecting to the server",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
			Usage:   "Enable debug mode",
		},
	}

	app.Before = func(context *cli.Context) (err error) {
		var config *common.ServerConfiguration
		var exePath string

		if exePath, err = os.Executable(); err != nil {
			logrus.Fatal(err)
		}
		if config, err = common.GetServerConfigFromFile(context.String("config"), exePath); err != nil {
			if context.IsSet("config") {
				logrus.Fatal(err)
			}
		}

		if config == nil {
			RuntimeEndpoint = context.String("runtime-endpoint")
			ImageEndpoint = context.String("image-endpoint")
			Timeout = context.Duration("timeout")
			Debug = context.Bool("debug")
		} else {
			// Command line flags overrides config file.
			if context.IsSet("runtime-endpoint") {
				RuntimeEndpoint = context.String("runtime-endpoint")
			} else if config.RuntimeEndpoint != "" {
				RuntimeEndpoint = config.RuntimeEndpoint
			} else {
				RuntimeEndpoint = context.String("runtime-endpoint")
			}
			if context.IsSet("image-endpoint") {
				ImageEndpoint = context.String("image-endpoint")
			} else if config.ImageEndpoint != "" {
				ImageEndpoint = config.ImageEndpoint
			} else {
				ImageEndpoint = context.String("image-endpoint")
			}
			if context.IsSet("timeout") {
				Timeout = context.Duration("timeout")
			} else if config.Timeout != 0 {
				Timeout = time.Duration(config.Timeout) * time.Second
			} else {
				Timeout = context.Duration("timeout")
			}
			if context.IsSet("debug") {
				Debug = context.Bool("debug")
			} else {
				Debug = config.Debug
			}
		}

		if Debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	// sort all flags
	for _, cmd := range app.Commands {
		sort.Sort(cli.FlagsByName(cmd.Flags))
	}
	sort.Sort(cli.FlagsByName(app.Flags))

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
