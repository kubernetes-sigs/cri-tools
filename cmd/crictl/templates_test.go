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

package main

import (
	"testing"
)

func TestTmplExecuteRawJSON(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		rawJSON  string
		tmplStr  string
		expected string
		hasErr   bool
	}{
		{
			rawJSON:  `{"ImageID": "abc", "Tag": "latest", "Size": 123456}`,
			tmplStr:  "{{.Size}}",
			expected: "123456",
		},
		{
			rawJSON:  `{"ImageID": "abcd", "Tag": "v1.0", "Size": 123456}`,
			tmplStr:  "{{.Size}} , {{.ImageID}}",
			expected: "123456 , abcd",
		},
		{
			rawJSON:  `{"ImageID": "aBcd", "Tag": "v1.0", "Size": 123456}`,
			tmplStr:  "{{title .ImageID}} {{lower .ImageID}} {{upper .ImageID}}",
			expected: "ABcd abcd ABCD",
		},
		{
			rawJSON: `{"ImageID": "aBcd", "Tag": "v1.0", "Size": 123456}`,
			tmplStr: "{{ .ImageName }}",
			hasErr:  true, // missing key
		},
	}

	for _, tc := range testcases {
		got, err := tmplExecuteRawJSON(tc.tmplStr, tc.rawJSON)
		if (err != nil) != tc.hasErr {
			t.Errorf("expected hasErr=%v, but got error=%v", tc.hasErr, err)
		}

		if tc.hasErr {
			continue
		}

		if got != tc.expected {
			t.Errorf("expected result=%v, but got=%v", tc.expected, got)
		}
	}
}
