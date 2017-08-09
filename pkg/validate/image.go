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
	"strings"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	// image name for test image api
	testImageName = "busybox"

	// name-tagged reference for test image
	testImageRef = testImageName + ":1.26.2"

	// Digested reference for test image
	busyboxDigestRef = testImageName + "@sha256:817a12c32a39bbe394944ba49de563e085f1d3c5266eb8e9723256bc4448680e"
)

var _ = framework.KubeDescribe("Image Manager", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.ImageManagerService

	BeforeEach(func() {
		c = f.CRIClient.CRIImageClient
	})

	It("public image with tag should be pulled and removed [Conformance]", func() {
		testPullPublicImage(c, testImageRef)
	})

	It("public image without tag should be pulled and removed [Conformance]", func() {
		testPullPublicImage(c, testImageName)
	})

	It("public image with digest should be pulled and removed [Conformance]", func() {
		testPullPublicImage(c, busyboxDigestRef)
	})

	It("image status get image fields should not be empty [Conformance]", func() {
		framework.PullPublicImage(c, testImageRef)

		defer removeImage(c, testImageRef)

		status := framework.ImageStatus(c, testImageRef)
		Expect(status.Id).NotTo(BeNil(), "Image Id should not be nil")
		Expect(len(status.RepoTags)).NotTo(Equal(0), "Should have repoTags in image")
		Expect(status.Size_).NotTo(BeNil(), "Image Size should not be nil")
	})

	It("listImage should get exactly 3 image in the result list [Conformance]", func() {
		// different tags refer to different images
		testImageList := []string{
			"busybox:1-uclibc",
			"busybox:1-musl",
			"busybox:1-glibc",
		}

		pullImageList(c, testImageList)

		defer removeImageList(c, testImageList)

		images := framework.ListImage(c, &runtimeapi.ImageFilter{})

		count := 0
		for _, imageName := range images {
			for _, imagesTag := range imageName.RepoTags {
				if strings.HasSuffix(imagesTag, "busybox:1-uclibc") {
					count = count + 1
				}
				if strings.HasSuffix(imagesTag, "busybox:1-musl") {
					count = count + 1
				}
				if strings.HasSuffix(imagesTag, "busybox:1-glibc") {
					count = count + 1
				}
			}
		}
		Expect(count).To(Equal(3), "Should have the specified three images in list")

	})

	It("listImage should get exactly 2 repoTags in the result image [Conformance]", func() {
		// different tags refer to the same image
		testImageList := []string{
			"busybox:1-uclibc",
			"busybox:1",
		}

		pullImageList(c, testImageList)

		defer removeImageList(c, testImageList)

		images := framework.ListImage(c, &runtimeapi.ImageFilter{})

		count := 0
		for _, imageName := range images {
			for _, imagesTag := range imageName.RepoTags {
				if strings.HasSuffix(imagesTag, "busybox:1-uclibc") {
					count = count + 1
				}
				if strings.HasSuffix(imagesTag, "busybox:1") {
					count = count + 1
				}
			}
			if count < 2 {
				count = 0
			}
		}
		Expect(count).To(Equal(2), "Should have two repoTags in single image in list")
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
func testPullPublicImage(c internalapi.ImageManagerService, imageName string) {
	if !strings.Contains(imageName, ":") {
		imageName = imageName + ":latest"
	}
	framework.PullPublicImage(c, imageName)

	By("Check image list to make sure pulling image success : " + imageName)
	status := framework.ImageStatus(c, imageName)
	Expect(status).NotTo(BeNil(), "Should have one image in list")

	testRemoveImage(c, imageName)
}

// pullImageList pulls the images listed in the imageList.
func pullImageList(c internalapi.ImageManagerService, imageList []string) {
	for _, imageName := range imageList {
		framework.PullPublicImage(c, imageName)
	}
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
