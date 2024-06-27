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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/kubelet/pkg/types"

	"github.com/docker/go-units"
	godigest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type containerByCreated []*pb.Container

func (a containerByCreated) Len() int      { return len(a) }
func (a containerByCreated) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a containerByCreated) Less(i, j int) bool {
	return a[i].CreatedAt > a[j].CreatedAt
}

type createOptions struct {
	// podID of the container
	podID string

	// the config and pod options
	*runOptions
}

type runOptions struct {
	// configPath is path to the config for container
	configPath string

	// podConfig is path to the config for sandbox
	podConfig string

	// the create timeout
	timeout time.Duration

	// the image pull options
	*pullOptions
}

type pullOptions struct {
	// pull the image on container creation; overrides default
	withPull bool

	// creds is string in the format `USERNAME[:PASSWORD]` for accessing the
	// registry during image pull
	creds string

	// auth is a base64 encoded 'USERNAME[:PASSWORD]' string used for
	// authentication with a registry when pulling an image
	auth string

	// Username to use for accessing the registry
	// password will be requested on the command line
	username string

	// timeout is the maximum time used for the image pull
	timeout time.Duration
}

var createPullFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:  "no-pull",
		Usage: "Do not pull the image on container creation (overrides pull-image-on-create=true in config)",
	},
	&cli.BoolFlag{
		Name:  "with-pull",
		Usage: "Pull the image on container creation (overrides pull-image-on-create=false in config)",
	},
	&cli.StringFlag{
		Name:  "creds",
		Value: "",
		Usage: "Use `USERNAME[:PASSWORD]` for accessing the registry",
	},
	&cli.StringFlag{
		Name:  "auth",
		Value: "",
		Usage: "Use `AUTH_STRING` for accessing the registry. AUTH_STRING is a base64 encoded 'USERNAME[:PASSWORD]'",
	},
	&cli.StringFlag{
		Name:  "username",
		Value: "",
		Usage: "Use `USERNAME` for accessing the registry. The password will be requested on the command line",
	},
	&cli.DurationFlag{
		Name:    "cancel-timeout",
		Aliases: []string{"T"},
		Usage:   "Seconds to wait for a container create request to complete before cancelling the request",
	},
	&cli.DurationFlag{
		Name:    "pull-timeout",
		Aliases: []string{"pt"},
		Usage:   "Maximum time to be used for pulling the image, disabled if set to 0s",
		EnvVars: []string{"CRICTL_PULL_TIMEOUT"},
	},
}

var runPullFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:  "no-pull",
		Usage: "Do not pull the image (overrides disable-pull-on-run=false in config)",
	},
	&cli.BoolFlag{
		Name:  "with-pull",
		Usage: "Pull the image (overrides disable-pull-on-run=true in config)",
	},
	&cli.StringFlag{
		Name:  "creds",
		Value: "",
		Usage: "Use `USERNAME[:PASSWORD]` for accessing the registry",
	},
	&cli.StringFlag{
		Name:  "auth",
		Value: "",
		Usage: "Use `AUTH_STRING` for accessing the registry. AUTH_STRING is a base64 encoded 'USERNAME[:PASSWORD]'",
	},
	&cli.StringFlag{
		Name:  "username",
		Value: "",
		Usage: "Use `USERNAME` for accessing the registry. password will be requested",
	},
	&cli.StringFlag{
		Name:    "runtime",
		Aliases: []string{"r"},
		Usage:   "Runtime handler to use. Available options are defined by the container runtime.",
	},
	&cli.DurationFlag{
		Name:    "cancel-timeout",
		Aliases: []string{"T"},
		Usage:   "Seconds to wait for a container create request before cancelling the request",
	},
	&cli.DurationFlag{
		Name:    "pull-timeout",
		Aliases: []string{"pt"},
		Usage:   "Maximum time to be used for pulling the image, disabled if set to 0s",
		EnvVars: []string{"CRICTL_PULL_TIMEOUT"},
	},
}

var createContainerCommand = &cli.Command{
	Name:      "create",
	Usage:     "Create a new container",
	ArgsUsage: "POD container-config.[json|yaml] pod-config.[json|yaml]",
	Flags:     createPullFlags,

	Action: func(c *cli.Context) (err error) {
		if c.Args().Len() != 3 {
			return cli.ShowSubcommandHelp(c)
		}
		if c.Bool("no-pull") == true && c.Bool("with-pull") == true {
			return errors.New("confict: no-pull and with-pull are both set")
		}

		withPull := (!c.Bool("no-pull") && PullImageOnCreate) || c.Bool("with-pull")

		var imageClient internalapi.ImageManagerService
		if withPull {
			imageClient, err = getImageService(c)
			if err != nil {
				return err
			}
		}

		opts := createOptions{
			podID: c.Args().Get(0),
			runOptions: &runOptions{
				configPath: c.Args().Get(1),
				podConfig:  c.Args().Get(2),
				pullOptions: &pullOptions{
					withPull: withPull,
					creds:    c.String("creds"),
					auth:     c.String("auth"),
					username: c.String("username"),
					timeout:  c.Duration("pull-timeout"),
				},
				timeout: c.Duration("cancel-timeout"),
			},
		}

		runtimeClient, err := getRuntimeService(c, opts.timeout)
		if err != nil {
			return err
		}

		ctrID, err := CreateContainer(imageClient, runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("creating container: %w", err)
		}
		fmt.Println(ctrID)
		return nil
	},
}

var startContainerCommand = &cli.Command{
	Name:      "start",
	Usage:     "Start one or more created containers",
	ArgsUsage: "CONTAINER-ID [CONTAINER-ID...]",
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return errors.New("ID cannot be empty")
		}
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		for i := 0; i < c.NArg(); i++ {
			containerID := c.Args().Get(i)
			if err := StartContainer(runtimeClient, containerID); err != nil {
				return fmt.Errorf("starting the container %q: %w", containerID, err)
			}
		}
		return nil
	},
}

var updateContainerCommand = &cli.Command{
	Name:      "update",
	Usage:     "Update one or more running containers",
	ArgsUsage: "CONTAINER-ID [CONTAINER-ID...]",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:  "cpu-count",
			Usage: "(Windows only) Number of CPUs available to the container",
		},
		&cli.Int64Flag{
			Name:  "cpu-maximum",
			Usage: "(Windows only) Portion of CPU cycles specified as a percentage * 100",
		},
		&cli.Int64Flag{
			Name:  "cpu-period",
			Usage: "CPU CFS period to be used for hardcapping (in usecs). 0 to use system default",
		},
		&cli.Int64Flag{
			Name:  "cpu-quota",
			Usage: "CPU CFS hardcap limit (in usecs). Allowed cpu time in a given period",
		},
		&cli.Int64Flag{
			Name:  "cpu-share",
			Usage: "CPU shares (relative weight vs. other containers)",
		},
		&cli.Int64Flag{
			Name:  "memory",
			Usage: "Memory limit (in bytes)",
		},
		&cli.StringFlag{
			Name:  "cpuset-cpus",
			Usage: "CPU(s) to use",
		},
		&cli.StringFlag{
			Name:  "cpuset-mems",
			Usage: "Memory node(s) to use",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return errors.New("ID cannot be empty")
		}
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		options := updateOptions{
			CPUCount:           c.Int64("cpu-count"),
			CPUMaximum:         c.Int64("cpu-maximum"),
			CPUPeriod:          c.Int64("cpu-period"),
			CPUQuota:           c.Int64("cpu-quota"),
			CPUShares:          c.Int64("cpu-share"),
			CpusetCpus:         c.String("cpuset-cpus"),
			CpusetMems:         c.String("cpuset-mems"),
			MemoryLimitInBytes: c.Int64("memory"),
		}

		for i := 0; i < c.NArg(); i++ {
			containerID := c.Args().Get(i)
			if err := UpdateContainerResources(runtimeClient, containerID, options); err != nil {
				return fmt.Errorf("updating container resources for %q: %w", containerID, err)
			}
		}
		return nil
	},
}

var stopContainerCommand = &cli.Command{
	Name:                   "stop",
	Usage:                  "Stop one or more running containers",
	ArgsUsage:              "CONTAINER-ID [CONTAINER-ID...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:    "timeout",
			Aliases: []string{"t"},
			Usage:   "Seconds to wait to kill the container after a graceful stop is requested",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return errors.New("ID cannot be empty")
		}
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		for i := 0; i < c.NArg(); i++ {
			containerID := c.Args().Get(i)
			if err := StopContainer(runtimeClient, containerID, c.Int64("timeout")); err != nil {
				return fmt.Errorf("stopping the container %q: %w", containerID, err)
			}
		}
		return nil
	},
}

var removeContainerCommand = &cli.Command{
	Name:                   "rm",
	Usage:                  "Remove one or more containers",
	ArgsUsage:              "CONTAINER-ID [CONTAINER-ID...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Force removal of the container, disregarding if running",
		},
		&cli.BoolFlag{
			Name:    "keep-logs",
			Aliases: []string{"k"},
			Usage:   "Preserve the container log file and its rotations",
		},
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Remove all containers",
		},
	},
	Action: func(ctx *cli.Context) error {
		runtimeClient, err := getRuntimeService(ctx, 0)
		if err != nil {
			return err
		}

		ids := ctx.Args().Slice()
		if ctx.Bool("all") {
			r, err := InterruptableRPC(nil, func(ctx context.Context) ([]*pb.Container, error) {
				return runtimeClient.ListContainers(ctx, nil)
			})
			if err != nil {
				return err
			}
			ids = nil
			for _, ctr := range r {
				ids = append(ids, ctr.GetId())
			}
		}

		if len(ids) == 0 {
			if ctx.Bool("all") {
				logrus.Info("No containers to remove")
				return nil
			}
			return cli.ShowSubcommandHelp(ctx)
		}

		errored := false
		for _, id := range ids {
			resp, err := InterruptableRPC(nil, func(ctx context.Context) (*pb.ContainerStatusResponse, error) {
				return runtimeClient.ContainerStatus(ctx, id, false)
			})
			if err != nil {
				logrus.Error(err)
				errored = true
				continue
			}
			if resp.GetStatus().GetState() == pb.ContainerState_CONTAINER_RUNNING {
				if ctx.Bool("force") {
					if err := StopContainer(runtimeClient, id, 0); err != nil {
						logrus.Errorf("stopping the container %q failed: %v", id, err)
						errored = true
						continue
					}
				} else {
					logrus.Errorf("container %q is running, please stop it first", id)
					errored = true
					continue
				}
			}

			err = RemoveContainer(runtimeClient, id)
			if err != nil {
				logrus.Errorf("removing container %q failed: %v", id, err)
				errored = true
				continue
			} else if !ctx.Bool("keep-logs") {
				logPath := resp.GetStatus().GetLogPath()
				if logPath != "" {
					logRotations, err := filepath.Glob(logPath + ".*")
					if err != nil {
						logRotations = []string{}
					}
					logRotations = append(logRotations, logPath)

					for _, logFile := range logRotations {
						err = os.Remove(logFile)
						if err != nil {
							logrus.Errorf("removing log file %s for container %q failed: %v", logFile, id, err)
						}
					}
				}
			}
		}

		if errored {
			return errors.New("unable to remove container(s)")
		}
		return nil
	},
}

var containerStatusCommand = &cli.Command{
	Name:      "inspect",
	Usage:     "Display the status of one or more containers",
	ArgsUsage: "CONTAINER-ID [CONTAINER-ID...]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|go-template|table",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Do not show verbose information",
		},
		&cli.StringFlag{
			Name:  "template",
			Usage: "The template string is only used when output is go-template; The Template format is golang template",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return errors.New("ID cannot be empty")
		}
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		for i := 0; i < c.NArg(); i++ {
			containerID := c.Args().Get(i)
			if err := ContainerStatus(runtimeClient, containerID, c.String("output"), c.String("template"), c.Bool("quiet")); err != nil {
				return fmt.Errorf("getting the status of the container %q: %w", containerID, err)
			}
		}
		return nil
	},
}

var listContainersCommand = &cli.Command{
	Name:                   "ps",
	Usage:                  "List containers",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Show verbose information for containers",
		},
		&cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "Filter by container id",
		},
		&cli.StringFlag{
			Name:  "name",
			Value: "",
			Usage: "filter by container name regular expression pattern",
		},
		&cli.StringFlag{
			Name:    "pod",
			Aliases: []string{"p"},
			Value:   "",
			Usage:   "Filter by pod id",
		},
		&cli.StringFlag{
			Name:  "image",
			Value: "",
			Usage: "Filter by container image",
		},
		&cli.StringFlag{
			Name:    "state",
			Aliases: []string{"s"},
			Value:   "",
			Usage:   "Filter by container state",
		},
		&cli.StringSliceFlag{
			Name:  "label",
			Usage: "Filter by key=value label",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Only display container IDs",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|table",
			Value:   "table",
		},
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Show all containers",
		},
		&cli.BoolFlag{
			Name:    "latest",
			Aliases: []string{"l"},
			Usage:   "Show the most recently created container (includes all states)",
		},
		&cli.IntFlag{
			Name:    "last",
			Aliases: []string{"n"},
			Usage:   "Show last n recently created containers (includes all states). Set 0 for unlimited.",
		},
		&cli.BoolFlag{
			Name:  "no-trunc",
			Usage: "Show output without truncating the ID",
		},
		&cli.BoolFlag{
			Name:    "resolve-image-path",
			Aliases: []string{"r"},
			Usage:   "Show image path instead of image id",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() != 0 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		imageClient, err := getImageService(c)
		if err != nil {
			return err
		}

		opts := listOptions{
			id:               c.String("id"),
			podID:            c.String("pod"),
			state:            c.String("state"),
			verbose:          c.Bool("verbose"),
			quiet:            c.Bool("quiet"),
			output:           c.String("output"),
			all:              c.Bool("all"),
			nameRegexp:       c.String("name"),
			latest:           c.Bool("latest"),
			last:             c.Int("last"),
			noTrunc:          c.Bool("no-trunc"),
			image:            c.String("image"),
			resolveImagePath: c.Bool("resolve-image-path"),
		}
		opts.labels, err = parseLabelStringSlice(c.StringSlice("label"))
		if err != nil {
			return err
		}

		if err = ListContainers(runtimeClient, imageClient, opts); err != nil {
			return fmt.Errorf("listing containers: %w", err)
		}
		return nil
	},
}

var runContainerCommand = &cli.Command{
	Name:      "run",
	Usage:     "Run a new container inside a sandbox",
	ArgsUsage: "container-config.[json|yaml] pod-config.[json|yaml]",
	Flags:     runPullFlags,

	Action: func(c *cli.Context) (err error) {
		if c.Args().Len() != 2 {
			return cli.ShowSubcommandHelp(c)
		}
		if c.Bool("no-pull") == true && c.Bool("with-pull") == true {
			return errors.New("confict: no-pull and with-pull are both set")
		}

		withPull := (!DisablePullOnRun && !c.Bool("no-pull")) || c.Bool("with-pull")

		var imageClient internalapi.ImageManagerService
		if withPull {
			imageClient, err = getImageService(c)
			if err != nil {
				return err
			}
		}

		opts := runOptions{
			configPath: c.Args().Get(0),
			podConfig:  c.Args().Get(1),
			pullOptions: &pullOptions{
				withPull: withPull,
				creds:    c.String("creds"),
				auth:     c.String("auth"),
				username: c.String("username"),
				timeout:  c.Duration("pull-timeout"),
			},
			timeout: c.Duration("cancel-timeout"),
		}

		runtimeClient, err := getRuntimeService(c, opts.timeout)
		if err != nil {
			return err
		}

		if err = RunContainer(imageClient, runtimeClient, opts, c.String("runtime")); err != nil {
			return fmt.Errorf("running container: %w", err)
		}
		return nil
	},
}

var checkpointContainerCommand = &cli.Command{
	Name:                   "checkpoint",
	Usage:                  "Checkpoint one or more running containers",
	ArgsUsage:              "CONTAINER-ID [CONTAINER-ID...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "export",
			Aliases: []string{"e"},
			Usage:   "Specify the name of the tar archive (/path/to/checkpoint.tar) used to export the checkpoint image.",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() == 0 {
			return errors.New("ID cannot be empty")
		}
		if c.String("export") == "" {
			return errors.New(
				"Cannot checkpoint a container without specifying the checkpoint destination. " +
					"Use --export=/path/to/checkpoint.tar",
			)
		}
		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		for i := 0; i < c.NArg(); i++ {
			containerID := c.Args().Get(i)
			err := CheckpointContainer(
				runtimeClient,
				containerID,
				c.String("export"),
			)
			if err != nil {
				return fmt.Errorf("checkpointing the container %q failed: %w", containerID, err)
			}
		}
		return nil
	},
}

// RunContainer starts a container in the provided sandbox
func RunContainer(
	iClient internalapi.ImageManagerService,
	rClient internalapi.RuntimeService,
	opts runOptions,
	runtime string,
) error {
	// Create the pod
	podSandboxConfig, err := loadPodSandboxConfig(opts.podConfig)
	if err != nil {
		return fmt.Errorf("load podSandboxConfig: %w", err)
	}
	// set the timeout for the RunPodSandbox request to 0, because the
	// timeout option is documented as being for container creation.
	podID, err := RunPodSandbox(rClient, podSandboxConfig, runtime)
	if err != nil {
		return fmt.Errorf("run pod sandbox: %w", err)
	}

	// Create the container
	containerOptions := createOptions{podID, &opts}
	ctrID, err := CreateContainer(iClient, rClient, containerOptions)
	if err != nil {
		return fmt.Errorf("creating container failed: %w", err)
	}

	// Start the container
	err = StartContainer(rClient, ctrID)
	if err != nil {
		return fmt.Errorf("starting the container %q: %w", ctrID, err)
	}
	return nil
}

// CreateContainer sends a CreateContainerRequest to the server, and parses
// the returned CreateContainerResponse.
func CreateContainer(
	iClient internalapi.ImageManagerService,
	rClient internalapi.RuntimeService,
	opts createOptions,
) (string, error) {

	config, err := loadContainerConfig(opts.configPath)
	if err != nil {
		return "", err
	}
	var podConfig *pb.PodSandboxConfig
	if opts.podConfig != "" {
		podConfig, err = loadPodSandboxConfig(opts.podConfig)
		if err != nil {
			return "", err
		}
	}

	image := config.GetImage().GetImage()
	if config.Image.UserSpecifiedImage == "" {
		config.Image.UserSpecifiedImage = image
	}

	// When there is a with-pull request or the image default mode is to
	// pull-image-on-create(true) and no-pull was not set we pull the image when
	// they ask for a create as a helper on the cli to reduce extra steps. As a
	// reminder if the image is already in cache only the manifest will be pulled
	// down to verify.
	if opts.withPull {
		auth, err := getAuth(opts.creds, opts.auth, opts.username)
		if err != nil {
			return "", err
		}

		// Try to pull the image before container creation
		ann := config.GetImage().GetAnnotations()
		if _, err := PullImageWithSandbox(iClient, image, auth, podConfig, ann, opts.pullOptions.timeout); err != nil {
			return "", err
		}
	}

	request := &pb.CreateContainerRequest{
		PodSandboxId:  opts.podID,
		Config:        config,
		SandboxConfig: podConfig,
	}
	logrus.Debugf("CreateContainerRequest: %v", request)
	r, err := InterruptableRPC(nil, func(ctx context.Context) (string, error) {
		return rClient.CreateContainer(ctx, opts.podID, config, podConfig)
	})
	logrus.Debugf("CreateContainerResponse: %v", r)
	if err != nil {
		return "", err
	}
	return r, nil
}

// StartContainer sends a StartContainerRequest to the server, and parses
// the returned StartContainerResponse.
func StartContainer(client internalapi.RuntimeService, id string) error {
	if id == "" {
		return errors.New("ID cannot be empty")
	}
	if _, err := InterruptableRPC(nil, func(ctx context.Context) (any, error) {
		return nil, client.StartContainer(ctx, id)
	}); err != nil {
		return err
	}
	fmt.Println(id)
	return nil
}

type updateOptions struct {
	// (Windows only) Number of CPUs available to the container.
	CPUCount int64
	// (Windows only) Portion of CPU cycles specified as a percentage * 100.
	CPUMaximum int64
	// CPU CFS (Completely Fair Scheduler) period. Default: 0 (not specified).
	CPUPeriod int64
	// CPU CFS (Completely Fair Scheduler) quota. Default: 0 (not specified).
	CPUQuota int64
	// CPU shares (relative weight vs. other containers). Default: 0 (not specified).
	CPUShares int64
	// Memory limit in bytes. Default: 0 (not specified).
	MemoryLimitInBytes int64
	// OOMScoreAdj adjusts the oom-killer score. Default: 0 (not specified).
	OomScoreAdj int64
	// CpusetCpus constrains the allowed set of logical CPUs. Default: "" (not specified).
	CpusetCpus string
	// CpusetMems constrains the allowed set of memory nodes. Default: "" (not specified).
	CpusetMems string
}

// UpdateContainerResources sends an UpdateContainerResourcesRequest to the server, and parses
// the returned UpdateContainerResourcesResponse.
func UpdateContainerResources(client internalapi.RuntimeService, id string, opts updateOptions) error {
	if id == "" {
		return errors.New("ID cannot be empty")
	}
	request := &pb.UpdateContainerResourcesRequest{
		ContainerId: id,
	}
	if goruntime.GOOS != "windows" {
		request.Linux = &pb.LinuxContainerResources{
			CpuPeriod:          opts.CPUPeriod,
			CpuQuota:           opts.CPUQuota,
			CpuShares:          opts.CPUShares,
			CpusetCpus:         opts.CpusetCpus,
			CpusetMems:         opts.CpusetMems,
			MemoryLimitInBytes: opts.MemoryLimitInBytes,
			OomScoreAdj:        opts.OomScoreAdj,
		}
	} else {
		request.Windows = &pb.WindowsContainerResources{
			CpuCount:           opts.CPUCount,
			CpuMaximum:         opts.CPUMaximum,
			CpuShares:          opts.CPUShares,
			MemoryLimitInBytes: opts.MemoryLimitInBytes,
		}
	}
	logrus.Debugf("UpdateContainerResourcesRequest: %v", request)
	resources := &pb.ContainerResources{Linux: request.Linux}
	if _, err := InterruptableRPC(nil, func(ctx context.Context) (any, error) {
		return nil, client.UpdateContainerResources(ctx, id, resources)
	}); err != nil {
		return err
	}
	fmt.Println(id)
	return nil
}

// StopContainer sends a StopContainerRequest to the server, and parses
// the returned StopContainerResponse.
func StopContainer(client internalapi.RuntimeService, id string, timeout int64) error {
	if id == "" {
		return errors.New("ID cannot be empty")
	}
	if _, err := InterruptableRPC(nil, func(ctx context.Context) (any, error) {
		return nil, client.StopContainer(ctx, id, timeout)
	}); err != nil {
		return err
	}
	fmt.Println(id)
	return nil
}

// CheckpointContainer sends a CheckpointContainerRequest to the server
func CheckpointContainer(
	rClient internalapi.RuntimeService,
	ID string,
	export string,
) error {
	if ID == "" {
		return errors.New("ID cannot be empty")
	}
	request := &pb.CheckpointContainerRequest{
		ContainerId: ID,
		Location:    export,
	}
	logrus.Debugf("CheckpointContainerRequest: %v", request)
	_, err := InterruptableRPC(nil, func(ctx context.Context) (*pb.ImageFsInfoResponse, error) {
		return nil, rClient.CheckpointContainer(ctx, request)
	})
	if err != nil {
		return err
	}
	fmt.Println(ID)
	return nil
}

// RemoveContainer sends a RemoveContainerRequest to the server, and parses
// the returned RemoveContainerResponse.
func RemoveContainer(client internalapi.RuntimeService, id string) error {
	if id == "" {
		return errors.New("ID cannot be empty")
	}
	if _, err := InterruptableRPC(nil, func(ctx context.Context) (any, error) {
		return nil, client.RemoveContainer(ctx, id)
	}); err != nil {
		return err
	}
	fmt.Println(id)
	return nil
}

// marshalContainerStatus converts container status into string and converts
// the timestamps into readable format.
func marshalContainerStatus(cs *pb.ContainerStatus) (string, error) {
	statusStr, err := protobufObjectToJSON(cs)
	if err != nil {
		return "", err
	}
	jsonMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(statusStr), &jsonMap)
	if err != nil {
		return "", err
	}

	jsonMap["createdAt"] = time.Unix(0, cs.CreatedAt).Format(time.RFC3339Nano)
	var startedAt, finishedAt time.Time
	if cs.State != pb.ContainerState_CONTAINER_CREATED {
		// If container is not in the created state, we have tried and
		// started the container. Set the startedAt.
		startedAt = time.Unix(0, cs.StartedAt)
	}
	if cs.State == pb.ContainerState_CONTAINER_EXITED ||
		(cs.State == pb.ContainerState_CONTAINER_UNKNOWN && cs.FinishedAt > 0) {
		// If container is in the exit state, set the finishedAt.
		// Or if container is in the unknown state and FinishedAt > 0, set the finishedAt
		finishedAt = time.Unix(0, cs.FinishedAt)
	}
	jsonMap["startedAt"] = startedAt.Format(time.RFC3339Nano)
	jsonMap["finishedAt"] = finishedAt.Format(time.RFC3339Nano)
	return marshalMapInOrder(jsonMap, *cs)
}

// ContainerStatus sends a ContainerStatusRequest to the server, and parses
// the returned ContainerStatusResponse.
func ContainerStatus(client internalapi.RuntimeService, id, output string, tmplStr string, quiet bool) error {
	verbose := !(quiet)
	if output == "" { // default to json output
		output = "json"
	}
	if id == "" {
		return errors.New("ID cannot be empty")
	}
	request := &pb.ContainerStatusRequest{
		ContainerId: id,
		Verbose:     verbose,
	}
	logrus.Debugf("ContainerStatusRequest: %v", request)
	r, err := InterruptableRPC(nil, func(ctx context.Context) (*pb.ContainerStatusResponse, error) {
		return client.ContainerStatus(ctx, id, verbose)
	})
	logrus.Debugf("ContainerStatusResponse: %v", r)
	if err != nil {
		return err
	}

	status, err := marshalContainerStatus(r.Status)
	if err != nil {
		return err
	}

	switch output {
	case "json", "yaml", "go-template":
		return outputStatusInfo(status, "", r.Info, output, tmplStr)
	case "table": // table output is after this switch block
	default:
		return fmt.Errorf("output option cannot be %s", output)
	}

	// output in table format
	fmt.Printf("ID: %s\n", r.Status.Id)
	if r.Status.Metadata != nil {
		if r.Status.Metadata.Name != "" {
			fmt.Printf("Name: %s\n", r.Status.Metadata.Name)
		}
		if r.Status.Metadata.Attempt != 0 {
			fmt.Printf("Attempt: %v\n", r.Status.Metadata.Attempt)
		}
	}
	fmt.Printf("State: %s\n", r.Status.State)
	ctm := time.Unix(0, r.Status.CreatedAt)
	fmt.Printf("Created: %v\n", units.HumanDuration(time.Now().UTC().Sub(ctm))+" ago")
	if r.Status.State != pb.ContainerState_CONTAINER_CREATED {
		stm := time.Unix(0, r.Status.StartedAt)
		fmt.Printf("Started: %v\n", units.HumanDuration(time.Now().UTC().Sub(stm))+" ago")
	}
	if r.Status.State == pb.ContainerState_CONTAINER_EXITED {
		if r.Status.FinishedAt > 0 {
			ftm := time.Unix(0, r.Status.FinishedAt)
			fmt.Printf("Finished: %v\n", units.HumanDuration(time.Now().UTC().Sub(ftm))+" ago")
		}
		fmt.Printf("Exit Code: %v\n", r.Status.ExitCode)
	}
	if r.Status.Labels != nil {
		fmt.Println("Labels:")
		for _, k := range getSortedKeys(r.Status.Labels) {
			fmt.Printf("\t%s -> %s\n", k, r.Status.Labels[k])
		}
	}
	if r.Status.Annotations != nil {
		fmt.Println("Annotations:")
		for _, k := range getSortedKeys(r.Status.Annotations) {
			fmt.Printf("\t%s -> %s\n", k, r.Status.Annotations[k])
		}
	}
	if verbose {
		fmt.Printf("Info: %v\n", r.GetInfo())
	}

	return nil
}

// ListContainers sends a ListContainerRequest to the server, and parses
// the returned ListContainerResponse.
func ListContainers(runtimeClient internalapi.RuntimeService, imageClient internalapi.ImageManagerService, opts listOptions) error {
	filter := &pb.ContainerFilter{}
	if opts.id != "" {
		filter.Id = opts.id
	}
	if opts.podID != "" {
		filter.PodSandboxId = opts.podID
	}
	st := &pb.ContainerStateValue{}
	if !opts.all && opts.state == "" {
		st.State = pb.ContainerState_CONTAINER_RUNNING
		filter.State = st
	}
	if opts.state != "" {
		st.State = pb.ContainerState_CONTAINER_UNKNOWN
		switch strings.ToLower(opts.state) {
		case "created":
			st.State = pb.ContainerState_CONTAINER_CREATED
			filter.State = st
		case "running":
			st.State = pb.ContainerState_CONTAINER_RUNNING
			filter.State = st
		case "exited":
			st.State = pb.ContainerState_CONTAINER_EXITED
			filter.State = st
		case "unknown":
			st.State = pb.ContainerState_CONTAINER_UNKNOWN
			filter.State = st
		default:
			log.Fatalf("--state should be one of created, running, exited or unknown")
		}
	}
	if opts.latest || opts.last > 0 {
		// Do not filter by state if latest/last is specified.
		filter.State = nil
	}
	if opts.labels != nil {
		filter.LabelSelector = opts.labels
	}
	r, err := InterruptableRPC(nil, func(ctx context.Context) ([]*pb.Container, error) {
		return runtimeClient.ListContainers(ctx, filter)
	})
	logrus.Debugf("ListContainerResponse: %v", r)
	if err != nil {
		return err
	}
	r = getContainersList(r, opts)

	switch opts.output {
	case "json":
		return outputProtobufObjAsJSON(&pb.ListContainersResponse{Containers: r})
	case "yaml":
		return outputProtobufObjAsYAML(&pb.ListContainersResponse{Containers: r})
	case "table":
	// continue; output will be generated after the switch block ends.
	default:
		return fmt.Errorf("unsupported output format %q", opts.output)
	}

	display := newTableDisplay(20, 1, 3, ' ', 0)
	if !opts.verbose && !opts.quiet {
		display.AddRow([]string{columnContainer, columnImage, columnCreated, columnState, columnName, columnAttempt, columnPodID, columnPodname})
	}
	for _, c := range r {
		if match, err := matchesImage(imageClient, opts.image, c.GetImage().GetImage()); err != nil {
			return fmt.Errorf("check image match: %w", err)
		} else if !match {
			continue
		}
		if opts.quiet {
			fmt.Printf("%s\n", c.Id)
			continue
		}

		createdAt := time.Unix(0, c.CreatedAt)
		ctm := units.HumanDuration(time.Now().UTC().Sub(createdAt)) + " ago"
		if !opts.verbose {
			id := c.Id
			image := c.Image.Image
			podID := c.PodSandboxId
			if !opts.noTrunc {
				id = getTruncatedID(id, "")
				podID = getTruncatedID(podID, "")
				// Now c.Image.Image is imageID in kubelet.
				if digest, err := godigest.Parse(image); err == nil {
					image = getTruncatedID(digest.String(), string(digest.Algorithm())+":")
				}
			}
			if opts.resolveImagePath {
				orig, err := getRepoImage(imageClient, image)
				if err != nil {
					return fmt.Errorf("failed to fetch repo image %v", err)
				}
				image = orig
			}
			podName := getPodNameFromLabels(c.Labels)
			display.AddRow([]string{id, image, ctm, convertContainerState(c.State), c.Metadata.Name,
				strconv.FormatUint(uint64(c.Metadata.Attempt), 10), podID, podName})
			continue
		}

		fmt.Printf("ID: %s\n", c.Id)
		fmt.Printf("PodID: %s\n", c.PodSandboxId)
		if c.Metadata != nil {
			if c.Metadata.Name != "" {
				fmt.Printf("Name: %s\n", c.Metadata.Name)
			}
			fmt.Printf("Attempt: %v\n", c.Metadata.Attempt)
		}
		fmt.Printf("State: %s\n", convertContainerState(c.State))
		if c.Image != nil {
			fmt.Printf("Image: %s\n", c.Image.Image)
		}
		fmt.Printf("Created: %v\n", ctm)
		if c.Labels != nil {
			fmt.Println("Labels:")
			for _, k := range getSortedKeys(c.Labels) {
				fmt.Printf("\t%s -> %s\n", k, c.Labels[k])
			}
		}
		if c.Annotations != nil {
			fmt.Println("Annotations:")
			for _, k := range getSortedKeys(c.Annotations) {
				fmt.Printf("\t%s -> %s\n", k, c.Annotations[k])
			}
		}
		fmt.Println()
	}

	display.Flush()
	return nil
}

func convertContainerState(state pb.ContainerState) string {
	switch state {
	case pb.ContainerState_CONTAINER_CREATED:
		return "Created"
	case pb.ContainerState_CONTAINER_RUNNING:
		return "Running"
	case pb.ContainerState_CONTAINER_EXITED:
		return "Exited"
	case pb.ContainerState_CONTAINER_UNKNOWN:
		return "Unknown"
	default:
		log.Fatalf("Unknown container state %q", state)
		return ""
	}
}

func getPodNameFromLabels(label map[string]string) string {
	podName, ok := label[types.KubernetesPodNameLabel]
	if ok {
		return podName
	}
	return "unknown"
}

func getContainersList(containersList []*pb.Container, opts listOptions) []*pb.Container {
	filtered := []*pb.Container{}
	for _, c := range containersList {
		// Filter by pod name/namespace regular expressions.
		if matchesRegex(opts.nameRegexp, c.Metadata.Name) {
			filtered = append(filtered, c)
		}
	}

	sort.Sort(containerByCreated(filtered))
	n := len(filtered)
	if opts.latest {
		n = 1
	}
	if opts.last > 0 {
		n = opts.last
	}
	n = func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}(n, len(filtered))

	return filtered[:n]
}
