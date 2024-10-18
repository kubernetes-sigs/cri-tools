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
	"path/filepath"
	"reflect"
	"slices"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
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
}

const (
	runtimeEndpointKey   = "runtime-endpoint"
	imageEndpointKey     = "image-endpoint"
	timeoutKey           = "timeout"
	debugKey             = "debug"
	pullImageOnCreateKey = "pull-image-on-create"
	disablePullOnRunKey  = "disable-pull-on-run"
)

// ReadConfig reads from a file with the given name and returns a config or
// an error if the file was unable to be parsed.
func ReadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file path: %w", err)
	}

	m := map[string]any{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal YAML config: %w", err)
	}
	logrus.Debugf("Unmarshaled config map: %v", m)

	for k := range m {
		if !slices.Contains([]string{
			runtimeEndpointKey,
			imageEndpointKey,
			timeoutKey,
			debugKey,
			pullImageOnCreateKey,
			disablePullOnRunKey,
		}, k) {
			return nil, fmt.Errorf("invalid config option: %s", k)
		}
	}

	c := &Config{}

	runtimeEndpoint, err := mapKeyValue[string](m, runtimeEndpointKey)
	if err != nil {
		return nil, err
	}
	c.RuntimeEndpoint = runtimeEndpoint

	imageEndpoint, err := mapKeyValue[string](m, imageEndpointKey)
	if err != nil {
		return nil, err
	}
	c.ImageEndpoint = imageEndpoint

	timeout, err := mapKeyValue[int](m, timeoutKey)
	if err != nil {
		return nil, err
	}
	c.Timeout = timeout

	debug, err := mapKeyValue[bool](m, debugKey)
	if err != nil {
		return nil, err
	}
	c.Debug = debug

	pullImageOnCreate, err := mapKeyValue[bool](m, pullImageOnCreateKey)
	if err != nil {
		return nil, err
	}
	c.PullImageOnCreate = pullImageOnCreate

	disablePullOnRun, err := mapKeyValue[bool](m, disablePullOnRunKey)
	if err != nil {
		return nil, err
	}
	c.DisablePullOnRun = disablePullOnRun

	return c, nil
}

func mapKeyValue[T any](m map[string]any, key string) (ret T, err error) {
	if value, ok := m[key]; ok {
		// Even Integer values will be interpreted as float
		if reflect.TypeOf(value).Kind() == reflect.Float64 {
			//nolint:forcetypeassert // type assertion done before
			value = int(value.(float64))
		}

		if typedValue, ok := value.(T); ok {
			return typedValue, nil
		} else {
			return ret, fmt.Errorf("invalid value \"%T\" for key %q", value, key)
		}
	}
	return ret, nil
}

// WriteConfig writes config to file and return
// an error if the file was unable to be written.
func WriteConfig(c *Config, path string) error {
	if c == nil {
		c = &Config{}
	}
	m := map[string]any{}

	if c.RuntimeEndpoint != "" {
		m[runtimeEndpointKey] = c.RuntimeEndpoint
	}

	if c.ImageEndpoint != "" {
		m[imageEndpointKey] = c.ImageEndpoint
	}

	if c.Timeout != 0 {
		m[timeoutKey] = c.Timeout
	}

	if c.Debug {
		m[debugKey] = c.Debug
	}

	if c.PullImageOnCreate {
		m[pullImageOnCreateKey] = c.PullImageOnCreate
	}

	if c.DisablePullOnRun {
		m[disablePullOnRunKey] = c.DisablePullOnRun
	}

	logrus.Debugf("Marshalling config map: %v", m)
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal YAML config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure config path dir: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}
