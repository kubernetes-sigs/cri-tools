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

package framework

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/onsi/ginkgo/v2"
)

var (
	testImagesFilePath       string
	benchmarkSettingFilePath string
)

// TestImageList aggregates references to the images used in tests.
type TestImageList struct {
	DefaultTestContainerImage string `yaml:"defaultTestContainerImage"`
	WebServerTestImage        string `yaml:"webServerTestImage"`
}

// BenchmarkingParamsType is the type of benchmarking-related params.
type BenchmarkingParamsType struct {
	// ContainersNumber is the number of Containers to run as part of
	// the container-related benchmarks.
	ContainersNumber int `yaml:"containersNumber"`

	// ContainersNumberParallel is the maximum number of container-related benchmarks
	// to run in parallel.
	ContainersNumberParallel int `yaml:"containersNumberParallel"`

	// ContainerBenchmarkTimeoutSeconds is the maximum number of seconds acceptable
	// for a Container lifecycle benchmark to take.
	ContainerBenchmarkTimeoutSeconds int `yaml:"containerBenchmarkTimeoutSeconds"`

	// PodsNumber is the number of Pods to run as part of the pod-related benchmarks.
	PodsNumber int `yaml:"podsNumber"`

	// PodsNumberParallel is the maximum number of pod -related benchmarks
	// to run in parallel.
	PodsNumberParallel int `yaml:"podsNumberParallel"`

	// PodBenchmarkTimeoutSeconds is the maximum number of seconds acceptable
	// for a Pod lifecycle benchmark to take.
	PodBenchmarkTimeoutSeconds int `yaml:"podBenchmarkTimeoutSeconds"`

	// ImagesNumber is the number of Images to run tests on in image-related benchmarks.
	ImagesNumber int `yaml:"imagesNumber"`

	// ImagesNumberParallel is the maximum number of image-related benchmarks
	// to run in parallel.
	ImagesNumberParallel int `yaml:"imagesNumberParallel"`

	// ImageBenchmarkTimeoutSeconds is the maximum of seconds acceptable for
	// image-related benchmarks.
	ImageBenchmarkTimeoutSeconds int `yaml:"imageBenchmarkTimeoutSeconds"`

	// ImagePullingBenchmarkImage is the string ref to the image to be used in
	// image pulling benchmarks. Internally defaults to BusyBox.
	ImagePullingBenchmarkImage string `yaml:"imagePullingBenchmarkImage"`

	// ImageListingBenchmarkImages is a list of string image refs to query
	// during image listing benchmarks.
	ImageListingBenchmarkImages []string `yaml:"imageListingBenchmarkImages"`

	// ImageBenchmarkTimeoutSeconds is the maximum of seconds acceptable for
	// benchmarks focused on Pod+Container start performance.
	PodContainerStartBenchmarkTimeoutSeconds int `yaml:"podContainerStartBenchmarkTimeoutSeconds"`
}

// TestContextType is the type of test context.
type TestContextType struct {
	// Report related settings.
	ReportDir    string
	ReportPrefix string

	// CRI client configurations.
	ConfigPath            string
	ImageServiceAddr      string
	ImageServiceTimeout   time.Duration
	RuntimeServiceAddr    string
	RuntimeServiceTimeout time.Duration
	RuntimeHandler        string

	// Test images-related settings.
	TestImageList TestImageList

	// Benchmarking settings.
	BenchmarkingOutputDir string
	BenchmarkingParams    BenchmarkingParamsType

	// Test configuration.
	IsLcow bool

	RegistryPrefix string

	// Use websocket connections over than SPDY for streaming tests.
	UseWebsocketForExec        bool
	UseWebsocketForAttach      bool
	UseWebsocketForPortForward bool
}

// TestContext is a test context.
var TestContext TestContextType

const (
	// DefaultRegistryPrefix specifies the default prefix used for images.
	DefaultRegistryPrefix = "registry.k8s.io"

	// DefaultRegistryE2ETestImagesPrefix is the default prefix for e2e test images.
	DefaultRegistryE2ETestImagesPrefix = DefaultRegistryPrefix + "/e2e-test-images/"

	ContainerdSockPathUnix    = "unix:///run/containerd/containerd.sock"
	ContainerdSockPathWindows = "npipe:////./pipe/containerd-containerd"

	OSWindows = "windows"
)

// RegisterFlags registers flags to e2e test suites.
func RegisterFlags() {
	suite, reporter := ginkgo.GinkgoConfiguration()

	// Turn on verbose by default to get spec names
	reporter.Verbose = true

	// Turn on ShowNodeEvents to get spec progress (especially on interrupt)
	reporter.ShowNodeEvents = true

	// Randomize specs as well as suites
	suite.RandomizeAllSpecs = true

	flag.StringVar(&TestContext.ReportPrefix, "report-prefix", "", "Optional prefix for JUnit XML reports. Default is empty, which doesn't prepend anything to the default name.")
	flag.StringVar(&TestContext.ReportDir, "report-dir", "", "Path to the directory where the JUnit XML reports should be saved. Default is empty, which doesn't generate these reports.")
	flag.StringVar(&TestContext.ImageServiceAddr, "image-endpoint", "", "Image service socket for client to connect.")
	flag.StringVar(&testImagesFilePath, "test-images-file", "", "Optional path to a YAML file containing references to custom container images to be used in tests.")
	flag.DurationVar(&TestContext.ImageServiceTimeout, "image-service-timeout", 300*time.Second, "Timeout when trying to connect to image service.")

	svcaddr := ContainerdSockPathUnix
	defaultConfigPath := "/etc/crictl.yaml"

	if runtime.GOOS == OSWindows {
		svcaddr = ContainerdSockPathWindows
		defaultConfigPath = filepath.Join(os.Getenv("USERPROFILE"), ".crictl", "crictl.yaml")
	}

	flag.StringVar(&TestContext.ConfigPath, "config", defaultConfigPath, "Location of the client config file. If not specified and the default does not exist, the program's directory is searched as well")
	flag.StringVar(&TestContext.RuntimeServiceAddr, "runtime-endpoint", svcaddr, "Runtime service socket for client to connect.")
	flag.DurationVar(&TestContext.RuntimeServiceTimeout, "runtime-service-timeout", 300*time.Second, "Timeout when trying to connect to a runtime service.")
	flag.StringVar(&TestContext.RuntimeHandler, "runtime-handler", "", "Runtime handler to use in the test.")

	flag.StringVar(&benchmarkSettingFilePath, "benchmarking-params-file", "", "Optional path to a YAML file specifying benchmarking configuration options.")
	flag.StringVar(&TestContext.BenchmarkingOutputDir, "benchmarking-output-dir", "", "Optional path to a directory in which benchmarking data should be placed.")

	if runtime.GOOS == OSWindows {
		flag.BoolVar(&TestContext.IsLcow, "lcow", false, "Run Linux container on Windows tests instead of Windows container tests")
	} else {
		TestContext.IsLcow = false
	}

	flag.StringVar(&TestContext.RegistryPrefix, "registry-prefix", DefaultRegistryPrefix, "A possible registry prefix added to all images, like 'localhost:5000'")

	flag.BoolVar(&TestContext.UseWebsocketForExec, "websocket-exec", false, "Use websocket connections over SPDY for exec streaming tests.")
	flag.BoolVar(&TestContext.UseWebsocketForAttach, "websocket-attach", false, "Use websocket connections over SPDY for attach streaming tests.")
	flag.BoolVar(&TestContext.UseWebsocketForPortForward, "websocket-portforward", false, "Use websocket connections over SPDY for portforward streaming tests.")
}

// LoadYamlConfigFiles loads any external file-based parameters into the TestContextType.
func (tc *TestContextType) LoadYamlConfigFiles() error {
	// Attempt to load custom images file:
	if testImagesFilePath != "" {
		err := LoadYamlFile(testImagesFilePath, &TestContext.TestImageList)
		if err != nil {
			return fmt.Errorf("error loading custom test images file: %w", err)
		}
	}

	Logf("Testing context container image list: %+v", TestContext.TestImageList)

	// Attempt to load benchmark settings file:
	if benchmarkSettingFilePath != "" {
		err := LoadYamlFile(benchmarkSettingFilePath, &TestContext.BenchmarkingParams)
		if err != nil {
			return err
		}
	}

	Logf("Testing context benchmarking params: %+v", TestContext.BenchmarkingParams)

	return nil
}
