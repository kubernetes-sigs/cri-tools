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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func fakeSandboxesWithCreatedAtDesc(names ...string) []*pb.PodSandbox {
	sandboxes := make([]*pb.PodSandbox, len(names))
	creationTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	for i, name := range names {
		sandboxes[i] = fakeSandbox(name, "", creationTime.UnixNano())
		creationTime = creationTime.Truncate(time.Hour)
	}

	return sandboxes
}

func fakeSandbox(name, namespace string, createdAt int64) *pb.PodSandbox {
	return &pb.PodSandbox{
		Metadata: &pb.PodSandboxMetadata{
			Name:      name,
			Namespace: namespace,
		},
		CreatedAt: createdAt,
	}
}

var _ = DescribeTable("convertPodState",
	func(state pb.PodSandboxState, expected string) {
		actual, err := convertPodState(state)
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).To(Equal(expected))
	},
	Entry("ready", pb.PodSandboxState_SANDBOX_READY, "Ready"),
	Entry("not ready", pb.PodSandboxState_SANDBOX_NOTREADY, "NotReady"),
)

var _ = DescribeTable("getSandboxesRuntimeHandler",
	func(handler string, expected string) {
		sandbox := &pb.PodSandbox{RuntimeHandler: handler}
		actual := getSandboxesRuntimeHandler(sandbox)
		Expect(actual).To(Equal(expected))
	},
	Entry("empty handler returns default", "", "(default)"),
	Entry("custom handler", "runc", "runc"),
	Entry("another handler", "kata", "kata"),
)

var _ = DescribeTable("getSandboxesList",
	func(input []*pb.PodSandbox, options *listOptions, indexes []int) {
		actual, err := getSandboxesList(input, options)
		Expect(err).NotTo(HaveOccurred())

		expected := make([]*pb.PodSandbox, 0, len(indexes))
		for _, i := range indexes {
			expected = append(expected, input[i])
		}

		Expect(actual).To(HaveExactElements(expected))
	},
	Entry("returns sandboxes in order by createdAt desc",
		[]*pb.PodSandbox{
			fakeSandbox("test0", "ns", time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC).UnixNano()),
			fakeSandbox("test1", "ns", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC).UnixNano()),
			fakeSandbox("test2", "ns", time.Date(2023, 1, 3, 12, 0, 0, 0, time.UTC).UnixNano()),
		},
		&listOptions{},
		[]int{2, 0, 1},
	),
	Entry("returns sandboxes filtered by name regexp",
		fakeSandboxesWithCreatedAtDesc("Test0", "Dev1", "Test2", "Dev3"),
		&listOptions{nameRegexp: "Test.*"},
		[]int{0, 2},
	),
	Entry("returns no sandboxes when regexp matches nothing",
		fakeSandboxesWithCreatedAtDesc("Test0", "Dev1"),
		&listOptions{nameRegexp: "Prod.*"},
		[]int{},
	),
	Entry("returns sandboxes filtered by namespace regexp",
		[]*pb.PodSandbox{
			fakeSandbox("a", "kube-system", time.Date(2023, 1, 3, 12, 0, 0, 0, time.UTC).UnixNano()),
			fakeSandbox("b", "default", time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC).UnixNano()),
			fakeSandbox("c", "kube-public", time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC).UnixNano()),
		},
		&listOptions{podNamespaceRegexp: "kube-.*"},
		[]int{0, 2},
	),
	Entry("returns the most recent sandbox with latest option",
		fakeSandboxesWithCreatedAtDesc("v0", "v1", "v2"),
		&listOptions{latest: true},
		[]int{0},
	),
	Entry("returns last n sandboxes with the last option",
		fakeSandboxesWithCreatedAtDesc("v0", "v1", "v2"),
		&listOptions{last: 2},
		[]int{0, 1},
	),
	Entry("returns all sandboxes when last exceeds length",
		fakeSandboxesWithCreatedAtDesc("v0", "v1"),
		&listOptions{last: 5},
		[]int{0, 1},
	),
	Entry("returns nothing when last is set and list is empty",
		fakeSandboxesWithCreatedAtDesc(),
		&listOptions{last: 2},
		[]int{},
	),
)
