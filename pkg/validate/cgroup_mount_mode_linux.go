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
	"os"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("CgroupMountMode", func() {
	f := framework.NewDefaultCRIFramework()

	var (
		rc              internalapi.RuntimeService
		ic              internalapi.ImageManagerService
		podID           string
		podConfig       *runtimeapi.PodSandboxConfig
		containerConfig *runtimeapi.ContainerConfig
		nsdelegate      bool
	)

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
		podID = ""
		nsdelegate = false

		// Run critest on the runtime's host to check its cgroup mount prerequisites.
		mounts, err := os.ReadFile("/proc/self/mounts")
		framework.ExpectNoError(err, "read cgroup mount prerequisites")

		cgroupV2 := false

		for line := range strings.SplitSeq(string(mounts), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 && fields[1] == "/sys/fs/cgroup" && fields[2] == "cgroup2" {
				cgroupV2 = true
				nsdelegate = slices.Contains(strings.Split(fields[3], ","), "nsdelegate")

				break
			}
		}

		if !cgroupV2 {
			Skip("CgroupMountMode requires cgroup v2")
		}

		containerConfig = &runtimeapi.ContainerConfig{
			Metadata: framework.BuildContainerMetadata(
				"cgroup-mount-mode-"+framework.NewUUID(),
				framework.DefaultAttempt,
			),
			Image: &runtimeapi.ImageSpec{
				Image: framework.TestContext.TestImageList.DefaultTestContainerImage,
			},
			Command: framework.DefaultContainerCommand,
			Linux:   &runtimeapi.LinuxContainerConfig{},
		}
	})

	AfterEach(func(ctx SpecContext) {
		if podID != "" {
			framework.CleanupPodSandbox(ctx, rc, podID)
		}
	})

	DescribeTable(
		"runtime should start a container with its default cgroup mount mode [Conformance]",
		func(ctx SpecContext, securityContext *runtimeapi.LinuxContainerSecurityContext) {
			podID, podConfig = framework.CreatePodSandboxForContainer(ctx, rc)
			containerConfig.Linux.SecurityContext = securityContext
			containerID := framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
			testStartContainer(ctx, rc, containerID)
		},
		Entry("without a security context", nil),
		Entry("with CGROUP_MOUNT_MODE_UNSPECIFIED", &runtimeapi.LinuxContainerSecurityContext{
			CgroupMountMode: runtimeapi.CgroupMountMode_CGROUP_MOUNT_MODE_UNSPECIFIED,
		}),
	)

	It(
		"runtime should mount cgroups read-only with CGROUP_MOUNT_MODE_READ_ONLY [Conformance]",
		func(ctx SpecContext) {
			podID, podConfig = framework.CreatePodSandboxForContainer(ctx, rc)
			containerConfig.Linux.SecurityContext = &runtimeapi.LinuxContainerSecurityContext{
				CgroupMountMode: runtimeapi.CgroupMountMode_CGROUP_MOUNT_MODE_READ_ONLY,
			}
			containerID := framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
			startContainer(ctx, rc, containerID)

			By("verifying cgroup creation fails on a read-only filesystem")

			_, stderr, err := rc.ExecSync(
				ctx,
				containerID,
				[]string{
					"mkdir",
					"/sys/fs/cgroup/critest",
				},
				time.Duration(defaultExecSyncTimeout)*time.Second,
			)
			Expect(err).To(HaveOccurred())
			Expect(string(stderr)).To(ContainSubstring("Read-only file system"))
		},
	)

	Context("when the runtime advertises cgroup_mount_mode", func() {
		BeforeEach(func(ctx SpecContext) {
			By("checking the RuntimeFeatures advertisement")

			status, err := rc.Status(ctx, false)
			framework.ExpectNoError(err, "get runtime features")
			framework.Logf(
				"RuntimeFeatures.cgroup_mount_mode: %t",
				status.GetFeatures().GetCgroupMountMode(),
			)

			if !status.GetFeatures().GetCgroupMountMode() {
				Skip("The runtime does not advertise cgroup_mount_mode")
			}

			containerConfig.Linux.SecurityContext = &runtimeapi.LinuxContainerSecurityContext{
				CgroupMountMode: runtimeapi.CgroupMountMode_CGROUP_MOUNT_MODE_WRITABLE,
			}
		})

		It(
			"runtime should allow cgroup creation with CGROUP_MOUNT_MODE_WRITABLE [Conformance]",
			func(ctx SpecContext) {
				if !nsdelegate {
					Skip("Writable cgroups require nsdelegate")
				}

				podID, podConfig = framework.CreatePodSandboxForContainer(ctx, rc)
				containerID := framework.CreateContainer(
					ctx,
					rc,
					ic,
					containerConfig,
					podID,
					podConfig,
				)
				startContainer(ctx, rc, containerID)

				By("creating and removing a cgroup in the container")

				execSyncContainer(ctx, rc, containerID, []string{"mkdir", "/sys/fs/cgroup/critest"})
				execSyncContainer(ctx, rc, containerID, []string{"rmdir", "/sys/fs/cgroup/critest"})
			},
		)

		It(
			"runtime should reject CGROUP_MOUNT_MODE_WRITABLE without nsdelegate [Conformance]",
			func(ctx SpecContext) {
				if nsdelegate {
					Skip("Requires cgroup v2 without nsdelegate")
				}

				podID, podConfig = framework.CreatePodSandboxForContainer(ctx, rc)

				By(
					"verifying the sandbox and image can create a container with an unspecified mount mode",
				)

				containerID := framework.CreateDefaultContainer(
					ctx,
					rc,
					ic,
					podID,
					podConfig,
					"cgroup-read-only-",
				)
				startContainer(ctx, rc, containerID)

				By("verifying container creation rejects the writable request")

				_, err := framework.CreateContainerWithError(
					ctx,
					rc,
					ic,
					containerConfig,
					podID,
					podConfig,
				)
				Expect(err).To(HaveOccurred())
			},
		)
	})
})
