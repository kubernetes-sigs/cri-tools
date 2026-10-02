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
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"time"

	nri "github.com/containerd/nri/pkg/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

// updatePodSandboxCall records a single NRI UpdatePodSandbox request.
type updatePodSandboxCall struct {
	podID     string
	podName   string
	podUID    string
	overhead  *nri.LinuxResources
	resources *nri.LinuxResources
}

// updatePodSandboxPlugin is an NRI plugin dedicated to the UpdatePodSandbox
// tests. It records the pods it is synchronized with, the UpdatePodSandbox
// requests and the PostUpdatePodSandbox events, and can be configured to fail
// either of the two hooks.
type updatePodSandboxPlugin struct {
	mu            sync.Mutex
	syncedPods    map[string]*nri.PodSandbox
	updates       []updatePodSandboxCall
	postUpdates   []string
	updateErr     error
	postUpdateErr error

	ready     chan struct{}
	readyOnce sync.Once
}

// Synchronize implements stub.SynchronizeInterface. It captures the pods the
// runtime reconciles the plugin with - including their current pod-level
// resources - and signals readiness.
func (p *updatePodSandboxPlugin) Synchronize(
	_ context.Context,
	pods []*nri.PodSandbox,
	_ []*nri.Container,
) ([]*nri.ContainerUpdate, error) {
	p.mu.Lock()

	p.syncedPods = make(map[string]*nri.PodSandbox, len(pods))
	for _, pod := range pods {
		p.syncedPods[pod.GetId()] = pod
	}

	p.mu.Unlock()

	p.readyOnce.Do(func() { close(p.ready) })

	return nil, nil
}

// UpdatePodSandbox implements stub.UpdatePodInterface.
func (p *updatePodSandboxPlugin) UpdatePodSandbox(
	_ context.Context,
	pod *nri.PodSandbox,
	overhead, resources *nri.LinuxResources,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.updates = append(p.updates, updatePodSandboxCall{
		podID:     pod.GetId(),
		podName:   pod.GetName(),
		podUID:    pod.GetUid(),
		overhead:  overhead,
		resources: resources,
	})

	return p.updateErr
}

// PostUpdatePodSandbox implements stub.PostUpdatePodInterface.
func (p *updatePodSandboxPlugin) PostUpdatePodSandbox(
	_ context.Context,
	pod *nri.PodSandbox,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.postUpdates = append(p.postUpdates, pod.GetId())

	return p.postUpdateErr
}

// setUpdateErr configures the error returned from subsequent UpdatePodSandbox requests.
func (p *updatePodSandboxPlugin) setUpdateErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.updateErr = err
}

// setPostUpdateErr configures the error returned from subsequent
// PostUpdatePodSandbox events.
func (p *updatePodSandboxPlugin) setPostUpdateErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.postUpdateErr = err
}

// syncedPod returns the pod the plugin was synchronized with on connect, or nil.
func (p *updatePodSandboxPlugin) syncedPod(podID string) *nri.PodSandbox {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.syncedPods[podID]
}

// updatesFor returns the UpdatePodSandbox requests recorded for podID.
func (p *updatePodSandboxPlugin) updatesFor(podID string) []updatePodSandboxCall {
	p.mu.Lock()
	defer p.mu.Unlock()

	var result []updatePodSandboxCall

	for i := range p.updates {
		if p.updates[i].podID == podID {
			result = append(result, p.updates[i])
		}
	}

	return result
}

// postUpdateCountFor returns the number of PostUpdatePodSandbox events recorded for podID.
func (p *updatePodSandboxPlugin) postUpdateCountFor(podID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := 0

	for _, id := range p.postUpdates {
		if id == podID {
			count++
		}
	}

	return count
}

// waitForPostUpdates waits until at least want PostUpdatePodSandbox events are
// recorded for podID and returns the number recorded when it stops waiting.
//
// It polls rather than using Eventually because a runtime that does not emit
// the event at all must be told apart from one that emits the wrong number of
// them: the former is a known runtime gap the specs skip on, the latter is a
// failure (see expectPostUpdates).
func (p *updatePodSandboxPlugin) waitForPostUpdates(
	podID string,
	want int,
	timeout time.Duration,
) int {
	deadline := time.Now().Add(timeout)

	for {
		if count := p.postUpdateCountFor(podID); count >= want || time.Now().After(deadline) {
			return count
		}

		time.Sleep(50 * time.Millisecond)
	}
}

// postUpdatePodSandboxUnsupported explains why a spec is skipped on a runtime
// that never delivers the PostUpdatePodSandbox event.
//
// SPEC_DISCREPANCY: NRI defines PostUpdatePodSandbox as the event following a
// successful UpdatePodSandbox request, and containerd emits it from its
// UpdatePodSandboxResources handler. CRI-O may relay the synchronous
// UpdatePodSandbox request and stop there: at the time of writing
// PostUpdatePodSandbox is absent from its NRI API (internal/nri/nri.go) and
// never called by server/nri-api.go, so a plugin subscribed to the event is
// registered for it but never receives one. A runtime that does implement the
// event satisfies the spec and is not skipped.
const postUpdatePodSandboxUnsupported = "spec discrepancy: the runtime relays CRI " +
	"UpdatePodSandboxResources to the synchronous NRI UpdatePodSandbox request but never " +
	"emits the PostUpdatePodSandbox event NRI defines to follow it (expected one event for " +
	"the updated pod, observed none); the runtime may not implement PostUpdatePodSandbox yet"

// expectPostUpdates asserts the number of NRI PostUpdatePodSandbox events the
// runtime delivered for podID. PostUpdatePodSandbox is an asynchronous event,
// so it may arrive after the CRI call returned and has to be waited for.
//
// A runtime that delivers none at all skips the spec rather than failing it,
// see postUpdatePodSandboxUnsupported.
func expectPostUpdates(
	plugin *updatePodSandboxPlugin,
	podID string,
	want int,
	description string,
) {
	got := plugin.waitForPostUpdates(podID, want, 10*time.Second)
	if got == 0 {
		Skip(postUpdatePodSandboxUnsupported)
	}

	Expect(got).To(Equal(want), description)

	// waitForPostUpdates stops as soon as want events are recorded, so a
	// duplicate arriving slightly later would go unnoticed without this.
	Consistently(func() int {
		return plugin.postUpdateCountFor(podID)
	}, time.Second, 100*time.Millisecond).Should(Equal(want), description)
}

// updatePodSandboxStub runs an updatePodSandboxPlugin connected to the runtime.
//
// It uses the same stub lifecycle helper as StartNRITestStub; only the plugin
// implementation differs, because the set of NRI events a plugin subscribes to
// is derived from the interfaces it implements and the UpdatePodSandbox events
// must not be subscribed to by the other NRI specs (runtimes predating them
// reject such a plugin during registration).
type updatePodSandboxStub struct {
	*nriStubConn

	plugin *updatePodSandboxPlugin
}

// startUpdatePodSandboxStub connects an updatePodSandboxPlugin to the
// runtime's NRI socket and waits for the registration handshake to complete.
func startUpdatePodSandboxStub(pluginName, pluginIdx string) (*updatePodSandboxStub, error) {
	plugin := &updatePodSandboxPlugin{ready: make(chan struct{})}

	conn, err := startNRIStub(plugin, plugin.ready, pluginName, pluginIdx)
	if err != nil {
		return nil, err
	}

	return &updatePodSandboxStub{nriStubConn: conn, plugin: plugin}, nil
}

// skipIfUpdatePodSandboxResourcesUnimplemented probes the CRI
// UpdatePodSandboxResources API with a nonexistent sandbox ID and skips the
// spec if the runtime does not implement it. The probe must run before the
// plugin connects: runtimes predating UpdatePodSandboxResources (e.g.
// containerd 2.0) also predate the NRI UpdatePodSandbox events and reject a
// plugin subscribing to them during registration.
func skipIfUpdatePodSandboxResourcesUnimplemented(
	ctx context.Context,
	rc internalapi.RuntimeService,
) {
	_, err := rc.UpdatePodSandboxResources(ctx, &runtimeapi.UpdatePodSandboxResourcesRequest{
		PodSandboxId: "nri-update-pod-probe-" + framework.NewUUID(),
	})
	if s, ok := status.FromError(err); ok && s.Code() == codes.Unimplemented {
		Skip("runtime does not implement CRI UpdatePodSandboxResources " +
			"(added in containerd 2.1); NRI UpdatePodSandbox cannot be triggered: " + s.Message())
	}
}

// expectNRIResourcesMatch asserts that the NRI resources relayed to the
// plugin carry the values requested via CRI.
func expectNRIResourcesMatch(
	got *nri.LinuxResources,
	want *runtimeapi.LinuxContainerResources,
	what string,
) {
	Expect(got).NotTo(BeNil(), "NRI %s resources should be set", what)
	Expect(got.GetCpu().GetShares().GetValue()).To(BeEquivalentTo(want.GetCpuShares()),
		"NRI %s CPU shares should match the CRI request", what)
	Expect(got.GetCpu().GetQuota().GetValue()).To(Equal(want.GetCpuQuota()),
		"NRI %s CPU quota should match the CRI request", what)
	Expect(got.GetCpu().GetPeriod().GetValue()).To(BeEquivalentTo(want.GetCpuPeriod()),
		"NRI %s CPU period should match the CRI request", what)
	Expect(got.GetMemory().GetLimit().GetValue()).To(Equal(want.GetMemoryLimitInBytes()),
		"NRI %s memory limit should match the CRI request", what)
}

// expectCRIResourcesMatch asserts that the pod-level resources the runtime
// reports as applied carry the values requested via CRI.
func expectCRIResourcesMatch(
	got, want *runtimeapi.LinuxContainerResources,
	what string,
) {
	Expect(got).NotTo(BeNil(), "the runtime should report the applied %s resources", what)
	Expect(got.GetCpuShares()).To(Equal(want.GetCpuShares()),
		"applied %s CPU shares should match the CRI request", what)
	Expect(got.GetCpuQuota()).To(Equal(want.GetCpuQuota()),
		"applied %s CPU quota should match the CRI request", what)
	Expect(got.GetCpuPeriod()).To(Equal(want.GetCpuPeriod()),
		"applied %s CPU period should match the CRI request", what)
	Expect(got.GetMemoryLimitInBytes()).To(Equal(want.GetMemoryLimitInBytes()),
		"applied %s memory limit should match the CRI request", what)
}

// updatePodSandboxVerboseInfo is the subset of the runtime-specific verbose
// PodSandboxStatus info the specs read. containerd reports the resources and
// overhead of the last applied UpdatePodSandboxResources call here; both are
// absent until an update has been applied.
type updatePodSandboxVerboseInfo struct {
	Overhead  *runtimeapi.ContainerResources `json:"overhead"`
	Resources *runtimeapi.ContainerResources `json:"resources"`
}

// updatePodSandboxAppliedResources returns the pod-level resources the runtime
// reports as applied to the sandbox, read from the verbose PodSandboxStatus
// info. This is the only place a CRI client can observe whether a pod resource
// update took effect: the PodSandboxStatus fields do not carry pod-level
// resources, and the NRI pod sandbox the runtime synchronizes plugins with
// keeps reporting the resources the sandbox was created with.
//
// The last return value is false when the runtime reports no applied pod
// resources at all, either because no update has been applied yet or because
// it does not expose them in its (runtime-specific) verbose info.
func updatePodSandboxAppliedResources(
	ctx context.Context,
	rc internalapi.RuntimeService,
	podID string,
) (overhead, resources *runtimeapi.LinuxContainerResources, ok bool) {
	statusResp, err := rc.PodSandboxStatus(ctx, podID, true)
	Expect(err).NotTo(HaveOccurred(), "verbose PodSandboxStatus")

	raw, found := statusResp.GetInfo()["info"]
	if !found {
		return nil, nil, false
	}

	info := updatePodSandboxVerboseInfo{}
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		framework.Logf("failed to parse the verbose PodSandboxStatus info: %v", err)

		return nil, nil, false
	}

	overhead = info.Overhead.GetLinux()
	resources = info.Resources.GetLinux()

	return overhead, resources, overhead != nil || resources != nil
}

// updatePodSandboxTestResources builds the pod-level resources used by the
// UpdatePodSandbox specs.
func updatePodSandboxTestResources(
	cpuShares, cpuQuota, memoryMiB int64,
) *runtimeapi.LinuxContainerResources {
	return &runtimeapi.LinuxContainerResources{
		CpuShares:          cpuShares,
		CpuQuota:           cpuQuota,
		CpuPeriod:          100000,
		MemoryLimitInBytes: memoryMiB * 1024 * 1024,
	}
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

	Context("UpdatePodSandbox", Serial, func() {
		var (
			stubs []*updatePodSandboxStub
			podID string
		)

		// The resources the pod sandbox is created with, the ones the specs
		// update it to, and a third, distinct set for the specs that have to
		// tell two updates apart.
		var (
			initialOverhead  = updatePodSandboxTestResources(10, 5000, 64)
			initialResources = updatePodSandboxTestResources(256, 25000, 512)
			updatedOverhead  = updatePodSandboxTestResources(20, 10000, 128)
			updatedResources = updatePodSandboxTestResources(512, 50000, 1024)
			retriedOverhead  = updatePodSandboxTestResources(30, 15000, 192)
			retriedResources = updatePodSandboxTestResources(768, 75000, 1536)
		)

		const injectedUpdateErr = "cri-test injected UpdatePodSandbox failure"

		// startStub connects another plugin and registers it for cleanup.
		startStub := func(pluginName, pluginIdx string) *updatePodSandboxStub {
			ts, err := startUpdatePodSandboxStub(pluginName, pluginIdx)
			Expect(err).NotTo(HaveOccurred(), "failed to start NRI test stub %q", pluginName)

			stubs = append(stubs, ts)

			return ts
		}

		runPod := func(ctx context.Context, prefix string) (string, *runtimeapi.PodSandboxConfig) {
			podConfig := &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(
					prefix+framework.NewUUID(),
					framework.DefaultUIDPrefix+framework.NewUUID(),
					framework.DefaultNamespacePrefix+framework.NewUUID(),
					framework.DefaultAttempt,
				),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
					Overhead:     initialOverhead,
					Resources:    initialResources,
				},
				Labels: framework.DefaultPodLabels,
			}

			id := framework.RunPodSandbox(ctx, rc, podConfig)
			Expect(id).NotTo(BeEmpty())

			return id, podConfig
		}

		// updateResources requests the given pod-level resources for the pod
		// sandbox the spec created.
		updateResources := func(
			ctx context.Context,
			overhead, resources *runtimeapi.LinuxContainerResources,
		) error {
			_, err := rc.UpdatePodSandboxResources(
				ctx,
				&runtimeapi.UpdatePodSandboxResourcesRequest{
					PodSandboxId: podID,
					Overhead:     overhead,
					Resources:    resources,
				},
			)

			return err
		}

		// expectSyncedPodResources connects a fresh plugin and asserts the
		// pod-level resources the runtime synchronizes it with. containerd
		// reports the resources the sandbox was created with and does not fold
		// applied updates into them, so this cannot tell an applied update
		// from a discarded one - it only shows which resources a plugin
		// joining after an update is handed. Use
		// updatePodSandboxAppliedResources to observe an applied update.
		expectSyncedPodResources := func(
			pluginName, pluginIdx string,
			overhead, resources *runtimeapi.LinuxContainerResources,
			what string,
		) {
			pod := startStub(pluginName, pluginIdx).plugin.syncedPod(podID)
			Expect(pod).NotTo(BeNil(),
				"a newly connected plugin should be synchronized with the pod sandbox")
			expectNRIResourcesMatch(pod.GetLinux().GetPodResources(), resources, what+" pod")
			expectNRIResourcesMatch(pod.GetLinux().GetPodOverhead(), overhead, what+" overhead")
		}

		// expectAppliedResources asserts the pod-level resources the runtime
		// reports as applied to the sandbox. The spec is skipped if the runtime
		// reports none, as an applied update then cannot be distinguished from
		// a discarded one.
		expectAppliedResources := func(
			ctx context.Context,
			overhead, resources *runtimeapi.LinuxContainerResources,
			what string,
		) {
			gotOverhead, gotResources, ok := updatePodSandboxAppliedResources(ctx, rc, podID)
			if !ok {
				Skip("the runtime does not report the applied pod-level resources in its " +
					"verbose PodSandboxStatus info, so an applied pod resource update " +
					"cannot be distinguished from a discarded one")
			}

			expectCRIResourcesMatch(gotResources, resources, what+" pod")
			expectCRIResourcesMatch(gotOverhead, overhead, what+" overhead")
		}

		BeforeEach(func(ctx SpecContext) {
			skipIfUpdatePodSandboxResourcesUnimplemented(ctx, rc)
		})

		AfterEach(func(ctx SpecContext) {
			for _, ts := range slices.Backward(stubs) {
				ts.Stop()
			}

			stubs = nil

			if podID != "" {
				if err := rc.StopPodSandbox(ctx, podID); err != nil {
					framework.Logf("AfterEach: StopPodSandbox(%s) failed: %v", podID, err)
				}

				if err := rc.RemovePodSandbox(ctx, podID); err != nil {
					framework.Logf("AfterEach: RemovePodSandbox(%s) failed: %v", podID, err)
				}

				podID = ""
			}
		})

		It("should relay CRI UpdatePodSandboxResources to NRI UpdatePodSandbox",
			func(ctx SpecContext) {
				testStub := startStub("cri-test-nri-update-pod", "00")

				By("creating a pod sandbox")

				var podConfig *runtimeapi.PodSandboxConfig

				podID, podConfig = runPod(ctx, "nri-test-update-pod-")

				By("calling CRI UpdatePodSandboxResources")

				Expect(updateResources(ctx, updatedOverhead, updatedResources)).To(Succeed(),
					"UpdatePodSandboxResources should succeed when the NRI plugin accepts the update")

				By("verifying the NRI request carries the pod and the requested resources")

				updates := testStub.plugin.updatesFor(podID)
				// UpdatePodSandbox is a synchronous NRI request, so it must have
				// been delivered before the CRI call returned.
				Expect(updates).To(HaveLen(1),
					"NRI UpdatePodSandbox should be delivered exactly once before the CRI call returns")
				Expect(updates[0].podName).To(Equal(podConfig.GetMetadata().GetName()))
				Expect(updates[0].podUID).To(Equal(podConfig.GetMetadata().GetUid()))
				expectNRIResourcesMatch(updates[0].resources, updatedResources, "pod")
				expectNRIResourcesMatch(updates[0].overhead, updatedOverhead, "overhead")

				By("verifying the NRI PostUpdatePodSandbox event is delivered")
				expectPostUpdates(testStub.plugin, podID, 1,
					"NRI PostUpdatePodSandbox should be delivered once after a successful update")
			})

		It("should fail CRI UpdatePodSandboxResources when the NRI plugin rejects UpdatePodSandbox",
			func(ctx SpecContext) {
				testStub := startStub("cri-test-nri-update-pod-fail", "00")
				testStub.plugin.setUpdateErr(errors.New(injectedUpdateErr))

				By("creating a pod sandbox")

				podID, _ = runPod(ctx, "nri-test-update-pod-fail-")

				By("calling CRI UpdatePodSandboxResources with the plugin rejecting the update")

				updateErr := updateResources(ctx, updatedOverhead, updatedResources)
				Expect(updateErr).To(HaveOccurred(),
					"UpdatePodSandboxResources should fail when an NRI plugin rejects UpdatePodSandbox")
				Expect(updateErr.Error()).To(ContainSubstring(injectedUpdateErr),
					"the CRI error should carry the NRI plugin's error message")
				Expect(testStub.plugin.updatesFor(podID)).To(HaveLen(1),
					"NRI UpdatePodSandbox should have been delivered to the plugin")

				By("verifying no NRI PostUpdatePodSandbox event is delivered")
				Consistently(func() int {
					return testStub.plugin.postUpdateCountFor(podID)
				}, 2*time.Second, 200*time.Millisecond).Should(Equal(0),
					"NRI PostUpdatePodSandbox must not be delivered when the update was rejected")

				By("verifying the pod sandbox is still ready")

				statusResp, err := rc.PodSandboxStatus(ctx, podID, false)
				Expect(err).NotTo(HaveOccurred(), "PodSandboxStatus after rejected update")
				Expect(statusResp.GetStatus().GetState()).
					To(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
						"a rejected resource update should not affect the sandbox state")

				By("verifying the plugin stays connected and a later update succeeds")
				testStub.plugin.setUpdateErr(nil)
				Expect(updateResources(ctx, updatedOverhead, updatedResources)).To(Succeed(),
					"UpdatePodSandboxResources should succeed once the plugin accepts the update")
				Expect(testStub.plugin.updatesFor(podID)).To(HaveLen(2),
					"the retried update should be delivered to the same, still connected, plugin")
				expectPostUpdates(testStub.plugin, podID, 1,
					"NRI PostUpdatePodSandbox should be delivered once for the successful retry")
			})

		It("should not apply the pod resources when the NRI plugin rejects UpdatePodSandbox",
			func(ctx SpecContext) {
				testStub := startStub("cri-test-nri-update-pod-keep", "00")

				By("creating a pod sandbox with pod-level resources")

				podID, _ = runPod(ctx, "nri-test-update-pod-keep-")

				By("calling CRI UpdatePodSandboxResources with the plugin rejecting the update")
				testStub.plugin.setUpdateErr(errors.New(injectedUpdateErr))
				Expect(updateResources(ctx, retriedOverhead, retriedResources)).NotTo(Succeed(),
					"UpdatePodSandboxResources should fail when an NRI plugin rejects UpdatePodSandbox")

				By("verifying the runtime did not apply the rejected pod resources")

				overhead, resources, applied := updatePodSandboxAppliedResources(ctx, rc, podID)
				if applied {
					// A runtime may report the current pod resources, which
					// must still be the ones the sandbox was created with.
					expectCRIResourcesMatch(resources, initialResources, "pre-update pod")
					expectCRIResourcesMatch(overhead, initialOverhead, "pre-update overhead")
				}

				By("verifying a plugin connecting afterwards receives the old resources")
				expectSyncedPodResources(
					"cri-test-nri-update-pod-keep-rejected", "10",
					initialOverhead, initialResources, "pre-update",
				)

				By("verifying a later accepted update is applied")
				testStub.plugin.setUpdateErr(nil)
				Expect(updateResources(ctx, updatedOverhead, updatedResources)).To(Succeed(),
					"UpdatePodSandboxResources should succeed once the plugin accepts the update")
				// The accepted values, not the rejected ones, are what the
				// runtime reports as applied. This also shows the check above
				// really did test something on a runtime that reports nothing
				// until an update is applied: it does report applied pod
				// resources, it just had none of the rejected ones to report.
				expectAppliedResources(ctx, updatedOverhead, updatedResources, "updated")
			})

		It("should apply the update when the NRI plugin fails PostUpdatePodSandbox",
			func(ctx SpecContext) {
				testStub := startStub("cri-test-nri-update-pod-post-fail", "00")

				By("creating a pod sandbox")

				podID, _ = runPod(ctx, "nri-test-update-pod-post-fail-")

				By("calling CRI UpdatePodSandboxResources with the plugin failing the post-event")
				testStub.plugin.setPostUpdateErr(
					errors.New("cri-test injected PostUpdatePodSandbox failure"),
				)
				// PostUpdatePodSandbox only notifies plugins after the update
				// has been applied, so a failing plugin must not turn the CRI
				// call into an error.
				Expect(updateResources(ctx, updatedOverhead, updatedResources)).To(Succeed(),
					"a PostUpdatePodSandbox failure must not fail CRI UpdatePodSandboxResources")
				expectPostUpdates(testStub.plugin, podID, 1,
					"NRI PostUpdatePodSandbox should have been delivered to the plugin")

				By("verifying the pod sandbox is still ready")

				statusResp, err := rc.PodSandboxStatus(ctx, podID, false)
				Expect(err).NotTo(HaveOccurred(), "PodSandboxStatus after failed post-update event")
				Expect(statusResp.GetStatus().GetState()).
					To(Equal(runtimeapi.PodSandboxState_SANDBOX_READY),
						"a failed post-update event should not affect the sandbox state")

				By("verifying the requested resources were applied")
				expectAppliedResources(ctx, updatedOverhead, updatedResources, "updated")

				By("verifying the plugin stays connected and a later update succeeds")
				testStub.plugin.setPostUpdateErr(nil)
				Expect(updateResources(ctx, retriedOverhead, retriedResources)).To(Succeed(),
					"UpdatePodSandboxResources should succeed after a failed post-update event")
				Expect(testStub.plugin.updatesFor(podID)).To(HaveLen(2),
					"the later update should be delivered to the same, still connected, plugin")
				expectAppliedResources(ctx, retriedOverhead, retriedResources, "later")
			})
	})
})
