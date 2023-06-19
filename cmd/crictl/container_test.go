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
	"time"

	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubernetes/pkg/kubelet/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeContainersWithCreatedAtAsc creates fake containers in the least recent order of the createdAt.
func fakeContainersWithCreatedAtAsc(names ...string) []*pb.Container {
	containers := []*pb.Container{}
	creationTime := time.Date(2023, 1, 1, 12, 00, 00, 00, time.UTC)
	for _, name := range names {
		containers = append(containers, &pb.Container{
			Metadata: &pb.ContainerMetadata{
				Name: name,
			},
			CreatedAt: creationTime.UnixNano(),
		})
		creationTime = creationTime.Add(time.Second)
	}
	return containers
}

var _ = Describe("container.go", func() {
	Describe("getContainersList", func() {
		containers := fakeContainersWithCreatedAtAsc("Test1", "Dev1", "Test2", "Dev2")
		It("returns containers filtered with the regexp in order by createdAt desc", func() {
			actual := getContainersList(containers, listOptions{nameRegexp: "Test.*"})
			Expect(actual).To(HaveExactElements([]*pb.Container{containers[2], containers[0]}))
		})
		It("returns the most recent container with the latest option", func() {
			actual := getContainersList(containers, listOptions{latest: true})
			Expect(actual).To(HaveExactElements([]*pb.Container{containers[3]}))
		})
		It("returns last n containers with the last option", func() {
			actual := getContainersList(containers, listOptions{last: 2})
			Expect(actual).To(HaveExactElements([]*pb.Container{containers[3], containers[2]}))
		})
		It("returns all containers when the last is larger than the input length in order by createdAt desc", func() {
			actual := getContainersList(containers, listOptions{last: 5})
			Expect(actual).To(HaveExactElements([]*pb.Container{containers[3], containers[2], containers[1], containers[0]}))
		})
	})

	Describe("getPodNameFromLabels", func() {
		It("returns the value of the `KubernetesPodNameLabel` label", func() {
			actual := getPodNameFromLabels(map[string]string{types.KubernetesPodNameLabel: "myPodName"})
			Expect(actual).To(Equal("myPodName"))
		})
		It("returns only the value of the `KubernetesPodNameLabel` when the label and the other labels in the input", func() {
			actual := getPodNameFromLabels(map[string]string{types.KubernetesPodNameLabel: "myPodName", "otherLabel": "otherValue"})
			Expect(actual).To(Equal("myPodName"))
		})
		It("returns unknown when `KubernetesPodNameLabel` is not in the input labels", func() {
			actual := getPodNameFromLabels(map[string]string{"otherLabel": "otherValue"})
			Expect(actual).To(Equal("unknown"))
		})
		It("returns unknown when the input labels is empty", func() {
			actual := getPodNameFromLabels(map[string]string{})
			Expect(actual).To(Equal("unknown"))
		})
	})
})
