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
	"path/filepath"
	"sort"

	"github.com/golang/glog"
	_ "github.com/kubernetes-incubator/cri-tools/pkg/validate"
	"github.com/urfave/cli"
)

func main() {
	var buildDependencies bool
	var ginkgoFlags, testFlags, runtimeServiceAddress, imageServiceAddress, focus string

	app := cli.NewApp()
	app.Name = "critest"
	app.Usage = "CRI tools for test."
	app.Version = "0.0.1"

	app.Flags = []cli.Flag{
		cli.BoolTFlag{
			Name:        "build-dependencies, b",
			Usage:       "If true, build all dependencies.",
			Destination: &buildDependencies,
		},
		cli.StringFlag{
			Name:        "ginkgo-flags, g",
			Usage:       "Space-separated list of arguments to pass to Ginkgo test runner.",
			Destination: &ginkgoFlags,
		},
		cli.StringFlag{
			Name:        "test-flags",
			Usage:       "Space-separated list of arguments to pass to CRI e2e test.",
			Destination: &testFlags,
		},
		cli.StringFlag{
			Name:        "runtime-endpoint, r",
			Value:       "/var/run/dockershim.sock",
			Usage:       "CRI runtime service address which is tested.",
			Destination: &runtimeServiceAddress,
		},
		cli.StringFlag{
			Name:        "image-endpoint, i",
			Usage:       "CRI image service address which is tested. Same with runtime-address if not specified.",
			Destination: &imageServiceAddress,
		},
		cli.StringFlag{
			Name:        "focus, f",
			Usage:       "CRI e2e test will only run the test that match the focus regular expression.",
			Destination: &focus,
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))

	app.Action = func(c *cli.Context) error {
		// Build dependencies - ginkgo and e2e.test
		if buildDependencies {
			if err := build(); err != nil {
				glog.Fatalf("Failed to build the dependencies: %v", err)
			}
		}

		// Run CRI e2e test
		outputDir, err := getBuildOutputDir()
		if err != nil {
			glog.Fatalf("Failed to get build output directory: %v", err)
		}
		glog.Infof("Got build output dir: %v", outputDir)
		ginkgo := filepath.Join(outputDir, "ginkgo")
		test := filepath.Join(outputDir, "e2e.test")

		if imageServiceAddress == "" {
			imageServiceAddress = runtimeServiceAddress
		}

		if focus != "" {
			ginkgoFlags = ginkgoFlags + " -focus=\"" + focus + "\""
		}

		return runCommand(ginkgo, ginkgoFlags, test, "--", testFlags, "--runtime-service-address="+runtimeServiceAddress, "--image-service-address="+imageServiceAddress)
	}

	if err := app.Run(os.Args); err != nil {
		glog.Fatal(err)
	}
}
