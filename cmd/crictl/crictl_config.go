/*
Copyright The Kubernetes Authors.

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
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	internalapi "k8s.io/cri-api/pkg/apis"
	remote "k8s.io/cri-client/pkg"

	"sigs.k8s.io/cri-tools/pkg/common"
)

const configKey = "crictl-config"

func newCrictlConfig(ctx *cli.Context, config *common.ServerConfiguration) *CrictlConfig {
	var cfg *CrictlConfig

	if config == nil {
		cfg = newCrictlConfigFromFlags(ctx)
	} else {
		cfg = newCrictlConfigFromFile(ctx, config)
	}

	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	return cfg
}

func newCrictlConfigFromFlags(ctx *cli.Context) *CrictlConfig {
	cfg := &CrictlConfig{}

	cfg.RuntimeEndpoint = ctx.String("runtime-endpoint")

	if ctx.IsSet("runtime-endpoint") {
		cfg.RuntimeEndpointIsSet = true
	}

	cfg.ImageEndpoint = ctx.String("image-endpoint")

	if ctx.IsSet("image-endpoint") {
		cfg.ImageEndpointIsSet = true
	}

	if ctx.IsSet("timeout") {
		cfg.Timeout = getTimeout(ctx.Duration("timeout"))
	} else {
		cfg.Timeout = ctx.Duration("timeout")
	}

	cfg.Debug = ctx.Bool("debug")
	cfg.MaxRetries = ctx.Int("max-retries")
	cfg.DisablePullOnRun = false

	return cfg
}

func newCrictlConfigFromFile(ctx *cli.Context, config *common.ServerConfiguration) *CrictlConfig {
	cfg := &CrictlConfig{}

	// Command line flags overrides config file.
	if ctx.IsSet("runtime-endpoint") { //nolint:gocritic
		cfg.RuntimeEndpoint = ctx.String("runtime-endpoint")
		cfg.RuntimeEndpointIsSet = true
	} else if config.RuntimeEndpoint != "" {
		cfg.RuntimeEndpoint = config.RuntimeEndpoint
		cfg.RuntimeEndpointIsSet = true
	} else {
		cfg.RuntimeEndpoint = ctx.String("runtime-endpoint")
	}

	if ctx.IsSet("image-endpoint") { //nolint:gocritic
		cfg.ImageEndpoint = ctx.String("image-endpoint")
		cfg.ImageEndpointIsSet = true
	} else if config.ImageEndpoint != "" {
		cfg.ImageEndpoint = config.ImageEndpoint
		cfg.ImageEndpointIsSet = true
	} else {
		cfg.ImageEndpoint = ctx.String("image-endpoint")
	}

	if ctx.IsSet("timeout") { //nolint:gocritic
		cfg.Timeout = getTimeout(ctx.Duration("timeout"))
	} else if config.Timeout > 0 { // 0/neg value set to default timeout
		cfg.Timeout = config.Timeout
	} else {
		cfg.Timeout = ctx.Duration("timeout")
	}

	if ctx.IsSet("debug") {
		cfg.Debug = ctx.Bool("debug")
	} else {
		cfg.Debug = config.Debug
	}

	if ctx.IsSet("max-retries") {
		cfg.MaxRetries = ctx.Int("max-retries")
	} else {
		cfg.MaxRetries = config.MaxRetries
	}

	cfg.PullImageOnCreate = config.PullImageOnCreate
	cfg.DisablePullOnRun = config.DisablePullOnRun

	return cfg
}

// CrictlConfig holds all resolved runtime configuration for a crictl session.
type CrictlConfig struct {
	RuntimeEndpoint      string
	RuntimeEndpointIsSet bool
	ImageEndpoint        string
	ImageEndpointIsSet   bool
	Timeout              time.Duration
	Debug                bool
	PullImageOnCreate    bool
	DisablePullOnRun     bool
	MaxRetries           int
	TracerProvider       *sdktrace.TracerProvider
	// RootSpan is the root OpenTelemetry span for the command.
	RootSpan trace.Span

	runtimeServiceOverride internalapi.RuntimeService
	imageServiceOverride   internalapi.ImageManagerService
}

func configFromContext(ctx *cli.Context) *CrictlConfig {
	cfg, _ := ctx.App.Metadata[configKey].(*CrictlConfig)

	return cfg
}

// GetRuntimeService returns the runtime service client. If an override is set
// (for testing), it is returned directly. Otherwise a new gRPC connection is
// created using the configured endpoint and timeout.
func (cfg *CrictlConfig) GetRuntimeService(ctx context.Context, timeout time.Duration) (internalapi.RuntimeService, error) {
	if cfg.runtimeServiceOverride != nil {
		return cfg.runtimeServiceOverride, nil
	}

	if cfg.RuntimeEndpointIsSet && cfg.RuntimeEndpoint == "" {
		return nil, errors.New("--runtime-endpoint is not set")
	}

	logrus.Debug("Get runtime connection")

	t := cfg.Timeout
	if timeout != 0 {
		t = timeout
	}

	logrus.Debugf("Using runtime connection timeout: %v", t)

	var tp trace.TracerProvider = noop.NewTracerProvider()
	if cfg.TracerProvider != nil {
		tp = cfg.TracerProvider
	}

	if !cfg.RuntimeEndpointIsSet {
		logrus.Warningf("runtime connect using default endpoints: %v. "+
			"As the default settings are now deprecated, you should set the "+
			"endpoint instead.", defaultRuntimeEndpoints)
		logrus.Debug("Note that performance maybe affected as each default " +
			"connection attempt takes n-seconds to complete before timing out " +
			"and going to the next in sequence.")

		var (
			res internalapi.RuntimeService
			err error
		)

		for _, endPoint := range defaultRuntimeEndpoints {
			logrus.Debugf("Connect using endpoint %q with %q timeout", endPoint, t)

			res, err = remote.NewRemoteRuntimeService(ctx, endPoint, t, tp, false)
			if err != nil {
				logrus.Error(err)

				continue
			}

			logrus.Debugf("Connected successfully using endpoint: %s", endPoint)

			break
		}

		return res, err
	}

	return connectWithRetry(ctx, cfg.MaxRetries, func() (internalapi.RuntimeService, error) {
		return remote.NewRemoteRuntimeService(ctx, cfg.RuntimeEndpoint, t, tp, false)
	})
}

// GetImageService returns the image service client. If an override is set
// (for testing), it is returned directly. Otherwise a new gRPC connection is
// created using the configured endpoint and timeout.
func (cfg *CrictlConfig) GetImageService(ctx context.Context) (internalapi.ImageManagerService, error) {
	if cfg.imageServiceOverride != nil {
		return cfg.imageServiceOverride, nil
	}

	if cfg.ImageEndpoint == "" {
		if cfg.RuntimeEndpointIsSet && cfg.RuntimeEndpoint == "" {
			return nil, errors.New("--image-endpoint is not set")
		}

		cfg.ImageEndpoint = cfg.RuntimeEndpoint
		cfg.ImageEndpointIsSet = cfg.RuntimeEndpointIsSet
	}

	logrus.Debug("Get image connection")

	var tp trace.TracerProvider = noop.NewTracerProvider()
	if cfg.TracerProvider != nil {
		tp = cfg.TracerProvider
	}

	if !cfg.ImageEndpointIsSet {
		logrus.Warningf("Image connect using default endpoints: %v. "+
			"As the default settings are now deprecated, you should set the "+
			"endpoint instead.", defaultRuntimeEndpoints)
		logrus.Debug("Note that performance maybe affected as each default " +
			"connection attempt takes n-seconds to complete before timing out " +
			"and going to the next in sequence.")

		var (
			res internalapi.ImageManagerService
			err error
		)

		for _, endPoint := range defaultRuntimeEndpoints {
			logrus.Debugf("Connect using endpoint %q with %q timeout", endPoint, cfg.Timeout)

			res, err = remote.NewRemoteImageService(ctx, endPoint, cfg.Timeout, tp, false)
			if err != nil {
				logrus.Error(err)

				continue
			}

			logrus.Debugf("Connected successfully using endpoint: %s", endPoint)

			break
		}

		return res, err
	}

	return connectWithRetry(ctx, cfg.MaxRetries, func() (internalapi.ImageManagerService, error) {
		return remote.NewRemoteImageService(ctx, cfg.ImageEndpoint, cfg.Timeout, tp, false)
	})
}

func connectWithRetry[T any](
	ctx context.Context,
	maxRetries int,
	connect func() (T, error),
) (T, error) {
	result, err := connect()
	if err == nil || maxRetries == 0 {
		return result, err
	}

	delay := 500 * time.Millisecond
	maxDelay := 5 * time.Second

	for attempt := 0; maxRetries < 0 || attempt < maxRetries; attempt++ {
		if maxRetries > 0 {
			logrus.Debugf("Connection attempt %d/%d failed: %v, retrying in %v", attempt+1, maxRetries, err, delay)
		} else {
			logrus.Debugf("Connection attempt %d failed: %v, retrying in %v", attempt+1, err, delay)
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}

		result, err = connect()
		if err == nil {
			return result, nil
		}

		delay = min(delay*2, maxDelay)
	}

	return result, err
}
