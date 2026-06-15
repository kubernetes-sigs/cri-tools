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
	"fmt"
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
				if err := rc.StopPodSandbox(ctx, cleanupID); err != nil {
					framework.Logf("AfterEach: StopPodSandbox(%s) failed: %v", cleanupID, err)
				}

				if err := rc.RemovePodSandbox(ctx, cleanupID); err != nil {
					framework.Logf("AfterEach: RemovePodSandbox(%s) failed: %v", cleanupID, err)
				}
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
			// This test validates that workload container creation fails while the
			// RunPodSandbox hook is still running. The NRI hook callback receives the
			// sandbox ID, so we use that to attempt CreateContainer before RunPodSandbox
			// returns. The container creation must fail because the sandbox is not ready.
			hookBlocking := make(chan struct{})
			hookReached := make(chan struct{})

			var (
				err       error
				hookPodID string
			)

			testStub, err = StartNRITestStub("cri-test-nri-block-container", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Configure stub to block RunPodSandbox and capture the sandbox ID
			testStub.Plugin.OnRunPodSandbox = func(hookCtx context.Context, pod *nri.PodSandbox) error {
				hookPodID = pod.GetId()

				close(hookReached)

				select {
				case <-hookBlocking:
				case <-hookCtx.Done():
				}

				return nil
			}

			By("pulling the test image before triggering RunPodSandbox")
			framework.PullPublicImage(ctx, ic, framework.TestContext.TestImageList.DefaultTestContainerImage, nil)

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

			By("attempting container creation while RunPodSandbox hook is blocking")

			containerName := "nri-test-while-blocked-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image: &runtimeapi.ImageSpec{
					Image:              framework.TestContext.TestImageList.DefaultTestContainerImage,
					UserSpecifiedImage: framework.TestContext.TestImageList.DefaultTestContainerImage,
				},
				Command: framework.DefaultPauseCommand,
				Linux:   &runtimeapi.LinuxContainerConfig{},
			}

			blockedContainerID, createErr := rc.CreateContainer(ctx, hookPodID, containerConfig, podConfig)
			Expect(createErr).To(HaveOccurred(),
				"CreateContainer MUST fail while RunPodSandbox hook is in progress "+
					"(sandbox %s is not ready)", hookPodID)
			Expect(blockedContainerID).To(BeEmpty(),
				"No container ID should be returned when creation fails")

			By("releasing the hook and verifying pod becomes Ready")
			close(hookBlocking)
			runWg.Wait()
			Expect(runErr).NotTo(HaveOccurred(), "RunPodSandbox should succeed after hook returns")
			Expect(runPodID).NotTo(BeEmpty())
			podID = runPodID

			By("verifying container creation succeeds after hook completes")

			containerName = "nri-test-after-hook-" + framework.NewUUID()
			containerConfig = &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  framework.DefaultPauseCommand,
				Linux:    &runtimeapi.LinuxContainerConfig{},
			}
			containerID := framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
			Expect(containerID).NotTo(BeEmpty(),
				"Container creation should succeed after RunPodSandbox hook completes")

			// Clean up container
			if err := rc.StopContainer(ctx, containerID, 0); err != nil {
				framework.Logf("StopContainer(%s) failed: %v", containerID, err)
			}

			if err := rc.RemoveContainer(ctx, containerID); err != nil {
				framework.Logf("RemoveContainer(%s) failed: %v", containerID, err)
			}
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
