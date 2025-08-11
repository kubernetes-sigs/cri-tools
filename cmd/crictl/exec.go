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
	"net/url"
	"time"

	mobyterm "github.com/moby/term"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	errorUtils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
	remoteclient "k8s.io/client-go/tools/remotecommand"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubectl/pkg/util/term"

	"sigs.k8s.io/cri-tools/pkg/common"
)

const (
	// TODO: make this configurable in kubelet.
	kubeletURLSchema = "http"
	kubeletURLHost   = "http://127.0.0.1:10250"

	transportFlag = "transport"

	detachSequence = "ctrl-p,ctrl-q"
)

var runtimeExecCommand = &cli.Command{
	Name:      "exec",
	Usage:     "Run a command in a running container",
	ArgsUsage: "[CONTAINER-ID] COMMAND [ARG...]",
	Description: `The CONTAINER-ID is only required if none of the following filter flags are set:
--image, --label, --last, --latest, --name, --pod, --state`,
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "sync",
			Aliases: []string{"s"},
			Usage:   "Run the command synchronously",
		},
		&cli.Int64Flag{
			Name:  "timeout",
			Usage: "Timeout in seconds",
		},
		&cli.BoolFlag{
			Name:    "tty",
			Aliases: []string{"t"},
			Usage:   "Allocate a pseudo-TTY",
		},
		&cli.BoolFlag{
			Name:    "interactive",
			Aliases: []string{"i"},
			Usage:   "Keep STDIN open",
		},
		&cli.StringFlag{
			Name:    transportFlag,
			Aliases: []string{"r"},
			Value:   common.TransportSpdy,
			Usage:   fmt.Sprintf("Transport protocol to use, one of: %s|%s", common.TransportSpdy, common.TransportWebsocket),
		},
		&cli.StringFlag{
			Name:  "name",
			Usage: "Exec command for all containers matching this name filter regular expression pattern",
		},
		&cli.StringFlag{
			Name:    "pod",
			Aliases: []string{"p"},
			Usage:   "Exec command for all containers matching this pod ID filter",
		},
		&cli.StringFlag{
			Name:  "image",
			Usage: "Exec command for all containers matching this container image filter",
		},
		&cli.StringFlag{
			Name:  "state",
			Usage: "Exec command for all containers matching this container state filter",
		},
		&cli.StringSliceFlag{
			Name:  "label",
			Usage: "Exec command for all containers matching this key=value label filter",
		},
		&cli.BoolFlag{
			Name:    "latest",
			Aliases: []string{"l"},
			Usage:   "Exec command for the most recently created container",
		},
		&cli.IntFlag{
			Name:    "last",
			Aliases: []string{"n"},
			Usage:   "Exec command for all last n containers, set to 0 for unlimited",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Do not print the container ID if multiple containers are selected",
		},
		&cli.BoolFlag{
			Name:    "ignore-errors",
			Aliases: []string{"e"},
			Usage:   "Ignore errors during command execution",
		},
		&cli.BoolFlag{
			Name:    "parallel",
			Aliases: []string{"x"},
			Usage:   "Run the command in parallel if multiple containers are selected",
		},
		&cli.StringFlag{
			Name:    flagTLSSNI,
			Usage:   "Server name used in the TLS client to check server certificates against",
			Aliases: []string{"tls-server-name"},
			Value:   "localhost",
		},
		&cli.StringFlag{
			Name:  flagTLSCA,
			Usage: "Path to the streaming TLS CA certificate",
		},
		&cli.StringFlag{
			Name:  flagTLSCert,
			Usage: "Path to the streaming TLS certificate",
		},
		&cli.StringFlag{
			Name:  flagTLSKey,
			Usage: "Path to the streaming TLS key",
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
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

		// Assume a regular exec where the first arg is the container ID.
		ids := []string{c.Args().First()}
		cmd := c.Args().Slice()[1:]
		outputContainerID := false
		ignoreErrors := c.Bool("ignore-errors")
		quiet := c.Bool("quiet")

		if ignoreErrors && !quiet {
			logrus.Warn("The `--ignore-errors` option has been set all errors returned by the command will be ignored.")
		}

		// If any of the filter flags are set, then we assume that no
		// CONTAINER-ID is provided as CLI parameter.
		if c.IsSet("name") ||
			c.IsSet("pod") ||
			c.IsSet("image") ||
			c.IsSet("state") ||
			c.IsSet("label") ||
			c.IsSet("latest") ||
			c.IsSet("last") {

			ids = []string{}
			cmd = c.Args().Slice()
			outputContainerID = !quiet && !c.Bool("parallel")

			opts := &listOptions{
				nameRegexp: c.String("name"),
				podID:      c.String("pod"),
				image:      c.String("image"),
				state:      c.String("state"),
				latest:     c.Bool("latest"),
				last:       c.Int("last"),
			}

			opts.labels, err = parseLabelStringSlice(c.StringSlice("label"))
			if err != nil {
				return err
			}

			ctrs, err := ListContainers(c.Context, runtimeClient, imageClient, opts)
			if err != nil {
				return fmt.Errorf("listing containers: %w", err)
			}

			for _, ctr := range ctrs {
				ids = append(ids, ctr.GetId())
			}

			if len(ids) == 0 {
				logrus.Error("No containers found per filter flags")

				return cli.ShowSubcommandHelp(c)
			}
		} else if c.NArg() < 2 {
			return cli.ShowSubcommandHelp(c)
		}

		opts := execOptions{
			timeout:   c.Int64("timeout"),
			tty:       c.Bool("tty"),
			stdin:     c.Bool("interactive"),
			cmd:       cmd,
			transport: c.String(transportFlag),
		}

		opts.tlsConfig, err = tlsConfigFromFlags(c)
		if err != nil {
			return fmt.Errorf("get TLS config from flags: %w", err)
		}

		funcs := []func() error{}
		for _, id := range ids {
			funcs = append(funcs, func() error {
				optsCopy := &execOptions{
					id:        id,
					cmd:       opts.cmd,
					stdin:     opts.stdin,
					timeout:   opts.timeout,
					tlsConfig: opts.tlsConfig,
					transport: opts.transport,
					tty:       opts.tty,
				}

				if outputContainerID {
					fmt.Println(id + ":")
				}
				if c.Bool("sync") {
					exitCode, err := ExecSync(c.Context, runtimeClient, optsCopy)
					if err != nil {
						return fmt.Errorf("execing command in container %s synchronously: %w", id, err)
					}
					if exitCode != 0 {
						return cli.Exit("non-zero exit code", exitCode)
					}
				} else {
					ctx, cancel := context.WithCancel(c.Context)
					defer cancel()
					err = Exec(ctx, runtimeClient, optsCopy)
					if err != nil {
						return fmt.Errorf("execing command in container %s: %w", id, err)
					}
				}

				return nil
			})
		}

		errs := errorUtils.AggregateGoroutines(funcs...)

		if ignoreErrors {
			logrus.Debugf("Ignoring errors: %v", errs)

			return nil
		}

		return errs
	},
}

const (
	flagTLSSNI  = "tls-sni"
	flagTLSCA   = "tls-ca"
	flagTLSCert = "tls-cert"
	flagTLSKey  = "tls-key"
)

func tlsConfigFromFlags(ctx *cli.Context) (*rest.TLSClientConfig, error) {
	cfg := &rest.TLSClientConfig{
		ServerName: ctx.String(flagTLSSNI),
		CAFile:     ctx.String(flagTLSCA),
		CertFile:   ctx.String(flagTLSCert),
		KeyFile:    ctx.String(flagTLSKey),
	}
	if cfg.CAFile == "" && cfg.CertFile == "" && cfg.KeyFile == "" {
		return &rest.TLSClientConfig{Insecure: true}, nil
	}

	if cfg.CertFile == "" || cfg.KeyFile == "" {
		return nil, fmt.Errorf(
			"all two flags --%s and --%s are required for TLS streaming, only --%s is optional",
			flagTLSCA, flagTLSCert, flagTLSKey,
		)
	}

	return cfg, nil
}

// ExecSync sends an ExecSyncRequest to the server, and parses
// the returned ExecSyncResponse. The function returns the corresponding exit
// code beside an general error.
func ExecSync(ctx context.Context, client internalapi.RuntimeService, opts *execOptions) (int, error) {
	request := &pb.ExecSyncRequest{
		ContainerId: opts.id,
		Cmd:         opts.cmd,
		Timeout:     opts.timeout,
	}
	logrus.Debugf("ExecSyncRequest: %v", request)

	timeoutDuration := time.Duration(opts.timeout) * time.Second

	type stdio struct {
		stdout, stderr []byte
	}

	io, err := InterruptableRPC(ctx, func(ctx context.Context) (*stdio, error) {
		stdout, stderr, err := client.ExecSync(ctx, opts.id, opts.cmd, timeoutDuration)
		if err != nil {
			return nil, err
		}

		return &stdio{stdout, stderr}, nil
	})
	if err != nil {
		return 1, err
	}

	fmt.Println(string(io.stdout))
	fmt.Println(string(io.stderr))

	return 0, nil
}

// Exec sends an ExecRequest to server, and parses the returned ExecResponse.
func Exec(ctx context.Context, client internalapi.RuntimeService, opts *execOptions) error {
	request := &pb.ExecRequest{
		ContainerId: opts.id,
		Cmd:         opts.cmd,
		Tty:         opts.tty,
		Stdin:       opts.stdin,
		Stdout:      true,
		Stderr:      !opts.tty,
	}

	logrus.Debugf("ExecRequest: %v", request)

	r, err := InterruptableRPC(ctx, func(ctx context.Context) (*pb.ExecResponse, error) {
		return client.Exec(ctx, request)
	})
	logrus.Debugf("ExecResponse: %v", r)

	if err != nil {
		return err
	}

	execURL := r.GetUrl()

	URL, err := url.Parse(execURL)
	if err != nil {
		return err
	}

	if URL.Host == "" {
		URL.Host = kubeletURLHost
	}

	if URL.Scheme == "" {
		URL.Scheme = kubeletURLSchema
	}

	logrus.Debugf("Exec URL: %v", URL)

	return stream(ctx, opts.stdin, opts.tty, opts.transport, URL, opts.tlsConfig)
}

func stream(ctx context.Context, in, tty bool, transport string, parsedURL *url.URL, tlsConfig *rest.TLSClientConfig) error {
	executor, err := common.GetExecutor(transport, parsedURL, tlsConfig)
	if err != nil {
		return fmt.Errorf("get executor: %w", err)
	}

	stdin, stdout, stderr := mobyterm.StdStreams()
	streamOptions := remoteclient.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	}

	if in {
		streamOptions.Stdin = stdin
	}

	logrus.Debugf("StreamOptions: %v", streamOptions)

	if !tty {
		return executor.StreamWithContext(ctx, streamOptions)
	}

	detachKeys, err := mobyterm.ToBytes(detachSequence)
	if err != nil {
		return errors.New("could not bind detach keys")
	}

	pr := mobyterm.NewEscapeProxy(streamOptions.Stdin, detachKeys)
	streamOptions.Stdin = pr

	if !in {
		return errors.New("tty=true must be specified with interactive=true")
	}

	t := term.TTY{
		In:  stdin,
		Out: stdout,
		Raw: true,
	}
	if !t.IsTerminalIn() {
		return errors.New("input is not a terminal")
	}

	streamOptions.TerminalSizeQueue = t.MonitorSize(t.GetSize())

	return t.Safe(func() error { return executor.StreamWithContext(ctx, streamOptions) })
}
