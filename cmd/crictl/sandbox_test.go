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
	"testing"
)

func TestPodFilterByRegex(t *testing.T) {
	testCases := []struct {
		desc      string
		pattern   string
		podString string
		isMatch   bool
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
			r := podMatchesRegex(tc.pattern, tc.podString)
			if r != tc.isMatch {
				t.Errorf("expected matched to be %v; actual result is %v", tc.isMatch, r)
			}

		})
	}
}
