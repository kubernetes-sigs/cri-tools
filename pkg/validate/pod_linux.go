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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("PodSandbox", func() {
	f := framework.NewDefaultCRIFramework()

	var (
		rc internalapi.RuntimeService
		ic internalapi.ImageManagerService
	)

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support sysctls", func() {
		var (
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
		)

		AfterEach(func(ctx SpecContext) {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		})

		It("should support safe sysctls", func(ctx SpecContext) {
			podID, podConfig = createSandboxWithSysctls(ctx, rc, map[string]string{
				"kernel.shm_rmid_forced": "1",
			})

			By("create a default container")

			containerID := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "container-shm-rmid-forced")

			By("start container")
			startContainer(ctx, rc, containerID)

			By("check sysctls kernel.shm_rmid_forced")
			checkSetSysctls(ctx, rc, containerID, "/proc/sys/kernel/shm_rmid_forced", "1")
		})

		It("should support unsafe sysctls", func(ctx SpecContext) {
			podID, podConfig = createSandboxWithSysctls(ctx, rc, map[string]string{
				"fs.mqueue.msg_max": "100",
			})

			By("create a default container")

			containerID := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "container-fs-mqueue-msg-max")

			By("start container")
			startContainer(ctx, rc, containerID)

			By("check sysctls fs.mqueue.msg_max")
			checkSetSysctls(ctx, rc, containerID, "/proc/sys/fs/mqueue/msg_max", "100")
		})
	})
})

// createSandboxWithSysctls creates a PodSandbox with specified sysctls.
func createSandboxWithSysctls(ctx context.Context, rc internalapi.RuntimeService, sysctls map[string]string) (string, *runtimeapi.PodSandboxConfig) {
	By("create a PodSandbox with sysctls")

	podSandboxName := "pod-sandbox-with-sysctls-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	podConfig := &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(ctx, rc),
			Sysctls:      sysctls,
		},
	}

	return framework.RunPodSandbox(ctx, rc, podConfig), podConfig
}

// checkSetSysctls checks whether sysctl settings is equal to expected string.
func checkSetSysctls(ctx context.Context, rc internalapi.RuntimeService, containerID, sysctlPath, expected string) {
	cmd := []string{"cat", sysctlPath}
	stdout, _, err := rc.ExecSync(ctx, containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.TrimSpace(string(stdout))).To(Equal(expected))
}
