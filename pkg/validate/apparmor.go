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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	"github.com/golang/glog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

var _ = framework.KubeDescribe("AppArmor", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	if isAppArmorEnabled() {
		BeforeEach(func() {
			rc = f.CRIClient.CRIRuntimeClient
			ic = f.CRIClient.CRIImageClient
			Expect(loadTestProfiles()).NotTo(HaveOccurred())
		})

		Context("runtime should support apparmor", func() {
			var sandboxID string
			var sandboxConfig *runtimeapi.PodSandboxConfig

			BeforeEach(func() {
				sandboxID, sandboxConfig = framework.CreatePodSandboxForContainer(rc)
			})

			AfterEach(func() {
				By("stop PodSandbox")
				rc.StopPodSandbox(sandboxID)
				By("delete PodSandbox")
				rc.RemovePodSandbox(sandboxID)
			})

			It("should fail with with an unloaded profile", func() {
				profile := apparmorProfileNamePrefix + "non-existant-profile"
				containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, false)
				checkContainerApparmor(rc, containerID, false)
			})

			It("should enforce a profile blocking writes", func() {
				profile := apparmorProfileNamePrefix + "cri-validate-apparmor-test-deny-write"
				containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, true)
				checkContainerApparmor(rc, containerID, false)
			})

			It("should enforce a permissive profile", func() {
				profile := apparmorProfileNamePrefix + "cri-validate-apparmor-test-audit-write"
				containerID := createContainerWithAppArmor(rc, ic, sandboxID, sandboxConfig, profile, true)
				checkContainerApparmor(rc, containerID, true)
			})
		})
	}
})

func createContainerWithAppArmor(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, sandboxID string, sandboxConfig *runtimeapi.PodSandboxConfig, profile string, shouldStart bool) string {
	By("create a container with apparmor")
	containerName := "apparmor-test-" + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"touch", "foo"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				ApparmorProfile: profile,
			},
		},
	}
	containerID := framework.CreateContainer(rc, ic, containerConfig, sandboxID, sandboxConfig)

	By("start container with apparmor")
	err := rc.StartContainer(containerID)
	if shouldStart {
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(err).To(HaveOccurred())
	}

	// wait container started and check the status.
	Eventually(func() runtimeapi.ContainerState {
		return getContainerStatus(rc, containerID).State
	}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

	return containerID
}

func checkContainerApparmor(rc internalapi.RuntimeService, containerID string, shoudRun bool) {
	By("get container status")
	status, err := rc.ContainerStatus(containerID)
	Expect(err).NotTo(HaveOccurred())

	if shoudRun {
		Expect(status.GetExitCode()).To(Equal(int32(0)))
	} else {
		Expect(status.GetExitCode()).NotTo(Equal(int32(0)))
	}
}

func loadTestProfiles() error {
	f, err := ioutil.TempFile("/tmp", "apparmor")
	if err != nil {
		return fmt.Errorf("failed to open temp file: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	// write test profiles to a temp file.
	if _, err = f.WriteString(testProfiles); err != nil {
		return fmt.Errorf("failed to write profiles to file: %v", err)
	}

	// load apparmor profiles into kernel.
	cmd := exec.Command("sudo", "apparmor_parser", "-r", "-W", f.Name())
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	out, err := cmd.Output()
	// apparmor_parser does not always return an error code, so consider any stderr output an error.
	if err != nil || stderr.Len() > 0 {
		if stderr.Len() > 0 {
			glog.Warning(stderr.String())
		}
		if len(out) > 0 {
			glog.Infof("apparmor_parser: %s", out)
		}

		return fmt.Errorf("failed to load profiles: %v", err)
	}

	glog.V(2).Infof("Loaded profiles: %v", out)
	return nil
}

// isAppArmorEnabled returns true if apparmor is enabled for the host.
// This function is forked from
// https://github.com/opencontainers/runc/blob/1a81e9ab1f138c091fe5c86d0883f87716088527/libcontainer/apparmor/apparmor.go
// to avoid the libapparmor dependency.
// TODO: replace with k8s.io/kubernetes/pkg/security/apparmor when vendor is possible.
func isAppArmorEnabled() bool {
	if _, err := os.Stat("/sys/kernel/security/apparmor"); err == nil && os.Getenv("container") == "" {
		if _, err = os.Stat("/sbin/apparmor_parser"); err == nil {
			buf, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled")
			return err == nil && len(buf) > 1 && buf[0] == 'Y'
		}
	}
	return false
}
