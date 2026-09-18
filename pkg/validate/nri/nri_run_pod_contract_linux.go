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
	"errors"
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

		It(
			"should not expose sandbox while RunPodSandbox hook is in progress",
			func(ctx SpecContext) {
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

				var (
					runErr   error
					runPodID string
					runWg    sync.WaitGroup
				)

				runWg.Go(func() {
					runPodID, runErr = rc.RunPodSandbox(
						ctx,
						podConfig,
						framework.TestContext.RuntimeHandler,
					)
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
						Expect(
							statusResp.GetStatus().GetState(),
						).NotTo(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
							"Sandbox MUST NOT report Ready state while RunPodSandbox hook is in progress")
					}
				}

				By("releasing the hook and verifying pod becomes Ready")
				close(hookBlocking)
				runWg.Wait()
				Expect(
					runErr,
				).NotTo(HaveOccurred(), "RunPodSandbox should succeed after hook returns")
				Expect(runPodID).NotTo(BeEmpty())
				podID = runPodID

				// After hook completes, sandbox should be Ready
				statusResp, err := rc.PodSandboxStatus(ctx, podID, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(
					statusResp.GetStatus().GetState(),
				).To(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
					"Sandbox should be Ready after RunPodSandbox hook completes")
			},
		)

		It(
			"should not start workload containers until RunPodSandbox hook completes",
			func(ctx SpecContext) {
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
				framework.PullPublicImage(
					ctx,
					ic,
					framework.TestContext.TestImageList.DefaultTestContainerImage,
					nil,
				)

				By("triggering RunPodSandbox in a goroutine")

				podSandboxName := "nri-test-block-container-" + framework.NewUUID()
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

				var (
					runErr   error
					runPodID string
					runWg    sync.WaitGroup
				)

				runWg.Go(func() {
					runPodID, runErr = rc.RunPodSandbox(
						ctx,
						podConfig,
						framework.TestContext.RuntimeHandler,
					)
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

				blockedContainerID, createErr := rc.CreateContainer(
					ctx,
					hookPodID,
					containerConfig,
					podConfig,
				)
				Expect(createErr).To(HaveOccurred(),
					"CreateContainer MUST fail while RunPodSandbox hook is in progress "+
						"(sandbox %s is not ready)", hookPodID)
				Expect(blockedContainerID).To(BeEmpty(),
					"No container ID should be returned when creation fails")

				By("releasing the hook and verifying pod becomes Ready")
				close(hookBlocking)
				runWg.Wait()
				Expect(
					runErr,
				).NotTo(HaveOccurred(), "RunPodSandbox should succeed after hook returns")
				Expect(runPodID).NotTo(BeEmpty())
				podID = runPodID

				By("verifying container creation succeeds after hook completes")

				containerName = "nri-test-after-hook-" + framework.NewUUID()
				containerConfig = &runtimeapi.ContainerConfig{
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
				containerID := framework.CreateContainer(
					ctx,
					rc,
					ic,
					containerConfig,
					podID,
					podConfig,
				)
				Expect(containerID).NotTo(BeEmpty(),
					"Container creation should succeed after RunPodSandbox hook completes")

				// Clean up container
				if err := rc.StopContainer(ctx, containerID, 0); err != nil {
					framework.Logf("StopContainer(%s) failed: %v", containerID, err)
				}

				if err := rc.RemoveContainer(ctx, containerID); err != nil {
					framework.Logf("RemoveContainer(%s) failed: %v", containerID, err)
				}
			},
		)

		It(
			"should fail RunPodSandbox and clean up when the NRI hook errors, then allow retry",
			func(ctx SpecContext) {
				var err error

				testStub, err = StartNRITestStub("cri-test-nri-run-error", "00")
				Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

				// Fail only the first RunPodSandbox invocation so the retry can pass.
				var failOnce sync.Once

				testStub.Plugin.OnRunPodSandbox = func(_ context.Context, _ *nri.PodSandbox) error {
					shouldFail := false

					failOnce.Do(func() { shouldFail = true })

					if shouldFail {
						return errors.New("induced NRI RunPodSandbox failure")
					}

					return nil
				}

				By("building the pod sandbox config")

				podSandboxName := "nri-test-run-error-" + framework.NewUUID()
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

				By("attempting RunPodSandbox while the NRI hook is failing")

				failedPodID, runErr := rc.RunPodSandbox(
					ctx,
					podConfig,
					framework.TestContext.RuntimeHandler,
				)
				Expect(runErr).To(HaveOccurred(),
					"RunPodSandbox MUST fail when the NRI RunPodSandbox hook returns an error")
				Expect(failedPodID).To(BeEmpty(),
					"No pod sandbox ID should be returned when RunPodSandbox fails")

				By("verifying the NRI RunPodSandbox hook actually fired")
				// The hook records its event before returning the error, so the
				// attempted sandbox ID is available for the cleanup/leak check.
				attemptedID := testStub.Plugin.LastRunPodSandboxID()
				Expect(attemptedID).NotTo(BeEmpty(),
					"NRI RunPodSandbox hook should have fired before the failure")

				By("verifying the failed sandbox is not left behind")

				pods, listErr := rc.ListPodSandbox(ctx, nil)
				Expect(listErr).NotTo(HaveOccurred())

				for _, pod := range pods {
					matches := pod.GetId() == attemptedID ||
						(pod.GetMetadata() != nil && pod.GetMetadata().GetName() == podSandboxName)
					Expect(matches).To(BeFalse(),
						"sandbox %s MUST be cleaned up after a failed RunPodSandbox", pod.GetId())
				}

				By("retrying RunPodSandbox after the NRI hook stops failing")

				podID = framework.RunPodSandbox(ctx, rc, podConfig)
				Expect(podID).NotTo(BeEmpty(),
					"RunPodSandbox retry should succeed after the NRI hook stops failing")

				By("verifying the retried sandbox becomes Ready")

				statusResp, err := rc.PodSandboxStatus(ctx, podID, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(
					statusResp.GetStatus().GetState(),
				).To(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
					"sandbox should become Ready after a successful RunPodSandbox retry")
			},
		)
	})
})
