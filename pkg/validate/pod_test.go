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

package validate

import "testing"

func TestMetricMayBeAbsent(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		metric   string
		cgroupV2 bool
		want     bool
	}{
		{
			name:     "required metric must be present on cgroup v2",
			metric:   "container_memory_rss",
			cgroupV2: true,
			want:     false,
		},
		{
			name:     "unimplemented metric may be absent",
			metric:   "container_ulimits_soft",
			cgroupV2: true,
			want:     true,
		},
		{
			name:     "cgroup v1-only metric may be absent on cgroup v2",
			metric:   "container_memory_failcnt",
			cgroupV2: true,
			want:     true,
		},
		{
			name:     "cgroup v1-only metric is required on cgroup v1",
			metric:   "container_memory_kernel_usage",
			cgroupV2: false,
			want:     false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := metricMayBeAbsent(tc.metric, tc.cgroupV2); got != tc.want {
				t.Errorf(
					"metricMayBeAbsent(%q, %v) = %v, want %v",
					tc.metric, tc.cgroupV2, got, tc.want,
				)
			}
		})
	}
}
