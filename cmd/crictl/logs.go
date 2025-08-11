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
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	timetypes "github.com/docker/docker/api/types/time"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/cri-client/pkg/logs"
	"k8s.io/klog/v2"
)

const (
	streamStdout = "stdout"
	streamStderr = "stderr"
)

var logsCommand = &cli.Command{
	Name:                   "logs",
	Usage:                  "Fetch the logs of a container",
	ArgsUsage:              "CONTAINER-ID",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "follow",
			Aliases: []string{"f"},
			Usage:   "Follow log output",
		},
		&cli.BoolFlag{
			Name:    "previous",
			Aliases: []string{"p"},
			Usage:   "Print the logs for the previous instance of the container in a pod if it exists",
		},
		&cli.BoolFlag{
			Name:    "reopen",
			Aliases: []string{"r"},
			Usage:   "Reopen the container logs for the provided container",
		},
		&cli.Int64Flag{
			Name:  "tail",
			Value: -1,
			Usage: "Number of lines to show from the end of the logs. Defaults to all",
		},
		&cli.Int64Flag{
			Name:  "limit-bytes",
			Value: -1,
			Usage: "Maximum bytes of logs to return. Defaults to no limit",
		},
		&cli.StringFlag{
			Name:  "since",
			Usage: "Show logs since timestamp (e.g. 2013-01-02T13:23:37) or relative (e.g. 42m for 42 minutes)",
		},
		&cli.BoolFlag{
			Name:    "timestamps",
			Aliases: []string{"t"},
			Usage:   "Show timestamps",
		},
		&cli.StringFlag{
			Name:    "stream",
			Aliases: []string{"s"},
			Usage:   "Show specified stream (stdout or stderr). Defaults to both.",
		},
	},
	Action: func(c *cli.Context) (retErr error) {
		containerID := c.Args().First()
		if containerID == "" {
			return errors.New("ID cannot be empty")
		}

		if c.NArg() > 1 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeService, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		if c.Bool("reopen") {
			if _, err := InterruptableRPC(c.Context, func(ctx context.Context) (any, error) {
				return nil, runtimeService.ReopenContainerLog(ctx, containerID)
			}); err != nil {
				return fmt.Errorf("reopen container logs: %w", err)
			}
			logrus.Info("Container logs reopened")
		}

		tailLines := c.Int64("tail")
		limitBytes := c.Int64("limit-bytes")
		since, err := parseTimestamp(c.String("since"))
		if err != nil {
			return err
		}
		timestamp := c.Bool("timestamps")
		previous := c.Bool("previous")
		stream := c.String("stream")

		switch stream {
		case streamStdout, streamStderr, "":
		default:
			return fmt.Errorf(`invalid stream %q, must be "stdout" or "stderr"`, stream)
		}
		if c.IsSet("tail") && c.IsSet("stream") {
			return errors.New("--tail and --stream are mutually exclusive")
		}

		logOptions := logs.NewLogOptions(&v1.PodLogOptions{
			Follow:     c.Bool("follow"),
			TailLines:  &tailLines,
			LimitBytes: &limitBytes,
			SinceTime:  since,
			Timestamps: timestamp,
		}, time.Now())
		status, err := InterruptableRPC(c.Context, func(ctx context.Context) (*pb.ContainerStatusResponse, error) {
			return runtimeService.ContainerStatus(ctx, containerID, false)
		})
		if err != nil {
			return err
		}
		logPath := status.GetStatus().GetLogPath()
		if logPath == "" {
			return errors.New("the container has not set log path")
		}
		if previous {
			containerAttempt := status.GetStatus().GetMetadata().GetAttempt()
			if containerAttempt == uint32(0) {
				return fmt.Errorf("previous terminated container %s not found", status.GetStatus().GetMetadata().GetName())
			}
			logPath = fmt.Sprintf("%s%s%s", logPath[:strings.LastIndex(logPath, "/")+1], strconv.FormatUint(uint64(containerAttempt-1), 10),
				logPath[strings.LastIndex(logPath, "."):])
		}
		// build a WithCancel context based on cli.context
		readLogCtx, cancelFn := context.WithCancel(c.Context)
		go func() {
			<-SetupInterruptSignalHandler()
			// cancel readLogCtx when Interrupt signal received
			cancelFn()
		}()
		defer func() {
			// We can not use the typed error "context.Canceled" here
			// because the upstream K8S dependency explicitly returns a fmt.Errorf("context cancelled").
			// So we need to compare the error in string.
			if retErr != nil && retErr.Error() == "context cancelled" {
				// Silent the "context cancelled" error.
				// In order to prevent the error msg when user hit Ctrl+C.
				retErr = nil
			}
			// Ensure no context leak
			cancelFn()
		}()
		logger := klog.Background()

		var (
			stdoutStream io.Writer = os.Stdout
			stderrStream io.Writer = os.Stderr
		)
		switch stream {
		case streamStdout:
			stderrStream = nil
		case streamStderr:
			stdoutStream = nil
		}

		return logs.ReadLogs(readLogCtx, &logger, logPath, status.GetStatus().GetId(), logOptions, runtimeService, stdoutStream, stderrStream)
	},
}

// parseTimestamp parses timestamp string as golang duration,
// then RFC3339 time and finally as a Unix timestamp.
func parseTimestamp(value string) (*metav1.Time, error) {
	if value == "" {
		return nil, nil
	}

	str, err := timetypes.GetTimestamp(value, time.Now())
	if err != nil {
		return nil, err
	}

	s, ns, err := timetypes.ParseTimestamps(str, 0)
	if err != nil {
		return nil, err
	}

	t := metav1.NewTime(time.Unix(s, ns))

	return &t, nil
}
