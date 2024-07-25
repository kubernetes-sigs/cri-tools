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

	"github.com/urfave/cli/v2"

	"sigs.k8s.io/cri-tools/pkg/common"
)

var configCommand = &cli.Command{
	Name:  "config",
	Usage: "Get and set crictl client configuration options",
	ArgsUsage: `[<crictl options>]

EXAMPLE:
   crictl config --set debug=true

CRICTL OPTIONS:
	 runtime-endpoint:	Container runtime endpoint
	 image-endpoint:	Image endpoint
	 timeout:	Timeout of connecting to server (default: 2s)
	 debug:	Enable debug output (default: false)
	 pull-image-on-create:	Enable pulling image on create requests (default: false)
	 disable-pull-on-run:	Disable pulling image on run requests (default: false)`,
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
		&cli.BoolFlag{
			Name:  "list",
			Usage: "show all option value",
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
			case "runtime-endpoint":
				fmt.Println(config.RuntimeEndpoint)
			case "image-endpoint":
				fmt.Println(config.ImageEndpoint)
			case "timeout":
				fmt.Println(config.Timeout)
			case "debug":
				fmt.Println(config.Debug)
			case "pull-image-on-create":
				fmt.Println(config.PullImageOnCreate)
			case "disable-pull-on-run":
				fmt.Println(config.DisablePullOnRun)
			default:
				return fmt.Errorf("no configuration option named %s", get)
			}
			return nil
		} else if c.IsSet("set") {
			settings := c.StringSlice("set")
			for _, setting := range settings {
				options := strings.Split(setting, ",")
				for _, option := range options {
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
			display.AddRow([]string{"runtime-endpoint", config.RuntimeEndpoint})
			display.AddRow([]string{"image-endpoint", config.ImageEndpoint})
			display.AddRow([]string{"timeout", strconv.Itoa(config.Timeout)})
			display.AddRow([]string{"debug", strconv.FormatBool(config.Debug)})
			display.AddRow([]string{"pull-image-on-create", strconv.FormatBool(config.PullImageOnCreate)})
			display.AddRow([]string{"disable-pull-on-run", strconv.FormatBool(config.DisablePullOnRun)})
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
		if err := setValue(key, value, config); err != nil {
			return err
		}
		return common.WriteConfig(config, configFile)
	},
}

func setValue(key, value string, config *common.Config) error {
	switch key {
	case "runtime-endpoint":
		config.RuntimeEndpoint = value
	case "image-endpoint":
		config.ImageEndpoint = value
	case "timeout":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse timeout value '%s': %w", value, err)
		}
		config.Timeout = n
	case "debug":
		debug, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse debug value '%s': %w", value, err)
		}
		config.Debug = debug
	case "pull-image-on-create":
		pi, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse pull-image-on-create value '%s': %w", value, err)
		}
		config.PullImageOnCreate = pi
	case "disable-pull-on-run":
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
