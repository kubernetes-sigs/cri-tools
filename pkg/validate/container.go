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
	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("Container", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support basic operations on container", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support creating container [Conformance]", func() {
			By("test create a default container")
			containerID := framework.TestCreateDefaultContainer(rc, ic, podID, podConfig)

			By("test list container")
			containers := framework.ListContainerForID(rc, containerID)
			Expect(framework.ContainerFound(containers, containerID)).To(BeTrue(), "Container should be created")
		})

		It("runtime should support starting container [Conformance]", func() {
			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-start-test-")

			By("test start container")
			framework.TestStartContainer(rc, containerID)
		})

		It("runtime should support stopping container [Conformance]", func() {
			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-stop-test-")

			By("start container")
			framework.StartContainer(rc, containerID)

			By("test stop container")
			framework.TestStopContainer(rc, containerID)
		})

		It("runtime should support removing container [Conformance]", func() {
			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-remove-test-")

			By("test remove container")
			framework.RemoveContainer(rc, containerID)
			containers := framework.ListContainerForID(rc, containerID)
			Expect(framework.ContainerFound(containers, containerID)).To(BeFalse(), "Container should be removed")
		})

		It("runtime should support execSync [Conformance]", func() {
			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-execSync-test-")

			By("start container")
			framework.StartContainer(rc, containerID)

			By("test execSync")
			framework.TestExecSync(rc, containerID)
		})
	})
})
