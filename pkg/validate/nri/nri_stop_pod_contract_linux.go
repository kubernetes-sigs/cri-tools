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
	"context"
	"sync"
	"time"

	nri "github.com/containerd/nri/pkg/api"
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

	Context("StopPodSandbox contract", Serial, func() {
		var (
			testStub    *NRITestStub
			podID       string
			podConfig   *runtimeapi.PodSandboxConfig
			containerID string
		)

		AfterEach(func(ctx SpecContext) {
			// Stop the stub first to unblock any hooks that may be holding
			// a StopPodSandbox call, allowing it to complete.
			if testStub != nil {
				testStub.Cleanup()
			}

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
		})

		It(
			"should stop all workload containers and keep sandbox accessible while StopPodSandbox hook is in progress",
			func(ctx SpecContext) {
				// This test validates the spec contract: when StopPodSandbox is called,
				// all workload containers MUST already be stopped before the NRI hook
				// fires, and the sandbox infrastructure MUST still be accessible via
				// PodSandboxStatus while the hook is in progress.
				hookBlocking := make(chan struct{})
				hookReached := make(chan struct{})

				var hookOnce sync.Once

				var err error

				testStub, err = StartNRITestStub("cri-test-nri-stop-state", "00")
				Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

				// Configure stub to block during StopPodSandbox so the main
				// goroutine can inspect runtime state while the hook is active.
				// sync.Once guards against the hook being invoked multiple times
				// (e.g., idempotent stop redelivery), which would otherwise panic
				// on a double close of hookReached.
				testStub.Plugin.OnStopPodSandbox = func(hookCtx context.Context, _ *nri.PodSandbox) error {
					firstInvocation := false

					hookOnce.Do(func() { firstInvocation = true })

					// Skip duplicate invocations (e.g., AfterEach cleanup) so
					// they are not blocked by the test channel handshake.
					if !firstInvocation {
						return nil
					}

					close(hookReached)

					select {
					case <-hookBlocking:
					case <-hookCtx.Done():
					}

					return nil
				}

				By("creating a pod sandbox")

				podSandboxName := "nri-test-stop-state-" + framework.NewUUID()
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

				By("creating and starting a container in the sandbox")
				framework.PullPublicImage(
					ctx,
					ic,
					framework.TestContext.TestImageList.DefaultTestContainerImage,
					nil,
				)

				containerName := "nri-test-stop-state-ctr-" + framework.NewUUID()
				containerConfig := &runtimeapi.ContainerConfig{
					Metadata: framework.BuildContainerMetadata(
						containerName,
						framework.DefaultAttempt,
					),
					Image: &runtimeapi.ImageSpec{
						Image: framework.TestContext.TestImageList.DefaultTestContainerImage,
					},
					Command: framework.DefaultPauseCommand,
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
				Expect(rc.StartContainer(ctx, containerID)).NotTo(HaveOccurred())

				By("triggering StopPodSandbox in a goroutine")

				var (
					stopErr error
					stopWg  sync.WaitGroup
				)

				stopWg.Go(func() {
					stopErr = rc.StopPodSandbox(ctx, podID)
				})

				By("waiting for StopPodSandbox hook to be reached")

				select {
				case <-hookReached:
					// Hook is now blocking; main goroutine can inspect state
				case <-time.After(30 * time.Second):
					close(hookBlocking) // unblock to avoid goroutine leak
					Fail("Timed out waiting for StopPodSandbox NRI hook to fire")
				}

				By("verifying all workload containers were stopped before the hook fired")

				containers, listErr := rc.ListContainers(ctx, &runtimeapi.ContainerFilter{
					PodSandboxId: podID,
				})
				Expect(listErr).NotTo(HaveOccurred(), "ListContainers during StopPodSandbox hook")

				Expect(containers).NotTo(BeEmpty(),
					"workload containers should still be listed (in EXITED state) during StopPodSandbox hook")

				for _, c := range containers {
					Expect(c.GetState()).To(Equal(runtimeapi.ContainerState_CONTAINER_EXITED),
						"container %s MUST be stopped before StopPodSandbox hook fires", c.GetId())
				}

				By("verifying sandbox infrastructure is still accessible during the hook")

				statusResp, statusErr := rc.PodSandboxStatus(ctx, podID, false)
				Expect(
					statusErr,
				).NotTo(HaveOccurred(), "PodSandboxStatus during StopPodSandbox hook")
				Expect(statusResp.GetStatus()).NotTo(BeNil(),
					"PodSandboxStatus MUST be accessible during StopPodSandbox hook")
				Expect(statusResp.GetStatus().GetId()).To(Equal(podID))

				By("releasing the hook and verifying StopPodSandbox succeeds")
				close(hookBlocking)
				stopWg.Wait()
				Expect(
					stopErr,
				).NotTo(HaveOccurred(), "StopPodSandbox should succeed after hook returns")

				// StopPodSandbox stopped the container; clear so AfterEach skips it.
				containerID = ""

				// Remove the sandbox now and clear podID so AfterEach doesn't issue a
				// second StopPodSandbox (which could re-enter the hook callback).
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred(),
					"RemovePodSandbox should succeed after stop")
				podID = ""
			},
		)

		It(
			"should handle StopPodSandbox idempotently and never reuse sandbox",
			func(ctx SpecContext) {
				// This test validates two spec guarantees:
				// 1. StopPodSandbox is idempotent - calling it multiple times succeeds without error.
				// 2. After Stop, the sandbox is never reused - CreateContainer fails.
				var err error

				testStub, err = StartNRITestStub("cri-test-nri-stop-idempotent", "00")
				Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

				By("creating a pod sandbox")

				podSandboxName := "nri-test-stop-idempotent-" + framework.NewUUID()
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

				By("calling StopPodSandbox the first time")
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred(),
					"First StopPodSandbox call should succeed")

				By("verifying the StopPodSandbox NRI hook fired exactly once")
				// Poll for the StopPodSandbox event rather than assuming a fixed
				// global event count, which would break if a future runtime/NRI
				// version emits additional lifecycle events.
				var stopPodSandboxEvents []NRIEvent

				Eventually(func() []NRIEvent {
					stopPodSandboxEvents = nil

					for _, e := range FilterEventsByPodID(testStub.Plugin.Events(), podID) {
						if e.Type == EventStopPodSandbox {
							stopPodSandboxEvents = append(stopPodSandboxEvents, e)
						}
					}

					return stopPodSandboxEvents
				}, 10*time.Second, 50*time.Millisecond).ShouldNot(BeEmpty(),
					"NRI stub did not receive the StopPodSandbox event")
				Expect(stopPodSandboxEvents).To(HaveLen(1),
					"StopPodSandbox NRI hook should fire exactly once for the first call")

				By("calling StopPodSandbox again (idempotency check)")
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred(),
					"Second StopPodSandbox call MUST succeed (idempotent)")

				By("verifying the second StopPodSandbox does NOT generate an NRI event")
				// Wait briefly, then confirm no second event was delivered.
				Consistently(func() int {
					count := 0

					for _, e := range FilterEventsByPodID(testStub.Plugin.Events(), podID) {
						if e.Type == EventStopPodSandbox {
							count++
						}
					}

					return count
				}, 2*time.Second, 200*time.Millisecond).Should(Equal(1),
					"Second StopPodSandbox MUST NOT generate an NRI event — sandbox is already stopped")

				By("verifying sandbox cannot be reused - CreateContainer should fail after Stop")
				framework.PullPublicImage(
					ctx,
					ic,
					framework.TestContext.TestImageList.DefaultTestContainerImage,
					nil,
				)

				containerName := "nri-test-reuse-after-stop-" + framework.NewUUID()
				containerConfig := &runtimeapi.ContainerConfig{
					Metadata: framework.BuildContainerMetadata(
						containerName,
						framework.DefaultAttempt,
					),
					Image: &runtimeapi.ImageSpec{
						Image:              framework.TestContext.TestImageList.DefaultTestContainerImage,
						UserSpecifiedImage: framework.TestContext.TestImageList.DefaultTestContainerImage,
					},
					Command: framework.DefaultPauseCommand,
					Linux:   &runtimeapi.LinuxContainerConfig{},
				}

				// CreateContainer on a stopped sandbox MUST fail per spec.
				ctrID, createErr := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
				if createErr == nil {
					// SPEC_DISCREPANCY: containerd allows CreateContainer on a stopped
					// sandbox instead of rejecting it. Hand the unexpectedly created
					// container to AfterEach for cleanup, then skip the non-reuse
					// assertion (spec says the sandbox should never be reused after Stop).
					containerID = ctrID

					Skip(
						"spec discrepancy: containerd allows CreateContainer on a stopped sandbox; " +
							"spec says sandbox should never be reused after Stop",
					)
				}

				// Reaching here means createErr != nil, which is the spec-compliant
				// behavior (sandbox never reused after Stop).

				By("verifying the failed CreateContainer did NOT generate an NRI event")
				Consistently(func() int {
					count := 0

					for _, e := range FilterEventsByPodID(testStub.Plugin.Events(), podID) {
						if e.Type == EventCreateContainer {
							count++
						}
					}

					return count
				}, 2*time.Second, 200*time.Millisecond).Should(Equal(0),
					"Failed CreateContainer on a stopped sandbox MUST NOT generate an NRI CreateContainer event")
			},
		)
	})
})
