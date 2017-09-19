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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("PodSandbox", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support basic operations on PodSandbox", func() {
		var podID string

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support running PodSandbox [Conformance]", func() {
			By("test run a default PodSandbox")
			podID = testRunDefaultPodSandbox(rc)

			By("test list PodSandbox")
			pods := listPodSanboxForID(rc, podID)
			Expect(podSandboxFound(pods, podID)).To(BeTrue(), "PodSandbox should be listed")
		})

		It("runtime should support stopping PodSandbox [Conformance]", func() {
			By("run PodSandbox")
			podID = framework.RunDefaultPodSandbox(rc, "PodSandbox-for-test-stop-")

			By("test stop PodSandbox")
			testStopPodSandbox(rc, podID)
		})

		It("runtime should support removing PodSandbox [Conformance]", func() {
			By("run PodSandbox")
			podID = framework.RunDefaultPodSandbox(rc, "PodSandbox-for-test-remove-")

			By("stop PodSandbox")
			stopPodSandbox(rc, podID)

			By("test remove PodSandbox")
			testRemovePodSandbox(rc, podID)
		})
	})

	Context("runtime should support sysctls", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("should support safe sysctls", func() {
			podID, podConfig = createSandboxWithSysctls(rc, map[string]string{
				"kernel.shm_rmid_forced": "1",
			})

			By("create a default container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-shm-rmid-forced")

			By("start container")
			startContainer(rc, containerID)

			By("check sysctls kernel.shm_rmid_forced")
			checkSetSysctls(rc, containerID, "/proc/sys/kernel/shm_rmid_forced", "1")
		})

		It("should support unsafe sysctls", func() {
			podID, podConfig = createSandboxWithSysctls(rc, map[string]string{
				"fs.mqueue.msg_max": "100",
			})

			By("create a default container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-fs-mqueue-msg-max")

			By("start container")
			startContainer(rc, containerID)

			By("check sysctls fs.mqueue.msg_max")
			checkSetSysctls(rc, containerID, "/proc/sys/fs/mqueue/msg_max", "100")
		})
	})
})

// podSandboxFound returns whether PodSandbox is found.
func podSandboxFound(podSandboxs []*runtimeapi.PodSandbox, podID string) bool {
	for _, podSandbox := range podSandboxs {
		if podSandbox.Id == podID {
			return true
		}
	}
	return false
}

// verifyPodSandboxStatus verifies whether PodSandbox status for given podID matches.
func verifyPodSandboxStatus(c internalapi.RuntimeService, podID string, expectedStatus runtimeapi.PodSandboxState, statusName string) {
	status := getPodSandboxStatus(c, podID)
	Expect(status.State).To(Equal(expectedStatus), "PodSandbox state should be "+statusName)
}

// testRunDefaultPodSandbox runs a PodSandbox and make sure it is ready.
func testRunDefaultPodSandbox(c internalapi.RuntimeService) string {
	podID := framework.RunDefaultPodSandbox(c, "PodSandbox-for-create-test-")
	verifyPodSandboxStatus(c, podID, runtimeapi.PodSandboxState_SANDBOX_READY, "ready")
	return podID
}

// getPodSandboxStatus gets PodSandboxStatus for podID.
func getPodSandboxStatus(c internalapi.RuntimeService, podID string) *runtimeapi.PodSandboxStatus {
	By("Get PodSandbox status for podID: " + podID)
	status, err := c.PodSandboxStatus(podID)
	framework.ExpectNoError(err, "failed to get PodSandbox %q status: %v", podID, err)
	return status
}

// stopPodSandbox stops the PodSandbox for podID.
func stopPodSandbox(c internalapi.RuntimeService, podID string) {
	By("Stop PodSandbox for podID: " + podID)
	err := c.StopPodSandbox(podID)
	framework.ExpectNoError(err, "Failed to stop PodSandbox: %v", err)
	framework.Logf("Stopped PodSandbox %q\n", podID)
}

// testStopPodSandbox stops the PodSandbox for podID and make sure it's not ready.
func testStopPodSandbox(c internalapi.RuntimeService, podID string) {
	stopPodSandbox(c, podID)
	verifyPodSandboxStatus(c, podID, runtimeapi.PodSandboxState_SANDBOX_NOTREADY, "not ready")
}

// removePodSandbox removes the PodSandbox for podID.
func removePodSandbox(c internalapi.RuntimeService, podID string) {
	By("Remove PodSandbox for podID: " + podID)
	err := c.RemovePodSandbox(podID)
	framework.ExpectNoError(err, "failed to remove PodSandbox: %v", err)
	framework.Logf("Removed PodSandbox %q\n", podID)
}

// testRemovePodSandbox removes a PodSandbox and make sure it is removed.
func testRemovePodSandbox(c internalapi.RuntimeService, podID string) {
	removePodSandbox(c, podID)
	pods := listPodSanboxForID(c, podID)
	Expect(podSandboxFound(pods, podID)).To(BeFalse(), "PodSandbox should be removed")
}

// listPodSanboxForID lists PodSandbox for podID.
func listPodSanboxForID(c internalapi.RuntimeService, podID string) []*runtimeapi.PodSandbox {
	By("List PodSandbox for podID: " + podID)
	filter := &runtimeapi.PodSandboxFilter{
		Id: podID,
	}
	return listPodSandbox(c, filter)
}

// listPodSandbox lists PodSandbox.
func listPodSandbox(c internalapi.RuntimeService, filter *runtimeapi.PodSandboxFilter) []*runtimeapi.PodSandbox {
	By("List PodSandbox.")
	pods, err := c.ListPodSandbox(filter)
	framework.ExpectNoError(err, "failed to list PodSandbox status: %v", err)
	framework.Logf("List PodSandbox succeed")
	return pods
}

// createLogTempDir creates the log temp directory for podSandbox.
func createLogTempDir(podSandboxName string) (string, string) {
	hostPath, err := ioutil.TempDir("", "/podLogTest")
	framework.ExpectNoError(err, "failed to create TempDir %q: %v", hostPath, err)
	podLogPath := filepath.Join(hostPath, podSandboxName)
	err = os.MkdirAll(podLogPath, 0777)
	framework.ExpectNoError(err, "failed to create host path %s: %v", podLogPath, err)

	return hostPath, podLogPath
}

// createPodSandboxWithLogDirectory creates a PodSandbox with log directory.
func createPodSandboxWithLogDirectory(c internalapi.RuntimeService) (string, *runtimeapi.PodSandboxConfig, string) {
	By("create a PodSandbox with log directory")
	podSandboxName := "PodSandbox-with-log-directory-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

	hostPath, podLogPath := createLogTempDir(podSandboxName)

	podConfig := &runtimeapi.PodSandboxConfig{
		Metadata:     framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		LogDirectory: podLogPath,
	}
	return framework.RunPodSandbox(c, podConfig), podConfig, hostPath
}

// createSandboxWithSysctls creates a PodSandbox with specified sysctls.
func createSandboxWithSysctls(rc internalapi.RuntimeService, sysctls map[string]string) (string, *runtimeapi.PodSandboxConfig) {
	By("create a PodSandbox with sysctls")
	podSandboxName := "pod-sandbox-with-sysctls-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

	podConfig := &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			Sysctls: sysctls,
		},
	}
	return framework.RunPodSandbox(rc, podConfig), podConfig
}

// checkSetSysctls checks whether sysctl settings is equal to expected string.
func checkSetSysctls(rc internalapi.RuntimeService, containerID, sysctlPath, expected string) {
	cmd := []string{"cat", sysctlPath}
	stdout, _, err := rc.ExecSync(containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.TrimSpace(string(stdout))).To(Equal(expected))
}
