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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

const (
	apparmorProfileNamePrefix = "localhost/"
	testProfiles              = `
#include <tunables/global>
profile cri-validate-apparmor-test-deny-write flags=(attach_disconnected) {
  #include <abstractions/base>
  file,
  # Deny all file writes.
  deny /** w,
}
profile cri-validate-apparmor-test-audit-write flags=(attach_disconnected) {
  #include <abstractions/base>
  file,
  # Only audit file writes.
  audit /** w,
}
`
)

// The AppArmor profile to the CRI via the deprecated apparmor_profile field
// in favor of the newer structured apparmor field.
// CRI provides the AppArmor profile via both fields to maintain backwards compatibility.
var _ = framework.KubeDescribe("AppArmor", func() {
	f := framework.NewDefaultCRIFramework()

	var (
		rc                internalapi.RuntimeService
		ic                internalapi.ImageManagerService
		checkAppArmorOnce sync.Once
		appArmorEnabled   bool
	)

	appArmorIsEnabled := func() bool {
		checkAppArmorOnce.Do(func() {
			buf, err := os.ReadFile("/sys/module/apparmor/parameters/enabled")
			appArmorEnabled = err == nil && len(buf) > 1 && buf[0] == 'Y'
		})

		return appArmorEnabled
	}

	BeforeEach(func() {
		if !appArmorIsEnabled() {
			Skip("AppArmor is disabled on this host")
		}
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
		Expect(loadTestProfiles()).NotTo(HaveOccurred())
	})

	Context("runtime should support deprecated apparmor_profile field", func() {
		var sandboxID string
		var sandboxConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			sandboxID, sandboxConfig = framework.CreatePodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(context.TODO(), sandboxID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), sandboxID)).NotTo(HaveOccurred())
		})

		It("should fail with an unloaded apparmor_profile", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				ApparmorProfile: apparmorProfileNamePrefix + "non-existent-profile",
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, false)
			Expect(containerID).To(BeEmpty())
		})

		It("should enforce a apparmor_profile blocking writes", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				ApparmorProfile: apparmorProfileNamePrefix + "cri-validate-apparmor-test-deny-write",
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, true)
			checkContainerApparmor(rc, containerID, false)
		})

		It("should enforce a permissive deprecated profile", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				ApparmorProfile: apparmorProfileNamePrefix + "cri-validate-apparmor-test-audit-write",
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, true)
			checkContainerApparmor(rc, containerID, true)
		})
	})

	Context("runtime should support apparmor field", func() {
		var sandboxID string
		var sandboxConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			sandboxID, sandboxConfig = framework.CreatePodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(context.TODO(), sandboxID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), sandboxID)).NotTo(HaveOccurred())
		})

		It("should fail with an unloaded apparmor_profile", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				Apparmor: &runtimeapi.SecurityProfile{
					ProfileType:  runtimeapi.SecurityProfile_Localhost,
					LocalhostRef: apparmorProfileNamePrefix + "non-existent-profile",
				},
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, false)
			Expect(containerID).To(BeEmpty())
		})

		It("should enforce a apparmor_profile blocking writes", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				Apparmor: &runtimeapi.SecurityProfile{
					ProfileType:  runtimeapi.SecurityProfile_Localhost,
					LocalhostRef: apparmorProfileNamePrefix + "cri-validate-apparmor-test-deny-write",
				},
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, true)
			checkContainerApparmor(rc, containerID, false)
		})

		It("should enforce a permissive deprecated profile", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				Apparmor: &runtimeapi.SecurityProfile{
					ProfileType:  runtimeapi.SecurityProfile_Localhost,
					LocalhostRef: apparmorProfileNamePrefix + "cri-validate-apparmor-test-audit-write",
				},
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, true)
			checkContainerApparmor(rc, containerID, true)
		})
	})

	Context("runtime should prefer new apparmor field", func() {
		var sandboxID string
		var sandboxConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			sandboxID, sandboxConfig = framework.CreatePodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(context.TODO(), sandboxID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), sandboxID)).NotTo(HaveOccurred())
		})

		It("should fail with an unloaded apparmor_profile", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				ApparmorProfile: apparmorProfileNamePrefix + "non-existent-profile",
				Apparmor: &runtimeapi.SecurityProfile{
					ProfileType:  runtimeapi.SecurityProfile_Localhost,
					LocalhostRef: apparmorProfileNamePrefix + "non-existent-profile",
				},
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, false)
			Expect(containerID).To(BeEmpty())
		})

		It("should enforce a apparmor_profile blocking writes", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				ApparmorProfile: apparmorProfileNamePrefix + "non-existent-profile",
				Apparmor: &runtimeapi.SecurityProfile{
					ProfileType:  runtimeapi.SecurityProfile_Localhost,
					LocalhostRef: apparmorProfileNamePrefix + "cri-validate-apparmor-test-deny-write",
				},
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, true)
			checkContainerApparmor(rc, containerID, false)
		})

		It("should work with apparmor profile", func() {
			profile := &runtimeapi.LinuxContainerSecurityContext{
				ApparmorProfile: apparmorProfileNamePrefix + "non-existent-profile",
				Apparmor: &runtimeapi.SecurityProfile{
					ProfileType:  runtimeapi.SecurityProfile_Localhost,
					LocalhostRef: apparmorProfileNamePrefix + "cri-validate-apparmor-test-audit-write",
				},
			}
			containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, true)
			checkContainerApparmor(rc, containerID, true)
		})
	})
})

func createContainerWithAppArmor(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, sandboxID string, sandboxConfig *runtimeapi.PodSandboxConfig, profile *runtimeapi.LinuxContainerSecurityContext, shouldSucceed bool) string {
	By("create a container with apparmor")

	containerName := "apparmor-test-" + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  []string{"touch", "/tmp/foo"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: profile,
		},
	}

	containerID, err := framework.CreateContainerWithError(rc, ic, containerConfig, sandboxID, sandboxConfig)
	if shouldSucceed {
		Expect(err).ToNot(HaveOccurred())
		By("start container with apparmor")

		err := rc.StartContainer(context.TODO(), containerID)
		Expect(err).NotTo(HaveOccurred())

		// wait container started and check the status.
		Eventually(func() runtimeapi.ContainerState {
			return getContainerStatus(rc, containerID).GetState()
		}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))
	} else {
		Expect(err).To(HaveOccurred())
	}

	return containerID
}

func checkContainerApparmor(rc internalapi.RuntimeService, containerID string, shouldRun bool) {
	By("get container status")

	resp, err := rc.ContainerStatus(context.TODO(), containerID, false)
	Expect(err).NotTo(HaveOccurred())

	if shouldRun {
		Expect(resp.GetStatus().GetExitCode()).To(Equal(int32(0)))
	} else {
		Expect(resp.GetStatus().GetExitCode()).NotTo(Equal(int32(0)))
	}
}

func loadTestProfiles() error {
	f, err := os.CreateTemp("/tmp", "apparmor")
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}

	defer os.Remove(f.Name())
	defer f.Close()

	// write test profiles to a temp file.
	if _, err = f.WriteString(testProfiles); err != nil {
		return fmt.Errorf("write profiles to file: %w", err)
	}

	// load apparmor profiles into kernel.
	cmd := exec.CommandContext(context.TODO(), "sudo", "apparmor_parser", "-r", "-W", f.Name())
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	out, err := cmd.Output()
	// apparmor_parser does not always return an error code, so consider any stderr output an error.
	if err != nil || stderr.Len() > 0 {
		if stderr.Len() > 0 {
			logrus.Warn(stderr.String())
		}

		if len(out) > 0 {
			logrus.Infof("apparmor_parser: %s", out)
		}

		return fmt.Errorf("load profiles: %w", err)
	}

	return nil
}
