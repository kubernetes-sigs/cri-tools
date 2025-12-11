/*
Copyright 2026 The Kubernetes Authors.

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
	"slices"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("Image Consistency", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.ImageManagerService

	BeforeEach(func() {
		c = f.CRIClient.CRIImageClient
	})

	// Test: Immediate call to ListImages (and other methods like GetImage) after removing the image must not have this image information
	It("should not list or get image status immediately after removal [Conformance]", Serial, func() {
		imageName := testImageWithTag

		// Ensure image is absent before test
		removeImage(c, imageName)

		By("Pulling image: " + imageName)
		framework.PullPublicImage(c, imageName, testImagePodSandbox)

		By("Removing image: " + imageName)
		removeImage(c, imageName)

		By("Verifying image is not listed")
		images := framework.ListImage(c, &runtimeapi.ImageFilter{})
		found := false
		for _, img := range images {
			if slices.Contains(img.GetRepoTags(), imageName) {
				found = true
			}
			if found {
				break
			}
		}
		Expect(found).To(BeFalse(), "Image %q should not be listed after removal", imageName)

		By("Verifying image status is nil")
		imageStatus := framework.ImageStatus(c, imageName)
		Expect(imageStatus).To(BeNil(), "Image status for %q should be nil after removal", imageName)
	})

	It("should list and get image status immediately after pulling [Conformance]", Serial, func() {
		imageName := testImageWithTag

		// Ensure image is absent before test
		removeImage(c, imageName)

		By("Pulling image: " + imageName)
		framework.PullPublicImage(c, imageName, testImagePodSandbox)
		// Defer removal to ensure cleanup even if test fails
		defer removeImage(c, imageName)

		By("Verifying image is listed")
		images := framework.ListImage(c, &runtimeapi.ImageFilter{})
		found := false
		for _, img := range images {
			if slices.Contains(img.GetRepoTags(), imageName) {
				found = true
			}
			if found {
				break
			}
		}
		Expect(found).To(BeTrue(), "Image %q should be listed after pulling", imageName)

		By("Verifying image status is not nil")
		imageStatus := framework.ImageStatus(c, imageName)
		Expect(imageStatus).NotTo(BeNil(), "Image status for %q should be available after pulling", imageName)
	})

	// TODO: Implement ImageFsInfo tests
	//
	// It("ImageFsInfo usage should increase momentarily when an image is pulled", func() {
	//   // 1. Get initial ImageFsInfo.
	//   // 2. Pull a new, unique image (e.g., by digest) to ensure it's not cached.
	//   // 3. Immediately (or within a very short timeout) check that the ImageFsInfo usage has increased.
	//   //    This validates that the runtime updates its stats promptly after a pull.
	// })
	//
	// It("ImageFsInfo usage should decrease eventually when an image is removed", func() {
	//   // 1. Pull a new, unique image.
	//   // 2. Get the ImageFsInfo after the pull.
	//   // 3. Remove the image.
	//   // 4. Poll (with a reasonable timeout) until the ImageFsInfo usage decreases.
	//   //    This validates that the runtime reclaims space and updates its stats after removal,
	//   //    acknowledging that the cleanup might be asynchronous.
	// })

	It("should not fail on simultaneous RemoveImage calls [Conformance]", Serial, func() {
		imageName := testImageWithTag
		removeImage(c, imageName) // Ensure image is not present

		By("Pulling an image to be removed")
		framework.PullPublicImage(c, imageName, testImagePodSandbox)

		// It is important to get the image ID after pulling, as CRI-O behavior may be different
		// when removing by name vs by ID.

		img := framework.ImageStatus(c, imageName)
		Expect(img).NotTo(BeNil(), "Image status for %q should be available after pulling", imageName)
		imageID := img.GetId()

		By("Concurrently removing the same image")
		var wg sync.WaitGroup
		// Channel to collect results from each goroutine
		type removeResult struct {
			err        error
			imageFound bool
		}
		results := make(chan removeResult, 5)

		for range 5 {
			wg.Go(func() {
				// Use the specific image ID for removal to avoid ambiguity
				remErr := c.RemoveImage(context.Background(), &runtimeapi.ImageSpec{Image: imageID})

				// Immediately check image status after removal attempt
				status := framework.ImageStatus(c, imageID)
				imageFound := (status != nil)

				results <- removeResult{err: remErr, imageFound: imageFound}
			})
		}
		wg.Wait()
		close(results)

		// Verify results: all calls should succeed, and image should be gone immediately after each call
		for res := range results {
			Expect(res.err).NotTo(HaveOccurred(), "Concurrent RemoveImage calls should not return an error")
			// Assert immediate disappearance of the image
			Expect(res.imageFound).To(BeFalse(), "Image should be missing immediately after RemoveImage call returns")
		}

		By("Verifying the image is completely removed (final check)")
		status := framework.ImageStatus(c, imageID)
		Expect(status).To(BeNil(), "Image should be removed after all concurrent calls")
	})
})
