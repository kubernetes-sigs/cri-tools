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
	"fmt"
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

	var rc internalapi.RuntimeService

	BeforeEach(func() {
		if framework.TestContext.NRISocketPath == "" {
			Skip("NRI socket not configured (use -nri-socket flag)")
		}

		rc = f.CRIClient.CRIRuntimeClient
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
})
