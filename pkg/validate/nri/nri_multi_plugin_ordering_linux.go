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

// multiPluginOrderingRecorder records the order in which RunPodSandbox hooks
// of several NRI plugins are entered and exited.
type multiPluginOrderingRecorder struct {
	mu      sync.Mutex
	entries []string
}

func (r *multiPluginOrderingRecorder) record(entry string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entry)
}

func (r *multiPluginOrderingRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.entries)
}

// multiPluginOrderingSandboxListed reports whether the sandbox with the given
// ID is returned by ListPodSandbox.
func multiPluginOrderingSandboxListed(
	ctx context.Context,
	rc internalapi.RuntimeService,
	podID string,
) (bool, error) {
	pods, err := rc.ListPodSandbox(ctx, nil)
	if err != nil {
		return false, err
	}

	for _, pod := range pods {
		if pod.GetId() == podID {
			return true, nil
		}
	}

	return false, nil
}

var _ = framework.KubeDescribe("NRI", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService

	BeforeEach(func() {
		if framework.TestContext.NRISocketPath == "" {
			Skip("NRI socket not configured (use -nri-socket flag)")
		}

		rc = f.CRIClient.CRIRuntimeClient
	})

	Context("multi-plugin ordering", Serial, func() {
		const (
			lowIdxPluginName  = "cri-test-nri-multi-order-low"
			highIdxPluginName = "cri-test-nri-multi-order-high"
		)

		var (
			stubs       []*NRITestStub
			releaseHigh func()
			runDone     chan struct{}
			runPodID    string
			hookPodID   string
			hookPodIDMu sync.Mutex
		)

		BeforeEach(func() {
			stubs = nil
			releaseHigh = nil
			runDone = nil
			runPodID = ""
			hookPodID = ""
		})

		AfterEach(func(ctx SpecContext) {
			// Release the blocked hook first so a pending RunPodSandbox call
			// can complete before the sandbox is torn down.
			if releaseHigh != nil {
				releaseHigh()
			}

			// Capture fallback sandbox IDs before the stubs reset their events.
			hookPodIDMu.Lock()
			cleanupIDs := []string{hookPodID}
			hookPodIDMu.Unlock()

			for _, s := range stubs {
				cleanupIDs = append(cleanupIDs, s.Plugin.LastRunPodSandboxID())
			}

			for _, s := range stubs {
				s.Cleanup()
			}

			if runDone != nil {
				select {
				case <-runDone:
					cleanupIDs = append(cleanupIDs, runPodID)
				case <-time.After(30 * time.Second):
					framework.Logf("AfterEach: RunPodSandbox did not return within 30s")
				}
			}

			slices.Sort(cleanupIDs)

			for _, id := range slices.Compact(cleanupIDs) {
				if id == "" {
					continue
				}

				if err := rc.StopPodSandbox(ctx, id); err != nil {
					framework.Logf("AfterEach: StopPodSandbox(%s) failed: %v", id, err)
				}

				if err := rc.RemovePodSandbox(ctx, id); err != nil {
					framework.Logf("AfterEach: RemovePodSandbox(%s) failed: %v", id, err)
				}
			}
		})

		It(
			"should invoke RunPodSandbox hooks of all plugins in index order and "+
				"not return or list the sandbox until the last plugin completes",
			func(ctx SpecContext) {
				// Spec (NRI pod sandbox lifecycle, "Multi-Plugin Coordination"):
				// hooks execute in plugin index order, and all RunPodSandbox hooks
				// complete before the sandbox is made available.
				recorder := &multiPluginOrderingRecorder{}

				highReached := make(chan struct{})
				highRelease := make(chan struct{})

				var releaseOnce sync.Once

				releaseHigh = func() { releaseOnce.Do(func() { close(highRelease) }) }

				// Hooks are installed through the configure callback so they are in
				// place before the plugin connects to the runtime.
				lowHook := func(p *NRITestPlugin) {
					p.OnRunPodSandbox = func(_ context.Context, _ *nri.PodSandbox) error {
						recorder.record(lowIdxPluginName + ":enter")
						recorder.record(lowIdxPluginName + ":exit")

						return nil
					}
				}

				var highReachedOnce sync.Once

				highHook := func(p *NRITestPlugin) {
					p.OnRunPodSandbox = func(hookCtx context.Context, pod *nri.PodSandbox) error {
						recorder.record(highIdxPluginName + ":enter")

						hookPodIDMu.Lock()
						hookPodID = pod.GetId()
						hookPodIDMu.Unlock()

						highReachedOnce.Do(func() { close(highReached) })

						select {
						case <-highRelease:
						case <-hookCtx.Done():
						}

						recorder.record(highIdxPluginName + ":exit")

						return nil
					}
				}

				By("registering the higher-index plugin before the lower-index plugin")
				// Registering in reverse index order ensures the observed ordering is
				// driven by the plugin index rather than by registration order.
				highStub, err := StartNRITestStub(highIdxPluginName, "11", highHook)
				Expect(
					err,
				).NotTo(HaveOccurred(), "failed to start NRI test stub %s", highIdxPluginName)

				stubs = append(stubs, highStub)

				lowStub, err := StartNRITestStub(lowIdxPluginName, "10", lowHook)
				Expect(
					err,
				).NotTo(HaveOccurred(), "failed to start NRI test stub %s", lowIdxPluginName)

				stubs = append(stubs, lowStub)

				By("triggering RunPodSandbox in a goroutine")

				podSandboxName := "nri-test-multi-order-" + framework.NewUUID()
				uid := framework.DefaultUIDPrefix + framework.NewUUID()
				namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
				podConfig := &runtimeapi.PodSandboxConfig{
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

				var runErr error

				done := make(chan struct{})
				runDone = done

				go func() {
					defer GinkgoRecover()
					defer close(done)

					runPodID, runErr = rc.RunPodSandbox(
						ctx,
						podConfig,
						framework.TestContext.RuntimeHandler,
					)
				}()

				By("waiting for the higher-index plugin to receive RunPodSandbox")
				Eventually(highReached, 30*time.Second).Should(BeClosed(),
					"plugin %s (index 11) should receive the RunPodSandbox hook", highIdxPluginName)

				By("verifying both plugins received RunPodSandbox in index order")
				Expect(recorder.snapshot()).To(Equal([]string{
					lowIdxPluginName + ":enter",
					lowIdxPluginName + ":exit",
					highIdxPluginName + ":enter",
				}), "the lower-index plugin's RunPodSandbox hook MUST complete before "+
					"the higher-index plugin's hook is invoked")

				hookPodIDMu.Lock()
				blockedPodID := hookPodID
				hookPodIDMu.Unlock()
				Expect(blockedPodID).NotTo(BeEmpty(),
					"the RunPodSandbox hook should receive the sandbox ID")

				By("verifying RunPodSandbox has not returned and the sandbox is neither " +
					"listed nor Ready while the higher-index plugin is blocking")
				// The observation window must stay well below the runtime's NRI plugin
				// request timeout (2s by default in containerd), after which the
				// blocked hook is abandoned and RunPodSandbox proceeds.
				Consistently(func(g Gomega) {
					g.Expect(done).NotTo(BeClosed(),
						"RunPodSandbox MUST NOT return while a plugin's RunPodSandbox hook is in progress")

					listed, err := multiPluginOrderingSandboxListed(ctx, rc, blockedPodID)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(listed).To(BeFalse(),
						"sandbox %s MUST NOT be listed while a plugin's RunPodSandbox hook is in progress",
						blockedPodID)

					// Ideally the sandbox should not be found at all. Some runtimes may
					// return a non-Ready status instead of NotFound — both are acceptable.
					statusResp, statusErr := rc.PodSandboxStatus(ctx, blockedPodID, false)
					if statusErr == nil && statusResp.GetStatus() != nil {
						g.Expect(statusResp.GetStatus().GetState()).NotTo(
							Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
							"sandbox MUST NOT report Ready while a plugin's RunPodSandbox hook is in progress")
					}
				}, time.Second, 100*time.Millisecond).Should(Succeed())

				By("releasing the higher-index plugin and verifying RunPodSandbox completes")
				releaseHigh()
				Eventually(done, 30*time.Second).Should(BeClosed(),
					"RunPodSandbox should return after all plugins complete their hooks")
				Expect(runErr).NotTo(HaveOccurred(),
					"RunPodSandbox should succeed after all plugins complete their hooks")
				Expect(runPodID).To(Equal(blockedPodID),
					"RunPodSandbox should return the sandbox ID seen by the NRI plugins")

				Expect(recorder.snapshot()).To(Equal([]string{
					lowIdxPluginName + ":enter",
					lowIdxPluginName + ":exit",
					highIdxPluginName + ":enter",
					highIdxPluginName + ":exit",
				}), "each plugin should receive RunPodSandbox exactly once")

				By("verifying the sandbox is Ready after all plugins complete")

				statusResp, err := rc.PodSandboxStatus(ctx, runPodID, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(statusResp.GetStatus().GetState()).To(
					Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
					"sandbox should be Ready after all plugins complete their RunPodSandbox hooks")
			},
		)
	})
})
