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

	"github.com/onsi/ginkgo/config"
	"gopkg.in/yaml.v3"
)

// TestImageList aggregates references to the images used in tests.
type TestImageList struct {
	DefaultTestContainerImage string `yaml:"defaultTestContainerImage"`
	WebServerTestImage        string `yaml:"webServerTestImage"`
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
	TestImagesFilePath string
	TestImageList      TestImageList

	// Benchmark setting.
	Number int

	// Test configuration.
	IsLcow bool

	RegistryPrefix string
}

// TestContext is a test context.
var TestContext TestContextType

// DefaultRegistryPrefix specifies the default prefix used for images
const DefaultRegistryPrefix = "docker.io/library"

// RegisterFlags registers flags to e2e test suites.
func RegisterFlags() {
	// Turn on verbose by default to get spec names
	config.DefaultReporterConfig.Verbose = true

	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	config.GinkgoConfig.EmitSpecProgress = true

	// Randomize specs as well as suites
	config.GinkgoConfig.RandomizeAllSpecs = true

	flag.StringVar(&TestContext.ReportPrefix, "report-prefix", "", "Optional prefix for JUnit XML reports. Default is empty, which doesn't prepend anything to the default name.")
	flag.StringVar(&TestContext.ReportDir, "report-dir", "", "Path to the directory where the JUnit XML reports should be saved. Default is empty, which doesn't generate these reports.")
	flag.StringVar(&TestContext.ImageServiceAddr, "image-endpoint", "", "Image service socket for client to connect.")
	flag.StringVar(&TestContext.TestImagesFilePath, "test-images-file", "", "Optional path to a YAML file containing references to custom container images to be used in tests.")
	flag.DurationVar(&TestContext.ImageServiceTimeout, "image-service-timeout", 300*time.Second, "Timeout when trying to connect to image service.")

	svcaddr := "unix:///var/run/dockershim.sock"
	defaultConfigPath := "/etc/crictl.yaml"
	if runtime.GOOS == "windows" {
		svcaddr = "npipe:////./pipe/dockershim"
		defaultConfigPath = filepath.Join(os.Getenv("USERPROFILE"), ".crictl", "crictl.yaml")
	}
	flag.StringVar(&TestContext.ConfigPath, "config", defaultConfigPath, "Location of the client config file. If not specified and the default does not exist, the program's directory is searched as well")
	flag.StringVar(&TestContext.RuntimeServiceAddr, "runtime-endpoint", svcaddr, "Runtime service socket for client to connect.")
	flag.DurationVar(&TestContext.RuntimeServiceTimeout, "runtime-service-timeout", 300*time.Second, "Timeout when trying to connect to a runtime service.")
	flag.StringVar(&TestContext.RuntimeHandler, "runtime-handler", "", "Runtime handler to use in the test.")
	flag.IntVar(&TestContext.Number, "number", 5, "Number of PodSandbox/container in listing benchmark test.")

	if runtime.GOOS == "windows" {
		flag.BoolVar(&TestContext.IsLcow, "lcow", false, "Run Linux container on Windows tests instead of Windows container tests")
	} else {
		TestContext.IsLcow = false
	}
	flag.StringVar(&TestContext.RegistryPrefix, "registry-prefix", DefaultRegistryPrefix, "A possible registry prefix added to all images, like 'localhost:5000/'")
}

// Loads the custom images mapping file (if defined) into the TestContextType.
func (tc TestContextType) LoadCustomImagesFileIntoTestingContext() error {
	Logf("Testing context container image list file: %s", TestContext.TestImagesFilePath)
	if TestContext.TestImagesFilePath != "" {
		fileContent, err := os.ReadFile(TestContext.TestImagesFilePath)
		if err != nil {
			return fmt.Errorf("error reading '%v' file contents: %v", TestContext.TestImagesFilePath, err)
		}

		err = yaml.Unmarshal(fileContent, &TestContext.TestImageList)
		if err != nil {
			return fmt.Errorf("error unmarshalling '%v' YAML file: %v", TestContext.TestImagesFilePath, err)
		}
	}

	Logf("Testing context container image list: %+v", TestContext.TestImageList)
	return nil
}
