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
	"os"
	"sort"

	"github.com/golang/glog"
	_ "github.com/kubernetes-incubator/cri-tools/pkg/benchmark"
	_ "github.com/kubernetes-incubator/cri-tools/pkg/validate"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "critest"
	app.Usage = "CRI tools for test."
	app.Version = "1.0.0-alpha.0"

	app.Commands = []cli.Command{
		validationCommand,
		benchmarkCommand,
	}

	app.Flags = []cli.Flag{
		cli.BoolTFlag{
			Name:  "compile, c",
			Usage: "If true, build all dependencies.",
		},
		cli.StringFlag{
			Name:  "ginkgo-flags, g",
			Usage: "Space-separated list of arguments to pass to Ginkgo test runner.",
		},
		cli.StringFlag{
			Name:   "runtime-endpoint, r",
			EnvVar: "CRI_RUNTIME_ENDPOINT",
			Value:  "/var/run/dockershim.sock",
			Usage:  "CRI runtime service address which is tested.",
		},
		cli.StringFlag{
			Name:   "image-endpoint, i",
			EnvVar: "CRI_IMAGE_ENDPOINT",
			Usage:  "CRI image service address which is tested. Same with runtime-address if not specified.",
		},
		cli.StringFlag{
			Name:  "focus, f",
			Usage: "critest will only run the test that match the focus regular expression.",
		},
		cli.StringFlag{
			Name:  "skip, s",
			Usage: "critest will not run the test that match the skip regular expression.",
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))

	if err := app.Run(os.Args); err != nil {
		glog.Fatal(err)
	}
}
