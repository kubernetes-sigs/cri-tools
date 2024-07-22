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
	"k8s.io/client-go/rest"
	remoteclient "k8s.io/client-go/tools/remotecommand"
	internalapi "k8s.io/cri-api/pkg/apis"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	// TODO: make this configurable in kubelet.
	kubeletURLSchema = "http"
	kubeletURLHost   = "http://127.0.0.1:10250"

	transportFlag      = "transport"
	transportWebsocket = "websocket"
	transportSpdy      = "spdy"
)

const detachSequence = "ctrl-p,ctrl-q"

var runtimeExecCommand = &cli.Command{
	Name:                   "exec",
	Usage:                  "Run a command in a running container",
	ArgsUsage:              "CONTAINER-ID COMMAND [ARG...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "sync",
			Aliases: []string{"s"},
			Usage:   "Run the command synchronously",
		},
		&cli.Int64Flag{
			Name:  "timeout",
			Value: 0,
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
			Value:   transportSpdy,
			Usage:   fmt.Sprintf("Transport protocol to use, one of: %s|%s", transportSpdy, transportWebsocket),
		},
	},
	Action: func(c *cli.Context) error {
		if c.NArg() < 2 {
			return cli.ShowSubcommandHelp(c)
		}

		runtimeClient, err := getRuntimeService(c, 0)
		if err != nil {
			return err
		}

		var opts = execOptions{
			id:        c.Args().First(),
			timeout:   c.Int64("timeout"),
			tty:       c.Bool("tty"),
			stdin:     c.Bool("interactive"),
			cmd:       c.Args().Slice()[1:],
			transport: c.String(transportFlag),
		}
		if c.Bool("sync") {
			exitCode, err := ExecSync(runtimeClient, opts)
			if err != nil {
				return fmt.Errorf("execing command in container synchronously: %w", err)
			}
			if exitCode != 0 {
				return cli.Exit("non-zero exit code", exitCode)
			}
			return nil
		}

		ctx, cancel := context.WithCancel(c.Context)
		defer cancel()

		err = Exec(ctx, runtimeClient, opts)
		if err != nil {
			return fmt.Errorf("execing command in container: %w", err)
		}
		return nil
	},
}

// ExecSync sends an ExecSyncRequest to the server, and parses
// the returned ExecSyncResponse. The function returns the corresponding exit
// code beside an general error.
func ExecSync(client internalapi.RuntimeService, opts execOptions) (int, error) {
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
	io, err := InterruptableRPC(nil, func(ctx context.Context) (*stdio, error) {
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

// Exec sends an ExecRequest to server, and parses the returned ExecResponse
func Exec(ctx context.Context, client internalapi.RuntimeService, opts execOptions) error {
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
	execURL := r.Url

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
	return stream(ctx, opts.stdin, opts.tty, opts.transport, URL)
}

func stream(ctx context.Context, in, tty bool, transport string, url *url.URL) error {
	executor, err := getExecutor(transport, url)
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
	} else {
		var detachKeys []byte
		detachKeys, err = mobyterm.ToBytes(detachSequence)
		if err != nil {
			return errors.New("could not bind detach keys")
		}
		pr := mobyterm.NewEscapeProxy(streamOptions.Stdin, detachKeys)
		streamOptions.Stdin = pr
	}
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

func getExecutor(transport string, url *url.URL) (exec remoteclient.Executor, err error) {
	config := &rest.Config{TLSClientConfig: rest.TLSClientConfig{Insecure: true}}

	switch transport {
	case transportSpdy:
		return remoteclient.NewSPDYExecutor(config, "POST", url)

	case transportWebsocket:
		return remoteclient.NewWebSocketExecutor(config, "GET", url.String())

	default:
		return nil, fmt.Errorf("unknown transport: %s", transport)

	}
}
