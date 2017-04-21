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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang/glog"
)

var buildTargets = []string{
	"github.com/kubernetes-incubator/cri-tools/pkg/validate",
	"github.com/kubernetes-incubator/cri-tools/vendor/github.com/onsi/ginkgo/ginkgo",
}

// build cri-tools binaries.
func build() error {
	glog.Infof("Building cri-tools binaries...")
	outputDir, err := getBuildOutputDir()
	if err != nil {
		glog.Fatalf("Failed to get build output directory: %v", err)
	}

	err = runCommand("go", "test", "-c", "-v", "-o", filepath.Join(outputDir, "e2e.test"), buildTargets[0])
	if err != nil {
		return fmt.Errorf("failed to build e2e.test %v", err)
	}

	err = runCommand("go", "build", "-o", filepath.Join(outputDir, "ginkgo"), buildTargets[1])
	if err != nil {
		return fmt.Errorf("failed to build go ginkgo %v", err)
	}
	return nil
}

// runCommand runs given command.
func runCommand(name string, args ...string) error {
	glog.Infof("Building command: %q %q", name, strings.Join(args, " "))
	cmd := exec.Command("sh", "-c", strings.Join(append([]string{name}, args...), " "))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getRootDir() (string, error) {
	// Get the directory of the current executable
	_, testExec, _, _ := runtime.Caller(0)
	path := filepath.Dir(testExec)

	// Look for the cri-tools source root directory
	if strings.Contains(path, "github.com/kubernetes-incubator/cri-tools") {
		splitPath := strings.Split(path, "github.com/kubernetes-incubator/cri-tools")
		return filepath.Join(splitPath[0], "github.com/kubernetes-incubator/cri-tools/"), nil
	}

	return "", fmt.Errorf("could not find cri-tools source root directory")
}

// getBuildOutputDir gets the dir which stores cri-tools binaries.
func getBuildOutputDir() (string, error) {
	k8sRoot, err := getRootDir()
	if err != nil {
		return "", err
	}
	buildOutputDir := filepath.Join(k8sRoot, "_output/bin")
	if err := os.MkdirAll(buildOutputDir, 0755); err != nil {
		return "", err
	}
	return buildOutputDir, nil
}

func parseFlag(rawFlag string) string {
	var flags []string

	list := strings.Split(rawFlag, "-")
	for _, flag := range list {
		if flag == "" {
			continue
		}
		if strings.Contains(flag, "=") {
			keyAndValue := strings.Split(flag, "=")
			flags = append(flags, "-"+strings.TrimSpace(keyAndValue[0])+"=\""+strings.TrimSpace(keyAndValue[1])+"\"")
		} else {
			keyAndValue := strings.SplitN(flag, " ", 2)
			flags = append(flags, "-"+strings.TrimSpace(keyAndValue[0])+"=\""+strings.TrimSpace(keyAndValue[1])+"\"")
		}
	}
	parsedFlag := strings.Join(flags, " ")
	return parsedFlag
}
