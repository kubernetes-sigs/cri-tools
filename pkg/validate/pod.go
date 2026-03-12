/*
Copyright 2017 The Kubernetes Authors.

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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("PodSandbox", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
	})

	Context("runtime should support basic operations on PodSandbox", func() {
		var podID string

		AfterEach(func(ctx SpecContext) {
			if podID != "" {
				By("stop PodSandbox")
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
				By("delete PodSandbox")
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
			}
		})

		It("runtime should support running PodSandbox [Conformance]", func(ctx SpecContext) {
			By("test run a default PodSandbox")

			podID = testRunDefaultPodSandbox(ctx, rc)

			By("test list PodSandbox")

			pods := listPodSandboxForID(ctx, rc, podID)
			Expect(podSandboxFound(pods, podID)).To(BeTrue(), "PodSandbox should be listed")
		})

		It("runtime should support stopping PodSandbox [Conformance]", func(ctx SpecContext) {
			By("run PodSandbox")

			podID = framework.RunDefaultPodSandbox(ctx, rc, "PodSandbox-for-test-stop-")

			By("test stop PodSandbox")
			testStopPodSandbox(ctx, rc, podID)
		})

		It("runtime should support removing PodSandbox [Conformance]", func(ctx SpecContext) {
			By("run PodSandbox")

			podID = framework.RunDefaultPodSandbox(ctx, rc, "PodSandbox-for-test-remove-")

			By("stop PodSandbox")
			stopPodSandbox(ctx, rc, podID)

			By("test remove PodSandbox")
			testRemovePodSandbox(ctx, rc, podID)
			podID = "" // no need to cleanup pod
		})
	})
})

// podSandboxFound returns whether PodSandbox is found.
func podSandboxFound(podSandboxs []*runtimeapi.PodSandbox, podID string) bool {
	for _, podSandbox := range podSandboxs {
		if podSandbox.GetId() == podID {
			return true
		}
	}

	return false
}

// verifyPodSandboxStatus verifies whether PodSandbox status for given podID matches.
func verifyPodSandboxStatus(ctx context.Context, c internalapi.RuntimeService, podID string, expectedStatus runtimeapi.PodSandboxState, statusName string) {
	status := getPodSandboxStatus(ctx, c, podID)
	Expect(status.GetState()).To(Equal(expectedStatus), "PodSandbox state should be "+statusName)
}

// testRunDefaultPodSandbox runs a PodSandbox and make sure it is ready.
func testRunDefaultPodSandbox(ctx context.Context, c internalapi.RuntimeService) string {
	podID := framework.RunDefaultPodSandbox(ctx, c, "PodSandbox-for-create-test-")
	verifyPodSandboxStatus(ctx, c, podID, runtimeapi.PodSandboxState_SANDBOX_READY, "ready")

	return podID
}

// getPodSandboxStatus gets PodSandboxStatus for podID.
func getPodSandboxStatus(ctx context.Context, c internalapi.RuntimeService, podID string) *runtimeapi.PodSandboxStatus {
	By("Get PodSandbox status for podID: " + podID)
	status, err := c.PodSandboxStatus(ctx, podID, false)
	framework.ExpectNoError(err, "failed to get PodSandbox %q status: %v", podID, err)

	return status.GetStatus()
}

// stopPodSandbox stops the PodSandbox for podID.
func stopPodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	By("Stop PodSandbox for podID: " + podID)
	err := c.StopPodSandbox(ctx, podID)
	framework.ExpectNoError(err, "Failed to stop PodSandbox: %v", err)
	framework.Logf("Stopped PodSandbox %q\n", podID)
}

// testStopPodSandbox stops the PodSandbox for podID and make sure it's not ready.
func testStopPodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	stopPodSandbox(ctx, c, podID)
	verifyPodSandboxStatus(ctx, c, podID, runtimeapi.PodSandboxState_SANDBOX_NOTREADY, "not ready")
}

// removePodSandbox removes the PodSandbox for podID.
func removePodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	By("Remove PodSandbox for podID: " + podID)
	err := c.RemovePodSandbox(ctx, podID)
	framework.ExpectNoError(err, "failed to remove PodSandbox: %v", err)
	framework.Logf("Removed PodSandbox %q\n", podID)
}

// testRemovePodSandbox removes a PodSandbox and make sure it is removed.
func testRemovePodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	removePodSandbox(ctx, c, podID)
	pods := listPodSandboxForID(ctx, c, podID)
	Expect(podSandboxFound(pods, podID)).To(BeFalse(), "PodSandbox should be removed")
}

// listPodSandboxForID lists PodSandbox for podID.
func listPodSandboxForID(ctx context.Context, c internalapi.RuntimeService, podID string) []*runtimeapi.PodSandbox {
	By("List PodSandbox for podID: " + podID)
	filter := &runtimeapi.PodSandboxFilter{
		Id: podID,
	}

	return listPodSandbox(ctx, c, filter)
}

// listPodSandbox lists PodSandbox.
func listPodSandbox(ctx context.Context, c internalapi.RuntimeService, filter *runtimeapi.PodSandboxFilter) []*runtimeapi.PodSandbox {
	By("List PodSandbox.")

	pods, err := c.ListPodSandbox(ctx, filter)
	framework.ExpectNoError(err, "failed to list PodSandbox status: %v", err)
	framework.Logf("List PodSandbox succeed")

	return pods
}

// createLogTempDir creates the log temp directory for podSandbox.
func createLogTempDir(podSandboxName string) (hostPath, podLogPath string) {
	hostPath, err := os.MkdirTemp("", "podLogTest")
	framework.ExpectNoError(err, "failed to create TempDir %q: %v", hostPath, err)
	podLogPath = filepath.Join(hostPath, podSandboxName)
	err = os.MkdirAll(podLogPath, 0o777)
	framework.ExpectNoError(err, "failed to create host path %s: %v", podLogPath, err)

	return hostPath, podLogPath
}

// createPodSandboxWithLogDirectory creates a PodSandbox with log directory.
func createPodSandboxWithLogDirectory(ctx context.Context, c internalapi.RuntimeService) (sandboxID string, podConfig *runtimeapi.PodSandboxConfig, hostPath string) {
	By("create a PodSandbox with log directory")

	podSandboxName := "PodSandbox-with-log-directory-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

	hostPath, podLogPath := createLogTempDir(podSandboxName)
	podConfig = &runtimeapi.PodSandboxConfig{
		Metadata:     framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		LogDirectory: podLogPath,
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(ctx, c),
		},
	}

	return framework.RunPodSandbox(ctx, c, podConfig), podConfig, hostPath
}
