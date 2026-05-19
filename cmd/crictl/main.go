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
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
	"sigs.k8s.io/cri-tools/pkg/tracing"
	"sigs.k8s.io/cri-tools/pkg/version"
)

const (
	defaultTimeout        = 2 * time.Second
	defaultTimeoutWindows = 200 * time.Second
)

func getTimeout(timeDuration time.Duration) time.Duration {
	if timeDuration.Seconds() > 0 {
		return timeDuration
	}

	if runtime.GOOS == framework.OSWindows {
		return defaultTimeoutWindows
	}

	return defaultTimeout // use default
}

func main() {
	app := cli.NewApp()
	app.Name = "crictl"
	app.Usage = "client for CRI"
	app.Version = version.Version
	app.Metadata = map[string]any{}

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
		podStatsCommand,
		podMetricsCommand,
		metricDescriptorsCommand,
		completionCommand,
		checkpointContainerCommand,
		runtimeConfigCommand,
		eventsCommand,
		updateRuntimeConfigCommand,
	}

	slices.SortFunc(app.Commands, func(a, b *cli.Command) int { return strings.Compare(a.Name, b.Name) })

	runtimeEndpointUsage := fmt.Sprintf("Endpoint of CRI container runtime "+
		"service (default: uses in order the first successful one of %v). "+
		"Default is now deprecated and the endpoint should be set instead.",
		defaultRuntimeEndpoints)

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
			Usage:   runtimeEndpointUsage,
		},
		&cli.StringFlag{
			Name:    "image-endpoint",
			Aliases: []string{"i"},
			EnvVars: []string{"IMAGE_SERVICE_ENDPOINT"},
			Usage: "Endpoint of CRI image manager service (default: uses " +
				"'runtime-endpoint' setting)",
		},
		&cli.DurationFlag{
			Name:    "timeout",
			Aliases: []string{"t"},
			Value:   defaultTimeout,
			Usage: "Timeout of connecting to the server in seconds (e.g. 2s, 20s.). " +
				"0 or less is set to default",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"D"},
			Usage:   "Enable debug mode",
		},
		&cli.BoolFlag{
			Name:  "enable-tracing",
			Usage: "Enable OpenTelemetry tracing.",
		},
		&cli.IntFlag{
			Name:  "tracing-sampling-rate-per-million",
			Usage: "Number of samples to collect per million OpenTelemetry spans. Set to 1000000 or -1 to always sample.",
			Value: -1,
		},
		&cli.StringFlag{
			Name:  "tracing-endpoint",
			Usage: "Address to which the gRPC tracing collector will send spans to.",
			Value: "127.0.0.1:4317",
		},
		&cli.StringFlag{
			Name:  "profile-cpu",
			Usage: "Write a pprof CPU profile to the provided path.",
		},
		&cli.StringFlag{
			Name:  "profile-mem",
			Usage: "Write a pprof memory profile to the provided path.",
		},
	}

	var cpuProfile *os.File

	defer func() {
		if cpuProfile != nil {
			pprof.StopCPUProfile()
			cpuProfile.Close()
		}
	}()

	app.Before = func(context *cli.Context) (err error) {
		var config *common.ServerConfiguration

		var exePath string

		cpuProfilePath := context.String("profile-cpu")
		if cpuProfilePath != "" {
			cpuProfilePath, err = filepath.Abs(cpuProfilePath)
			if err != nil {
				return fmt.Errorf("unable to get absolute memory profile path: %w", err)
			}

			logrus.Infof("Creating CPU profile in: %s", cpuProfilePath)

			cpuProfile, err = os.Create(cpuProfilePath)
			if err != nil {
				return fmt.Errorf("could not create CPU profile %q: %w", cpuProfilePath, err)
			}

			if err := pprof.StartCPUProfile(cpuProfile); err != nil {
				return fmt.Errorf("could not start CPU profiling in %q: %w", cpuProfilePath, err)
			}
		}

		if exePath, err = os.Executable(); err != nil {
			logrus.Fatal(err)
		}

		if config, err = common.GetServerConfigFromFile(context.String("config"), exePath); err != nil {
			if context.IsSet("config") {
				logrus.Fatal(err)
			}
		}

		cfg := newCrictlConfig(context, config)

		// Configure tracing if enabled
		if context.IsSet("enable-tracing") {
			cfg.TracerProvider, err = tracing.Init(
				context.Context,
				context.String("tracing-endpoint"),
				context.Int("tracing-sampling-rate-per-million"),
			)
			if err != nil {
				return fmt.Errorf("init tracing: %w", err)
			}
		}

		context.App.Metadata[configKey] = cfg

		return nil
	}

	app.After = func(ctx *cli.Context) (err error) {
		memProfilePath := ctx.String("profile-mem")
		if memProfilePath != "" {
			memProfilePath, err = filepath.Abs(memProfilePath)
			if err != nil {
				return fmt.Errorf("unable to get absolute memory profile path: %w", err)
			}

			logrus.Infof("Creating memory profile in: %s", memProfilePath)

			file, err := os.Create(memProfilePath)
			if err != nil {
				return fmt.Errorf("could not create memory profile %q: %w", memProfilePath, err)
			}
			defer file.Close()

			// Ensure up to date data
			runtime.GC()

			if err := pprof.WriteHeapProfile(file); err != nil {
				return fmt.Errorf("could not write memory profile in %q: %w", memProfilePath, err)
			}
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

	// Ensure that all spans are processed.
	if v, ok := app.Metadata[configKey]; ok {
		cfg, _ := v.(*CrictlConfig)
		if cfg.TracerProvider != nil {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
			defer cancel()

			if err := cfg.TracerProvider.Shutdown(ctx); err != nil {
				logrus.Errorf("Unable to shutdown tracer provider: %v", err)
			}
		}
	}
}
