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

var _ = framework.KubeDescribe("PodSandbox", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.RuntimeService

	BeforeEach(func() {
		c = f.CRIClient.CRIRuntimeClient
	})

	Context("runtime should support basic operations on PodSandbox", func() {
		var podID string
		var config *runtimeapi.PodSandboxConfig

		AfterEach(func() {
			By("stop PodSandbox")
			c.StopPodSandbox(podID)
			By("delete PodSandbox")
			c.RemovePodSandbox(podID)
		})

		It("runtime should support running PodSandbox [Conformance]", func() {
			By("test run a default PodSandbox")
			podID = framework.TestRunDefaultPodSandbox(c)

			By("test list PodSandbox")
			pods := framework.ListPodSandboxForID(c, podID)
			Expect(framework.PodSandboxFound(pods, podID)).To(BeTrue(), "PodSandbox should be listed")
		})

		It("runtime should support stopping PodSandbox [Conformance]", func() {
			By("run PodSandbox")
			podID, _ = framework.RunDefaultPodSandbox(c, "PodSandbox-for-test-stop-")

			By("test stop PodSandbox")
			framework.TestStopPodSandbox(c, podID)
		})

		It("runtime should support removing PodSandbox [Conformance]", func() {
			By("run PodSandbox")
			podID, _ = framework.RunDefaultPodSandbox(c, "PodSandbox-for-test-remove-")

			By("stop PodSandbox")
			framework.StopPodSandbox(c, podID)

			By("test remove PodSandbox")
			framework.TestRemovePodSandbox(c, podID)
		})
		It("runtime should support get PodSandbox status [conformance]", func() {
			By("run PodSandbox")
			podID, config = framework.RunDefaultPodSandbox(c, "PodSandbox-for-test-PodSandbox-status-")

			By("test PodSandbox run status")
			framework.VerifyPodSandboxStatus(c, podID, runtimeapi.PodSandboxState_SANDBOX_READY, "ready")
			Expect(framework.GetPodSandboxStatus(c, podID).Metadata).To(Equal(config.Metadata), "PodSandbox should be listed")

			By("stop PodSandbox")
			framework.StopPodSandbox(c, podID)

			By("test PodSandbox stop status")
			framework.VerifyPodSandboxStatus(c, podID, runtimeapi.PodSandboxState_SANDBOX_NOTREADY, "not ready")

			By("remove PodSandbox")
			framework.RemovePodSandbox(c, podID)
		})
	})
})
