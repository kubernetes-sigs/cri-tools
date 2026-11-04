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

package nri

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

// invalidRequestUnknownID is an object ID that no runtime will have allocated.
const invalidRequestUnknownID = "nri-test-invalid-request-unknown-id"

// expectNoNRIEventsSince asserts that no NRI event beyond the first
// baseline events is recorded by the plugin over a short time window.
func expectNoNRIEventsSince(plugin *NRITestPlugin, baseline int, description string) {
	GinkgoHelper()

	Consistently(func() []NRIEvent {
		return plugin.Events()[baseline:]
	}, 2*time.Second, 200*time.Millisecond).Should(BeEmpty(),
		"NRI hooks MUST NOT fire for %s", description)
}

// invalidRequestContainerConfig returns a valid container config with a
// unique name, used as the base for the invalid CreateContainer requests.
func invalidRequestContainerConfig() *runtimeapi.ContainerConfig {
	return &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(
			"nri-test-invalid-req-ctr-"+framework.NewUUID(),
			framework.DefaultAttempt,
		),
		Image: &runtimeapi.ImageSpec{
			Image: framework.TestContext.TestImageList.DefaultTestContainerImage,
		},
		Command: framework.DefaultPauseCommand,
		Linux:   &runtimeapi.LinuxContainerConfig{},
	}
}

var _ = framework.KubeDescribe("NRI", func() {
	f := framework.NewDefaultCRIFramework()

	var (
		rc internalapi.RuntimeService
		ic internalapi.ImageManagerService
	)

	BeforeEach(func() {
		if framework.TestContext.NRISocketPath == "" {
			Skip("NRI socket not configured (use -nri-socket flag)")
		}

		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("invalid CRI requests", Serial, func() {
		var (
			testStub *NRITestStub
			podIDs   []string
		)

		BeforeEach(func() {
			podIDs = nil

			var err error

			testStub, err = StartNRITestStub("cri-test-nri-invalid-req", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")
		})

		AfterEach(func(ctx SpecContext) {
			if testStub != nil {
				testStub.Cleanup()
				testStub = nil
			}

			for _, id := range podIDs {
				if err := rc.StopPodSandbox(ctx, id); err != nil {
					framework.Logf("AfterEach: StopPodSandbox(%s) failed: %v", id, err)
				}

				if err := rc.RemovePodSandbox(ctx, id); err != nil {
					framework.Logf("AfterEach: RemovePodSandbox(%s) failed: %v", id, err)
				}
			}
		})

		It(
			"should not invoke NRI hooks for CreateContainer in a non-existing sandbox",
			func(ctx SpecContext) {
				// A pod config is still required by the CRI call; it describes a
				// sandbox that was never created.
				podConfig := &runtimeapi.PodSandboxConfig{
					Metadata: framework.BuildPodSandboxMetadata(
						"nri-test-invalid-req-missing-pod-"+framework.NewUUID(),
						framework.DefaultUIDPrefix+framework.NewUUID(),
						framework.DefaultNamespacePrefix+framework.NewUUID(),
						framework.DefaultAttempt,
					),
					Labels: framework.DefaultPodLabels,
				}

				baseline := len(testStub.Plugin.Events())

				By("creating a container in a non-existing sandbox")

				_, err := framework.CreateContainerWithError(
					ctx,
					rc,
					ic,
					invalidRequestContainerConfig(),
					invalidRequestUnknownID,
					podConfig,
				)
				Expect(err).To(HaveOccurred(),
					"CreateContainer for a non-existing sandbox MUST fail")

				expectNoNRIEventsSince(testStub.Plugin, baseline,
					"CreateContainer in a non-existing sandbox")
			},
		)

		It(
			"should not invoke NRI hooks for CreateContainer without metadata",
			func(ctx SpecContext) {
				By("creating a pod sandbox")

				podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)
				podIDs = append(podIDs, podID)

				baseline := len(testStub.Plugin.Events())

				By("creating a container without metadata")

				containerConfig := invalidRequestContainerConfig()
				containerConfig.Metadata = nil

				ctrID, err := framework.CreateContainerWithError(
					ctx, rc, ic, containerConfig, podID, podConfig,
				)
				Expect(err).To(HaveOccurred(),
					"CreateContainer without container metadata MUST fail, got container %q", ctrID)

				expectNoNRIEventsSince(testStub.Plugin, baseline,
					"CreateContainer without metadata")
			},
		)

		It("should not invoke NRI hooks for RunPodSandbox without metadata", func(ctx SpecContext) {
			baseline := len(testStub.Plugin.Events())

			By("running a pod sandbox without metadata")

			podID := framework.RunPodSandboxError(ctx, rc, &runtimeapi.PodSandboxConfig{
				Labels: framework.DefaultPodLabels,
			})
			if podID != "" {
				podIDs = append(podIDs, podID)
			}

			expectNoNRIEventsSince(testStub.Plugin, baseline,
				"RunPodSandbox without metadata")
		})

		It(
			"should not invoke NRI hooks for RunPodSandbox duplicating an existing sandbox's metadata",
			func(ctx SpecContext) {
				By("creating a pod sandbox")

				podID, podConfig := framework.CreatePodSandboxForContainer(ctx, rc)
				podIDs = append(podIDs, podID)

				baseline := len(testStub.Plugin.Events())

				By("running a second pod sandbox with the same name, uid, namespace and attempt")

				dupID, err := rc.RunPodSandbox(ctx, podConfig, framework.TestContext.RuntimeHandler)
				if dupID != "" && dupID != podID {
					podIDs = append(podIDs, dupID)
				}

				// The CRI metadata uniquely identifies a sandbox, so the runtime MUST
				// NOT create a second sandbox for the duplicate request. Runtimes do
				// that in either of two ways: containerd rejects the request because
				// the sandbox name is already reserved, while CRI-O returns the
				// existing sandbox ID so that a kubelet retry is idempotent. Both are
				// acceptable; creating another sandbox is not, and no NRI hook may run
				// in either case.
				if err == nil {
					Expect(dupID).To(Equal(podID),
						"RunPodSandbox with metadata (name/uid/namespace/attempt) identical to the "+
							"existing sandbox %q MUST either fail or return that same sandbox, "+
							"got new sandbox %q", podID, dupID)
				} else {
					framework.Logf("RunPodSandbox with duplicated metadata was rejected: %v", err)
				}

				expectNoNRIEventsSince(testStub.Plugin, baseline,
					"RunPodSandbox duplicating an existing sandbox's metadata")
			},
		)

		It(
			"should not invoke NRI hooks for container operations on an unknown container ID",
			func(ctx SpecContext) {
				baseline := len(testStub.Plugin.Events())

				By("starting an unknown container")
				Expect(rc.StartContainer(ctx, invalidRequestUnknownID)).To(HaveOccurred(),
					"StartContainer for an unknown container MUST fail")

				// The CRI allows StopContainer and RemoveContainer to succeed for a
				// container that does not exist, so only the absence of NRI hooks is
				// asserted for them.
				By("stopping an unknown container")

				if err := rc.StopContainer(ctx, invalidRequestUnknownID, 0); err != nil {
					framework.Logf("StopContainer(%s) returned: %v", invalidRequestUnknownID, err)
				}

				By("removing an unknown container")

				if err := rc.RemoveContainer(ctx, invalidRequestUnknownID); err != nil {
					framework.Logf("RemoveContainer(%s) returned: %v", invalidRequestUnknownID, err)
				}

				expectNoNRIEventsSince(testStub.Plugin, baseline,
					"StartContainer, StopContainer or RemoveContainer on an unknown container")
			},
		)

		It(
			"should not invoke NRI hooks for pod sandbox operations on an unknown sandbox ID",
			func(ctx SpecContext) {
				baseline := len(testStub.Plugin.Events())

				// The CRI allows StopPodSandbox and RemovePodSandbox to succeed for a
				// sandbox that does not exist, so only the absence of NRI hooks is
				// asserted.
				By("stopping an unknown pod sandbox")

				if err := rc.StopPodSandbox(ctx, invalidRequestUnknownID); err != nil {
					framework.Logf("StopPodSandbox(%s) returned: %v", invalidRequestUnknownID, err)
				}

				By("removing an unknown pod sandbox")

				if err := rc.RemovePodSandbox(ctx, invalidRequestUnknownID); err != nil {
					framework.Logf(
						"RemovePodSandbox(%s) returned: %v",
						invalidRequestUnknownID,
						err,
					)
				}

				expectNoNRIEventsSince(testStub.Plugin, baseline,
					"StopPodSandbox or RemovePodSandbox on an unknown sandbox")
			},
		)
	})
})
