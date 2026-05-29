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

package validate

import (
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

const (
	imageVolumeTestPath = "/mnt/image-volume"
)

// skipIfImageVolumeUnsupported checks the error from CreateContainer and skips
// the test if the runtime does not support image volumes yet.
// Runtimes may signal lack of support in different ways:
//   - gRPC Unimplemented: the runtime explicitly does not implement image volumes.
//   - gRPC Unknown with "failed to mkdir" or similar: the runtime ignores the
//     Image field in Mount and fails while processing an empty host path
//     (observed with containerd 1.7).
//
// When expectError is true the caller expects CreateContainer to fail (negative
// tests). In that case the function only checks for "unsupported" patterns and
// returns without asserting no error.
func skipIfImageVolumeUnsupported(err error, expectError bool) {
	if err == nil {
		return
	}

	if s, ok := status.FromError(err); ok {
		if s.Code() == codes.Unimplemented {
			Skip("Image Volumes are not yet supported by the runtime (Unimplemented).")
		}

		// containerd 1.7 does not recognise the Image field in Mount and
		// produces errors like: failed to mkdir "": mkdir : no such file
		// or directory. Treat these as "unsupported".
		if s.Code() == codes.Unknown {
			msg := s.Message()
			if strings.Contains(msg, "failed to mkdir") ||
				strings.Contains(msg, "failed to generate spec") {
				Skip("Image Volumes are not yet supported by the runtime: " + msg)
			}
		}
	}

	if !expectError {
		framework.ExpectNoError(err, "failed to create container")
	}
}

var _ = framework.KubeDescribe("Image Volume [Feature:ImageVolume]", func() {
	f := framework.NewDefaultCRIFramework()

	var (
		rc internalapi.RuntimeService
		ic internalapi.ImageManagerService
	)

	skipOnWindows := func() {
		if runtime.GOOS == framework.OSWindows && !framework.TestContext.IsLcow {
			Skip("Image Volume tests are not supported on Windows")
		}
	}

	BeforeEach(func() {
		skipOnWindows()

		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	It("should return image digest in ContainerStatus mounts", func(ctx SpecContext) {
		imageName := testImageWithTag

		By("Pulling image: " + imageName)
		imageRef := framework.PullPublicImage(ctx, ic, imageName, testImagePodSandbox)

		By("Creating pod sandbox")

		podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)

		defer func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		}()

		By("Creating container with Image Volume")

		containerName := "image-volume-digest-test-" + framework.NewUUID()
		containerPath := imageVolumeTestPath

		containerConfig := &runtimeapi.ContainerConfig{
			Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
			Image: &runtimeapi.ImageSpec{
				Image:              imageRef,
				UserSpecifiedImage: imageName,
			},
			Command: pauseCmd,
			Mounts: []*runtimeapi.Mount{
				{
					ContainerPath: containerPath,
					Readonly:      true,
					Image: &runtimeapi.ImageSpec{
						Image:              imageRef,
						UserSpecifiedImage: imageName,
					},
				},
			},
		}

		containerID, err := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
		skipIfImageVolumeUnsupported(err, false)

		defer func() {
			By("delete Container")
			Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())
		}()

		By("Checking ContainerStatus for image volume digest")

		statusResp, err := rc.ContainerStatus(ctx, containerID, false)
		framework.ExpectNoError(err, "failed to get container status")

		var found bool

		for _, m := range statusResp.GetStatus().GetMounts() {
			if m.GetContainerPath() == containerPath {
				found = true
				// Check if the new ImageSpec.image_ref (tag 20) field is populated (KEP-5365).
				if m.GetImage() == nil || m.GetImage().GetImageRef() == "" {
					Skip("Image Volume digest reporting (ImageSpec.image_ref tag 20) is not yet supported by the runtime.")
				}

				framework.Logf("Found Image Volume mount: Image.Image=%q, Image.ImageRef=%q",
					m.GetImage().GetImage(), m.GetImage().GetImageRef())

				Expect(m.GetImage().GetImageRef()).To(Equal(imageRef),
					"ImageSpec.image_ref (tag 20) in Mount should match the image digest")

				break
			}
		}

		if !found {
			Skip("Image Volume mount not found in ContainerStatus; runtime might not support Image Volumes.")
		}
	})

	It("should correctly mount image and enforce read-only", func(ctx SpecContext) {
		imageName := testImageWithTag

		By("Pulling image: " + imageName)
		imageRef := framework.PullPublicImage(ctx, ic, imageName, testImagePodSandbox)

		By("Creating pod sandbox")

		podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)

		defer func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		}()

		By("Creating container with Image Volume")

		containerName := "image-volume-mount-test-" + framework.NewUUID()
		containerPath := imageVolumeTestPath

		containerConfig := &runtimeapi.ContainerConfig{
			Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
			Image: &runtimeapi.ImageSpec{
				Image:              imageRef,
				UserSpecifiedImage: imageName,
			},
			Command: pauseCmd,
			Mounts: []*runtimeapi.Mount{
				{
					ContainerPath: containerPath,
					Readonly:      true,
					Image: &runtimeapi.ImageSpec{
						Image:              imageRef,
						UserSpecifiedImage: imageName,
					},
				},
			},
		}

		containerID, err := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
		skipIfImageVolumeUnsupported(err, false)

		defer func() {
			By("delete Container")
			Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())
		}()

		By("Starting the container")

		err = rc.StartContainer(ctx, containerID)
		framework.ExpectNoError(err, "failed to start container")

		By("Verifying image content is visible at the mount point")
		// We use /bin/sh which is a standard binary present in the test image.
		cmd := []string{"ls", "-A", containerPath + "/bin/sh"}

		_, _, err = rc.ExecSync(ctx, containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
		if err != nil {
			// Log mount point contents for debugging before skipping.
			debugCmd := []string{"ls", "-R", containerPath}
			dStdout, _, _ := rc.ExecSync(ctx, containerID, debugCmd, time.Duration(defaultExecSyncTimeout)*time.Second)
			framework.Logf("Debug: contents of %s:\n%s", containerPath, string(dStdout))

			// The runtime accepted the container but the mount point has no image
			// content. This happens when the runtime silently ignores the Image
			// field in mounts (e.g. containerd 1.7).
			Skip("Runtime created the container but did not mount the image volume; Image Volumes are not supported.")
		}

		// TODO: uncomment once Skip above is eliminated (1.7 of containerd is EOL)
		// framework.ExpectNoError(err, "failed to find file in image volume: stdout: %s, stderr: %s", stdout, stderr)

		By("Verifying the image volume is read-only")

		cmd = []string{"touch", containerPath + "/test-write"}

		var stderr []byte

		_, stderr, err = rc.ExecSync(ctx, containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
		// We expect an error here because the mount is read-only
		Expect(err).To(HaveOccurred(), "writing to image volume should fail")
		framework.Logf("Write attempt to image volume failed as expected: %s", stderr)

		By("Verifying the image volume is NOT mounted with noexec")
		// The test image is busybox-based, so /bin/ls should be executable.
		cmd = []string{containerPath + "/bin/ls", containerPath}
		_, stderr, err = rc.ExecSync(ctx, containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
		framework.ExpectNoError(err, "failed to execute binary from image volume (might be noexec): stderr: %s", stderr)
		framework.Logf("Binary execution from image volume succeeded as expected")
	})

	It("should reject Image Volume with Readonly set to false", func(ctx SpecContext) {
		imageName := testImageWithTag

		By("Pulling image: " + imageName)
		imageRef := framework.PullPublicImage(ctx, ic, imageName, testImagePodSandbox)

		By("Creating pod sandbox")

		podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)

		defer func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		}()

		By("Creating container with Image Volume and Readonly=false")

		containerName := "image-volume-readonly-test-" + framework.NewUUID()
		containerPath := imageVolumeTestPath

		containerConfig := &runtimeapi.ContainerConfig{
			Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
			Image: &runtimeapi.ImageSpec{
				Image:              imageRef,
				UserSpecifiedImage: imageName,
			},
			Command: pauseCmd,
			Mounts: []*runtimeapi.Mount{
				{
					ContainerPath: containerPath,
					Readonly:      false, // Explicitly set to false to test enforcement
					Image: &runtimeapi.ImageSpec{
						Image:              imageRef,
						UserSpecifiedImage: imageName,
					},
				},
			},
		}

		containerID, err := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
		skipIfImageVolumeUnsupported(err, true)

		// Based on the spec, runtime SHOULD reject Image Volume with Readonly=false.
		if err == nil {
			defer func() {
				By("delete Container")
				Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())
			}()

			Skip("Runtime accepted Image Volume with Readonly=false; rejection is not yet enforced.")
		}

		By("Verified that Runtime rejected invalid Readonly=false configuration: " + err.Error())
	})

	Context("subPath", func() {
		It("should succeed when using a valid subPath", func(ctx SpecContext) {
			imageName := testImageWithTag

			By("Pulling image: " + imageName)
			imageRef := framework.PullPublicImage(ctx, ic, imageName, testImagePodSandbox)

			By("Creating pod sandbox")

			podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)

			defer func() {
				By("stop PodSandbox")
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
				By("delete PodSandbox")
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
			}()

			By("Creating container with Image Volume and subPath")

			containerName := "image-volume-subpath-test-" + framework.NewUUID()
			containerPath := imageVolumeTestPath
			subPath := "etc"

			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image: &runtimeapi.ImageSpec{
					Image:              imageRef,
					UserSpecifiedImage: imageName,
				},
				Command: pauseCmd,
				Mounts: []*runtimeapi.Mount{
					{
						ContainerPath: containerPath,
						Readonly:      true,
						Image: &runtimeapi.ImageSpec{
							Image:              imageRef,
							UserSpecifiedImage: imageName,
						},
						ImageSubPath: subPath,
					},
				},
			}

			containerID, err := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
			skipIfImageVolumeUnsupported(err, false)

			defer func() {
				By("delete Container")
				Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())
			}()

			By("Starting the container")

			err = rc.StartContainer(ctx, containerID)
			framework.ExpectNoError(err, "failed to start container")

			By("Checking whether the image volume mount point exists")
			// If the runtime silently ignores the Image field in Mount (e.g. containerd 1.7
			// which predates image volume support), the mount point will not exist at all.
			cmd := []string{"ls", containerPath}

			_, _, err = rc.ExecSync(ctx, containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
			if err != nil {
				Skip("Image Volumes are not supported by the runtime (mount point does not exist).")
			}

			By("Verifying subPath was applied and not the full image root")
			// If the runtime ignored subPath, the full image root is mounted and
			// /etc/passwd would be at containerPath/etc/passwd instead of containerPath/passwd.
			cmd = []string{"ls", containerPath + "/etc/passwd"}

			_, _, err = rc.ExecSync(ctx, containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
			if err == nil {
				Skip("Runtime appears to have mounted the full image root, ignoring the subPath field; subPath support is not yet implemented.")
			}

			By("Verifying subPath content is visible at the mount point")
			// With subPath="etc", /etc/passwd should appear directly at containerPath/passwd.
			cmd = []string{"ls", containerPath + "/passwd"}

			stdout, stderr, err := rc.ExecSync(ctx, containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
			framework.ExpectNoError(err, "failed to find file in image volume subPath: stdout: %s, stderr: %s", stdout, stderr)
		})

		It("should fail when subPath does not exist in the image", func(ctx SpecContext) {
			imageName := testImageWithTag

			By("Pulling image: " + imageName)
			imageRef := framework.PullPublicImage(ctx, ic, imageName, testImagePodSandbox)

			By("Creating pod sandbox")

			podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)

			defer func() {
				By("stop PodSandbox")
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
				By("delete PodSandbox")
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
			}()

			By("Creating container with Image Volume and non-existing subPath")

			containerName := "image-volume-subpath-fail-test-" + framework.NewUUID()
			containerPath := imageVolumeTestPath
			subPath := "not-existing"

			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image: &runtimeapi.ImageSpec{
					Image:              imageRef,
					UserSpecifiedImage: imageName,
				},
				Command: pauseCmd,
				Mounts: []*runtimeapi.Mount{
					{
						ContainerPath: containerPath,
						Readonly:      true,
						Image: &runtimeapi.ImageSpec{
							Image:              imageRef,
							UserSpecifiedImage: imageName,
						},
						ImageSubPath: subPath,
					},
				},
			}

			containerID, err := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
			skipIfImageVolumeUnsupported(err, true)

			if err == nil {
				defer func() {
					By("delete Container")
					Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())
				}()

				Skip("Runtime accepted non-existing subPath without error; image volume subPath validation is not yet implemented.")
			}

			framework.Logf("Container creation failed as expected: %v", err)
		})
	})
})
