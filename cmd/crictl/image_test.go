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
	expected := make([]string, 0, len(actual))

	for _, img := range actual {
		expected = append(expected, img.GetId())
	}

	Expect(images).To(Equal(expected))
}

var _ = DescribeTable("parseCreds",
	func(input, expectedUser, expectedPass string, expectErr bool) {
		user, pass, err := parseCreds(input)
		if expectErr {
			Expect(err).To(HaveOccurred())

			return
		}

		Expect(err).NotTo(HaveOccurred())
		Expect(user).To(Equal(expectedUser))
		Expect(pass).To(Equal(expectedPass))
	},
	Entry("user:password", "alice:secret", "alice", "secret", false),
	Entry("user only (no colon)", "alice", "alice", "", false),
	Entry("user with empty password", "alice:", "alice", "", false),
	Entry("password containing colons", "alice:p:a:ss", "alice", "p:a:ss", false),
	Entry("empty string", "", "", "", true),
	Entry("empty username", ":secret", "", "", true),
)

var _ = DescribeTable("getAuth",
	func(creds, auth string, expectedUser, expectedPass, expectedAuth string, expectNil, expectErr bool) {
		cfg, err := getAuth(creds, auth, "")
		if expectErr {
			Expect(err).To(HaveOccurred())

			return
		}

		Expect(err).NotTo(HaveOccurred())

		if expectNil {
			Expect(cfg).To(BeNil())

			return
		}

		Expect(cfg.GetUsername()).To(Equal(expectedUser))
		Expect(cfg.GetPassword()).To(Equal(expectedPass))
		Expect(cfg.GetAuth()).To(Equal(expectedAuth))
	},
	Entry("creds only", "alice:secret", "", "alice", "secret", "", false, false),
	Entry("auth only", "", "dG9rZW4=", "", "", "dG9rZW4=", false, false),
	Entry("both creds and auth", "alice:secret", "dG9rZW4=", "", "", "", false, true),
	Entry("neither creds nor auth", "", "", "", "", "", true, false),
)

var _ = DescribeTable("normalizeRepoTagPair",
	func(repoTags []string, imageName string, expected [][]string) {
		actual := normalizeRepoTagPair(repoTags, imageName)
		Expect(actual).To(Equal(expected))
	},
	Entry("standard image:tag",
		[]string{"docker.io/library/nginx:latest"},
		"nginx",
		[][]string{{"docker.io/library/nginx", "latest"}},
	),
	Entry("multiple tags",
		[]string{"nginx:latest", "nginx:1.25"},
		"nginx",
		[][]string{{"nginx", "latest"}, {"nginx", "1.25"}},
	),
	Entry("empty repoTags uses imageName with <none> tag",
		[]string{},
		"docker.io/library/nginx",
		[][]string{{"docker.io/library/nginx", "<none>"}},
	),
	Entry("nil repoTags uses imageName with <none> tag",
		nil,
		"docker.io/library/nginx",
		[][]string{{"docker.io/library/nginx", "<none>"}},
	),
	Entry("<none> name replaced by imageName",
		[]string{"<none>:latest"},
		"myimage",
		[][]string{{"myimage", "latest"}},
	),
	Entry("tag without colon yields error pair",
		[]string{"malformed"},
		"img",
		[][]string{{"errorRepoTag", "errorRepoTag"}},
	),
)

var _ = DescribeTable("normalizeRepoDigest",
	func(repoDigests []string, expectedRepo, expectedDigest string) {
		repo, digest := normalizeRepoDigest(repoDigests)
		Expect(repo).To(Equal(expectedRepo))
		Expect(digest).To(Equal(expectedDigest))
	},
	Entry("standard digest",
		[]string{"docker.io/library/nginx@sha256:abc123"},
		"docker.io/library/nginx", "sha256:abc123",
	),
	Entry("empty list",
		[]string{},
		"<none>", "<none>",
	),
	Entry("nil list",
		nil,
		"<none>", "<none>",
	),
	Entry("malformed digest (no @)",
		[]string{"docker.io/library/nginx"},
		"errorName", "errorRepoDigest",
	),
	Entry("multiple digests uses first",
		[]string{"docker.io/library/nginx@sha256:abc", "docker.io/library/nginx@sha256:def"},
		"docker.io/library/nginx", "sha256:abc",
	),
)

func assertFilterByName(input []*pb.Image, nameFilter string, expectedIDs []string) {
	actual := filterByName(nameFilter, input)
	ids := make([]string, 0, len(actual))

	for _, img := range actual {
		ids = append(ids, img.GetId())
	}

	Expect(expectedIDs).To(Equal(ids))
}

var _ = DescribeTable("filterByName", assertFilterByName,
	Entry("filters by exact repo:tag",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/busybox@sha256:1"}, []string{"docker.io/library/busybox:latest"}),
			fakeImage("2", []string{"docker.io/library/nginx@sha256:2"}, []string{"docker.io/library/nginx:latest"}),
		},
		"docker.io/library/busybox:latest",
		[]string{"1"},
	),
	Entry("filters by repo name only (no tag)",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/busybox@sha256:1"}, []string{"docker.io/library/busybox:latest"}),
			fakeImage("2", []string{"docker.io/library/nginx@sha256:2"}, []string{"docker.io/library/nginx:1.0"}),
			fakeImage("3", []string{"docker.io/library/busybox@sha256:3"}, []string{"docker.io/library/busybox:1.36"}),
		},
		"docker.io/library/busybox",
		[]string{"1", "3"},
	),
	Entry("returns empty when no match",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/busybox@sha256:1"}, []string{"docker.io/library/busybox:latest"}),
			fakeImage("2", []string{"docker.io/library/nginx@sha256:2"}, []string{"docker.io/library/nginx:latest"}),
		},
		"docker.io/library/alpine:latest",
		[]string{},
	),
	Entry("matches by repo digest",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/library/busybox@sha256:abc123"}, []string{"docker.io/library/busybox:latest"}),
			fakeImage("2", []string{"docker.io/library/nginx@sha256:def456"}, []string{"docker.io/library/nginx:latest"}),
		},
		"docker.io/library/busybox@sha256:abc123",
		[]string{"1"},
	),
	Entry("filters by repo:tag with non-matching tag",
		[]*pb.Image{
			fakeImage("1", []string{"docker.io/kindest/kindnetd@sha256:1"}, []string{"docker.io/kindest/kindnetd:v20250214"}),
			fakeImage("2", []string{"docker.io/kindest/local-path-helper@sha256:2"}, []string{"docker.io/kindest/local-path-helper:v20241212"}),
		},
		"docker.io/kindest/kindnetd:v20250214",
		[]string{"1"},
	),
)

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
