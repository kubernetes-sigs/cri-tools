/*
Copyright 2018 The Kubernetes Authors.

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
	"os"
	"path"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("Container Mount Propagation", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support mount propagation", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig = createPrivilegedPodSandbox(rc, true)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
		})

		testMountPropagation := func(propagation runtimeapi.MountPropagation) {
			By("create host path and flag file")
			mntSource, propagationSrcDir, propagationMntPoint, clearHostPath := createHostPathForMountPropagation(podID, propagation)
			defer clearHostPath() // clean up the TempDir

			By("create container with volume")
			containerID := createMountPropagationContainer(rc, ic, "mount-propagation-test-", podID, podConfig, mntSource, propagation)

			By("test start container with volume")
			testStartContainer(rc, containerID)

			By("create a propatation mount point in host")
			createPropagationMountPoint(propagationSrcDir, propagationMntPoint)

			By("check whether propagationMntPoint contains file or dir in container")
			command := []string{"ls", "-A", propagationMntPoint}
			output := execSyncContainer(rc, containerID, command)

			switch propagation {
			case runtimeapi.MountPropagation_PROPAGATION_PRIVATE:
				Expect(output).To(BeEmpty(), "len(output) should be zero.")
			case runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL, runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
				Expect(output).NotTo(BeEmpty(), "len(output) should not be zero.")
			}

			By("create a directory named containerMntPoint as a mount point in container")
			containerMntPoint := path.Join(mntSource, "containerMntPoint")
			command = []string{"sh", "-c", "mkdir -p " + containerMntPoint}
			execSyncContainer(rc, containerID, command)

			By("mount /etc to the mount point in container")
			command = []string{"sh", "-c", "mount --bind /etc " + containerMntPoint}
			execSyncContainer(rc, containerID, command)

			By("check whether containerMntPoint contains file or dir in host")
			fileInfo, err := os.ReadDir(containerMntPoint)
			framework.ExpectNoError(err, "failed to ReadDir %q in Host", containerMntPoint)

			switch propagation {
			case runtimeapi.MountPropagation_PROPAGATION_PRIVATE, runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
				Expect(fileInfo).To(BeEmpty(), "len(fileInfo) should be zero.")
			case runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL:
				Expect(fileInfo).NotTo(BeEmpty(), "len(fileInfo) should not be zero.")
			}
		}

		It("mount with 'rprivate' should not support propagation", func() {
			testMountPropagation(runtimeapi.MountPropagation_PROPAGATION_PRIVATE)
		})

		It("mount with 'rshared' should support propagation from host to container and vice versa", func() {
			testMountPropagation(runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL)
		})

		It("mount with 'rslave' should support propagation from host to container", func() {
			testMountPropagation(runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER)
		})
	})
})

var _ = framework.KubeDescribe("Container OOM", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should output OOMKilled reason", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig = createPrivilegedPodSandbox(rc, true)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
		})

		It("should terminate with exitCode 137 and reason OOMKilled", func() {
			By("create container")
			containerID := createOOMKilledContainer(rc, ic, "OOM-test-", podID, podConfig)

			By("start container")
			startContainer(rc, containerID)

			By("container is stopped because of OOM")
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

			Eventually(func() string {
				return getContainerStatus(rc, containerID).GetReason()
			}, time.Minute, time.Second*4).Should(Equal("OOMKilled"))
			state := getContainerStatus(rc, containerID)

			By("exit code is 137")
			Expect(state.GetExitCode()).To(BeEquivalentTo(137))
		})
	})
})

// createHostPath creates the hostPath for mount propagation test.
func createHostPathForMountPropagation(podID string, propagationOpt runtimeapi.MountPropagation) (mntSource, propagationSrcDir, propagationMntPoint string, clearHostPath func()) {
	hostPath, err := os.MkdirTemp("", "test"+podID)
	framework.ExpectNoError(err, "failed to create TempDir %q: %v", hostPath, err)

	mntSource = filepath.Join(hostPath, "mnt")
	propagationMntPoint = filepath.Join(mntSource, "propagationMnt")
	err = os.MkdirAll(propagationMntPoint, 0o700)
	framework.ExpectNoError(err, "failed to create volume dir %q: %v", propagationMntPoint, err)

	propagationSrcDir = filepath.Join(hostPath, "propagationSrcDir")
	err = os.MkdirAll(propagationSrcDir, 0o700)
	framework.ExpectNoError(err, "failed to create volume dir %q: %v", propagationSrcDir, err)

	_, err = os.Create(filepath.Join(propagationSrcDir, "flagFile"))
	framework.ExpectNoError(err, "failed to create volume file \"flagFile\": %v", err)

	switch propagationOpt {
	case runtimeapi.MountPropagation_PROPAGATION_PRIVATE:
		err := unix.Mount(mntSource, mntSource, "bind", unix.MS_BIND|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to mount \"mntSource\": %v", err)
		err = unix.Mount("", mntSource, "", unix.MS_PRIVATE|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to set \"mntSource\" to \"rprivate\": %v", err)
	case runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER,
		runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL:
		err := unix.Mount(mntSource, mntSource, "bind", unix.MS_BIND|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to mount \"mntSource\": %v", err)
		err = unix.Mount("", mntSource, "", unix.MS_SHARED|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to set \"mntSource\" to \"rshared\": %v", err)
	default:
		err := unix.Mount(mntSource, mntSource, "bind", unix.MS_BIND|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to mount \"mntSource\": %v", err)
		err = unix.Mount("", mntSource, "", unix.MS_PRIVATE|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to set \"mntSource\" to \"rprivate\": %v", err)
	}

	clearHostPath = func() {
		By("clean up the TempDir")

		err := unix.Unmount(propagationMntPoint, unix.MNT_DETACH)
		framework.ExpectNoError(err, "failed to unmount \"propagationMntPoint\": %v", err)
		err = unix.Unmount(mntSource, unix.MNT_DETACH)
		framework.ExpectNoError(err, "failed to unmount \"mntSource\": %v", err)

		os.RemoveAll(hostPath)
		framework.ExpectNoError(err, "failed to remove \"hostPath\": %v", err)
	}

	return mntSource, propagationSrcDir, propagationMntPoint, clearHostPath
}

// createMountPropagationContainer creates a container with volume and
// privileged security constraints. It also validates the container status
// after creation and fails if any error occurs.
func createMountPropagationContainer(
	rc internalapi.RuntimeService,
	ic internalapi.ImageManagerService,
	prefix string,
	podID string,
	podConfig *runtimeapi.PodSandboxConfig,
	hostPath string,
	propagation runtimeapi.MountPropagation,
) string {
	By("create a container with volume and name")

	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  pauseCmd,
		// Set Privileged in order to executing mount command in container
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				Privileged: true,
			},
		},
		Mounts: []*runtimeapi.Mount{
			{
				HostPath:      hostPath,
				ContainerPath: hostPath,
				Propagation:   propagation,
			},
		},
	}

	containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)

	By("verifying container status")

	resp, err := rc.ContainerStatus(context.TODO(), containerID, true)
	framework.ExpectNoError(err, "unable to get container status")
	Expect(resp.GetStatus().GetMounts()).To(HaveLen(1))
	Expect(resp.GetStatus().GetMounts()[0].GetContainerPath()).To(Equal(hostPath))
	Expect(resp.GetStatus().GetMounts()[0].GetHostPath()).To(Equal(hostPath))
	Expect(resp.GetStatus().GetMounts()[0].GetReadonly()).To(BeFalse())
	Expect(resp.GetStatus().GetMounts()[0].GetSelinuxRelabel()).To(BeFalse())

	By("verifying container status mount propagation")
	Expect(resp.GetStatus().GetMounts()[0].GetPropagation()).To(Equal(propagation))

	return containerID
}

// createPropagationMountPoint mount "propagationSrcDir" at "propagationMntPoint",
// this will be used to check whether mount can be propagated from host to container or not.
func createPropagationMountPoint(propagationSrcDir, propagationMntPoint string) {
	err := unix.Mount(propagationSrcDir, propagationMntPoint, "bind", unix.MS_BIND|unix.MS_REC, "")
	framework.ExpectNoError(err, "failed to mount \"propagationMntPoint\": %v", err)
}

func createOOMKilledContainer(
	rc internalapi.RuntimeService,
	ic internalapi.ImageManagerService,
	prefix string,
	podID string,
	podConfig *runtimeapi.PodSandboxConfig,
) string {
	By("create a container that will be killed by OOMKiller")

	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command: []string{
			"sh",
			"-c",
			"dd if=/dev/zero of=/dev/null bs=20M",
		},
		Linux: &runtimeapi.LinuxContainerConfig{
			Resources: &runtimeapi.LinuxContainerResources{
				MemoryLimitInBytes:     15 * 1024 * 1024,
				MemorySwapLimitInBytes: 15 * 1024 * 1024,
			},
		},
	}

	containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)

	By("verifying container status")

	_, err := rc.ContainerStatus(context.TODO(), containerID, true)
	framework.ExpectNoError(err, "unable to get container status")

	return containerID
}

var _ = framework.KubeDescribe("Container Mount Readonly", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support readonly mounts", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig = createPrivilegedPodSandbox(rc, true)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
		})

		testRRO := func(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, rro bool) {
			if rro && !runtimeSupportsRRO(rc, framework.TestContext.RuntimeHandler) {
				Skip("runtime does not implement recursive readonly mounts")

				return
			}

			By("create host path")
			hostPath, clearHostPath := createHostPathForRROMount(podID)
			defer clearHostPath() // clean up the TempDir

			By("create container with volume")
			containerID := createRROMountContainer(rc, ic, podID, podConfig, hostPath, "/mnt", rro)

			By("test start container with volume")
			testStartContainer(rc, containerID)

			By("check whether `touch /mnt/tmpfs/file` succeeds")
			command := []string{"touch", "/mnt/tmpfs/file"}
			if rro {
				command = []string{"sh", "-c", `touch /mnt/tmpfs/foo 2>&1 | grep -q "Read-only file system"`}
			}
			execSyncContainer(rc, containerID, command)
		}

		It("should support non-recursive readonly mounts", func() {
			testRRO(rc, ic, false)
		})
		It("should support recursive readonly mounts", func() {
			testRRO(rc, ic, true)
		})
		testRROInvalidPropagation := func(prop runtimeapi.MountPropagation) {
			if !runtimeSupportsRRO(rc, framework.TestContext.RuntimeHandler) {
				Skip("runtime does not implement recursive readonly mounts")

				return
			}
			hostPath, clearHostPath := createHostPathForRROMount(podID)
			defer clearHostPath() // clean up the TempDir
			mounts := []*runtimeapi.Mount{
				{
					HostPath:          hostPath,
					ContainerPath:     "/mnt",
					Readonly:          true,
					RecursiveReadOnly: true,
					SelinuxRelabel:    true,
					Propagation:       prop,
				},
			}
			const expectErr = true
			createMountContainer(rc, ic, podID, podConfig, mounts, expectErr)
		}
		It("should reject a recursive readonly mount with PROPAGATION_HOST_TO_CONTAINER", func() {
			testRROInvalidPropagation(runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER)
		})
		It("should reject a recursive readonly mount with PROPAGATION_BIDIRECTIONAL", func() {
			testRROInvalidPropagation(runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL)
		})
		It("should reject a recursive readonly mount with ReadOnly: false", func() {
			if !runtimeSupportsRRO(rc, framework.TestContext.RuntimeHandler) {
				Skip("runtime does not implement recursive readonly mounts")

				return
			}
			hostPath, clearHostPath := createHostPathForRROMount(podID)
			defer clearHostPath() // clean up the TempDir
			mounts := []*runtimeapi.Mount{
				{
					HostPath:          hostPath,
					ContainerPath:     "/mnt",
					Readonly:          false,
					RecursiveReadOnly: true,
					SelinuxRelabel:    true,
				},
			}
			const expectErr = true
			createMountContainer(rc, ic, podID, podConfig, mounts, expectErr)
		})
	})
})

func runtimeSupportsRRO(rc internalapi.RuntimeService, runtimeHandlerName string) bool {
	ctx := context.Background()
	status, err := rc.Status(ctx, false)
	framework.ExpectNoError(err, "failed to check runtime status")

	for _, h := range status.GetRuntimeHandlers() {
		if h.GetName() == runtimeHandlerName {
			if f := h.GetFeatures(); f != nil {
				return f.GetRecursiveReadOnlyMounts()
			}
		}
	}

	return false
}

// createHostPathForRROMount creates the hostPath for RRO mount test.
//
// hostPath contains a "tmpfs" directory with tmpfs mounted on it.
func createHostPathForRROMount(podID string) (hostPath string, clearHostPath func()) {
	hostPath, err := os.MkdirTemp("", "test"+podID)
	framework.ExpectNoError(err, "failed to create TempDir %q: %v", hostPath, err)

	tmpfsMntPoint := filepath.Join(hostPath, "tmpfs")
	err = os.MkdirAll(tmpfsMntPoint, 0o700)
	framework.ExpectNoError(err, "failed to create tmpfs dir %q: %v", tmpfsMntPoint, err)

	err = unix.Mount("none", tmpfsMntPoint, "tmpfs", 0, "")
	framework.ExpectNoError(err, "failed to mount tmpfs on dir %q: %v", tmpfsMntPoint, err)

	clearHostPath = func() {
		By("clean up the TempDir")

		err := unix.Unmount(tmpfsMntPoint, unix.MNT_DETACH)
		framework.ExpectNoError(err, "failed to unmount \"tmpfsMntPoint\": %v", err)
		err = os.RemoveAll(hostPath)
		framework.ExpectNoError(err, "failed to remove \"hostPath\": %v", err)
	}

	return hostPath, clearHostPath
}

func createRROMountContainer(
	rc internalapi.RuntimeService,
	ic internalapi.ImageManagerService,
	podID string,
	podConfig *runtimeapi.PodSandboxConfig,
	hostPath, containerPath string,
	rro bool,
) string {
	mounts := []*runtimeapi.Mount{
		{
			HostPath:          hostPath,
			ContainerPath:     containerPath,
			Readonly:          true,
			RecursiveReadOnly: rro,
			SelinuxRelabel:    true,
		},
	}

	return createMountContainer(rc, ic, podID, podConfig, mounts, false)
}

func createMountContainer(
	rc internalapi.RuntimeService,
	ic internalapi.ImageManagerService,
	podID string,
	podConfig *runtimeapi.PodSandboxConfig,
	mounts []*runtimeapi.Mount,
	expectErr bool,
) string {
	By("create a container with volume and name")

	containerName := "test-mount-" + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  pauseCmd,
		Mounts:   mounts,
	}

	if expectErr {
		_, err := framework.CreateContainerWithError(rc, ic, containerConfig, podID, podConfig)
		Expect(err).To(HaveOccurred())

		return ""
	}

	containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)

	By("verifying container status")

	resp, err := rc.ContainerStatus(context.TODO(), containerID, true)
	framework.ExpectNoError(err, "unable to get container status")
	Expect(resp.GetStatus().GetMounts()).To(HaveLen(len(mounts)))

	return containerID
}
