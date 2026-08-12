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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var runWithArgsForTestMu sync.Mutex

func TestConfigCommandCreatesMissingExplicitConfigOnSet(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "nested", "crictl.yaml")

	err := runWithArgsForTest(
		t,
		"crictl",
		"--config",
		configPath,
		"config",
		"--set",
		"debug=true",
	)
	if err != nil {
		t.Fatalf("run config --set: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	if !strings.Contains(string(content), "debug: true") {
		t.Fatalf("expected config file to contain debug=true, got:\n%s", string(content))
	}
}

func TestConfigCommandCreatesMissingExplicitConfigOnPositionalSet(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "nested", "crictl.yaml")

	err := runWithArgsForTest(
		t,
		"crictl",
		"--config",
		configPath,
		"config",
		"debug",
		"true",
	)
	if err != nil {
		t.Fatalf("run config positional set: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	if !strings.Contains(string(content), "debug: true") {
		t.Fatalf("expected config file to contain debug=true, got:\n%s", string(content))
	}
}

func TestConfigCommandListDoesNotCreateMissingExplicitConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "nested", "crictl.yaml")

	err := runWithArgsForTest(
		t,
		"crictl",
		"--config",
		configPath,
		"config",
		"--list",
	)
	if err == nil {
		t.Fatal("expected config --list to fail for a missing explicit config file")
	}

	_, err = os.Stat(configPath)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config file to remain absent, got err=%v", err)
	}
}

func runWithArgsForTest(t *testing.T, args ...string) error {
	t.Helper()

	runWithArgsForTestMu.Lock()
	t.Cleanup(func() {
		runWithArgsForTestMu.Unlock()
	})

	oldArgs := os.Args
	os.Args = args

	t.Cleanup(func() {
		os.Args = oldArgs
	})

	return run()
}
