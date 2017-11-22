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
	"time"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	"github.com/opencontainers/selinux/go-selinux"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("SELinux", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	if selinux.GetEnabled() {
		BeforeEach(func() {
			rc = f.CRIClient.CRIRuntimeClient
			ic = f.CRIClient.CRIImageClient
		})

		Context("runtime should support selinux", func() {
			var sandboxID string
			var sandboxConfig *runtimeapi.PodSandboxConfig

			BeforeEach(func() {
				sandboxID, sandboxConfig = framework.CreatePodSandboxForContainer(rc)
			})

			AfterEach(func() {
				By("stop PodSandbox")
				rc.StopPodSandbox(sandboxID)
				By("delete PodSandbox")
				rc.RemovePodSandbox(sandboxID)
			})

			It("should work with just selinux level set", func() {
				options := &runtimeapi.SELinuxOption{
					Level: "s0",
				}
				containerID := createContainerWithSelinux(rc, ic, sandboxID, sandboxConfig, options, true, true)
				checkContainerSelinux(rc, containerID, true)
			})

			It("should work with selinux set", func() {
				options := &runtimeapi.SELinuxOption{
					User:  "system_u",
					Role:  "system_r",
					Type:  "svirt_lxc_net_t",
					Level: "s0:c4,c5",
				}
				containerID := createContainerWithSelinux(rc, ic, sandboxID, sandboxConfig, options, true, true)
				checkContainerSelinux(rc, containerID, true)
			})

			It("should error on create with wrong options", func() {
				options := &runtimeapi.SELinuxOption{
					User: "system_u",
					Role: "system_r",
					Type: "svirt_lxc_net_t",
					// s0,c4,c5 is wrong, should have been s0:c4,c5
					Level: "s0,c4,c5",
				}
				_ = createContainerWithSelinux(rc, ic, sandboxID, sandboxConfig, options, false, false)
			})
		})
	}
})

func createContainerWithSelinux(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, sandboxID string, sandboxConfig *runtimeapi.PodSandboxConfig, options *runtimeapi.SELinuxOption, shouldStart, shouldCreate bool) string {
	By("create a container with selinux")
	containerName := "selinux-test-" + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"touch", "foo"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				SelinuxOptions: options,
			},
		},
	}
	containerID, err := framework.CreateContainerWithError(rc, ic, containerConfig, sandboxID, sandboxConfig)
	if !shouldCreate {
		Expect(err).To(HaveOccurred())
		return ""
	}

	Expect(err).NotTo(HaveOccurred())

	By("start container with selinux")
	err = rc.StartContainer(containerID)
	if shouldStart {
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(err).To(HaveOccurred())
	}

	// wait container exited and check the status.
	Eventually(func() runtimeapi.ContainerState {
		return getContainerStatus(rc, containerID).State
	}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

	return containerID
}

func checkContainerSelinux(rc internalapi.RuntimeService, containerID string, shoudRun bool) {
	By("get container status")
	status, err := rc.ContainerStatus(containerID)
	Expect(err).NotTo(HaveOccurred())

	if shoudRun {
		Expect(status.GetExitCode()).To(Equal(int32(0)))
	} else {
		Expect(status.GetExitCode()).NotTo(Equal(int32(0)))
	}
}
