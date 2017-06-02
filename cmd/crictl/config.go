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
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// Config is the internal representation of the yaml that determines how
// the app start
type Config struct {
	RuntimeEndpoint string `yaml:"runtime-endpoint"`
	ImageEndpoint   string `yaml:"image-endpoint"`
	Timeout         int    `yaml:"timeout"`
	Debug           bool   `yaml:"debug"`
}

// ReadConfig reads from a file with the given name and returns a config or
// an error if the file was unable to be parsed.
func ReadConfig(filepath string) (*Config, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	config := Config{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, err
}
