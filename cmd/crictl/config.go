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

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"sigs.k8s.io/cri-tools/pkg/common"
)

var configCommand = &cli.Command{
	Name:  "config",
	Usage: "Get, set and list crictl configuration options",
	ArgsUsage: `[<crictl options>]

EXAMPLES:
   # Set the key "debug" to "true"
   crictl config --set debug=true

   # Set the key "debug" to "true"
   crictl config debug true

   # Get the value of "debug"
   crictl config --get debug

   # Show the full configuration
   crictl config --list

CRICTL OPTIONS:
	 runtime-endpoint:	Container Runtime Interface (CRI) runtime endpoint (default: "")
	 image-endpoint:	Container Runtime Interface (CRI) image endpoint (default: "")
	 timeout:	Timeout of connecting to server (default: 2)
	 debug:	Enable debug output (default: false)
	 pull-image-on-create:	Enable pulling image on create requests (default: false)
	 disable-pull-on-run:	Disable pulling image on run requests (default: false)`,
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "get",
			Usage: "Show the option value",
		},
		&cli.StringSliceFlag{
			Name:  "set",
			Usage: "Set option (can specify multiple or separate values with commas: opt1=val1,opt2=val2)",
		},
		&cli.BoolFlag{
			Name:  "list",
			Usage: "Show all option value",
		},
	},
	Action: func(c *cli.Context) error {
		configFile := c.String("config")
		if _, err := os.Stat(configFile); err != nil {
			if err := common.WriteConfig(nil, configFile); err != nil {
				return err
			}
		}
		// Get config from file.
		config, err := common.ReadConfig(configFile)
		if err != nil {
			return fmt.Errorf("load config file: %w", err)
		}
		if c.IsSet("get") {
			get := c.String("get")
			switch get {
			case common.RuntimeEndpoint:
				fmt.Println(config.RuntimeEndpoint)
			case common.ImageEndpoint:
				fmt.Println(config.ImageEndpoint)
			case common.Timeout:
				fmt.Println(config.Timeout)
			case common.Debug:
				fmt.Println(config.Debug)
			case common.PullImageOnCreate:
				fmt.Println(config.PullImageOnCreate)
			case common.DisablePullOnRun:
				fmt.Println(config.DisablePullOnRun)
			default:
				return fmt.Errorf("no configuration option named %s", get)
			}

			return nil
		} else if c.IsSet("set") {
			settings := c.StringSlice("set")
			for _, setting := range settings {
				options := strings.SplitSeq(setting, ",")
				for option := range options {
					pair := strings.Split(option, "=")
					if len(pair) != 2 {
						return fmt.Errorf("incorrectly specified option: %v", setting)
					}
					key := pair[0]
					value := pair[1]
					if err := setValue(key, value, config); err != nil {
						return err
					}
				}
			}

			return common.WriteConfig(config, configFile)
		} else if c.Bool("list") {
			display := newDefaultTableDisplay()
			display.AddRow([]string{columnKey, columnValue})
			display.AddRow([]string{common.RuntimeEndpoint, config.RuntimeEndpoint})
			display.AddRow([]string{common.ImageEndpoint, config.ImageEndpoint})
			display.AddRow([]string{common.Timeout, strconv.Itoa(config.Timeout)})
			display.AddRow([]string{common.Debug, strconv.FormatBool(config.Debug)})
			display.AddRow([]string{common.PullImageOnCreate, strconv.FormatBool(config.PullImageOnCreate)})
			display.AddRow([]string{common.DisablePullOnRun, strconv.FormatBool(config.DisablePullOnRun)})
			display.ClearScreen()
			display.Flush()

			return nil
		}

		// default for backwards compatibility
		key := c.Args().First()
		if key == "" {
			return cli.ShowSubcommandHelp(c)
		}
		value := c.Args().Get(1)
		logrus.Infof("No --get, --set or --list provided, setting key %q to value %q", key, value)
		if err := setValue(key, value, config); err != nil {
			return fmt.Errorf("set %q to %q: %w", key, value, err)
		}

		return common.WriteConfig(config, configFile)
	},
}

func setValue(key, value string, config *common.Config) error {
	switch key {
	case common.RuntimeEndpoint:
		config.RuntimeEndpoint = value
	case common.ImageEndpoint:
		config.ImageEndpoint = value
	case common.Timeout:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse timeout value '%s': %w", value, err)
		}

		config.Timeout = n
	case common.Debug:
		debug, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse debug value '%s': %w", value, err)
		}

		config.Debug = debug
	case common.PullImageOnCreate:
		pi, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse pull-image-on-create value '%s': %w", value, err)
		}

		config.PullImageOnCreate = pi
	case common.DisablePullOnRun:
		pi, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse disable-pull-on-run value '%s': %w", value, err)
		}

		config.DisablePullOnRun = pi
	default:
		return fmt.Errorf("no configuration option named %s", key)
	}

	return nil
}
