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
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/kubernetes-sigs/cri-tools/pkg/common"
)

var configCommand = &cli.Command{
	Name:                   "config",
	Usage:                  "Get and set crictl options",
	ArgsUsage:              "[<options>]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "get",
			Usage: "show the option value",
		},
		&cli.StringSliceFlag{
			Name:  "set",
			Usage: "set option (can specify multiple or separate values with commas: opt1=val1,opt2=val2)",
		},
	},
	Action: func(context *cli.Context) error {
		configFile := context.String("config")
		if _, err := os.Stat(configFile); err != nil {
			if err := common.WriteConfig(nil, configFile); err != nil {
				return err
			}
		}
		// Get config from file.
		config, err := common.ReadConfig(configFile)
		if err != nil {
			return errors.Wrap(err, "load config file")
		}
		if context.IsSet("get") {
			get := context.String("get")
			switch get {
			case "runtime-endpoint":
				fmt.Println(config.RuntimeEndpoint)
			case "image-endpoint":
				fmt.Println(config.ImageEndpoint)
			case "timeout":
				fmt.Println(config.Timeout)
			case "debug":
				fmt.Println(config.Debug)
			default:
				logrus.Fatalf("No section named %s", get)
			}
			return nil
		} else if context.IsSet("set") {
			settings := context.StringSlice("set")
			for _, setting := range settings {
				options := strings.Split(setting, ",")
				for _, option := range options {
					pair := strings.Split(option, "=")
					if len(pair) != 2 {
						return fmt.Errorf("incorrectly specified option: %v", setting)
					}
					key := pair[0]
					value := pair[1]
					switch key {
					case "runtime-endpoint":
						config.RuntimeEndpoint = value
					case "image-endpoint":
						config.ImageEndpoint = value
					case "timeout":
						n, err := strconv.Atoi(value)
						if err != nil {
							logrus.Fatal(err)
						}
						config.Timeout = n
					case "debug":
						var debug bool
						if value == "true" {
							debug = true
						} else if value == "false" {
							debug = false
						} else {
							logrus.Fatal("use true|false for debug")
						}
						config.Debug = debug
					default:
						logrus.Fatalf("No section named %s", key)
					}
				}
			}
			return common.WriteConfig(config, configFile)
		} else {
			return cli.ShowSubcommandHelp(context)
		}
	},
}
