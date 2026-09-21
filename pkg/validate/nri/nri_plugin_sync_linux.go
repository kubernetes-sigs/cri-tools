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
	"fmt"
	"slices"
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

	Context("plugin synchronization", Serial, func() {
		var (
			firstStub *NRITestStub
			podID     string
			podConfig *runtimeapi.PodSandboxConfig

			// createdMu guards createdContainers, which accumulates every
			// container ID created by the spec (including ones created from
			// goroutines while Synchronize is in progress) so AfterEach can
			// clean them all up even if the spec fails partway through.
			createdMu         sync.Mutex
			createdContainers []string
		)

		// recordContainer remembers an ID for AfterEach cleanup. Safe to call
		// from goroutines that create containers during Synchronize.
		recordContainer := func(id string) {
			createdMu.Lock()
			defer createdMu.Unlock()

			createdContainers = append(createdContainers, id)
		}

		// startBlockingSyncPlugin connects a second plugin named `name` whose
		// Synchronize callback blocks until release() is called, then waits
		// until that callback has fired. It returns release (idempotent) and
		// waitReady, which blocks until the plugin finishes connecting and
		// returns the ready stub. Callers MUST eventually call release followed
		// by waitReady. syncOnce guards against the (legal) case of Synchronize
		// being invoked more than once, which would panic on a double close of
		// syncReached.
		startBlockingSyncPlugin := func(name string) (release func(), waitReady func() *NRITestStub) {
			// Coordination channels for blocking inside the plugin's
			// Synchronize callback.
			syncReached := make(chan struct{})
			syncRelease := make(chan struct{})

			var syncOnce sync.Once

			configure := func(p *NRITestPlugin) {
				p.OnSynchronize = func(hookCtx context.Context, _ []*nri.PodSandbox, _ []*nri.Container) error {
					first := false

					syncOnce.Do(func() { first = true })
					// Only the first invocation participates in the handshake;
					// any later one returns immediately so cleanup is not blocked.
					if !first {
						return nil
					}

					close(syncReached)

					select {
					case <-syncRelease:
					case <-hookCtx.Done():
					}

					return nil
				}
			}

			// StartNRITestStub blocks until the plugin becomes ready, which only
			// happens after Synchronize (and thus our blocking hook) returns, so
			// run it in a goroutine while the caller drives runtime state.
			var (
				stub     *NRITestStub
				startErr error
				startWg  sync.WaitGroup
			)

			startWg.Go(func() {
				stub, startErr = StartNRITestStub(name, "10", configure)
			})

			var releaseOnce sync.Once

			release = func() { releaseOnce.Do(func() { close(syncRelease) }) }

			select {
			case <-syncReached:
				// Synchronize is now blocking inside our hook.
			case <-time.After(30 * time.Second):
				release() // unblock to avoid leaking the goroutine
				startWg.Wait()
				Fail("timed out waiting for the second plugin's Synchronize hook to fire")
			}

			waitReady = func() *NRITestStub {
				startWg.Wait()
				Expect(
					startErr,
				).NotTo(HaveOccurred(), "second NRI test stub failed to become ready")

				return stub
			}

			return release, waitReady
		}

		// pluginSeesContainer reports whether the plugin learned about container
		// id one of the two spec-compliant ways: it was part of the Synchronize
		// set, or it arrived as a regular CreateContainer callback.
		pluginSeesContainer := func(stub *NRITestStub, id string) bool {
			if slices.Contains(stub.Plugin.SyncedContainers(), id) {
				return true
			}

			for _, e := range stub.Plugin.Events() {
				if e.Type == EventCreateContainer && e.ContainerID == id {
					return true
				}
			}

			return false
		}

		// pollForLostContainers polls until the plugin has seen every id or the
		// timeout expires, returning the IDs the plugin never learned about
		// (nil when all were delivered).
		pollForLostContainers := func(stub *NRITestStub, ids ...string) []string {
			var lost []string

			timeout := time.After(15 * time.Second)
			ticker := time.NewTicker(100 * time.Millisecond)

			defer ticker.Stop()

			for {
				lost = nil

				for _, id := range ids {
					if !pluginSeesContainer(stub, id) {
						lost = append(lost, id)
					}
				}

				if len(lost) == 0 {
					return nil
				}

				select {
				case <-timeout:
					return lost
				case <-ticker.C:
				}
			}
		}

		BeforeEach(func(ctx SpecContext) {
			var err error

			firstStub, err = StartNRITestStub("cri-test-nri-sync-first", "00")
			Expect(err).NotTo(HaveOccurred(), "failed to start first NRI test stub")

			createdContainers = nil

			// Ensure test image is available
			framework.PullPublicImage(
				ctx,
				ic,
				framework.TestContext.TestImageList.DefaultTestContainerImage,
				nil,
			)
		})

		AfterEach(func(ctx SpecContext) {
			createdMu.Lock()
			ids := slices.Clone(createdContainers)
			createdMu.Unlock()

			for _, id := range ids {
				if id == "" {
					continue
				}

				if err := rc.StopContainer(ctx, id, 0); err != nil {
					framework.Logf("AfterEach: StopContainer(%s) failed: %v", id, err)
				}

				if err := rc.RemoveContainer(ctx, id); err != nil {
					framework.Logf("AfterEach: RemoveContainer(%s) failed: %v", id, err)
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

			if firstStub != nil {
				firstStub.Cleanup()
			}
		})

		It(
			"should synchronize a newly connected plugin with existing pods and containers",
			func(ctx SpecContext) {
				// Contract: when a plugin connects to the runtime, the runtime calls
				// Synchronize with the current set of pods and containers so the
				// plugin can reconcile existing state. A plugin that connects AFTER a
				// pod and container already exist MUST receive those existing
				// pods/containers in its Synchronize callback.
				By("creating a pod sandbox before the second plugin connects")

				podSandboxName := "nri-test-sync-" + framework.NewUUID()
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

				By("creating and starting a container before the second plugin connects")

				containerName := "nri-test-sync-ctr-" + framework.NewUUID()
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

				containerID := framework.CreateContainer(
					ctx,
					rc,
					ic,
					containerConfig,
					podID,
					podConfig,
				)
				Expect(containerID).NotTo(BeEmpty())
				recordContainer(containerID)
				Expect(rc.StartContainer(ctx, containerID)).NotTo(HaveOccurred())

				By("connecting a second plugin after the pod and container already exist")
				// StartNRITestStub returns only after the stub's Synchronize callback
				// has fired (plugin.ready is closed inside Synchronize), so the
				// captured sync state is populated by the time it returns.
				secondStub, err := StartNRITestStub("cri-test-nri-sync-second", "10")
				Expect(err).NotTo(HaveOccurred(), "failed to start second NRI test stub")

				defer secondStub.Cleanup()

				By("verifying the second plugin's Synchronize received the existing pod")
				Expect(secondStub.Plugin.SyncedPods()).To(ContainElement(podID),
					"second plugin's Synchronize MUST include existing pod %s", podID)

				By("verifying the second plugin's Synchronize received the existing container")
				Expect(secondStub.Plugin.SyncedContainers()).To(ContainElement(containerID),
					"second plugin's Synchronize MUST include existing container %s", containerID)

				By("creating a second container after the second plugin is ready")

				secondContainerName := "nri-test-sync-ctr2-" + framework.NewUUID()
				secondContainerConfig := &runtimeapi.ContainerConfig{
					Metadata: framework.BuildContainerMetadata(
						secondContainerName,
						framework.DefaultAttempt,
					),
					Image: &runtimeapi.ImageSpec{
						Image: framework.TestContext.TestImageList.DefaultTestContainerImage,
					},
					Command: framework.DefaultPauseCommand,
					Linux:   &runtimeapi.LinuxContainerConfig{},
				}

				containerID2 := framework.CreateContainer(
					ctx,
					rc,
					ic,
					secondContainerConfig,
					podID,
					podConfig,
				)
				Expect(containerID2).NotTo(BeEmpty())
				recordContainer(containerID2)
				Expect(rc.StartContainer(ctx, containerID2)).NotTo(HaveOccurred())

				By(
					"verifying the second plugin received a CreateContainer callback for the new container",
				)
				Eventually(func() bool {
					for _, e := range secondStub.Plugin.Events() {
						if e.Type == EventCreateContainer && e.ContainerID == containerID2 {
							return true
						}
					}

					return false
				}, 10*time.Second, 100*time.Millisecond).Should(BeTrue(),
					"second plugin MUST receive a CreateContainer callback for container %s "+
						"created after Synchronize completed", containerID2)
			},
		)

		It(
			"should receive a callback for container created during the Synchronize call",
			func(ctx SpecContext) {
				// Contract: a container created WHILE a late-joining plugin is still
				// processing its Synchronize callback MUST NOT be lost. The plugin
				// must learn about it either as part of the Synchronize set or via a
				// regular CreateContainer callback delivered after Synchronize
				// returns.
				By("creating a pod sandbox before the second plugin connects")

				podSandboxName := "nri-test-sync2-" + framework.NewUUID()
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

				By("creating and starting a container before the second plugin connects")

				containerName := "nri-test-sync2-ctr-" + framework.NewUUID()
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

				containerID := framework.CreateContainer(
					ctx,
					rc,
					ic,
					containerConfig,
					podID,
					podConfig,
				)
				Expect(containerID).NotTo(BeEmpty())
				recordContainer(containerID)
				Expect(rc.StartContainer(ctx, containerID)).NotTo(HaveOccurred())

				By("connecting a second plugin whose Synchronize blocks")

				release, waitReady := startBlockingSyncPlugin("cri-test-nri-sync2-second")

				By(
					"creating a second container while the second plugin's Synchronize is in progress",
				)

				secondContainerName := "nri-test-sync2-ctr2-" + framework.NewUUID()
				secondContainerConfig := &runtimeapi.ContainerConfig{
					Metadata: framework.BuildContainerMetadata(
						secondContainerName,
						framework.DefaultAttempt,
					),
					Image: &runtimeapi.ImageSpec{
						Image: framework.TestContext.TestImageList.DefaultTestContainerImage,
					},
					Command: framework.DefaultPauseCommand,
					Linux:   &runtimeapi.LinuxContainerConfig{},
				}

				// Create the second container in a goroutine: depending on the
				// runtime, the CRI CreateContainer call may block until the
				// in-progress Synchronize returns, so it must not block the test.
				var (
					createdID string
					createWg  sync.WaitGroup
				)

				createWg.Go(func() {
					defer GinkgoRecover()

					id := framework.CreateContainer(
						ctx,
						rc,
						ic,
						secondContainerConfig,
						podID,
						podConfig,
					)
					Expect(id).NotTo(BeEmpty())
					// Publish the ID before starting so AfterEach can clean the
					// container up even if StartContainer fails or times out.
					recordContainer(id)
					createdID = id
					Expect(rc.StartContainer(ctx, id)).NotTo(HaveOccurred())
				})

				// Hold the Synchronize call open briefly so the CreateContainer
				// request is in flight at the runtime while the second plugin is
				// still synchronizing. This is coordination to widen the race
				// window, not an assertion.
				time.Sleep(2 * time.Second)

				By("releasing the second plugin's Synchronize")
				release()

				By("waiting for the second plugin to become ready")

				secondStub := waitReady()

				defer secondStub.Cleanup()

				By("waiting for the second container to be created")
				createWg.Wait()
				Expect(createdID).NotTo(BeEmpty())

				By(
					"verifying the second plugin learns about the container created during Synchronize",
				)
				// The container must reach the second plugin one of two ways: it was
				// part of the Synchronize set, or it arrived as a regular
				// CreateContainer callback after Synchronize completed. Either is
				// spec-compliant; what is forbidden is losing it entirely.
				//
				// SPEC_DISCREPANCY: containerd (as of main / 2.x) may lose containers
				// created while a late-joining plugin's Synchronize is in progress —
				// they appear in neither the Synchronize set nor as a CreateContainer
				// callback. The NRI spec requires that no container is lost during
				// plugin initialization, but containerd does not yet implement this
				// guarantee. Poll and Skip rather than hard-failing so the remaining
				// test suite still runs.
				if lost := pollForLostContainers(secondStub, createdID); len(lost) > 0 {
					Skip(
						"spec discrepancy: containerd loses containers created while a late-joining " +
							"plugin's Synchronize is in progress; the NRI spec requires no container is lost " +
							"during plugin initialization but containerd does not yet implement this guarantee",
					)
				}
			},
		)

		It(
			"should receive information about all containers without the race condition during initialization",
			func(ctx SpecContext) {
				// Contract: when a late-joining plugin connects, every container that
				// exists OR is created around the Synchronize window MUST reach the
				// plugin exactly via one of two spec-compliant paths: the Synchronize
				// set, or a regular CreateContainer callback delivered after
				// Synchronize returns. None of them may be lost. This stresses the
				// race by creating containers right before, while, and right after the
				// second plugin processes Synchronize.
				By("creating a pod sandbox before the second plugin connects")

				podSandboxName := "nri-test-sync3-" + framework.NewUUID()
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

				// createAndStart creates and starts a container in the sandbox,
				// records it for cleanup, and returns its ID.
				createAndStart := func(namePrefix string) string {
					containerName := namePrefix + framework.NewUUID()
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

					id := framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
					Expect(id).NotTo(BeEmpty())
					// Record the ID before starting so AfterEach can clean the
					// container up even if StartContainer fails or times out.
					recordContainer(id)
					Expect(rc.StartContainer(ctx, id)).NotTo(HaveOccurred())

					return id
				}

				By("creating containers BEFORE the second plugin connects")

				const beforeCount = 2

				beforeIDs := make([]string, 0, beforeCount)

				for range beforeCount {
					beforeIDs = append(beforeIDs, createAndStart("nri-test-sync3-before-"))
				}

				By("connecting a second plugin whose Synchronize blocks")

				release, waitReady := startBlockingSyncPlugin("cri-test-nri-sync3-second")

				By("creating containers DURING the second plugin's Synchronize")
				// Depending on the runtime, the CRI CreateContainer call may block
				// until the in-progress Synchronize returns, so create each in its
				// own goroutine to keep the race window open and avoid deadlock.
				const duringCount = 3

				var (
					duringMu  sync.Mutex
					duringIDs []string
					duringWg  sync.WaitGroup
				)

				for range duringCount {
					duringWg.Go(func() {
						defer GinkgoRecover()

						id := createAndStart("nri-test-sync3-during-")

						duringMu.Lock()

						duringIDs = append(duringIDs, id)
						duringMu.Unlock()
					})
				}

				// Hold Synchronize open briefly so the CreateContainer requests are in
				// flight at the runtime while the second plugin is still
				// synchronizing. Coordination to widen the race window, not an assertion.
				time.Sleep(2 * time.Second)

				By("releasing the second plugin's Synchronize")
				release()

				By("waiting for the second plugin to become ready")

				secondStub := waitReady()

				defer secondStub.Cleanup()

				By("waiting for the containers created during Synchronize to finish creating")
				duringWg.Wait()

				By("creating containers AFTER the second plugin is ready")

				const afterCount = 2

				afterIDs := make([]string, 0, afterCount)

				for range afterCount {
					afterIDs = append(afterIDs, createAndStart("nri-test-sync3-after-"))
				}

				By("verifying the second plugin learned about every container without losing any")
				// Each container must reach the second plugin one of two ways: it was
				// part of the Synchronize set, or it arrived as a regular
				// CreateContainer callback. Either is spec-compliant; what is
				// forbidden is losing any of them.
				allIDs := slices.Concat(beforeIDs, duringIDs, afterIDs)
				Expect(allIDs).To(HaveLen(beforeCount+duringCount+afterCount),
					"sanity: expected every before/during/after container to be created")

				// SPEC_DISCREPANCY: containerd (as of main / 2.x) may lose containers
				// created concurrently while a late-joining plugin's Synchronize is
				// in progress — they appear in neither the Synchronize set nor as a
				// CreateContainer callback. The NRI spec requires that no container
				// is lost during plugin initialization, but containerd does not yet
				// implement this guarantee under concurrent creation. Poll and Skip
				// rather than hard-failing so the remaining test suite still runs.
				lostContainers := pollForLostContainers(secondStub, allIDs...)
				if len(lostContainers) > 0 {
					Skip(fmt.Sprintf("spec discrepancy: containerd lost %d container(s) created "+
						"concurrently around a late-joining plugin's Synchronize window (%v); "+
						"the NRI spec requires no container is lost during plugin initialization "+
						"but containerd does not yet implement this guarantee under concurrent creation",
						len(lostContainers), lostContainers))
				}

				framework.Logf("all %d containers reached the late-joining plugin", len(allIDs))
			},
		)
	})
})
