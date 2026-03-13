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
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("Idempotence", func() {
	f := framework.NewDefaultCRIFramework()

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
		It("should not return an error if not found", func(ctx SpecContext) {
			By("stop not existing sandbox")
			Expect(rc.StopPodSandbox(ctx, uuid.New().String())).NotTo(HaveOccurred())
		})

		It("should not return an error if already stopped", func(ctx SpecContext) {
			By("run PodSandbox")

			podID := framework.RunDefaultPodSandbox(ctx, rc, "idempotence-pod-")

			By("stop sandbox")
			testStopPodSandbox(ctx, rc, podID)

			By("stop sandbox again")
			testStopPodSandbox(ctx, rc, podID)

			By("delete sandbox as cleanup")
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		})
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L53-L54
	Context("RemovePodSandbox", func() {
		It("should not return an error if not found", func(ctx SpecContext) {
			By("remove not existing sandbox")
			Expect(rc.RemovePodSandbox(ctx, uuid.New().String())).NotTo(HaveOccurred())
		})
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L67-L68
	Context("StopContainer", func() {
		It("should not return an error if not found", func(ctx SpecContext) {
			By("test stop not existing container")
			Expect(rc.StopContainer(ctx, uuid.New().String(), 0)).NotTo(HaveOccurred())
		})

		It("should not return an error if already stopped", func(ctx SpecContext) {
			By("create sandbox")

			podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)

			By("create container")

			containerID := framework.CreatePauseContainer(ctx, rc, ic, podID, podConfig, "idempotence-container-")

			By("start container")
			startContainer(ctx, rc, containerID)

			By("stop container")
			testStopContainer(ctx, rc, containerID)

			By("remove container")
			removeContainer(ctx, rc, containerID)

			By("remove container again")
			removeContainer(ctx, rc, containerID)

			By("stop sandbox as cleanup")
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())

			By("delete sandbox as cleanup")
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		})
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L74-L75
	Context("RemoveContainer", func() {
		It("should not return an error if not found", func(ctx SpecContext) {
			By("remove not existing container")
			Expect(rc.RemoveContainer(ctx, uuid.New().String())).NotTo(HaveOccurred())
		})
	})

	// https://github.com/kubernetes/cri-api/blob/c20fa40/pkg/apis/runtime/v1/api.proto#L156-L157
	Context("RemoveImage", func() {
		It("should not return an error if not found", func(ctx SpecContext) {
			By("remove not existing image")

			const fakeImageID = "0000000000000000000000000000000000000000000000000000000000000000"
			Expect(ic.RemoveImage(ctx, &runtimeapi.ImageSpec{Image: fakeImageID})).NotTo(HaveOccurred())

			By("remove the image again")
			Expect(ic.RemoveImage(ctx, &runtimeapi.ImageSpec{Image: fakeImageID})).NotTo(HaveOccurred())
		})
	})
})
