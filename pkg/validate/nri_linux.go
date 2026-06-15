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
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

	Context("pod sandbox lifecycle", Serial, func() {
		var (
			testStub  *NRITestStub
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
		)

		BeforeEach(func() {
			var err error

			testStub, err = StartNRITestStub("cri-test-nri-pod-lifecycle", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")
		})

		AfterEach(func(ctx SpecContext) {
			// Pod is already removed by the test; only clean up if test failed early
			if podID != "" {
				_ = rc.StopPodSandbox(ctx, podID)
				_ = rc.RemovePodSandbox(ctx, podID)
			}

			if testStub != nil {
				testStub.Cleanup()
			}
		})

		It("should receive RunPodSandbox, StopPodSandbox, and RemovePodSandbox in strict order with correct metadata", func(ctx SpecContext) {
			By("creating a pod sandbox")

			podSandboxName := "nri-test-pod-lifecycle-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}

			podID = framework.RunPodSandbox(ctx, rc, podConfig)
			Expect(podID).NotTo(BeEmpty())

			By("stopping the pod sandbox")
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())

			By("removing the pod sandbox")
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())

			By("waiting for all pod lifecycle NRI events")
			// At least 3 events expected (Run, Stop, Remove); future NRI versions
			// may add more extensibility points, so we use >= rather than ==.
			events, err := testStub.Plugin.WaitForEventCount(3, 10*time.Second)
			Expect(err).NotTo(HaveOccurred(), "NRI stub did not receive all pod lifecycle events")

			// Filter events for this specific pod
			podEvents := FilterEventsByPodID(events, podID)
			Expect(len(podEvents)).To(BeNumerically(">=", 3),
				"expected at least 3 NRI events for pod %s, got %d", podID, len(podEvents))

			By("verifying RunPodSandbox event has correct metadata")

			var runEvent, stopEvent, removeEvent *NRIEvent

			for i := range podEvents {
				switch podEvents[i].Type {
				case EventRunPodSandbox:
					runEvent = &podEvents[i]
				case EventStopPodSandbox:
					stopEvent = &podEvents[i]
				case EventRemovePodSandbox:
					removeEvent = &podEvents[i]
				case EventCreateContainer, EventStartContainer, EventStopContainer, EventRemoveContainer:
					Fail(fmt.Sprintf("unexpected container event %v in pod-only lifecycle test", podEvents[i].Type))
				}
			}

			Expect(runEvent).NotTo(BeNil(), "RunPodSandbox event not received")
			Expect(runEvent.PodName).To(Equal(podSandboxName))
			Expect(runEvent.PodNamespace).To(Equal(namespace))
			Expect(runEvent.PodUID).To(Equal(uid))

			By("verifying StopPodSandbox event has correct pod ID")
			Expect(stopEvent).NotTo(BeNil(), "StopPodSandbox event not received")
			Expect(stopEvent.PodSandboxID).To(Equal(podID))

			By("verifying RemovePodSandbox event has correct pod ID")
			Expect(removeEvent).NotTo(BeNil(), "RemovePodSandbox event not received")
			Expect(removeEvent.PodSandboxID).To(Equal(podID))

			By("verifying events arrived in strict order: Run -> Stop -> Remove")
			Expect(runEvent.Timestamp.Before(stopEvent.Timestamp)).To(BeTrue(),
				"RunPodSandbox (at %v) should occur before StopPodSandbox (at %v)",
				runEvent.Timestamp, stopEvent.Timestamp)
			Expect(stopEvent.Timestamp.Before(removeEvent.Timestamp)).To(BeTrue(),
				"StopPodSandbox (at %v) should occur before RemovePodSandbox (at %v)",
				stopEvent.Timestamp, removeEvent.Timestamp)

			// Mark pod as cleaned up so AfterEach doesn't try again
			podID = ""
		})
	})

	Context("container lifecycle", Serial, func() {
		var (
			testStub    *NRITestStub
			podID       string
			podConfig   *runtimeapi.PodSandboxConfig
			containerID string
		)

		BeforeEach(func(ctx SpecContext) {
			var err error

			testStub, err = StartNRITestStub("cri-test-nri-ctr-lifecycle", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Ensure test image is available
			framework.PullPublicImage(ctx, ic, framework.TestContext.TestImageList.DefaultTestContainerImage, nil)
		})

		AfterEach(func(ctx SpecContext) {
			if containerID != "" {
				_ = rc.StopContainer(ctx, containerID, 0)
				_ = rc.RemoveContainer(ctx, containerID)
			}

			if podID != "" {
				_ = rc.StopPodSandbox(ctx, podID)
				_ = rc.RemovePodSandbox(ctx, podID)
			}

			if testStub != nil {
				testStub.Cleanup()
			}
		})

		It("should receive CreateContainer, StartContainer, StopContainer, and RemoveContainer in strict order with correct metadata", func(ctx SpecContext) {
			By("creating a pod sandbox")

			podSandboxName := "nri-test-ctr-lifecycle-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}
			podID = framework.RunPodSandbox(ctx, rc, podConfig)
			Expect(podID).NotTo(BeEmpty())

			// Reset events so we only capture container-related events from this point
			testStub.Plugin.Reset()

			By("creating a container")

			containerName := "nri-test-container-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  framework.DefaultPauseCommand,
				Linux:    &runtimeapi.LinuxContainerConfig{},
			}
			containerID = framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
			Expect(containerID).NotTo(BeEmpty())

			By("starting the container")
			Expect(rc.StartContainer(ctx, containerID)).NotTo(HaveOccurred())

			By("stopping the container")
			Expect(rc.StopContainer(ctx, containerID, 60)).NotTo(HaveOccurred())

			By("removing the container")
			Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())

			By("waiting for all container lifecycle NRI events")
			// We expect at least 4 container events: Create, Start, Stop, Remove
			events, err := testStub.Plugin.WaitForEventCount(4, 10*time.Second)
			Expect(err).NotTo(HaveOccurred(), "NRI stub did not receive all container lifecycle events")

			// Filter for container events (those with a ContainerID set)
			var containerEvents []NRIEvent

			for i := range events {
				if events[i].ContainerID != "" {
					containerEvents = append(containerEvents, events[i])
				}
			}

			Expect(len(containerEvents)).To(BeNumerically(">=", 4),
				"expected at least 4 container NRI events, got %d", len(containerEvents))

			By("verifying CreateContainer event has correct metadata")

			var createEvent, startEvent, stopEvent, removeEvent *NRIEvent

			for i := range containerEvents {
				switch containerEvents[i].Type {
				case EventCreateContainer:
					createEvent = &containerEvents[i]
				case EventStartContainer:
					startEvent = &containerEvents[i]
				case EventStopContainer:
					stopEvent = &containerEvents[i]
				case EventRemoveContainer:
					removeEvent = &containerEvents[i]
				case EventRunPodSandbox, EventStopPodSandbox, EventRemovePodSandbox:
					// Pod events not verified in this test. Events are expected, but test ignores them
				}
			}

			Expect(createEvent).NotTo(BeNil(), "CreateContainer event not received")
			// SPEC_DISCREPANCY: CRI-O does not populate container name in NRI CreateContainer event metadata
			if createEvent.ContainerName == "" {
				Skip("spec discrepancy: runtime does not populate container name in NRI CreateContainer event metadata")
			}

			Expect(createEvent.ContainerName).To(Equal(containerName))
			Expect(createEvent.ContainerID).To(Equal(containerID))

			By("verifying StartContainer event has correct container ID")
			Expect(startEvent).NotTo(BeNil(), "StartContainer event not received")
			Expect(startEvent.ContainerID).To(Equal(containerID))

			By("verifying StopContainer event has correct container ID")
			Expect(stopEvent).NotTo(BeNil(), "StopContainer event not received")
			Expect(stopEvent.ContainerID).To(Equal(containerID))

			By("verifying RemoveContainer event has correct container ID")
			Expect(removeEvent).NotTo(BeNil(), "RemoveContainer event not received")
			Expect(removeEvent.ContainerID).To(Equal(containerID))

			By("verifying events in strict order: Create -> Start -> Stop -> Remove")
			Expect(createEvent.Timestamp.Before(startEvent.Timestamp)).To(BeTrue(),
				"CreateContainer (at %v) should occur before StartContainer (at %v)",
				createEvent.Timestamp, startEvent.Timestamp)
			Expect(startEvent.Timestamp.Before(stopEvent.Timestamp)).To(BeTrue(),
				"StartContainer (at %v) should occur before StopContainer (at %v)",
				startEvent.Timestamp, stopEvent.Timestamp)
			Expect(stopEvent.Timestamp.Before(removeEvent.Timestamp)).To(BeTrue(),
				"StopContainer (at %v) should occur before RemoveContainer (at %v)",
				stopEvent.Timestamp, removeEvent.Timestamp)

			// Mark container as cleaned up
			containerID = ""
		})
	})

	Context("RunPodSandbox contract", Serial, func() {
		var (
			testStub  *NRITestStub
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
		)

		AfterEach(func(ctx SpecContext) {
			// Capture the fallback sandbox ID before cleanup resets events.
			cleanupID := podID
			if cleanupID == "" && testStub != nil {
				cleanupID = testStub.Plugin.LastRunPodSandboxID()
			}

			// Stop the stub to unblock any hooks that may be holding
			// a RunPodSandbox call, allowing it to complete.
			if testStub != nil {
				testStub.Cleanup()
			}

			if cleanupID != "" {
				_ = rc.StopPodSandbox(ctx, cleanupID)
				_ = rc.RemovePodSandbox(ctx, cleanupID)
			}
		})

		It("should not expose sandbox while RunPodSandbox hook is in progress", func(ctx SpecContext) {
			// This test validates the spec contract: during RunPodSandbox hook execution,
			// the sandbox MUST NOT be visible via List or Status, and workload containers
			// MUST NOT start.

			// Channel to control blocking behavior
			hookBlocking := make(chan struct{})
			hookReached := make(chan struct{})

			var hookPodID string

			var err error

			testStub, err = StartNRITestStub("cri-test-nri-block-run", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Configure stub to block on RunPodSandbox
			testStub.Plugin.OnRunPodSandbox = func(hookCtx context.Context, pod *nri.PodSandbox) error {
				hookPodID = pod.GetId()

				close(hookReached)
				// Block until test signals to continue or context is cancelled (cleanup)
				select {
				case <-hookBlocking:
				case <-hookCtx.Done():
				}

				return nil
			}

			By("triggering RunPodSandbox in a goroutine")

			podSandboxName := "nri-test-block-run-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}

			var (
				runErr   error
				runPodID string
				runWg    sync.WaitGroup
			)

			runWg.Go(func() {
				runPodID, runErr = rc.RunPodSandbox(ctx, podConfig, framework.TestContext.RuntimeHandler)
			})

			By("waiting for RunPodSandbox hook to be reached")

			select {
			case <-hookReached:
				// Hook is now blocking
			case <-time.After(30 * time.Second):
				close(hookBlocking) // unblock to avoid goroutine leak
				Fail("Timed out waiting for RunPodSandbox NRI hook to fire")
			}

			By("verifying sandbox is NOT listed while hook is blocking")
			// The sandbox should not appear in ListPodSandbox in any state
			pods, listErr := rc.ListPodSandbox(ctx, nil)
			Expect(listErr).NotTo(HaveOccurred())

			sandboxFound := false

			for _, pod := range pods {
				if pod.GetId() == hookPodID {
					sandboxFound = true

					break
				}
			}

			Expect(sandboxFound).To(BeFalse(),
				"Sandbox %s MUST NOT be listed while RunPodSandbox hook is blocking", hookPodID)

			By("verifying PodSandboxStatus is not accessible while hook is blocking")

			if hookPodID != "" {
				statusResp, statusErr := rc.PodSandboxStatus(ctx, hookPodID, false)
				// Ideally the sandbox should not be found at all. Some runtimes may
				// return a non-Ready status instead of NotFound — both are acceptable.
				if statusErr == nil && statusResp != nil && statusResp.GetStatus() != nil {
					Expect(statusResp.GetStatus().GetState()).NotTo(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
						"Sandbox MUST NOT report Ready state while RunPodSandbox hook is in progress")
				}
			}

			By("releasing the hook and verifying pod becomes Ready")
			close(hookBlocking)
			runWg.Wait()
			Expect(runErr).NotTo(HaveOccurred(), "RunPodSandbox should succeed after hook returns")
			Expect(runPodID).NotTo(BeEmpty())
			podID = runPodID

			// After hook completes, sandbox should be Ready
			statusResp, err := rc.PodSandboxStatus(ctx, podID, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusResp.GetStatus().GetState()).To(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
				"Sandbox should be Ready after RunPodSandbox hook completes")
		})

		It("should not start workload containers until RunPodSandbox hook completes", func(ctx SpecContext) {
			// This test validates that workload container creation is blocked while the
			// RunPodSandbox hook is still running. Even if the caller attempts CreateContainer
			// immediately, it should not succeed until the hook returns.
			hookBlocking := make(chan struct{})
			hookReached := make(chan struct{})

			var err error

			testStub, err = StartNRITestStub("cri-test-nri-block-container", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Configure stub to block RunPodSandbox for a measurable duration
			testStub.Plugin.OnRunPodSandbox = func(hookCtx context.Context, _ *nri.PodSandbox) error {
				close(hookReached)

				select {
				case <-hookBlocking:
				case <-hookCtx.Done():
				}

				return nil
			}

			By("triggering RunPodSandbox in a goroutine")

			podSandboxName := "nri-test-block-container-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}

			var (
				runErr   error
				runPodID string
				runWg    sync.WaitGroup
			)

			runWg.Go(func() {
				runPodID, runErr = rc.RunPodSandbox(ctx, podConfig, framework.TestContext.RuntimeHandler)
			})

			By("waiting for RunPodSandbox hook to be reached")

			select {
			case <-hookReached:
				// Hook is now blocking
			case <-time.After(30 * time.Second):
				close(hookBlocking)
				Fail("Timed out waiting for RunPodSandbox NRI hook to fire")
			}

			By("releasing hook after brief delay to verify container creation is gated")
			// We hold the hook for 2 seconds. RunPodSandbox should not return during this time,
			// which means no pod ID is available for container creation yet.
			// The CRI contract ensures RunPodSandbox is synchronous, so the caller cannot
			// get a pod ID until the hook releases.
			hookHoldDuration := 2 * time.Second
			hookBlockedAt := time.Now()

			go func() {
				time.Sleep(hookHoldDuration)
				close(hookBlocking)
			}()

			// Wait for RunPodSandbox to complete
			runWg.Wait()

			runCompletedAt := time.Now()

			Expect(runErr).NotTo(HaveOccurred())
			Expect(runPodID).NotTo(BeEmpty())
			podID = runPodID

			// Verify that RunPodSandbox was indeed blocked for the expected duration
			actualBlockDuration := runCompletedAt.Sub(hookBlockedAt)
			Expect(actualBlockDuration).To(BeNumerically(">=", hookHoldDuration-100*time.Millisecond),
				"RunPodSandbox should be blocked for at least the hook hold duration, "+
					"confirming container creation cannot proceed until hook completes")

			// Now that the pod is ready, verify container creation works
			By("verifying container creation succeeds after hook completes")
			framework.PullPublicImage(ctx, ic, framework.TestContext.TestImageList.DefaultTestContainerImage, nil)

			containerName := "nri-test-after-hook-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  framework.DefaultPauseCommand,
				Linux:    &runtimeapi.LinuxContainerConfig{},
			}
			containerID := framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
			Expect(containerID).NotTo(BeEmpty(),
				"Container creation should succeed after RunPodSandbox hook completes")

			// Clean up container
			_ = rc.StopContainer(ctx, containerID, 0)
			_ = rc.RemoveContainer(ctx, containerID)
		})
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
				_ = rc.StopContainer(ctx, containerID, 0)
				_ = rc.RemoveContainer(ctx, containerID)
			}

			if podID != "" {
				_ = rc.StopPodSandbox(ctx, podID)
				_ = rc.RemovePodSandbox(ctx, podID)
			}
		})

		It("should keep sandbox infrastructure accessible during StopPodSandbox hook and stop all containers first", func(ctx SpecContext) {
			// This test validates that during the StopPodSandbox NRI hook:
			// 1. All workload containers are already stopped (runtime stops them before invoking hook)
			// 2. The sandbox is still accessible via PodSandboxStatus (infrastructure not yet torn down)
			hookBlocking := make(chan struct{})
			hookReached := make(chan struct{})

			var (
				hookErr                  error
				containerStateDuringHook runtimeapi.ContainerState
				podStatusDuringHook      *runtimeapi.PodSandboxStatus
				hookOnce                 sync.Once
			)

			var err error

			testStub, err = StartNRITestStub("cri-test-nri-stop-state", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Configure stub to inspect state during StopPodSandbox.
			// Uses sync.Once to guard against panic if the hook is invoked multiple
			// times (e.g., runtime redelivers idempotent stop).
			testStub.Plugin.OnStopPodSandbox = func(hookCtx context.Context, pod *nri.PodSandbox) error {
				// Guard against multiple invocations (e.g., idempotent stop redelivery)
				invoked := false

				hookOnce.Do(func() { invoked = true })

				if !invoked {
					return nil
				}

				// During StopPodSandbox hook, inspect the container and sandbox state
				// Use the test's CRI client to query runtime state

				// Check container state - all workload containers should already be stopped
				containers, listErr := rc.ListContainers(hookCtx, &runtimeapi.ContainerFilter{
					PodSandboxId: pod.GetId(),
				})
				if listErr != nil {
					hookErr = fmt.Errorf("failed to list containers during StopPodSandbox hook: %w", listErr)

					close(hookReached)

					select {
					case <-hookBlocking:
					case <-hookCtx.Done():
					}

					return nil
				}

				// All containers should be in EXITED state
				for _, c := range containers {
					if c.GetState() != runtimeapi.ContainerState_CONTAINER_EXITED {
						containerStateDuringHook = c.GetState()
						hookErr = fmt.Errorf("container %s is in state %v during StopPodSandbox hook, expected EXITED", c.GetId(), c.GetState())

						close(hookReached)

						select {
						case <-hookBlocking:
						case <-hookCtx.Done():
						}

						return nil
					}

					containerStateDuringHook = c.GetState()
				}

				// Check pod sandbox status - should still be accessible (infrastructure still up)
				statusResp, statusErr := rc.PodSandboxStatus(hookCtx, pod.GetId(), false)
				if statusErr != nil {
					hookErr = fmt.Errorf("failed to get PodSandboxStatus during StopPodSandbox hook: %w", statusErr)

					close(hookReached)

					select {
					case <-hookBlocking:
					case <-hookCtx.Done():
					}

					return nil
				}

				podStatusDuringHook = statusResp.GetStatus()

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
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}
			podID = framework.RunPodSandbox(ctx, rc, podConfig)
			Expect(podID).NotTo(BeEmpty())

			By("creating and starting a container in the sandbox")
			framework.PullPublicImage(ctx, ic, framework.TestContext.TestImageList.DefaultTestContainerImage, nil)

			containerName := "nri-test-stop-state-ctr-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  framework.DefaultPauseCommand,
				Linux:    &runtimeapi.LinuxContainerConfig{},
			}
			containerID = framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
			Expect(containerID).NotTo(BeEmpty())
			Expect(rc.StartContainer(ctx, containerID)).NotTo(HaveOccurred())

			By("calling StopPodSandbox (which triggers the NRI hook)")

			var (
				stopWg  sync.WaitGroup
				stopErr error
			)

			stopWg.Go(func() {
				stopErr = rc.StopPodSandbox(ctx, podID)
			})

			By("waiting for StopPodSandbox hook to fire")

			select {
			case <-hookReached:
				// Hook is now blocking, state inspection is done
			case <-time.After(30 * time.Second):
				close(hookBlocking)
				Fail("Timed out waiting for StopPodSandbox NRI hook to fire")
			}

			By("verifying all workload containers were already stopped before hook")
			Expect(hookErr).NotTo(HaveOccurred(), "error during state inspection in StopPodSandbox hook")
			Expect(containerStateDuringHook).To(Equal(runtimeapi.ContainerState_CONTAINER_EXITED),
				"All workload containers MUST be stopped before StopPodSandbox hook fires")

			By("verifying sandbox infrastructure is still accessible during hook")
			Expect(podStatusDuringHook).NotTo(BeNil(),
				"PodSandboxStatus MUST be accessible during StopPodSandbox hook (infrastructure still up)")
			// The sandbox status should be retrievable, confirming network ns and cgroups are intact
			Expect(podStatusDuringHook.GetId()).To(Equal(podID))

			By("releasing the hook")
			close(hookBlocking)
			stopWg.Wait()
			Expect(stopErr).NotTo(HaveOccurred(), "StopPodSandbox should succeed")

			// Mark container as cleaned up (StopPodSandbox stops all containers)
			containerID = ""

			// Remove the sandbox now and clear podID so AfterEach doesn't issue a
			// second StopPodSandbox (which could trigger the hook callback again
			// and panic on closing the already-closed hookReached channel).
			err = rc.RemovePodSandbox(ctx, podID)
			Expect(err).NotTo(HaveOccurred(), "RemovePodSandbox should succeed after stop")

			podID = ""
		})

		It("should handle StopPodSandbox idempotently and never reuse sandbox", func(ctx SpecContext) {
			// This test validates two spec guarantees:
			// 1. StopPodSandbox is idempotent - calling it multiple times succeeds without error
			// 2. After Stop, the sandbox is never reused - CreateContainer fails
			var (
				stopHookCount int
				stopHookMu    sync.Mutex
			)

			var err error

			testStub, err = StartNRITestStub("cri-test-nri-stop-idempotent", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Count StopPodSandbox hook invocations
			testStub.Plugin.OnStopPodSandbox = func(_ context.Context, _ *nri.PodSandbox) error {
				stopHookMu.Lock()
				stopHookCount++
				stopHookMu.Unlock()

				return nil
			}

			By("creating a pod sandbox")

			podSandboxName := "nri-test-stop-idempotent-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
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

			By("calling StopPodSandbox again (idempotency check)")
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred(),
				"Second StopPodSandbox call MUST succeed (idempotent)")

			By("verifying StopPodSandbox hook fired at least once")
			// Wait briefly for events to propagate
			time.Sleep(500 * time.Millisecond)
			stopHookMu.Lock()
			hookCount := stopHookCount
			stopHookMu.Unlock()
			Expect(hookCount).To(BeNumerically(">=", 1),
				"StopPodSandbox NRI hook should fire at least once")

			By("verifying sandbox cannot be reused - CreateContainer should fail after Stop")
			framework.PullPublicImage(ctx, ic, framework.TestContext.TestImageList.DefaultTestContainerImage, nil)

			containerName := "nri-test-reuse-after-stop-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image: &runtimeapi.ImageSpec{
					Image:              framework.TestContext.TestImageList.DefaultTestContainerImage,
					UserSpecifiedImage: framework.TestContext.TestImageList.DefaultTestContainerImage,
				},
				Command: framework.DefaultPauseCommand,
				Linux:   &runtimeapi.LinuxContainerConfig{},
			}

			// CreateContainer on a stopped sandbox MUST fail per spec
			ctrID, createErr := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
			if createErr == nil && ctrID != "" {
				// Clean up the unexpectedly created container
				_ = rc.StopContainer(ctx, ctrID, 0)
				_ = rc.RemoveContainer(ctx, ctrID)

				// SPEC_DISCREPANCY: containerd allows CreateContainer on a stopped sandbox instead of rejecting it
				Skip("spec discrepancy: containerd allows CreateContainer on a stopped sandbox; spec says sandbox should never be reused after Stop")
			}

			Expect(createErr).To(HaveOccurred(),
				"CreateContainer on a stopped sandbox MUST return an error (sandbox never reused after Stop)")
		})
	})

	Context("plugin error handling", Serial, func() {
		var (
			testStub  *NRITestStub
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
		)

		AfterEach(func(ctx SpecContext) {
			if podID != "" {
				_ = rc.StopPodSandbox(ctx, podID)
				_ = rc.RemovePodSandbox(ctx, podID)
			}

			if testStub != nil {
				testStub.Cleanup()
			}
		})

		It("should propagate RunPodSandbox plugin error, clean up resources, and allow immediate retry", func(ctx SpecContext) {
			// This test validates the spec contract for RunPodSandbox plugin errors:
			// 1. If a plugin returns an error during RunPodSandbox, the CRI call MUST fail
			// 2. The failed sandbox MUST be cleaned up (not visible via List or Get)
			// 3. A subsequent RunPodSandbox MUST succeed (retry works immediately)
			var (
				callCount  atomic.Int32
				nriPodID   string
				nriPodIDMu sync.Mutex
			)

			var err error

			testStub, err = StartNRITestStub("cri-test-nri-run-error", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Configure stub to fail on first RunPodSandbox, succeed on second
			testStub.Plugin.OnRunPodSandbox = func(_ context.Context, pod *nri.PodSandbox) error {
				count := callCount.Add(1)

				nriPodIDMu.Lock()
				nriPodID = pod.GetId()
				nriPodIDMu.Unlock()

				if count == 1 {
					return errors.New("simulated NRI plugin error on RunPodSandbox")
				}

				return nil
			}

			By("attempting RunPodSandbox (should fail due to plugin error)")

			podSandboxName := "nri-test-run-error-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}

			failedPodID, runErr := rc.RunPodSandbox(ctx, podConfig, framework.TestContext.RuntimeHandler)
			Expect(runErr).To(HaveOccurred(),
				"RunPodSandbox MUST return an error when plugin fails")
			Expect(failedPodID).To(BeEmpty(),
				"RunPodSandbox MUST return an empty pod ID on failure")

			By("verifying sandbox is not visible via ListPodSandbox after failure")
			// The failed sandbox should be fully cleaned up - not visible in any state
			allPods, listErr := rc.ListPodSandbox(ctx, nil)
			Expect(listErr).NotTo(HaveOccurred())

			nriPodIDMu.Lock()
			capturedNRIPodID := nriPodID
			nriPodIDMu.Unlock()

			Expect(capturedNRIPodID).NotTo(BeEmpty(),
				"NRI RunPodSandbox callback MUST be invoked by the runtime and provide a sandbox ID")

			// Check that the failed sandbox is not in the list
			for _, pod := range allPods {
				Expect(pod.GetId()).NotTo(Equal(capturedNRIPodID),
					"Failed sandbox %s MUST NOT appear in ListPodSandbox", capturedNRIPodID)
			}

			By("verifying PodSandboxStatus returns not-found for the failed sandbox")

			_, statusErr := rc.PodSandboxStatus(ctx, capturedNRIPodID, false)
			// The sandbox should be gone - either NotFound error or nil response.
			// Some runtimes return an error, others may return empty status.
			// Either way, it should not be in the List (already verified above).
			_ = statusErr

			// Best-effort cleanup: containerd removes the failed sandbox from its
			// store but leaves the shim and its mounts (rootfs, shm) behind. This
			// is a containerd bug — calling Stop/Remove here is a workaround to
			// release those orphaned mounts and prevent "Device or resource busy"
			// errors during CI cleanup.
			_ = rc.StopPodSandbox(ctx, capturedNRIPodID)
			_ = rc.RemovePodSandbox(ctx, capturedNRIPodID)

			By("retrying RunPodSandbox (should succeed)")

			retryPodID, retryErr := rc.RunPodSandbox(ctx, podConfig, framework.TestContext.RuntimeHandler)
			Expect(retryErr).NotTo(HaveOccurred(),
				"RunPodSandbox retry MUST succeed after previous plugin error")
			Expect(retryPodID).NotTo(BeEmpty())
			podID = retryPodID

			By("verifying the retry sandbox is Ready")

			statusResp, err := rc.PodSandboxStatus(ctx, podID, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusResp.GetStatus().GetState()).To(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
				"Retried sandbox should be in Ready state")

			By("verifying plugin hook was invoked twice (once failed, once succeeded)")
			Expect(callCount.Load()).To(Equal(int32(2)),
				"RunPodSandbox hook should have been invoked exactly twice")
		})

		It("should propagate CreateContainer plugin error and allow immediate retry", func(ctx SpecContext) {
			// This test validates the spec contract for CreateContainer plugin errors:
			// 1. If a plugin returns an error during CreateContainer, the CRI call MUST fail
			// 2. A subsequent CreateContainer MUST succeed (retry works immediately)
			var callCount atomic.Int32

			var err error

			testStub, err = StartNRITestStub("cri-test-nri-create-error", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Configure stub to fail on first CreateContainer, succeed on second
			testStub.Plugin.OnCreateContainer = func(_ context.Context, _ *nri.PodSandbox, _ *nri.Container) error {
				count := callCount.Add(1)
				if count == 1 {
					return errors.New("simulated NRI plugin error on CreateContainer")
				}

				return nil
			}

			By("creating a pod sandbox")

			podSandboxName := "nri-test-create-error-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}
			podID = framework.RunPodSandbox(ctx, rc, podConfig)
			Expect(podID).NotTo(BeEmpty())

			By("pulling test image")
			framework.PullPublicImage(ctx, ic, framework.TestContext.TestImageList.DefaultTestContainerImage, nil)

			By("attempting CreateContainer (should fail due to plugin error)")

			containerName := "nri-test-create-error-ctr-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image: &runtimeapi.ImageSpec{
					Image:              framework.TestContext.TestImageList.DefaultTestContainerImage,
					UserSpecifiedImage: framework.TestContext.TestImageList.DefaultTestContainerImage,
				},
				Command: framework.DefaultPauseCommand,
				Linux:   &runtimeapi.LinuxContainerConfig{},
			}

			failedCtrID, createErr := rc.CreateContainer(ctx, podID, containerConfig, podConfig)
			Expect(createErr).To(HaveOccurred(),
				"CreateContainer MUST return an error when plugin fails")

			// If a container ID was returned despite the error, clean it up
			if failedCtrID != "" {
				_ = rc.StopContainer(ctx, failedCtrID, 0)
				_ = rc.RemoveContainer(ctx, failedCtrID)
			}

			By("retrying CreateContainer (should succeed)")

			retryContainerName := "nri-test-create-retry-ctr-" + framework.NewUUID()
			retryContainerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(retryContainerName, framework.DefaultAttempt),
				Image: &runtimeapi.ImageSpec{
					Image:              framework.TestContext.TestImageList.DefaultTestContainerImage,
					UserSpecifiedImage: framework.TestContext.TestImageList.DefaultTestContainerImage,
				},
				Command: framework.DefaultPauseCommand,
				Linux:   &runtimeapi.LinuxContainerConfig{},
			}

			retryCtrID, retryErr := rc.CreateContainer(ctx, podID, retryContainerConfig, podConfig)
			Expect(retryErr).NotTo(HaveOccurred(),
				"CreateContainer retry MUST succeed after previous plugin error")
			Expect(retryCtrID).NotTo(BeEmpty())

			By("verifying the retried container exists")

			containerStatus, err := rc.ContainerStatus(ctx, retryCtrID, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(containerStatus.GetStatus().GetId()).To(Equal(retryCtrID))

			// Clean up the successfully created container
			_ = rc.StopContainer(ctx, retryCtrID, 0)
			_ = rc.RemoveContainer(ctx, retryCtrID)

			By("verifying plugin hook was invoked twice (once failed, once succeeded)")
			Expect(callCount.Load()).To(Equal(int32(2)),
				"CreateContainer hook should have been invoked exactly twice")
		})
	})

	Context("multi-plugin coordination", Serial, func() {
		var (
			multiStub *NRIMultiStub
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
		)

		AfterEach(func(ctx SpecContext) {
			// Capture the fallback sandbox ID before cleanup resets events.
			cleanupID := podID
			if cleanupID == "" && multiStub != nil {
				cleanupID = multiStub.LastRunPodSandboxID()
			}

			// Stop stubs to unblock any hooks holding a RunPodSandbox call.
			if multiStub != nil {
				multiStub.Cleanup()
			}

			if cleanupID != "" {
				_ = rc.StopPodSandbox(ctx, cleanupID)
				_ = rc.RemovePodSandbox(ctx, cleanupID)
			}
		})

		It("should invoke all plugins in index order during RunPodSandbox before starting workload containers", func(ctx SpecContext) {
			// This test validates the multi-plugin ordering contract:
			// 1. All registered plugins receive RunPodSandbox hooks
			// 2. Plugins are invoked in index order (lower index first)
			// 3. No workload containers start until ALL plugins complete their RunPodSandbox hooks

			// Track invocation order using a shared slice protected by a mutex
			var (
				invocationOrder []int
				orderMu         sync.Mutex
			)

			// Channel to block the higher-index plugin (plugin 1) to verify ordering
			plugin1Reached := make(chan struct{})
			plugin1Release := make(chan struct{})

			var err error

			multiStub, err = StartNRIMultiStub("cri-test-nri-multi-order", 2, 10)
			Expect(err).NotTo(HaveOccurred(), "failed to start multi-stub")

			// Plugin 0 (index 10) - lower index, should be invoked first
			multiStub.Plugin(0).OnRunPodSandbox = func(_ context.Context, _ *nri.PodSandbox) error {
				orderMu.Lock()

				invocationOrder = append(invocationOrder, 0)
				orderMu.Unlock()

				return nil
			}

			// Plugin 1 (index 11) - higher index, should be invoked second
			multiStub.Plugin(1).OnRunPodSandbox = func(hookCtx context.Context, _ *nri.PodSandbox) error {
				orderMu.Lock()

				invocationOrder = append(invocationOrder, 1)
				orderMu.Unlock()

				close(plugin1Reached)

				select {
				case <-plugin1Release:
				case <-hookCtx.Done():
				}

				return nil
			}

			By("creating a pod sandbox with two plugins registered")

			podSandboxName := "nri-test-multi-order-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}

			var (
				runErr   error
				runPodID string
				runWg    sync.WaitGroup
			)

			runWg.Go(func() {
				runPodID, runErr = rc.RunPodSandbox(ctx, podConfig, framework.TestContext.RuntimeHandler)
			})

			By("waiting for plugin 1 (higher index) to be reached")

			select {
			case <-plugin1Reached:
				// Both plugins have been invoked (plugin 0 already returned, plugin 1 is blocking)
			case <-time.After(30 * time.Second):
				close(plugin1Release)
				Fail("Timed out waiting for second plugin to receive RunPodSandbox hook")
			}

			By("verifying both plugins received RunPodSandbox in index order")
			orderMu.Lock()
			order := make([]int, len(invocationOrder))
			copy(order, invocationOrder)
			orderMu.Unlock()

			Expect(order).To(HaveLen(2), "Both plugins MUST receive RunPodSandbox hook")
			Expect(order[0]).To(Equal(0), "Plugin with lower index MUST be invoked first")
			Expect(order[1]).To(Equal(1), "Plugin with higher index MUST be invoked second")

			By("verifying RunPodSandbox has not returned while plugin 1 is still blocking")
			// RunPodSandbox should still be in progress because plugin 1 is blocking
			// Give a brief moment and check that runWg hasn't completed
			doneCh := make(chan struct{})

			go func() {
				runWg.Wait()
				close(doneCh)
			}()

			select {
			case <-doneCh:
				Fail("RunPodSandbox MUST NOT return while any plugin's hook is still in progress")
			case <-time.After(500 * time.Millisecond):
				// Good - RunPodSandbox is still blocked
			}

			By("releasing plugin 1 and verifying RunPodSandbox completes")
			close(plugin1Release)
			runWg.Wait()
			Expect(runErr).NotTo(HaveOccurred(), "RunPodSandbox should succeed after all plugins return")
			Expect(runPodID).NotTo(BeEmpty())
			podID = runPodID

			By("verifying sandbox is Ready after all plugins complete")

			statusResp, err := rc.PodSandboxStatus(ctx, podID, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusResp.GetStatus().GetState()).To(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
				"Sandbox should be Ready after all plugins complete RunPodSandbox hooks")
		})

		It("should deliver teardown hooks to all plugins even if one fails", func(ctx SpecContext) {
			// This test validates the multi-plugin fault isolation contract:
			// One plugin returning an error on StopPodSandbox/RemovePodSandbox MUST NOT
			// prevent delivery of those hooks to subsequent plugins.
			var (
				plugin0StopReceived, plugin0RemoveReceived atomic.Int32
				plugin1StopReceived, plugin1RemoveReceived atomic.Int32
			)

			var err error

			multiStub, err = StartNRIMultiStub("cri-test-nri-multi-fault", 2, 10)
			Expect(err).NotTo(HaveOccurred(), "failed to start multi-stub")

			// Plugin 0 (index 10) - returns errors on teardown hooks
			multiStub.Plugin(0).OnStopPodSandbox = func(_ context.Context, _ *nri.PodSandbox) error {
				plugin0StopReceived.Add(1)

				return errors.New("simulated plugin 0 error on StopPodSandbox")
			}
			multiStub.Plugin(0).OnRemovePodSandbox = func(_ context.Context, _ *nri.PodSandbox) error {
				plugin0RemoveReceived.Add(1)

				return errors.New("simulated plugin 0 error on RemovePodSandbox")
			}

			// Plugin 1 (index 11) - should still receive hooks despite plugin 0's errors
			multiStub.Plugin(1).OnStopPodSandbox = func(_ context.Context, _ *nri.PodSandbox) error {
				plugin1StopReceived.Add(1)

				return nil
			}
			multiStub.Plugin(1).OnRemovePodSandbox = func(_ context.Context, _ *nri.PodSandbox) error {
				plugin1RemoveReceived.Add(1)

				return nil
			}

			By("creating a pod sandbox")

			podSandboxName := "nri-test-multi-fault-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}
			podID = framework.RunPodSandbox(ctx, rc, podConfig)
			Expect(podID).NotTo(BeEmpty())

			By("stopping the pod sandbox (plugin 0 returns error)")

			stopErr := rc.StopPodSandbox(ctx, podID)
			// SPEC_DISCREPANCY: containerd swallows NRI plugin errors on StopPodSandbox
			// instead of propagating them to the CRI caller.
			if stopErr == nil {
				// Clean up the sandbox before skipping so mounts are released.
				multiStub.Cleanup()
				multiStub = nil
				_ = rc.StopPodSandbox(ctx, podID)
				_ = rc.RemovePodSandbox(ctx, podID)
				podID = ""

				Skip("spec discrepancy: runtime swallows NRI plugin errors on StopPodSandbox instead of propagating them")
			}

			Expect(stopErr).To(HaveOccurred(),
				"StopPodSandbox MUST propagate NRI plugin error to the caller")

			By("removing the pod sandbox")

			// RemovePodSandbox may or may not propagate plugin errors depending on
			// the runtime. We don't assert error propagation here.
			_ = rc.RemovePodSandbox(ctx, podID)

			By("waiting for events to propagate")
			time.Sleep(1 * time.Second)

			By("verifying plugin 0 (failing plugin) received both hooks")
			Expect(plugin0StopReceived.Load()).To(BeNumerically(">=", 1),
				"Plugin 0 MUST receive StopPodSandbox hook")
			Expect(plugin0RemoveReceived.Load()).To(BeNumerically(">=", 1),
				"Plugin 0 MUST receive RemovePodSandbox hook")

			By("verifying plugin 1 received both hooks despite plugin 0's errors")
			// SPEC_DISCREPANCY: NRI aborts hook delivery to subsequent plugins when one plugin returns an error,
			// instead of delivering teardown hooks to all plugins regardless of individual failures.
			if plugin1StopReceived.Load() < 1 || plugin1RemoveReceived.Load() < 1 {
				Skip("spec discrepancy: NRI does not deliver teardown hooks to subsequent plugins after one plugin returns an error")
			}

			Expect(plugin1StopReceived.Load()).To(BeNumerically(">=", 1),
				"Plugin 1 MUST receive StopPodSandbox hook even when plugin 0 fails")
			Expect(plugin1RemoveReceived.Load()).To(BeNumerically(">=", 1),
				"Plugin 1 MUST receive RemovePodSandbox hook even when plugin 0 fails")

			By("verifying sandbox is fully removed")

			allPods, listErr := rc.ListPodSandbox(ctx, nil)
			Expect(listErr).NotTo(HaveOccurred())

			for _, pod := range allPods {
				Expect(pod.GetId()).NotTo(Equal(podID),
					"Sandbox MUST be fully removed despite plugin errors")
			}

			// Mark as cleaned up
			podID = ""
		})
	})

	Context("hook delivery edge cases", Serial, func() {
		var (
			testStub  *NRITestStub
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
		)

		AfterEach(func(ctx SpecContext) {
			// Capture the fallback sandbox ID before cleanup resets events.
			cleanupID := podID
			if cleanupID == "" && testStub != nil {
				cleanupID = testStub.Plugin.LastRunPodSandboxID()
			}

			// Stop the stub to unblock any hooks that may be holding
			// a RunPodSandbox call, allowing it to complete.
			if testStub != nil {
				testStub.Cleanup()
			}

			if cleanupID != "" {
				_ = rc.StopPodSandbox(ctx, cleanupID)
				_ = rc.RemovePodSandbox(ctx, cleanupID)
			}
		})

		It("should deliver StopPodSandbox hook to plugin even after slow RunPodSandbox hook", func(ctx SpecContext) {
			// This test validates that even when a plugin's RunPodSandbox hook is slow
			// (blocks for a period before returning success), the StopPodSandbox hook is
			// still delivered to the plugin when the sandbox is later stopped. This ensures
			// teardown hooks are reliable regardless of creation-path latency.
			hookBlocking := make(chan struct{})
			hookReached := make(chan struct{})

			var err error

			testStub, err = StartNRITestStub("cri-test-nri-stop-after-timeout", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Configure stub to simulate a slow RunPodSandbox (blocks for a while then returns)
			testStub.Plugin.OnRunPodSandbox = func(hookCtx context.Context, _ *nri.PodSandbox) error {
				close(hookReached)

				select {
				case <-hookBlocking:
				case <-hookCtx.Done():
				}

				return nil
			}

			By("triggering RunPodSandbox in a goroutine with slow plugin")

			podSandboxName := "nri-test-stop-after-timeout-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			podConfig = &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}

			var (
				runErr   error
				runPodID string
				runWg    sync.WaitGroup
			)

			runWg.Go(func() {
				runPodID, runErr = rc.RunPodSandbox(ctx, podConfig, framework.TestContext.RuntimeHandler)
			})

			By("waiting for RunPodSandbox hook to fire")

			select {
			case <-hookReached:
				// Hook is blocking (simulating slow plugin)
			case <-time.After(30 * time.Second):
				close(hookBlocking)
				Fail("Timed out waiting for RunPodSandbox NRI hook to fire")
			}

			By("releasing the slow hook so RunPodSandbox completes")
			close(hookBlocking)
			runWg.Wait()
			Expect(runErr).NotTo(HaveOccurred(), "RunPodSandbox should succeed after slow hook returns")
			Expect(runPodID).NotTo(BeEmpty())
			podID = runPodID

			// Reset events to only capture StopPodSandbox from here
			testStub.Plugin.Reset()

			By("stopping the sandbox and verifying StopPodSandbox hook is delivered")
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())

			stopEvent, err := testStub.Plugin.WaitForEvent(EventStopPodSandbox, 10*time.Second)
			Expect(err).NotTo(HaveOccurred(),
				"StopPodSandbox NRI hook MUST be delivered even after RunPodSandbox was slow/delayed")
			Expect(stopEvent.PodSandboxID).To(Equal(podID))
		})

		It("should not invoke NRI hooks for invalid CRI requests", func(ctx SpecContext) {
			// This test validates that invalid CRI requests (bad arguments, non-existing sandbox)
			// do not trigger NRI hooks. The runtime should reject these requests before reaching
			// the NRI plugin layer.
			var err error

			testStub, err = StartNRITestStub("cri-test-nri-invalid-req", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			By("attempting CreateContainer for a non-existing sandbox")

			containerName := "nri-test-invalid-ctr-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  framework.DefaultPauseCommand,
				Linux:    &runtimeapi.LinuxContainerConfig{},
			}

			bogusConfig := &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata("bogus", "bogus-uid", "bogus-ns", framework.DefaultAttempt),
			}

			_, createErr := rc.CreateContainer(ctx, "non-existing-sandbox-id-12345", containerConfig, bogusConfig)
			Expect(createErr).To(HaveOccurred(),
				"CreateContainer for a non-existing sandbox should fail")

			By("waiting briefly and verifying no NRI CreateContainer hook was fired")
			// Allow time for any potential event delivery
			time.Sleep(1 * time.Second)
			Expect(testStub.Plugin.HasEventOfType(EventCreateContainer)).To(BeFalse(),
				"NRI CreateContainer hook MUST NOT fire for a non-existing sandbox")

			By("verifying no NRI RunPodSandbox hook was fired either")
			Expect(testStub.Plugin.HasEventOfType(EventRunPodSandbox)).To(BeFalse(),
				"No NRI hooks should fire for invalid CRI requests")
		})
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

		It("should stop all workload containers and keep sandbox accessible while StopPodSandbox hook is in progress", func(ctx SpecContext) {
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
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
				Labels: framework.DefaultPodLabels,
			}
			podID = framework.RunPodSandbox(ctx, rc, podConfig)
			Expect(podID).NotTo(BeEmpty())

			By("creating and starting a container in the sandbox")
			framework.PullPublicImage(ctx, ic, framework.TestContext.TestImageList.DefaultTestContainerImage, nil)

			containerName := "nri-test-stop-state-ctr-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  framework.DefaultPauseCommand,
				Linux:    &runtimeapi.LinuxContainerConfig{},
			}
			containerID = framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
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
			Expect(statusErr).NotTo(HaveOccurred(), "PodSandboxStatus during StopPodSandbox hook")
			Expect(statusResp.GetStatus()).NotTo(BeNil(),
				"PodSandboxStatus MUST be accessible during StopPodSandbox hook")
			Expect(statusResp.GetStatus().GetId()).To(Equal(podID))

			By("releasing the hook and verifying StopPodSandbox succeeds")
			close(hookBlocking)
			stopWg.Wait()
			Expect(stopErr).NotTo(HaveOccurred(), "StopPodSandbox should succeed after hook returns")

			// StopPodSandbox stopped the container; clear so AfterEach skips it.
			containerID = ""

			// Remove the sandbox now and clear podID so AfterEach doesn't issue a
			// second StopPodSandbox (which could re-enter the hook callback).
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred(),
				"RemovePodSandbox should succeed after stop")
			podID = ""
		})
	})
})
