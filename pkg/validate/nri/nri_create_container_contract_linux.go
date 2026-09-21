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

	Context("CreateContainer contract", Serial, func() {
		var (
			testStub  *NRITestStub
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
			// containerID holds the successfully created retry container so
			// AfterEach can remove it even if an inline assertion fails.
			containerID string
		)

		BeforeEach(func(ctx SpecContext) {
			var err error

			testStub, err = StartNRITestStub("cri-test-nri-create", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub")

			// Ensure the test image is available before creating containers.
			framework.PullPublicImage(
				ctx,
				ic,
				framework.TestContext.TestImageList.DefaultTestContainerImage,
				nil,
			)

			By("creating a pod sandbox")

			podSandboxName := "nri-test-create-" + framework.NewUUID()
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
		})

		AfterEach(func(ctx SpecContext) {
			// Stop the stub first so a still-failing hook cannot interfere with
			// teardown of the container or sandbox.
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
			"should not expose container while CreateContainer hook is in progress",
			func(ctx SpecContext) {
				// This test validates the spec contract: during CreateContainer hook
				// execution, the container MUST NOT be visible via ListContainers or
				// ContainerStatus.
				hookBlocking := make(chan struct{})
				hookReached := make(chan struct{})

				var hookContainerID string

				testStub.Plugin.OnCreateContainer = func(hookCtx context.Context, _ *nri.PodSandbox, container *nri.Container) error {
					hookContainerID = container.GetId()

					close(hookReached)

					select {
					case <-hookBlocking:
					case <-hookCtx.Done():
					}

					return nil
				}

				By("triggering CreateContainer in a goroutine")

				containerName := "nri-test-block-create-" + framework.NewUUID()
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

				var (
					createErr error
					createdID string
					createWg  sync.WaitGroup
				)

				createWg.Go(func() {
					createdID, createErr = rc.CreateContainer(
						ctx,
						podID,
						containerConfig,
						podConfig,
					)
				})

				By("waiting for CreateContainer hook to be reached")

				select {
				case <-hookReached:
					// Hook is now blocking
				case <-time.After(30 * time.Second):
					close(hookBlocking) // unblock to avoid goroutine leak
					Fail("Timed out waiting for CreateContainer NRI hook to fire")
				}

				By("verifying container is NOT listed while hook is blocking")

				containers, listErr := rc.ListContainers(ctx, &runtimeapi.ContainerFilter{
					PodSandboxId: podID,
				})
				Expect(listErr).NotTo(HaveOccurred())

				for _, c := range containers {
					Expect(c.GetId()).NotTo(Equal(hookContainerID),
						"Container %s MUST NOT be listed while CreateContainer hook is blocking", hookContainerID)
				}

				By("verifying ContainerStatus is not accessible while hook is blocking")

				if hookContainerID != "" {
					statusResp, statusErr := rc.ContainerStatus(ctx, hookContainerID, false)
					if statusErr == nil && statusResp != nil && statusResp.GetStatus() != nil {
						Expect(
							statusResp.GetStatus().GetState(),
						).NotTo(Equal(runtimeapi.ContainerState_CONTAINER_CREATED),
							"Container MUST NOT report CREATED state while CreateContainer hook is in progress")
					}
				}

				By("releasing the hook and verifying container is created")
				close(hookBlocking)
				createWg.Wait()
				Expect(
					createErr,
				).NotTo(HaveOccurred(), "CreateContainer should succeed after hook returns")
				Expect(createdID).NotTo(BeEmpty())
				containerID = createdID

				// After hook completes, container should be in CREATED state
				statusResp, err := rc.ContainerStatus(ctx, containerID, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(
					statusResp.GetStatus().GetState(),
				).To(Equal(runtimeapi.ContainerState_CONTAINER_CREATED),
					"Container should be in CREATED state after CreateContainer hook completes")
			},
		)

		It(
			"should fail CreateContainer when the NRI hook errors, leak nothing, and allow retry",
			func(ctx SpecContext) {
				// Fail only the first CreateContainer invocation so the retry passes.
				var failOnce sync.Once

				testStub.Plugin.OnCreateContainer = func(_ context.Context, _ *nri.PodSandbox, _ *nri.Container) error {
					shouldFail := false

					failOnce.Do(func() { shouldFail = true })

					if shouldFail {
						return errors.New("induced NRI CreateContainer failure")
					}

					return nil
				}

				// Reset events so we only observe container events from this point.
				testStub.Plugin.Reset()

				By("attempting CreateContainer while the NRI hook is failing")

				containerName := "nri-test-create-error-ctr-" + framework.NewUUID()
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

				failedContainerID, createErr := framework.CreateContainerWithError(
					ctx,
					rc,
					ic,
					containerConfig,
					podID,
					podConfig,
				)
				Expect(createErr).To(HaveOccurred(),
					"CreateContainer MUST fail when the NRI CreateContainer hook returns an error")
				Expect(failedContainerID).To(BeEmpty(),
					"No container ID should be returned when CreateContainer fails")

				By("verifying the NRI CreateContainer hook actually fired")
				// The hook records its event before returning the error, confirming
				// the failure was induced on the creation path as intended.
				Eventually(func() int {
					count := 0

					for _, e := range testStub.Plugin.Events() {
						if e.Type == EventCreateContainer {
							count++
						}
					}

					return count
				}, 10*time.Second, 50*time.Millisecond).Should(BeNumerically(">=", 1),
					"NRI CreateContainer hook should have fired before the failure")

				By("verifying the failed CreateContainer leaked no container")

				containers, listErr := rc.ListContainers(ctx, &runtimeapi.ContainerFilter{
					PodSandboxId: podID,
				})
				Expect(listErr).NotTo(HaveOccurred(), "ListContainers after failed CreateContainer")

				for _, c := range containers {
					if c.GetMetadata() != nil && c.GetMetadata().GetName() == containerName {
						containerID = c.GetId() // hand to AfterEach for cleanup
						Fail(
							fmt.Sprintf(
								"container %s was leaked after a failed CreateContainer",
								c.GetId(),
							),
						)
					}
				}

				By("verifying the failed CreateContainer did not start a container")
				// A failed CreateContainer MUST NOT result in a started container.
				// The runtime may emit Stop/Remove events as part of internal cleanup
				// of the partially created container, but a StartContainer event must
				// never appear. Use Consistently so events delivered slightly after the
				// failure are still caught.
				Consistently(func() int {
					count := 0

					for _, e := range testStub.Plugin.Events() {
						if e.Type == EventStartContainer {
							count++
						}
					}

					return count
				}, 2*time.Second, 200*time.Millisecond).Should(BeZero(),
					"a failed CreateContainer MUST NOT result in a started container")

				By("retrying CreateContainer after the NRI hook stops failing")

				retryName := "nri-test-create-retry-ctr-" + framework.NewUUID()
				retryConfig := &runtimeapi.ContainerConfig{
					Metadata: framework.BuildContainerMetadata(retryName, framework.DefaultAttempt),
					Image: &runtimeapi.ImageSpec{
						Image: framework.TestContext.TestImageList.DefaultTestContainerImage,
					},
					Command: framework.DefaultPauseCommand,
					Linux:   &runtimeapi.LinuxContainerConfig{},
				}

				retryID := framework.CreateContainer(ctx, rc, ic, retryConfig, podID, podConfig)
				Expect(retryID).NotTo(BeEmpty(),
					"CreateContainer retry should succeed after the NRI hook stops failing")
				// Hand the retry container to AfterEach immediately so a failure in
				// the start/stop/remove assertions below cannot leak it.
				containerID = retryID

				By("verifying the retried container can be started, stopped, and removed")
				Expect(rc.StartContainer(ctx, retryID)).NotTo(HaveOccurred(),
					"the retried container should start successfully")
				Expect(rc.StopContainer(ctx, retryID, 0)).NotTo(HaveOccurred(),
					"the retried container should stop successfully")
				Expect(rc.RemoveContainer(ctx, retryID)).NotTo(HaveOccurred(),
					"the retried container should be removable")
				// Clear containerID so AfterEach does not attempt to stop/remove it again.
				containerID = ""
			},
		)
	})
})
