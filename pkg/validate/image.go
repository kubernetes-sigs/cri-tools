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
	"context"
	"fmt"
	"runtime"
	"slices"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("Image Manager", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.ImageManagerService

	BeforeEach(func() {
		c = f.CRIClient.CRIImageClient
	})

	It("public image with tag should be pulled and removed [Conformance]", Serial, func() {
		testPullPublicImage(c, testImageWithTag, testImagePodSandbox, func(s *runtimeapi.Image) {
			Expect(s.GetRepoTags()).To(Equal([]string{testImageWithTag}))
		})
	})

	It("public image should timeout if requested [Conformance]", Serial, func() {
		imageName := framework.PrepareImageName(testImageWithTag)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		res, err := c.PullImage(ctx, &runtimeapi.ImageSpec{Image: imageName}, nil, nil)

		Expect(res).To(BeEmpty())
		Expect(err).To(HaveOccurred())

		statusErr, ok := status.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(statusErr.Code()).To(Equal(codes.DeadlineExceeded))
	})

	It("public image without tag should be pulled and removed [Conformance]", Serial, func() {
		testPullPublicImage(c, testImageWithoutTag, testImagePodSandbox, func(s *runtimeapi.Image) {
			Expect(s.GetRepoTags()).To(Equal([]string{testImageWithoutTag + ":latest"}))
		})
	})

	It("public image with digest should be pulled and removed [Conformance]", Serial, func() {
		testPullPublicImage(c, testImageWithDigest, testImagePodSandbox, func(s *runtimeapi.Image) {
			Expect(s.GetRepoTags()).To(BeEmpty())
			Expect(s.GetRepoDigests()).To(Equal([]string{testImageWithDigest}))
		})
	})

	It("image status should support all kinds of references [Conformance]", Serial, func() {
		imageName := testImageWithAllReferences
		// Make sure image does not exist before testing.
		removeImage(c, imageName)

		framework.PullPublicImage(c, imageName, testImagePodSandbox)

		status := framework.ImageStatus(c, imageName)
		Expect(status).NotTo(BeNil(), "should get image status")
		idStatus := framework.ImageStatus(c, status.GetId())
		Expect(idStatus).To(Equal(status), "image status with %q", status.GetId())
		for _, tag := range status.GetRepoTags() {
			tagStatus := framework.ImageStatus(c, tag)
			Expect(tagStatus).To(Equal(status), "image status with %q", tag)
		}
		for _, digest := range status.GetRepoDigests() {
			digestStatus := framework.ImageStatus(c, digest)
			Expect(digestStatus).To(Equal(status), "image status with %q", digest)
		}

		testRemoveImage(c, imageName)
	})

	if runtime.GOOS != framework.OSWindows || framework.TestContext.IsLcow {
		It("image status get image fields should not have Uid|Username empty [Conformance]", Serial, func() {
			for _, item := range []struct {
				description string
				image       string
				uid         int64
				username    string
			}{
				{
					description: "UID only",
					image:       testImageUserUID,
					uid:         imageUserUID,
					username:    "",
				},
				{
					description: "Username only",
					image:       testImageUserUsername,
					uid:         int64(0),
					username:    imageUserUsername,
				},
				{
					description: "UID:group",
					image:       testImageUserUIDGroup,
					uid:         imageUserUIDGroup,
					username:    "",
				},
				{
					description: "Username:group",
					image:       testImageUserUsernameGroup,
					uid:         int64(0),
					username:    imageUserUsernameGroup,
				},
			} {
				framework.PullPublicImage(c, item.image, testImagePodSandbox)
				defer removeImage(c, item.image)

				status := framework.ImageStatus(c, item.image)
				Expect(status.GetUid().GetValue()).To(Equal(item.uid), fmt.Sprintf("%s, Image Uid should be %d", item.description, item.uid))
				Expect(status.GetUsername()).To(Equal(item.username), fmt.Sprintf("%s, Image Username should be %s", item.description, item.username))
			}
		})
	}

	It("listImage should get exactly 3 image in the result list [Conformance]", Serial, func() {
		// Make sure test image does not exist.
		removeImageList(c, testDifferentTagDifferentImageList)
		ids := pullImageList(c, testDifferentTagDifferentImageList, testImagePodSandbox)
		Expect(ids).To(HaveLen(3), "3 image ids should be returned")

		defer removeImageList(c, testDifferentTagDifferentImageList)

		images := framework.ListImage(c, &runtimeapi.ImageFilter{})

		for i, id := range ids {
			for _, img := range images {
				if img.GetId() == id {
					Expect(img.GetRepoTags()).To(HaveLen(1), "Should only have 1 repo tag")
					Expect(img.GetRepoTags()[0]).To(Equal(testDifferentTagDifferentImageList[i]), "Repo tag should be correct")

					break
				}
			}
		}
	})

	It("listImage should get exactly 3 repoTags in the result image [Conformance]", Serial, func() {
		// Make sure test image does not exist.
		removeImageList(c, testDifferentTagSameImageList)
		ids := pullImageList(c, testDifferentTagSameImageList, testImagePodSandbox)
		slices.Sort(ids)
		ids = slices.Compact(ids)
		Expect(ids).To(HaveLen(1), "Only 1 image id should be returned")

		defer removeImageList(c, testDifferentTagSameImageList)

		images := framework.ListImage(c, &runtimeapi.ImageFilter{})

		sort.Strings(testDifferentTagSameImageList)
		for _, img := range images {
			if img.GetId() == ids[0] {
				sort.Strings(img.GetRepoTags())
				Expect(img.GetRepoTags()).To(Equal(testDifferentTagSameImageList), "Should have 3 repoTags in single image")

				break
			}
		}
	})

	It("removing image by one tag should remove all tags [Conformance]", Serial, func() {
		imageName1 := testDifferentTagSameImageList[0]
		imageName2 := testDifferentTagSameImageList[1]
		imageName3 := testDifferentTagSameImageList[2]

		// Ensure images are absent before test
		removeImageList(c, []string{imageName1, imageName2, imageName3})

		By("Pulling image with multiple tags")
		pullImageList(c, []string{imageName1, imageName2, imageName3}, testImagePodSandbox)

		By("Verifying all tags are present on a single image")
		images := framework.ListImage(c, &runtimeapi.ImageFilter{})
		var foundImage *runtimeapi.Image
		for _, img := range images {
			// Check if the image has one of our tags. Since they all point to the same image, finding one is enough.
			if slices.Contains(img.GetRepoTags(), imageName1) {
				foundImage = img
			}
			if foundImage != nil {
				break
			}
		}
		Expect(foundImage).NotTo(BeNil(), "Should find the pulled image")
		Expect(foundImage.GetRepoTags()).To(HaveLen(3), "Should have exactly three tags")
		Expect(foundImage.GetRepoTags()).To(ContainElements(imageName1, imageName2, imageName3), "Should contain all three tags")

		imageID := foundImage.GetId() // Get the ID for later verification

		By("Removing image by a single tag: " + imageName1)
		removeImage(c, imageName1)

		By("Verifying the image is completely removed")
		status1 := framework.ImageStatus(c, imageName1)
		Expect(status1).To(BeNil(), "Image should be gone when checking by first tag")
		status2 := framework.ImageStatus(c, imageName2)
		Expect(status2).To(BeNil(), "Image should be gone when checking by second tag")
		status3 := framework.ImageStatus(c, imageName3)
		Expect(status3).To(BeNil(), "Image should be gone when checking by third tag")

		idStatus := framework.ImageStatus(c, imageID)
		Expect(idStatus).To(BeNil(), "Image should be gone when checking by its ID")
	})

	It("removing image from one registry should remove all tags from other registries [Conformance]", Serial, func() {
		imageName1 := testSameImageDifferentRegistries[0]
		imageName2 := testSameImageDifferentRegistries[1]

		// Ensure images are absent before test
		removeImageList(c, []string{imageName1, imageName2})

		By("Pulling the same image from different registries")
		pullImageList(c, []string{imageName1, imageName2}, testImagePodSandbox)

		By("Verifying all tags are present on a single image")
		images := framework.ListImage(c, &runtimeapi.ImageFilter{})
		var foundImage *runtimeapi.Image
		for _, img := range images {
			// Check if the image has one of our tags. Since they all point to the same image, finding one is enough.
			if slices.Contains(img.GetRepoTags(), imageName1) {
				foundImage = img
			}
			if foundImage != nil {
				break
			}
		}
		Expect(foundImage).NotTo(BeNil(), "Should find the pulled image")
		Expect(foundImage.GetRepoTags()).To(HaveLen(2), "Should have exactly two tags")
		Expect(foundImage.GetRepoTags()).To(ContainElements(imageName1, imageName2), "Should contain tags from both registries")

		imageID := foundImage.GetId() // Get the ID for later verification

		By("Removing image by a single tag: " + imageName1)
		removeImage(c, imageName1)

		By("Verifying the image is completely removed")
		status1 := framework.ImageStatus(c, imageName1)
		Expect(status1).To(BeNil(), "Image should be gone when checking by first tag")
		status2 := framework.ImageStatus(c, imageName2)
		Expect(status2).To(BeNil(), "Image should be gone when checking by second tag")

		idStatus := framework.ImageStatus(c, imageID)
		Expect(idStatus).To(BeNil(), "Image should be gone when checking by its ID")
	})
})

// testRemoveImage removes the image name imageName and check if it successes.
func testRemoveImage(c internalapi.ImageManagerService, imageName string) {
	By("Remove image : " + imageName)
	image, err := c.ImageStatus(context.TODO(), &runtimeapi.ImageSpec{Image: imageName}, false)
	framework.ExpectNoError(err, "failed to get image status: %v", err)

	if image.GetImage() != nil {
		By("Remove image by ID : " + image.GetImage().GetId())
		err = c.RemoveImage(context.TODO(), &runtimeapi.ImageSpec{Image: image.GetImage().GetId()})
		framework.ExpectNoError(err, "failed to remove image: %v", err)
	}

	By("Check image list empty")

	imageStatus := framework.ImageStatus(c, imageName)
	Expect(imageStatus).To(BeNil(), "Should have none image in list")
}

// testPullPublicImage pulls the image named imageName, make sure it success and remove the image.
func testPullPublicImage(c internalapi.ImageManagerService, imageName string, podConfig *runtimeapi.PodSandboxConfig, statusCheck func(*runtimeapi.Image)) {
	// Make sure image does not exist before testing.
	removeImage(c, imageName)

	framework.PullPublicImage(c, imageName, podConfig)

	By("Check image list to make sure pulling image success : " + imageName)
	imageStatus := framework.ImageStatus(c, imageName)
	Expect(imageStatus).NotTo(BeNil(), "Should have one image in list")
	Expect(imageStatus.GetId()).NotTo(BeNil(), "Image Id should not be nil")
	Expect(imageStatus.GetSize()).NotTo(BeNil(), "Image Size should not be nil")

	if statusCheck != nil {
		statusCheck(imageStatus)
	}

	testRemoveImage(c, imageName)
}

// pullImageList pulls the images listed in the imageList.
func pullImageList(c internalapi.ImageManagerService, imageList []string, podConfig *runtimeapi.PodSandboxConfig) (ids []string) {
	for _, imageName := range imageList {
		ids = append(ids, framework.PullPublicImage(c, imageName, podConfig))
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
	image, err := c.ImageStatus(context.TODO(), &runtimeapi.ImageSpec{Image: imageName}, false)
	framework.ExpectNoError(err, "failed to get image status: %v", err)

	if image.GetImage() != nil {
		By("Remove image by ID : " + image.GetImage().GetId())
		err = c.RemoveImage(context.TODO(), &runtimeapi.ImageSpec{Image: image.GetImage().GetId()})
		framework.ExpectNoError(err, "failed to remove image: %v", err)
	}
}
