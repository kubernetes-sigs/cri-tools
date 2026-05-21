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
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestNewLogOptions(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		opts := NewLogOptions(false, false, time.Time{}, nil, nil)
		g := NewWithT(t)
		g.Expect(opts.Follow).To(BeFalse())
		g.Expect(opts.Timestamp).To(BeFalse())
		g.Expect(opts.Since).To(Equal(time.Time{}))
		g.Expect(opts.TailLines).To(BeNil())
		g.Expect(opts.LimitBytes).To(BeNil())
	})

	t.Run("follow and timestamps enabled", func(t *testing.T) {
		t.Parallel()

		opts := NewLogOptions(true, true, time.Time{}, nil, nil)
		g := NewWithT(t)
		g.Expect(opts.Follow).To(BeTrue())
		g.Expect(opts.Timestamp).To(BeTrue())
	})

	t.Run("since is set when non-zero", func(t *testing.T) {
		t.Parallel()

		since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		opts := NewLogOptions(false, false, since, nil, nil)
		g := NewWithT(t)
		g.Expect(opts.Since).To(Equal(since))
	})

	t.Run("positive tail lines", func(t *testing.T) {
		t.Parallel()

		var lines int64 = 100

		opts := NewLogOptions(false, false, time.Time{}, &lines, nil)
		g = NewWithT(t)
		g.Expect(opts.TailLines).To(Equal(&lines))
	})

	t.Run("negative tail lines are ignored", func(t *testing.T) {
		t.Parallel()

		var lines int64 = -1

		opts := NewLogOptions(false, false, time.Time{}, &lines, nil)
		g := NewWithT(t)
		g.Expect(opts.TailLines).To(BeNil())
	})

	t.Run("positive limit bytes", func(t *testing.T) {
		t.Parallel()

		var limit int64 = 1024

		opts := NewLogOptions(false, false, time.Time{}, nil, &limit)
		g := NewWithT(t)
		g.Expect(opts.LimitBytes).To(Equal(&limit))
	})

	t.Run("negative limit bytes are ignored", func(t *testing.T) {
		t.Parallel()

		var limit int64 = -1

		opts := NewLogOptions(false, false, time.Time{}, nil, &limit)
		g := NewWithT(t)
		g.Expect(opts.LimitBytes).To(BeNil())
	})

	t.Run("zero tail lines are accepted", func(t *testing.T) {
		t.Parallel()

		var lines int64

		opts := NewLogOptions(false, false, time.Time{}, &lines, nil)
		g := NewWithT(t)
		g.Expect(opts.TailLines).To(Equal(&lines))
	})
}

func TestParseTimestamp(t *testing.T) {
	t.Parallel()

	now := time.Now()

	testCases := []struct {
		desc     string
		input    string
		expected time.Time
		expError bool
	}{
		{
			desc:     "empty input returns zero time",
			input:    "",
			expected: time.Time{},
			expError: false,
		},
		{
			desc:     "relative duration (10m)",
			input:    "10m",
			expected: now.Add(-10 * time.Minute),
			expError: false,
		},
		{
			desc:     "RFC3339 timestamp",
			input:    "2023-01-01T12:00:00Z",
			expected: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			expError: false,
		},
		{
			desc:     "Unix timestamp",
			input:    "1672574400",
			expected: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			expError: false,
		},
		{
			desc:     "invalid input",
			input:    "invalid",
			expected: time.Time{},
			expError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			actual, err := parseTimestamp(tc.input)
			if tc.expError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				if tc.input == "10m" {
					// Relative time is hard to match exactly, so we check if it's within a reasonable range
					g.Expect(actual.Unix()).To(BeNumerically("~", tc.expected.Unix(), 5))
				} else {
					g.Expect(actual.Unix()).To(Equal(tc.expected.Unix()))
				}
			}
		})
	}
}
