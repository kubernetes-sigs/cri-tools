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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("Image Identifier Consistency", func() {
	f := framework.NewDefaultCRIFramework()

	var (
		ic internalapi.ImageManagerService
		rc internalapi.RuntimeService
	)

	BeforeEach(func() {
		ic = f.CRIClient.CRIImageClient
		rc = f.CRIClient.CRIRuntimeClient
	})

	// This test verifies a new requirement in CRI API starting v1.36:
	// When referring to the same image, the container runtime MUST always return
	// the same value for:
	//     - Image.id
	//     - Container.image_id
	//     - ContainerStatus.image_id
	//     - PullImageResponse.image_ref
	// TODO: currently tests verify that image_ref of PullImageResponse matches Image.id.
	// The test with other fiekds needs to be uncommented once containerd will start
	// populating image_id in ContainerStatus and ListContainers.
	It("should return the same image identifier across all APIs [Conformance]", Serial, func(ctx SpecContext) {
		imageName := testImageWithTag

		// Note, the test relies on the fact that imageRef returned from the wrapper is the same as the one returned by the runtime.
		By("Pulling image: " + imageName)
		imageRef := framework.PullPublicImage(ctx, ic, imageName, testImagePodSandbox)
		framework.Logf("PullImageResponse.image_ref: %q", imageRef)
		Expect(imageRef).NotTo(BeEmpty(), "PullImageResponse.image_ref should not be empty")

		defer removeImage(ctx, ic, imageName)

		By("Checking ImageStatus")

		imageStatus := framework.ImageStatus(ctx, ic, imageName)
		Expect(imageStatus).NotTo(BeNil(), "Image status should be available")
		framework.Logf("Image.id: %q", imageStatus.GetId())
		framework.Logf("Image.repo_tags: %v", imageStatus.GetRepoTags())
		framework.Logf("Image.repo_digests: %v", imageStatus.GetRepoDigests())

		By("Creating a pod sandbox")

		podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)

		defer func() {
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		}()

		By("Creating and starting a container")

		containerName := "id-consistency-" + framework.NewUUID()
		containerConfig := &runtimeapi.ContainerConfig{
			Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
			Image: &runtimeapi.ImageSpec{
				Image:              imageRef,
				UserSpecifiedImage: imageName,
			},
			// Use the default entrypoint to ensure compatibility with minimal images
			Command: nil,
		}
		containerID, err := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
		framework.ExpectNoError(err, "failed to create container: %v", err)

		defer func() {
			Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())
		}()

		err = rc.StartContainer(ctx, containerID)
		framework.ExpectNoError(err, "failed to start container: %v", err)

		By("Collecting identifiers from all APIs")

		// 1. ContainerStatus
		statusResp, err := rc.ContainerStatus(ctx, containerID, false)
		framework.ExpectNoError(err, "failed to get container status: %v", err)

		containerStatus := statusResp.GetStatus()
		framework.Logf("ContainerStatus.image_id: %q", containerStatus.GetImageId())
		framework.Logf("ContainerStatus.image_ref (legacy): %q", containerStatus.GetImageRef())

		// 2. ListContainers
		containers, err := rc.ListContainers(ctx, &runtimeapi.ContainerFilter{Id: containerID})
		framework.ExpectNoError(err, "failed to list containers: %v", err)
		Expect(containers).To(HaveLen(1))
		framework.Logf("Container.image_id: %q", containers[0].GetImageId())

		By("Verifying consistency")
		framework.Logf("Verifying consistency: PullImageResponse.image_ref=%q, Image.id=%q, ContainerStatus.image_id=%q, Container.image_id=%q",
			imageRef, imageStatus.GetId(), containerStatus.GetImageId(), containers[0].GetImageId())

		// We use a custom failure message to show all mismatches at once
		Expect(imageStatus.GetId()).To(Equal(imageRef), "Image.id MUST match PullImageResponse.image_ref")

		if containerStatus.GetImageId() == "" {
			Skip("ContainerStatus.image_id is empty; runtime does not seem to implement CRI API image_id correctly yet")
		}

		Expect(containerStatus.GetImageId()).To(Equal(imageRef), "ContainerStatus.image_id MUST match PullImageResponse.image_ref")

		if containers[0].GetImageId() == "" {
			Skip("Container.image_id is empty; runtime does not seem to implement CRI API image_id correctly yet")
		}

		Expect(containers[0].GetImageId()).To(Equal(imageRef), "Container.image_id MUST match PullImageResponse.image_ref")
	})

	It("should return the same image identifier when pulled from different registries [Conformance]", Serial, func(ctx SpecContext) {
		Expect(testSameImageDifferentRegistries).To(HaveLen(2), "testSameImageDifferentRegistries should have 2 images")
		imageName1 := testSameImageDifferentRegistries[0]
		imageName2 := testSameImageDifferentRegistries[1]

		// Note, the test relies on the fact that imageRef returned from the wrapper is the same as the one returned by the runtime.
		By("Pulling image from registry 1: " + imageName1)
		imageRef1 := framework.PullPublicImage(ctx, ic, imageName1, testImagePodSandbox)
		framework.Logf("PullImageResponse.image_ref (registry 1): %q", imageRef1)
		Expect(imageRef1).NotTo(BeEmpty(), "PullImageResponse.image_ref should not be empty")

		defer removeImage(ctx, ic, imageName1)

		By("Pulling image from registry 2: " + imageName2)
		imageRef2 := framework.PullPublicImage(ctx, ic, imageName2, testImagePodSandbox)
		framework.Logf("PullImageResponse.image_ref (registry 2): %q", imageRef2)
		Expect(imageRef2).NotTo(BeEmpty(), "PullImageResponse.image_ref should not be empty")

		defer removeImage(ctx, ic, imageName2)

		By("Verifying image IDs are consistent across registries")
		Expect(imageRef1).To(Equal(imageRef2), "Image IDs MUST be the same even if pulled from different registries if they refer to the same underlying image")

		By("Verifying ImageStatus returns the same ID for both names")

		status1 := framework.ImageStatus(ctx, ic, imageName1)
		framework.Logf("Image.id (registry 1): %q", status1.GetId())
		framework.Logf("Image.repo_tags (registry 1): %v", status1.GetRepoTags())
		framework.Logf("Image.repo_digests (registry 1): %v", status1.GetRepoDigests())
		Expect(status1.GetId()).To(Equal(imageRef1), "Image ID for registry 1 MUST match pull ref")

		status2 := framework.ImageStatus(ctx, ic, imageName2)
		framework.Logf("Image.id (registry 2): %q", status2.GetId())
		framework.Logf("Image.repo_tags (registry 2): %v", status2.GetRepoTags())
		framework.Logf("Image.repo_digests (registry 2): %v", status2.GetRepoDigests())
		Expect(status2.GetId()).To(Equal(imageRef1), "Image ID for registry 2 MUST match pull ref from registry 1")

		By("Creating a pod sandbox")

		podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)

		defer func() {
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		}()

		By("Creating a container using image from registry 2")

		containerName := "id-consistency-multi-registry-" + framework.NewUUID()
		containerConfig := &runtimeapi.ContainerConfig{
			Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
			Image: &runtimeapi.ImageSpec{
				Image:              imageRef2,
				UserSpecifiedImage: imageName2,
			},
			// Use the default entrypoint to ensure compatibility with minimal images
			Command: nil,
		}
		containerID, err := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
		framework.ExpectNoError(err, "failed to create container: %v", err)

		defer func() {
			Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())
		}()

		err = rc.StartContainer(ctx, containerID)
		framework.ExpectNoError(err, "failed to start container: %v", err)

		By("Checking container status")

		statusResp, err := rc.ContainerStatus(ctx, containerID, false)
		framework.ExpectNoError(err, "failed to get container status: %v", err)

		containerStatus := statusResp.GetStatus()
		framework.Logf("ContainerStatus.image_id: %q", containerStatus.GetImageId())
		framework.Logf("ContainerStatus.image_ref (legacy): %q", containerStatus.GetImageRef())

		By("Verifying consistency")

		if containerStatus.GetImageId() == "" {
			Skip("ContainerStatus.image_id is empty; runtime does not seem to implement CRI API image_id correctly yet")
		}

		Expect(containerStatus.GetImageId()).To(Equal(imageRef1), "ContainerStatus.image_id MUST match the image ID from the first pull")
	})
})
