/*
Copyright 2024 The Kubernetes Authors.

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

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("Idempotence", func() {
	f := framework.NewDefaultCRIFramework()
	c := context.Background()

	var (
		rc internalapi.RuntimeService
		ic internalapi.ImageManagerService
	)

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L45-L46
	Context("StopPodSandbox", func() {
		It("should not return an error if not found", func() {
			By("stop not existing sandbox")
			Expect(rc.StopPodSandbox(c, uuid.New().String())).NotTo(HaveOccurred())
		})

		It("should not return an error if already stopped", func() {
			By("run PodSandbox")
			podID := framework.RunDefaultPodSandbox(rc, "idempotence-pod-")

			By("stop sandbox")
			testStopPodSandbox(rc, podID)

			By("stop sandbox again")
			testStopPodSandbox(rc, podID)

			By("delete sandbox as cleanup")
			Expect(rc.RemovePodSandbox(c, podID)).NotTo(HaveOccurred())
		})
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L53-L54
	Context("RemovePodSandbox", func() {
		It("should not return an error if not found", func() {
			By("remove not existing sandbox")
			Expect(rc.RemovePodSandbox(c, uuid.New().String())).NotTo(HaveOccurred())
		})
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L67-L68
	Context("StopContainer", func() {
		It("should not return an error if not found", func() {
			By("test stop not existing container")
			Expect(rc.StopContainer(c, uuid.New().String(), 0)).NotTo(HaveOccurred())
		})

		It("should not return an error if already stopped", func() {
			By("create sandbox")
			podID, podConfig := framework.CreatePodSandboxForContainer(rc)

			By("create container")
			containerID := framework.CreatePauseContainer(rc, ic, podID, podConfig, "idempotence-container-")

			By("start container")
			startContainer(rc, containerID)

			By("stop container")
			testStopContainer(rc, containerID)

			By("remove container")
			removeContainer(rc, containerID)

			By("remove container again")
			removeContainer(rc, containerID)

			By("stop sandbox as cleanup")
			Expect(rc.StopPodSandbox(c, podID)).NotTo(HaveOccurred())

			By("delete sandbox as cleanup")
			Expect(rc.RemovePodSandbox(c, podID)).NotTo(HaveOccurred())
		})
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L74-L75
	Context("RemoveContainer", func() {
		It("should not return an error if not found", func() {
			By("remove not existing container")
			Expect(rc.RemoveContainer(c, uuid.New().String())).NotTo(HaveOccurred())
		})
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L156-L157
	Context("RemoveImage", func() {
		It("should not return an error if not found", func() {
			By("remove not existing image")
			const fakeImageID = "0000000000000000000000000000000000000000000000000000000000000000"
			Expect(ic.RemoveImage(c, &runtimeapi.ImageSpec{Image: fakeImageID})).NotTo(HaveOccurred())

			By("remove the image again")
			Expect(ic.RemoveImage(c, &runtimeapi.ImageSpec{Image: fakeImageID})).NotTo(HaveOccurred())
		})
	})
})
