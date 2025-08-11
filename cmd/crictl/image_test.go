/*
Copyright 2024 The Kubernetes Authors.

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func fakeImage(id string, digest, tags []string) *pb.Image {
	return &pb.Image{Id: id, RepoDigests: digest, RepoTags: tags}
}

func assert(input []*pb.Image, options, images []string) {
	actual, _ := filterImagesList(input, options)
	expected := []string{}

	for _, img := range actual {
		expected = append(expected, img.GetId())
	}

	Expect(images).To(Equal(expected))
}

var _ = DescribeTable("filterImagesListByDangling", assert,
	Entry("returns filtered images with dangling --filter=dangling=true",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/busybox@sha256:1"}, []string{}),
			fakeImage("2", []string{"docker.io/library/nginx@sha256:2"}, []string{"latest"}),
		},
		[]string{"dangling=true"},
		[]string{"1"},
	),
	Entry("returns filtered images with dangling --filter=dangling=false",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"1.2.2"}),
		},
		[]string{"dangling=false"},
		[]string{"2", "4"},
	),
)

var _ = DescribeTable("filterImagesListByReference", assert,
	Entry("returns filtered images with one reference --filter=reference=busybox",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/busybox@sha256:1"}, []string{"latest"}),
			fakeImage("2", []string{"docker.io/library/nginx@sha256:2"}, []string{"latest"}),
		},
		[]string{"reference=busybox"},
		[]string{"1"},
	),
	Entry("returns filtered images with many reference --filter=reference=busybox, --filter=reference=k8s",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"0.0.0"}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{"1.0.0"}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"1.2.2"}),
			fakeImage("5", []string{"registry.k8s.io/e2e-test-images/busybox@sha256:5"}, []string{"1.2.2"}),
		},
		[]string{"reference=busybox", "reference=k8s"},
		[]string{"5"},
	),
)

var _ = DescribeTable("filterImagesListByBefore", assert,
	Entry("returns filtered images with --filter=before=<image-name>[:<tag>]",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"docker.io/library/server:0.0.0"}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"docker.io/library/busybox:1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{"docker.io/library/nginx:1.0.0"}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"docker.io/library/app:1.2.2"}),
		},
		[]string{"before=docker.io/library/nginx:1.0.0"},
		[]string{"4"},
	),
	Entry("returns filtered images with --filter=before=<image id>",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"docker.io/library/server:0.0.0"}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"docker.io/library/busybox:1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{"docker.io/library/nginx:1.0.0"}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"docker.io/library/app:1.2.2"}),
		},
		[]string{"before=1"},
		[]string{"2", "3", "4"},
	),
	Entry("returns filtered images with --filter=before=<image@digest>",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"docker.io/library/server:0.0.0"}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"docker.io/library/busybox:1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{"docker.io/library/nginx:1.0.0"}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"docker.io/library/app:1.2.2"}),
		},
		[]string{"before=docker.io/library/busybox@sha256:2"},
		[]string{"3", "4"},
	),
)

var _ = DescribeTable("filterImagesListBySince", assert,
	Entry("returns filtered images with --filter=since=<image-name>[:<tag>]",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"docker.io/library/server:0.0.0"}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"docker.io/library/busybox:1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{"docker.io/library/nginx:1.0.0"}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"docker.io/library/app:1.2.2"}),
		},
		[]string{"since=docker.io/library/busybox:1.2.0"},
		[]string{"1"},
	),
	Entry("returns filtered images with --filter=since=<image id>",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"docker.io/library/server:0.0.0"}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"docker.io/library/busybox:1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{"docker.io/library/nginx:1.0.0"}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"docker.io/library/app:1.2.2"}),
		},
		[]string{"since=3"},
		[]string{"1", "2"},
	),
	Entry("returns filtered images with --filter=since=<image@digest>",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"docker.io/library/server:0.0.0"}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"docker.io/library/busybox:1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{"docker.io/library/nginx:1.0.0"}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"docker.io/library/app:1.2.2"}),
		},
		[]string{"since=docker.io/library/nginx@sha256:3"},
		[]string{"1", "2"},
	),
)

var _ = DescribeTable("filterImagesListByChainable", assert,
	Entry("returns filtered images with --filter=since=<image-id> and --filter=reference=<ref>",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"docker.io/library/server:0.0.0"}),
			fakeImage("2", []string{"docker.io/library/busybox@sha256:2"}, []string{"docker.io/library/busybox:1.2.0"}),
			fakeImage("3", []string{"docker.io/library/nginx@sha256:3"}, []string{"docker.io/library/nginx:1.0.0"}),
			fakeImage("4", []string{"docker.io/library/app@sha256:4"}, []string{"docker.io/library/app:1.2.2"}),
		},
		[]string{"since=3", "reference=busybox"},
		[]string{"2"},
	),
	Entry("returns filtered images with --filter=since=<image-id> and --filter=reference=<ref>",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"0.0.0"}),
			fakeImage("2", []string{"registry.k8s.io/e2e-test-images/busybox@sha256:5"}, []string{"1.2.2"}),
			fakeImage("3", []string{"docker.io/library/busybox@sha256:2"}, []string{"1.2.0"}),
			fakeImage("4", []string{"docker.io/library/nginx@sha256:3"}, []string{"1.0.0"}),
			fakeImage("5", []string{"docker.io/library/app@sha256:4"}, []string{"1.2.2"}),
		},
		[]string{"since=5", "reference=busybox"},
		[]string{"2", "3"},
	),
	Entry("returns empty images list --filter=since=<image-id> and --filter=reference=<ref>",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/server@sha256:1"}, []string{"0.0.0"}),
			fakeImage("2", []string{"registry.k8s.io/e2e-test-images/busybox@sha256:5"}, []string{"1.2.2"}),
			fakeImage("3", []string{"docker.io/library/busybox@sha256:2"}, []string{"1.2.0"}),
			fakeImage("4", []string{"docker.io/library/nginx@sha256:3"}, []string{"1.0.0"}),
			fakeImage("5", []string{"docker.io/library/app@sha256:4"}, []string{"1.2.2"}),
		},
		[]string{"since=5", "reference=kubefun"},
		[]string{},
	),
)
