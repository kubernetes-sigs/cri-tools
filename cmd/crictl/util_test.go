/*
Copyright 2018 The Kubernetes Authors.

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
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"
)

func TestGetSortedKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		input    map[string]string
		expected []string
	}{
		{
			desc:     "returns keys in alphabetical order",
			input:    map[string]string{"c": "3", "a": "1", "b": "2"},
			expected: []string{"a", "b", "c"},
		},
		{
			desc:     "empty map returns empty slice",
			input:    map[string]string{},
			expected: []string{},
		},
		{
			desc:     "single key",
			input:    map[string]string{"only": "value"},
			expected: []string{"only"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			result := getSortedKeys(tc.input)
			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d keys, got %d", len(tc.expected), len(result))
			}

			for i, k := range tc.expected {
				if result[i] != k {
					t.Errorf("expected key[%d]=%q, got %q", i, k, result[i])
				}
			}
		})
	}
}

func TestNameFilterByRegex(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		pattern string
		name    string
		isMatch bool
	}{
		{
			"exact name should match",
			"i_am_a_container",
			"i_am_a_container",
			true,
		},
		{
			"prefix should match",
			"i_am",
			"i_am_a_container",
			true,
		},
		{
			"empty pattern should match",
			"",
			"i_am_a_container",
			true,
		},
		{
			"unmatched pattern should not match",
			"foo",
			"i_am_a_container",
			false,
		},
		{
			"regular expression should match",
			"iner$",
			"i_am_a_container",
			true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			r := matchesRegex(tc.pattern, tc.name)
			if r != tc.isMatch {
				t.Errorf("expected matched to be %v; actual result is %v", tc.isMatch, r)
			}
		})
	}
}

//nolint:paralleltest //should not run in parallel
func TestOutputStatusData(t *testing.T) {
	const (
		statusResponse = `{"conditions":[
			{
				"message": "no network config found in C:\\Program Files",
				"reason": "NetworkPluginNotReady",
				"status": false,
				"type": "NetworkReady"
			}
		]}`
		handlerResponse = `[
			{
				"features": {
					"recursive_read_only_mounts": true
				},
				"name": "runc"
			},
			{
				"features": {
					"recursive_read_only_mounts": true,
					"user_namespaces": true
				},
				"name": "crun"
			}
		]`
		featuresResponse = `{
			"supplemental_groups_policy": true
		}`
		emptyResponse = ""
	)

	testCases := []struct {
		name        string
		status      string
		handlers    string
		features    string
		info        map[string]string
		format      string
		tmplStr     string
		expectedOut string
	}{
		{
			name:        "YAML format",
			status:      statusResponse,
			handlers:    handlerResponse,
			features:    featuresResponse,
			info:        map[string]string{"key1": `{"foo": "bar"}`, "key2": `{"bar": "baz"}`},
			format:      outputTypeYAML,
			tmplStr:     "",
			expectedOut: "features:\n  supplemental_groups_policy: true\nkey1:\n  foo: bar\nkey2:\n  bar: baz\nruntimeHandlers:\n- features:\n    recursive_read_only_mounts: true\n  name: runc\n- features:\n    recursive_read_only_mounts: true\n    user_namespaces: true\n  name: crun\nstatus:\n  conditions:\n  - message: no network config found in C:\\Program Files\n    reason: NetworkPluginNotReady\n    status: false\n    type: NetworkReady",
		},
		{
			name:        "YAML format with empty status response",
			status:      emptyResponse,
			handlers:    handlerResponse,
			format:      outputTypeYAML,
			tmplStr:     "",
			expectedOut: "runtimeHandlers:\n- features:\n    recursive_read_only_mounts: true\n  name: runc\n- features:\n    recursive_read_only_mounts: true\n    user_namespaces: true\n  name: crun",
		},
		{
			name:        "YAML format with empty handlers response",
			status:      statusResponse,
			handlers:    emptyResponse,
			format:      outputTypeYAML,
			tmplStr:     "",
			expectedOut: "status:\n  conditions:\n  - message: no network config found in C:\\Program Files\n    reason: NetworkPluginNotReady\n    status: false\n    type: NetworkReady",
		},
		{
			name:        "YAML format with empty features response",
			status:      statusResponse,
			handlers:    handlerResponse,
			features:    emptyResponse,
			format:      outputTypeYAML,
			tmplStr:     "",
			expectedOut: "runtimeHandlers:\n- features:\n    recursive_read_only_mounts: true\n  name: runc\n- features:\n    recursive_read_only_mounts: true\n    user_namespaces: true\n  name: crun\nstatus:\n  conditions:\n  - message: no network config found in C:\\Program Files\n    reason: NetworkPluginNotReady\n    status: false\n    type: NetworkReady",
		},
		{
			name:        "JSON format",
			status:      statusResponse,
			handlers:    handlerResponse,
			format:      outputTypeJSON,
			tmplStr:     "",
			expectedOut: "{\n  \"runtimeHandlers\": [\n    {\n      \"features\": {\n        \"recursive_read_only_mounts\": true\n      },\n      \"name\": \"runc\"\n    },\n    {\n      \"features\": {\n        \"recursive_read_only_mounts\": true,\n        \"user_namespaces\": true\n      },\n      \"name\": \"crun\"\n    }\n  ],\n  \"status\": {\n    \"conditions\": [\n      {\n        \"message\": \"no network config found in C:\\\\Program Files\",\n        \"reason\": \"NetworkPluginNotReady\",\n        \"status\": false,\n        \"type\": \"NetworkReady\"\n      }\n    ]\n  }\n}",
		},
		{
			name:        "Go template format",
			status:      statusResponse,
			handlers:    handlerResponse,
			format:      outputTypeGoTemplate,
			tmplStr:     `NetworkReady: {{ (index .status.conditions 0).status }}`,
			expectedOut: "NetworkReady: false",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			captureOutput := func(f func() error) (string, error) {
				var err error

				old := os.Stdout

				r, w, _ := os.Pipe()
				os.Stdout = w

				defer func() {
					os.Stdout = old
				}()

				err = f()
				if err != nil {
					return "", err
				}

				err = w.Close()
				if err != nil {
					return "", err
				}

				out, err := io.ReadAll(r)

				return strings.TrimRight(string(out), "\n"), err
			}

			outStr, err := captureOutput(func() error {
				data := []statusData{{json: tc.status, runtimeHandlers: tc.handlers, features: tc.features, info: tc.info}}

				err := outputStatusData(data, tc.format, tc.tmplStr)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				return nil
			})
			if err != nil {
				Expect(err).ToNot(HaveOccurred())
			}

			if outStr != tc.expectedOut {
				t.Errorf("Expected output:\n%s\nGot:\n%s", tc.expectedOut, outStr)
			}
		})
	}
}

func TestLoadContainerConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expError bool
	}{
		{
			name: "valid yaml",
			content: `
metadata:
  name: test-container
image: { image: "busybox" }
`,
		},
		{
			name:    "valid json",
			content: `{"metadata": {"name": "test-container"}, "image": {"image": "busybox"}}`,
		},
		{
			name:     "missing metadata",
			content:  "image: { image: \"busybox\" }",
			expError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpfile, err := os.CreateTemp(t.TempDir(), "container-config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}

			if _, err := tmpfile.WriteString(tt.content); err != nil {
				t.Fatal(err)
			}

			tmpfile.Close()

			config, err := loadContainerConfig(tmpfile.Name())
			if tt.expError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if config == nil {
					t.Fatal("expected config, got nil")
				}

				if config.GetMetadata().GetName() != "test-container" {
					t.Errorf("expected name %q, got %q", "test-container", config.GetMetadata().GetName())
				}

				if config.GetImage().GetImage() != "busybox" {
					t.Errorf("expected image %q, got %q", "busybox", config.GetImage().GetImage())
				}
			}
		})
	}
}

func TestLoadContainerConfigNotFound(t *testing.T) {
	t.Parallel()

	const filename = "non-existent-file.yaml"

	_, err := loadContainerConfig(filename)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error message to contain %q, got %q", "not found", err.Error())
	}

	if !strings.Contains(err.Error(), filename) {
		t.Errorf("expected error message to contain %q, got %q", filename, err.Error())
	}
}

func TestLoadPodSandboxConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expError bool
	}{
		{
			name: "valid yaml",
			content: `
metadata:
  name: test-pod
  namespace: default
`,
		},
		{
			name: "valid json",
			content: `
{
  "metadata": {
    "name": "test-pod",
    "namespace": "default"
  }
}
`,
		},
		{
			name: "missing namespace",
			content: `
metadata:
  name: test-pod
`,
			expError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpfile, err := os.CreateTemp(t.TempDir(), "pod-config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}

			if _, err := tmpfile.WriteString(tt.content); err != nil {
				t.Fatal(err)
			}

			tmpfile.Close()

			config, err := loadPodSandboxConfig(tmpfile.Name())
			if tt.expError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if config == nil {
					t.Fatal("expected config, got nil")
				}

				if config.GetMetadata().GetName() != "test-pod" {
					t.Errorf("expected name %q, got %q", "test-pod", config.GetMetadata().GetName())
				}

				if config.GetMetadata().GetNamespace() != "default" {
					t.Errorf("expected namespace %q, got %q", "default", config.GetMetadata().GetNamespace())
				}

				if config.GetMetadata().GetUid() == "" {
					t.Error("expected UID to be non-empty")
				}
			}
		})
	}
}

func TestLoadPodSandboxConfigNotFound(t *testing.T) {
	t.Parallel()

	const filename = "non-existent-file.yaml"

	_, err := loadPodSandboxConfig(filename)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error message to contain %q, got %q", "not found", err.Error())
	}

	if !strings.Contains(err.Error(), filename) {
		t.Errorf("expected error message to contain %q, got %q", filename, err.Error())
	}
}

func TestTruncateCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		total    int
		latest   bool
		last     int
		expected int
	}{
		{"zero items", 0, false, 0, 0},
		{"no truncation", 5, false, 0, 5},
		{"latest", 5, true, 0, 1},
		{"latest with empty", 0, true, 0, 0},
		{"last 3", 5, false, 3, 3},
		{"last exceeds total", 5, false, 10, 5},
		{"last overrides latest", 5, true, 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := truncateCount(tt.total, tt.latest, tt.last)
			if got != tt.expected {
				t.Errorf("truncateCount(%d, %v, %d) = %d, want %d",
					tt.total, tt.latest, tt.last, got, tt.expected)
			}
		})
	}
}

type fakeItem struct{ id string }

func (f fakeItem) GetId() string { return f.id } //nolint:staticcheck // matches protobuf GetId() interface

func newCLIContext(args []string, all bool) *cli.Context {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("all", false, "")

	var setArgs []string
	if all {
		setArgs = append(setArgs, "--all")
	}

	setArgs = append(setArgs, args...)
	_ = fs.Parse(setArgs)

	app := &cli.App{Writer: io.Discard}

	return cli.NewContext(app, fs, nil)
}

func TestCollectIDs(t *testing.T) {
	t.Parallel()

	errList := errors.New("list failed")

	tests := []struct {
		name        string
		args        []string
		all         bool
		listFn      func(context.Context) ([]fakeItem, error)
		expectedIDs []string
		expectErr   bool
	}{
		{
			name: "explicit args returned as-is",
			args: []string{"id1", "id2"},
			listFn: func(context.Context) ([]fakeItem, error) {
				t.Fatal("listFn should not be called")

				return nil, nil
			},
			expectedIDs: []string{"id1", "id2"},
		},
		{
			name: "all flag collects IDs from listFn",
			all:  true,
			listFn: func(context.Context) ([]fakeItem, error) {
				return []fakeItem{{id: "a"}, {id: "b"}, {id: "c"}}, nil
			},
			expectedIDs: []string{"a", "b", "c"},
		},
		{
			name: "all flag with empty list returns nil",
			all:  true,
			listFn: func(context.Context) ([]fakeItem, error) {
				return nil, nil
			},
		},
		{
			name: "all flag with list error",
			all:  true,
			listFn: func(context.Context) ([]fakeItem, error) {
				return nil, errList
			},
			expectErr: true,
		},
		{
			name: "no args and no all flag returns error",
			listFn: func(context.Context) ([]fakeItem, error) {
				t.Fatal("listFn should not be called")

				return nil, nil
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cliCtx := newCLIContext(tt.args, tt.all)

			ids, err := collectIDs(context.Background(), cliCtx, tt.listFn, "thing")
			if tt.expectErr {
				if err == nil && ids != nil {
					t.Fatal("expected error or nil result, got ids")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(ids) != len(tt.expectedIDs) {
				t.Fatalf("expected %d ids, got %d", len(tt.expectedIDs), len(ids))
			}

			for i, id := range ids {
				if id != tt.expectedIDs[i] {
					t.Errorf("ids[%d] = %q, want %q", i, id, tt.expectedIDs[i])
				}
			}
		})
	}
}
