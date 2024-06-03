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
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestNameFilterByRegex(t *testing.T) {
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
			"reguar expression should match",
			"iner$",
			"i_am_a_container",
			true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			r := matchesRegex(tc.pattern, tc.name)
			if r != tc.isMatch {
				t.Errorf("expected matched to be %v; actual result is %v", tc.isMatch, r)
			}
		})
	}
}

func TestOutputStatusInfo(t *testing.T) {
	const (
		statusResponse = `{"conditions":[{
				"message": "no network config found in C:\\Program Files",
        		"reason": "NetworkPluginNotReady",
        		"status": false,
        		"type": "NetworkReady"
      		}]}`
	)
	testCases := []struct {
		name        string
		status      string
		handlers    string
		info        map[string]string
		format      string
		tmplStr     string
		expectedOut string
	}{
		{
			name:        "YAML format",
			status:      statusResponse,
			handlers:    `{"handlers":["handler1","handler2"]}`,
			info:        map[string]string{"key1": "value1", "key2": "/var/lib"},
			format:      "yaml",
			tmplStr:     "",
			expectedOut: "key1: value1\nkey2: /var/lib\nruntimeHandlers:\n  handlers:\n  - handler1\n  - handler2\nstatus:\n  conditions:\n  - message: no network config found in C:\\Program Files\n    reason: NetworkPluginNotReady\n    status: false\n    type: NetworkReady",
		},
		{
			name:        "JSON format",
			status:      statusResponse,
			handlers:    `{"handlers":["handler1","handler2"]}`,
			info:        map[string]string{"key1": "\"value1\"", "key2": "\"C:\\ProgramFiles\""},
			format:      "json",
			tmplStr:     "",
			expectedOut: "{\n  \"key1\": \"value1\",\n  \"key2\": \"C:\\\\ProgramFiles\",\n  \"runtimeHandlers\": {\n    \"handlers\": [\n      \"handler1\",\n      \"handler2\"\n    ]\n  },\n  \"status\": {\n    \"conditions\": [\n      {\n        \"message\": \"no network config found in C:\\\\Program Files\",\n        \"reason\": \"NetworkPluginNotReady\",\n        \"status\": false,\n        \"type\": \"NetworkReady\"\n      }\n    ]\n  }\n}",
		},
		{
			name:        "Go template format",
			status:      statusResponse,
			handlers:    `{"handlers":["handler1","handler2"]}`,
			info:        map[string]string{"key1": "value1", "key2": "value2"},
			format:      "go-template",
			tmplStr:     `NetworkReady: {{ (index .status.conditions 0).status }}`,
			expectedOut: "NetworkReady: false",
		},
	}

	// Run tests
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
				err := outputStatusInfo(tc.status, tc.handlers, tc.info, tc.format, tc.tmplStr)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				return nil
			})

			if err != nil {
				Expect(err).To(BeNil())
			}

			if outStr != tc.expectedOut {
				t.Errorf("Expected output:\n%s\nGot:\n%s", tc.expectedOut, outStr)
			}
		})
	}
}
