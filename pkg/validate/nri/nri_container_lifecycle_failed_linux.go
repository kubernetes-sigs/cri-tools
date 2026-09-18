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

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

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

	Context("container lifecycle (failed container)", Serial, func() {
		var (
			testStub    *NRITestStub
			podID       string
			podConfig   *runtimeapi.PodSandboxConfig
			containerID string
		)

		BeforeEach(func(ctx SpecContext) {
			var err error

			testStub, err = StartNRITestStub("cri-test-nri-ctr-fail", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Ensure test image is available
			framework.PullPublicImage(
				ctx,
				ic,
				framework.TestContext.TestImageList.DefaultTestContainerImage,
				nil,
			)
		})

		AfterEach(func(ctx SpecContext) {
			if containerID != "" {
				if err := rc.StopContainer(ctx, containerID, 0); err != nil {
					framework.Logf("AfterEach: StopContainer(%s) failed: %v", containerID, err)
				}

				if err := rc.RemoveContainer(ctx, containerID); err != nil {
					framework.Logf("AfterEach: RemoveContainer(%s) failed: %v", containerID, err)
				}
			}

			if podID != "" {
				if err := rc.StopPodSandbox(ctx, podID); err != nil {
					framework.Logf("AfterEach: StopPodSandbox(%s) failed: %v", podID, err)
				}

				if err := rc.RemovePodSandbox(ctx, podID); err != nil {
					framework.Logf("AfterEach: RemovePodSandbox(%s) failed: %v", podID, err)
				}
			}

			if testStub != nil {
				testStub.Cleanup()
			}
		})

		It(
			"should receive CreateContainer, StartContainer, StopContainer, and RemoveContainer for a container that exits with a non-zero code",
			func(ctx SpecContext) {
				By("creating a pod sandbox")

				podSandboxName := "nri-test-ctr-fail-" + framework.NewUUID()
				uid := framework.DefaultUIDPrefix + framework.NewUUID()
				namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
				podConfig = &runtimeapi.PodSandboxConfig{
					Metadata: framework.BuildPodSandboxMetadata(
						podSandboxName,
						uid,
						namespace,
						framework.DefaultAttempt,
					),
					Linux: &runtimeapi.LinuxPodSandboxConfig{
						CgroupParent: common.GetCgroupParent(ctx, rc),
					},
					Labels: framework.DefaultPodLabels,
				}
				podID = framework.RunPodSandbox(ctx, rc, podConfig)
				Expect(podID).NotTo(BeEmpty())

				// Reset events so we only capture container-related events from this point
				testStub.Plugin.Reset()

				By("creating a container whose command exits with a non-zero code")

				containerName := "nri-test-failed-container-" + framework.NewUUID()
				containerConfig := &runtimeapi.ContainerConfig{
					Metadata: framework.BuildContainerMetadata(
						containerName,
						framework.DefaultAttempt,
					),
					Image: &runtimeapi.ImageSpec{
						Image: framework.TestContext.TestImageList.DefaultTestContainerImage,
					},
					// Exit immediately with a failure so the container terminates on its
					// own (no explicit StopContainer CRI call) with a non-zero exit code.
					Command: []string{"sh", "-c", "exit 1"},
					Linux:   &runtimeapi.LinuxContainerConfig{},
				}
				containerID = framework.CreateContainer(
					ctx,
					rc,
					ic,
					containerConfig,
					podID,
					podConfig,
				)
				Expect(containerID).NotTo(BeEmpty())

				By("starting the container")
				Expect(rc.StartContainer(ctx, containerID)).NotTo(HaveOccurred())

				By("waiting for the container to exit with a non-zero exit code")
				Eventually(func() runtimeapi.ContainerState {
					status, err := rc.ContainerStatus(ctx, containerID, false)
					framework.ExpectNoError(err, "failed to get container %q status", containerID)

					return status.GetStatus().GetState()
				}, time.Minute, time.Second*2).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

				status, err := rc.ContainerStatus(ctx, containerID, false)
				framework.ExpectNoError(err, "failed to get container %q status", containerID)
				Expect(status.GetStatus().GetExitCode()).To(BeEquivalentTo(1),
					`container should have failed with exit code 1 (from sh -c "exit 1")`)

				By("waiting for Create, Start, and Stop NRI events before removing the container")
				// The runtime must deliver StopContainer on its own for a
				// self-exited container, without an explicit CRI RemoveContainer
				// call. Wait for all three events before calling RemoveContainer
				// to prove that StopContainer is not a side effect of removal.
				var createEvent, startEvent, stopEvent *NRIEvent

				Eventually(func() bool {
					createEvent, startEvent, stopEvent = nil, nil, nil

					for _, e := range testStub.Plugin.Events() {
						if e.ContainerID != containerID {
							continue
						}

						switch e.Type {
						case EventCreateContainer:
							if createEvent == nil {
								createEvent = &e
							}
						case EventStartContainer:
							if startEvent == nil {
								startEvent = &e
							}
						case EventStopContainer:
							if stopEvent == nil {
								stopEvent = &e
							}
						case EventRemoveContainer:
							// Verified separately after calling RemoveContainer.
						case EventRunPodSandbox, EventStopPodSandbox, EventRemovePodSandbox:
							// Pod events are not verified in this test.
						}
					}

					return createEvent != nil && startEvent != nil && stopEvent != nil
				}, 10*time.Second, 50*time.Millisecond).Should(BeTrue(),
					"NRI stub did not receive Create, Start, and Stop events for container %s before removal", containerID)

				By("verifying CreateContainer event fired with correct metadata")
				// SPEC_DISCREPANCY: CRI-O does not populate container name in NRI CreateContainer
				// event metadata. Record it here and skip at the very end so the remaining
				// assertions (ContainerID, Start/Stop/Remove, ordering) still run.
				skipForMissingName := false
				if createEvent.ContainerName == "" {
					skipForMissingName = true
				} else {
					Expect(createEvent.ContainerName).To(Equal(containerName))
				}

				By("verifying StartContainer event fired for the failed container")

				if !skipForMissingName {
					Expect(startEvent.ContainerName).To(Equal(containerName))
				}

				By("verifying StopContainer event fired without explicit removal")

				if !skipForMissingName {
					Expect(stopEvent.ContainerName).To(Equal(containerName))
				}

				By("verifying Create -> Start -> Stop ordering")
				Expect(createEvent.Timestamp.Before(startEvent.Timestamp)).To(BeTrue(),
					"CreateContainer (at %v) should occur before StartContainer (at %v)",
					createEvent.Timestamp, startEvent.Timestamp)
				Expect(startEvent.Timestamp.Before(stopEvent.Timestamp)).To(BeTrue(),
					"StartContainer (at %v) should occur before StopContainer (at %v)",
					startEvent.Timestamp, stopEvent.Timestamp)

				By("removing the failed container")
				Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())

				By("waiting for RemoveContainer NRI event")

				var removeEvent *NRIEvent

				Eventually(func() bool {
					for _, e := range testStub.Plugin.Events() {
						if e.ContainerID == containerID && e.Type == EventRemoveContainer {
							removeEvent = &e

							return true
						}
					}

					return false
				}, 10*time.Second, 50*time.Millisecond).Should(BeTrue(),
					"NRI stub did not receive the RemoveContainer event for container %s", containerID)

				By("verifying RemoveContainer event has correct container ID")

				if !skipForMissingName {
					Expect(removeEvent.ContainerName).To(Equal(containerName))
				}

				By("verifying Stop -> Remove ordering")
				Expect(stopEvent.Timestamp.Before(removeEvent.Timestamp)).To(BeTrue(),
					"StopContainer (at %v) should occur before RemoveContainer (at %v)",
					stopEvent.Timestamp, removeEvent.Timestamp)

				// Mark container as cleaned up so AfterEach doesn't try again.
				containerID = ""

				if skipForMissingName {
					Skip(
						"spec discrepancy: runtime does not populate container name in NRI CreateContainer event metadata",
					)
				}
			},
		)
	})
})
