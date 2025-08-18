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
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onsi/ginkgo/v2"
	ginkgotypes "github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"

	_ "sigs.k8s.io/cri-tools/pkg/benchmark"
	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
	_ "sigs.k8s.io/cri-tools/pkg/validate"
	versionconst "sigs.k8s.io/cri-tools/pkg/version"
)

const (
	parallelFlag  = "parallel"
	benchmarkFlag = "benchmark"
	versionFlag   = "version"
)

var (
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	isBenchMark = flag.Bool(benchmarkFlag, false, "Run benchmarks instead of validation tests")
	parallel    = flag.Int(parallelFlag, 1, "The number of parallel test nodes to run (default 1)")
	version     = flag.Bool(versionFlag, false, "Display version of critest")
)

func init() {
	framework.RegisterFlags()
	getConfigFromFile()
}

// Load server configuration from file and use each config settings if that
// option is not set in the CLI.
func getConfigFromFile() {
	var configFromFile *common.ServerConfiguration

	currentPath, _ := os.Getwd()
	configFromFile, _ = common.GetServerConfigFromFile(framework.TestContext.ConfigPath, currentPath)

	if configFromFile != nil {
		// Command line flags take precedence over config file.
		if !isFlagSet("runtime-endpoint") && configFromFile.RuntimeEndpoint != "" {
			framework.TestContext.RuntimeServiceAddr = configFromFile.RuntimeEndpoint
		}

		if !isFlagSet("image-endpoint") && configFromFile.ImageEndpoint != "" {
			framework.TestContext.ImageServiceAddr = configFromFile.ImageEndpoint
		}
	}
}

func isFlagSet(name string) bool {
	found := false

	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})

	return found
}

// runTestSuite runs cri validation tests and benchmark tests.
func runTestSuite(t *testing.T) {
	t.Helper()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "CRI validation")
}

func generateTempTestName() (string, error) {
	suffix := make([]byte, 10)
	for i := range suffix {
		suffix[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	dir, err := os.MkdirTemp("", "cri-test")
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "critest-"+string(suffix)+".test"), nil
}

func runParallelTestSuite(t *testing.T) {
	t.Helper()

	criPath, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to lookup path of critest: %v", err)
	}

	t.Logf("critest path: %s", criPath)

	tempFileName, err := generateTempTestName()
	if err != nil {
		t.Fatalf("Failed to generate temp test name: %v", err)
	}

	err = os.Symlink(criPath, tempFileName)
	if err != nil {
		t.Fatalf("Failed to lookup path of critest: %v", err)
	}

	defer os.Remove(tempFileName)

	ginkgoArgs, err := generateGinkgoRunFlags()
	if err != nil {
		t.Fatalf("Failed to generate ginkgo args: %v", err)
	}

	ginkgoArgs = append(ginkgoArgs, fmt.Sprintf("--nodes=%d", *parallel))

	var testArgs []string

	flag.Visit(func(f *flag.Flag) {
		// NOTE(fuweid):
		//
		// The ginkgo has changed the flag var from string to string slice
		// for some, like ginkgo.Skip/Focus.
		//
		// The --skip flag's config is https://github.com/onsi/ginkgo/blob/v2.0.0/types/config.go#L284.
		// And the value will be appended to https://github.com/onsi/ginkgo/blob/v2.0.0/types/config.go#L22.
		// The flag var is https://github.com/onsi/ginkgo/blob/v2.0.0/types/flags.go#L428,
		// which means that we can't get value by interface String().
		//
		// So we need to skip the "ginkgo.*" flags and use ginkgo API
		// to generate the flags.
		if strings.HasPrefix(f.Name, "ginkgo.") {
			return
		}

		if f.Name == parallelFlag || f.Name == benchmarkFlag {
			return
		}

		testArgs = append(testArgs, fmt.Sprintf("-%s=%s", f.Name, f.Value.String()))
	})

	var args []string

	args = append(args, ginkgoArgs...)
	args = append(args, tempFileName, "--")
	args = append(args, testArgs...)

	cmd := exec.CommandContext(t.Context(), "ginkgo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run tests in parallel: %v", err)
	}
}

func TestCRISuite(t *testing.T) {
	t.Parallel()
	fmt.Printf("critest version: %s\n", versionconst.Version)

	if *version {
		// print version only and exit
		return
	}

	if *isBenchMark {
		if err := flag.Set("ginkgo.focus", "benchmark"); err != nil {
			t.Fatalf("set ginkgo benchmark focus: %v", err)
		}

		if err := flag.Set("ginkgo.succinct", "true"); err != nil {
			t.Fatalf("set ginkgo succinct: %v", err)
		}
	} else {
		// Skip benchmark measurements for validation tests.
		if err := flag.Set("ginkgo.skip", "benchmark"); err != nil {
			t.Fatalf("skip ginkgo benchmarks: %v", err)
		}
	}

	if *parallel > 1 {
		runParallelTestSuite(t)
	} else {
		runTestSuite(t)
	}
}

// generateGinkgoRunFlags is based on ginkgotypes.GenerateGinkgoTestRunArgs.
//
// Since the GenerateGinkgoTestRunArgs adds "ginkgo." as prefix for each
// flags and we use --nodes instead of ParallelConfigFlags, we need to call
// GenerateFlagArgs to get what we want.
func generateGinkgoRunFlags() ([]string, error) {
	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()

	flags := ginkgotypes.SuiteConfigFlags
	flags = flags.CopyAppend(ginkgotypes.ReporterConfigFlags...)

	bindings := map[string]any{
		"S": &suiteConfig,
		"R": &reporterConfig,
	}

	return ginkgotypes.GenerateFlagArgs(flags, bindings)
}
