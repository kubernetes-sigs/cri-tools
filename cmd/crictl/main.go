package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

const (
	defaultTimeout = 10 * time.Second
)

func getRuntimeClientConnection(context *cli.Context) (*grpc.ClientConn, error) {
	runtimeEndpoint := context.GlobalString("runtime-endpoint")
	if runtimeEndpoint == "" {
		return nil, fmt.Errorf("--runtime-endpoint is not set")
	}
	conn, err := grpc.Dial(context.GlobalString("runtime-endpoint"), grpc.WithInsecure(), grpc.WithTimeout(context.GlobalDuration("timeout")),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return conn, nil
}

func main() {
	app := cli.NewApp()
	app.Name = "crictl"
	app.Usage = "client for CRI"
	app.Version = "0.0.1"

	app.Commands = []cli.Command{
		runtimeVersionCommand,
		runtimePodSandboxCommand,
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "runtime-endpoint",
			Usage: "CRI server runtime endpoint",
		},
		cli.StringFlag{
			Name:  "image-endpoint",
			Usage: "CRI server image endpoint",
		},
		cli.DurationFlag{
			Name:  "timeout",
			Value: defaultTimeout,
			Usage: "Timeout of connecting to server",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug output",
		},
	}

	if err := app.Run(os.Args); err != nil {
		glog.Fatal(err)
	}
}
