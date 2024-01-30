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

	"github.com/kubernetes-sigs/cri-tools/pkg/framework"
	"golang.org/x/sys/unix"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			rc.StopPodSandbox(context.TODO(), podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(context.TODO(), podID)
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
				Expect(len(output)).To(BeZero(), "len(output) should be zero.")
			case runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL, runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
				Expect(len(output)).NotTo(BeZero(), "len(output) should not be zero.")
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
				Expect(len(fileInfo)).To(BeZero(), "len(fileInfo) should be zero.")
			case runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL:
				Expect(len(fileInfo)).NotTo(BeZero(), "len(fileInfo) should not be zero.")
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
			rc.StopPodSandbox(context.TODO(), podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(context.TODO(), podID)
		})

		It("should terminate with exitCode 137 and reason OOMKilled", func() {
			By("create container")
			containerID := createOOMKilledContainer(rc, ic, "OOM-test-", podID, podConfig)

			By("start container")
			startContainer(rc, containerID)

			By("container is stopped because of OOM")
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

			state := getContainerStatus(rc, containerID)

			By("exit code is 137")
			Expect(state.ExitCode, int32(137))

			By("reason is OOMKilled")
			Expect(state.Reason, "OOMKilled")
		})
	})
})

// createHostPath creates the hostPath for mount propagation test.
func createHostPathForMountPropagation(podID string, propagationOpt runtimeapi.MountPropagation) (string, string, string, func()) {
	hostPath, err := os.MkdirTemp("", "test"+podID)
	framework.ExpectNoError(err, "failed to create TempDir %q: %v", hostPath, err)

	mntSource := filepath.Join(hostPath, "mnt")
	propagationMntPoint := filepath.Join(mntSource, "propagationMnt")
	err = os.MkdirAll(propagationMntPoint, 0700)
	framework.ExpectNoError(err, "failed to create volume dir %q: %v", propagationMntPoint, err)

	propagationSrcDir := filepath.Join(hostPath, "propagationSrcDir")
	err = os.MkdirAll(propagationSrcDir, 0700)
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

	clearHostPath := func() {
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
	Expect(len(resp.Status.Mounts), 1)
	Expect(resp.Status.Mounts[0].ContainerPath).To(Equal(hostPath))
	Expect(resp.Status.Mounts[0].HostPath).To(Equal(hostPath))
	Expect(resp.Status.Mounts[0].Readonly).To(BeFalse())
	Expect(resp.Status.Mounts[0].SelinuxRelabel).To(BeFalse())

	By("verifying container status mount propagation")
	Expect(resp.Status.Mounts[0].Propagation).To(Equal(propagation))

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
