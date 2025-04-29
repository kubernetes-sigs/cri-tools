/*
Copyright 2023 The Kubernetes Authors.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubelet/pkg/types"
)

// fakeContainersWithCreatedAtDesc creates fake containers in the least recent order of the createdAt.
func fakeContainersWithCreatedAtDesc(names ...string) []*pb.Container {
	containers := make([]*pb.Container, len(names))
	creationTime := time.Date(2023, 1, 1, 12, 0o0, 0o0, 0o0, time.UTC)

	for i, name := range names {
		containers[i] = fakeContainer(name, creationTime.UnixNano())
		creationTime = creationTime.Truncate(time.Hour)
	}

	return containers
}

func fakeContainer(name string, createdAt int64) *pb.Container {
	return &pb.Container{
		Metadata: &pb.ContainerMetadata{
			Name: name,
		},
		CreatedAt: createdAt,
	}
}

var _ = DescribeTable("getContainersList",
	func(input []*pb.Container, options *listOptions, indexes []int) {
		actual, err := getContainersList(context.Background(), nil, input, options)
		Expect(err).NotTo(HaveOccurred())
		var expected []*pb.Container
		for _, i := range indexes {
			expected = append(expected, input[i])
		}
		Expect(actual).To(HaveExactElements(expected))
	},
	Entry("returns containers in order by createdAt desc",
		[]*pb.Container{
			fakeContainer("test0", time.Date(2023, 1, 2, 12, 0o0, 0o0, 0o0, time.UTC).UnixNano()),
			fakeContainer("test1", time.Date(2023, 1, 1, 12, 0o0, 0o0, 0o0, time.UTC).UnixNano()),
			fakeContainer("test2", time.Date(2023, 1, 3, 12, 0o0, 0o0, 0o0, time.UTC).UnixNano()),
		},
		&listOptions{},
		[]int{2, 0, 1},
	),
	Entry("regards a container with no creation date as the oldest container",
		[]*pb.Container{
			{
				Metadata: &pb.ContainerMetadata{
					Name: "v0",
				},
			},
			fakeContainer("v1", time.Date(2023, 1, 1, 12, 0o0, 0o0, 0o0, time.UTC).UnixNano()),
		},
		&listOptions{},
		[]int{1, 0},
	),
	Entry("returns containers filtered with the regexp",
		fakeContainersWithCreatedAtDesc("Test0", "Dev1", "Test2", "Dev3"),
		&listOptions{nameRegexp: "Test.*"},
		[]int{0, 2},
	),
	Entry("returns no containers when there are no containers matched with the regexp",
		fakeContainersWithCreatedAtDesc("Test0", "Dev1", "Test2", "Dev3"),
		&listOptions{nameRegexp: "Prod.*"},
		[]int{},
	),
	Entry("returns the most recent container with the latest option",
		fakeContainersWithCreatedAtDesc("v0", "v1", "v2"),
		&listOptions{latest: true},
		[]int{0},
	),
	Entry("returns last n containers with the last option",
		fakeContainersWithCreatedAtDesc("v0", "v1", "v2"),
		&listOptions{last: 2},
		[]int{0, 1},
	),
	Entry("prioritizes last more than latest",
		fakeContainersWithCreatedAtDesc("v0", "v1", "v2"),
		&listOptions{last: 2, latest: true},
		[]int{0, 1},
	),
	Entry("returns all containers when the last is larger than the input length in order by createdAt desc",
		fakeContainersWithCreatedAtDesc("v0", "v1", "v2"),
		&listOptions{last: 5},
		[]int{0, 1, 2},
	),
	Entry("returns nothing when last is set and there are no containers",
		fakeContainersWithCreatedAtDesc(),
		&listOptions{last: 2},
		[]int{},
	),
)

var _ = DescribeTable("getPodNameFromLabels",
	func(input map[string]string, expected string) {
		actual := getPodNameFromLabels(input)
		Expect(actual).To(Equal(expected))
	},
	Entry("returns the value of the `KubernetesPodNameLabel` label",
		map[string]string{types.KubernetesPodNameLabel: "myPodName"},
		"myPodName",
	),
	Entry("returns only the value of the `KubernetesPodNameLabel` when the label and the other labels in the input",
		map[string]string{types.KubernetesPodNameLabel: "myPodName", "otherLabel": "otherValue"},
		"myPodName",
	),
	Entry("returns unknown when `KubernetesPodNameLabel` is not in the input labels",
		map[string]string{"otherLabel": "otherValue"},
		"unknown",
	),
	Entry("returns the value of the `KubernetesPodNameLabel` label",
		map[string]string{},
		"unknown",
	),
)
