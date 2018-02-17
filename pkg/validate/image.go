/*
Copyright 2017 The Kubernetes Authors.

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

import (
	"sort"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// image reference without
	testImageWithoutTag = "gcr.io/cri-tools/test-image-latest"

	// name-tagged reference for test image
	testImageWithTag = "gcr.io/cri-tools/test-image-tag:test"

	// digested reference for test image
	testImageWithDigest = "gcr.io/cri-tools/test-image-digest@sha256:9179135b4b4cc5a8721e09379244807553c318d92fa3111a65133241551ca343"
)

var _ = framework.KubeDescribe("Image Manager", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.ImageManagerService

	BeforeEach(func() {
		c = f.CRIClient.CRIImageClient
	})

	It("public image with tag should be pulled and removed [Conformance]", func() {
		testPullPublicImage(c, testImageWithTag, func(s *runtimeapi.Image) {
			Expect(s.RepoTags).To(Equal([]string{testImageWithTag}))
		})
	})

	It("public image without tag should be pulled and removed [Conformance]", func() {
		testPullPublicImage(c, testImageWithoutTag, func(s *runtimeapi.Image) {
			Expect(s.RepoTags).To(Equal([]string{testImageWithoutTag + ":latest"}))
		})
	})

	It("public image with digest should be pulled and removed [Conformance]", func() {
		testPullPublicImage(c, testImageWithDigest, func(s *runtimeapi.Image) {
			Expect(s.RepoTags).To(BeEmpty())
			Expect(s.RepoDigests).To(Equal([]string{testImageWithDigest}))
		})
	})

	It("listImage should get exactly 3 image in the result list [Conformance]", func() {
		// different tags refer to different images
		testImageList := []string{
			"gcr.io/cri-tools/test-image-1:latest",
			"gcr.io/cri-tools/test-image-2:latest",
			"gcr.io/cri-tools/test-image-3:latest",
		}

		// Make sure test image does not exist.
		removeImageList(c, testImageList)
		ids := pullImageList(c, testImageList)
		ids = removeDuplicates(ids)
		Expect(len(ids)).To(Equal(3), "3 image ids should be returned")

		defer removeImageList(c, testImageList)

		images := framework.ListImage(c, &runtimeapi.ImageFilter{})

		for i, id := range ids {
			for _, img := range images {
				if img.Id == id {
					Expect(len(img.RepoTags)).To(Equal(1), "Should only have 1 repo tag")
					Expect(img.RepoTags[0]).To(Equal(testImageList[i]), "Repo tag should be correct")
					break
				}
			}
		}
	})

	It("listImage should get exactly 3 repoTags in the result image [Conformance]", func() {
		// different tags refer to the same image
		testImageList := []string{
			"gcr.io/cri-tools/test-image-tags:1",
			"gcr.io/cri-tools/test-image-tags:2",
			"gcr.io/cri-tools/test-image-tags:3",
		}

		// Make sure test image does not exist.
		removeImageList(c, testImageList)
		ids := pullImageList(c, testImageList)
		ids = removeDuplicates(ids)
		Expect(len(ids)).To(Equal(1), "Only 1 image id should be returned")

		defer removeImageList(c, testImageList)

		images := framework.ListImage(c, &runtimeapi.ImageFilter{})

		sort.Strings(testImageList)
		for _, img := range images {
			if img.Id == ids[0] {
				sort.Strings(img.RepoTags)
				Expect(img.RepoTags).To(Equal(testImageList), "Should have 3 repoTags in single image")
				break
			}
		}
	})
})

// testRemoveImage removes the image name imageName and check if it successes.
func testRemoveImage(c internalapi.ImageManagerService, imageName string) {
	By("Remove image : " + imageName)
	removeImage(c, imageName)

	By("Check image list empty")
	status := framework.ImageStatus(c, imageName)
	Expect(status).To(BeNil(), "Should have none image in list")
}

// testPullPublicImage pulls the image named imageName, make sure it success and remove the image.
func testPullPublicImage(c internalapi.ImageManagerService, imageName string, statusCheck func(*runtimeapi.Image)) {
	// Make sure image does not exist before testing.
	removeImage(c, imageName)

	framework.PullPublicImage(c, imageName)

	By("Check image list to make sure pulling image success : " + imageName)
	status := framework.ImageStatus(c, imageName)
	Expect(status).NotTo(BeNil(), "Should have one image in list")
	Expect(status.Id).NotTo(BeNil(), "Image Id should not be nil")
	Expect(status.Size_).NotTo(BeNil(), "Image Size should not be nil")
	if statusCheck != nil {
		statusCheck(status)
	}

	testRemoveImage(c, imageName)
}

// pullImageList pulls the images listed in the imageList.
func pullImageList(c internalapi.ImageManagerService, imageList []string) []string {
	var ids []string
	for _, imageName := range imageList {
		ids = append(ids, framework.PullPublicImage(c, imageName))
	}
	return ids
}

// removeImageList removes the images listed in the imageList.
func removeImageList(c internalapi.ImageManagerService, imageList []string) {
	for _, imageName := range imageList {
		removeImage(c, imageName)
	}
}

// removeImage removes the image named imagesName.
func removeImage(c internalapi.ImageManagerService, imageName string) {
	By("Remove image : " + imageName)
	image, err := c.ImageStatus(&runtimeapi.ImageSpec{Image: imageName})
	framework.ExpectNoError(err, "failed to get image status: %v", err)

	if image != nil {
		By("Remove image by ID : " + image.Id)
		err = c.RemoveImage(&runtimeapi.ImageSpec{Image: image.Id})
		framework.ExpectNoError(err, "failed to remove image: %v", err)
	}
}

// removeDuplicates remove duplicates strings from a list
func removeDuplicates(ss []string) []string {
	encountered := map[string]bool{}
	result := []string{}
	for _, s := range ss {
		if encountered[s] == true {
			continue
		}
		encountered[s] = true
		result = append(result, s)
	}
	return result
}
