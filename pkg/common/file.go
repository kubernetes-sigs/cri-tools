/*
Copyright 2020 The Kubernetes Authors.

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

package common

import (
	"fmt"
	"os"
	gofilepath "path/filepath"
	"strconv"

	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

// Config is the internal representation of the yaml that defines
// server configuration.
type Config struct {
	RuntimeEndpoint   string
	ImageEndpoint     string
	Timeout           int
	Debug             bool
	PullImageOnCreate bool
	DisablePullOnRun  bool
	yamlData          *yaml.Node // YAML representation of config
}

const (
	// RuntimeEndpoint is the YAML key for the runtime endpoint config option.
	RuntimeEndpoint = "runtime-endpoint"

	// ImageEndpoint is the YAML key for the image endpoint config option.
	ImageEndpoint = "image-endpoint"

	// Timeout is the YAML key for the timeout config option.
	Timeout = "timeout"

	// Debug is the YAML key for the debug config option.
	Debug = "debug"

	// PullImageOnCreate is the YAML key for the pull image on create config option.
	PullImageOnCreate = "pull-image-on-create"

	// DisablePullOnRun is the YAML key for the disable pull on run config option.
	DisablePullOnRun = "disable-pull-on-run"
)

// ReadConfig reads from a file with the given name and returns a config or
// an error if the file was unable to be parsed.
func ReadConfig(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	yamlConfig := &yaml.Node{}

	err = yaml.Unmarshal(data, yamlConfig)
	if err != nil {
		return nil, err
	}

	config, err := getConfigOptions(yamlConfig)
	if err != nil {
		return nil, err
	}

	return config, err
}

// WriteConfig writes config to file
// an error if the file was unable to be written to.
func WriteConfig(c *Config, filepath string) error {
	if c == nil {
		c = &Config{}
	}

	if c.yamlData == nil {
		c.yamlData = &yaml.Node{}
	}

	setConfigOptions(c)

	data, err := yaml.Marshal(c.yamlData)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(gofilepath.Dir(filepath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0o644)
}

// Extracts config options from the yaml data which is loaded from file.
func getConfigOptions(yamlData *yaml.Node) (*Config, error) {
	config := &Config{yamlData: yamlData}

	if len(yamlData.Content) == 0 ||
		yamlData.Content[0].Content == nil {
		return config, nil
	}

	contentLen := len(yamlData.Content[0].Content)

	// YAML representation contains 2 yaml ScalarNodes per config option.
	// One is config option name and other is the value of the option
	// These ScalarNodes help preserve comments associated with
	// the YAML entry
	for index := 0; index < contentLen-1; {
		configOption := yamlData.Content[0].Content[index]
		name := configOption.Value
		value := yamlData.Content[0].Content[index+1].Value

		var err error

		switch name {
		case RuntimeEndpoint:
			config.RuntimeEndpoint = value
		case ImageEndpoint:
			config.ImageEndpoint = value
		case Timeout:
			config.Timeout, err = strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("parsing config option '%s': %w", name, err)
			}
		case Debug:
			config.Debug, err = strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("parsing config option '%s': %w", name, err)
			}
		case PullImageOnCreate:
			config.PullImageOnCreate, err = strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("parsing config option '%s': %w", name, err)
			}
		case DisablePullOnRun:
			config.DisablePullOnRun, err = strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("parsing config option '%s': %w", name, err)
			}
		default:
			return nil, fmt.Errorf("Config option '%s' is not valid", name)
		}

		index += 2
	}

	return config, nil
}

// Set config options on yaml data for persistece to file.
func setConfigOptions(config *Config) {
	setConfigOption(RuntimeEndpoint, config.RuntimeEndpoint, config.yamlData)
	setConfigOption(ImageEndpoint, config.ImageEndpoint, config.yamlData)
	setConfigOption(Timeout, strconv.Itoa(config.Timeout), config.yamlData)
	setConfigOption(Debug, strconv.FormatBool(config.Debug), config.yamlData)
	setConfigOption(PullImageOnCreate, strconv.FormatBool(config.PullImageOnCreate), config.yamlData)
	setConfigOption(DisablePullOnRun, strconv.FormatBool(config.DisablePullOnRun), config.yamlData)
}

// Set config option on yaml.
func setConfigOption(configName, configValue string, yamlData *yaml.Node) {
	if len(yamlData.Content) == 0 {
		yamlData.Kind = yaml.DocumentNode
		yamlData.Content = make([]*yaml.Node, 1)
		yamlData.Content[0] = &yaml.Node{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
		}
	}

	contentLen := 0
	foundOption := false

	if yamlData.Content[0].Content != nil {
		contentLen = len(yamlData.Content[0].Content)
	}

	// Set value on existing config option
	for index := 0; index < contentLen-1; {
		name := yamlData.Content[0].Content[index].Value
		if name == configName {
			// Set the value, even if we have the option defined multiple times.
			yamlData.Content[0].Content[index+1].Value = configValue
			foundOption = true
		}

		index += 2
	}

	// New config option to set
	// YAML representation contains 2 yaml ScalarNodes per config option.
	// One is config option name and other is the value of the option
	// These ScalarNodes help preserve comments associated with
	// the YAML entry
	if !foundOption {
		const (
			tagPrefix = "!!"
			tagStr    = tagPrefix + "str"
			tagBool   = tagPrefix + "bool"
			tagInt    = tagPrefix + "int"
		)

		name := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: configName,
			Tag:   tagStr,
		}

		var tagType string

		switch configName {
		case Timeout:
			tagType = tagInt
		case Debug:
			tagType = tagBool
		case PullImageOnCreate:
			tagType = tagBool
		case DisablePullOnRun:
			tagType = tagBool
		default:
			tagType = tagStr
		}

		value := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: configValue,
			Tag:   tagType,
		}
		yamlData.Content[0].Content = append(yamlData.Content[0].Content, name, value)
	}
}
