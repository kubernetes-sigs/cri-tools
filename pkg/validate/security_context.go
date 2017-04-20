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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	nginxContainerImage string = "gcr.io/google_containers/nginx-slim:0.7"
)

var _ = framework.KubeDescribe("Security Context", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support podSandbox with security context", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support podSandbox with HostPID", func() {
			By("create podSandbox for security context HostPID")
			podID, podConfig = createNamespacePodSandbox(rc, false, true, false)

			By("create nginx container")
			containerID, nginxContainerName := createNamespaceContainer(rc, ic, podID, podConfig, "nginx-container-", nginxContainerImage, true, nil)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("get nginx container pid")
			command := []string{"cat", "/var/run/nginx.pid"}
			output := execSyncContainer(rc, containerID, command)
			nginxPid := strings.TrimSpace(string(output))
			framework.Logf("Nginx's pid is %q", nginxPid)

			By("create busybox container with hostPID")
			command = []string{"sh", "-c", "sleep 1000"}
			containerID, _ = createNamespaceContainer(rc, ic, podID, podConfig, "container-with-HostPID-test-", defaultContainerImage, true, command)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("should show its pid in the hostPID namespace container")
			cmd := []string{"pidof", "nginx", "||", "true"}
			output = execSyncContainer(rc, containerID, cmd)
			pids := strings.TrimSpace(string(output))
			framework.Logf("Got nginx's pid %q from pod %q", pids, nginxContainerName)

			if pids == "" {
				framework.Failf("nginx's pid should be seen by hostpid containers")
			}

			if !strings.Contains(pids, nginxPid) {
				framework.Failf("nginx's pid should be seen by hostpid containers")
			}

		})

	})

	Context("runtime should support container with security context", func() {
		var podID, hostPath string
		var podConfig *runtimeapi.PodSandboxConfig

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support RunAsUser", func() {
			By("create pod")
			podID, podConfig = createPodSandboxForContainer(rc)

			By("create container for security context RunAsUser")
			containerID, expectedLogMessage := createRunAsUserContainer(rc, ic, podID, podConfig, "container-with-RunAsUser-test-")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("verify RunAsUser for container")
			command := []string{"id", "-u"}
			verifyExecSyncOutput(rc, containerID, command, expectedLogMessage)
		})

		It("runtime should support RunAsUserName", func() {
			By("create pod")
			podID, podConfig = createPodSandboxForContainer(rc)

			By("create container for security context RunAsUser")
			containerID, expectedLogMessage := createRunAsUserNameContainer(rc, ic, podID, podConfig, "container-with-RunAsUserName-test-")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("verify RunAsUserName for container")
			command := []string{"id", "-nu"}
			verifyExecSyncOutput(rc, containerID, command, expectedLogMessage)
		})

		It("runtime should support that ReadOnlyRootfs is false", func() {
			By("create pod with log")
			podID, podConfig, hostPath = createPodSandboxWithLogDirectory(rc)

			defer os.RemoveAll(hostPath) //clean up the TempDir

			By("create container with ReadOnlyRootfs_false")
			readOnlyRootfs := false
			logPath, containerID := createReadOnlyRootfsContainer(rc, ic, podID, podConfig, "container-with-ReadOnlyRootfs-false-test-", readOnlyRootfs)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("Check whether rootfs is writable")
			checkRootfs(podConfig, logPath, readOnlyRootfs)
		})

		It("runtime should support that ReadOnlyRootfs is true", func() {
			By("create pod with log")
			podID, podConfig, hostPath = createPodSandboxWithLogDirectory(rc)

			defer os.RemoveAll(hostPath) //clean up the TempDir

			By("create container with ReadOnlyRootfs_true")
			readOnlyRootfs := true
			logPath, containerID := createReadOnlyRootfsContainer(rc, ic, podID, podConfig, "container-with-ReadOnlyRootfs-true-test-", readOnlyRootfs)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

			By("Check whether rootfs is read-only")
			checkRootfs(podConfig, logPath, readOnlyRootfs)
		})
	})

})

// createRunAsUserContainer creates the container with specified RunAsUser in ContainerConfig.
func createRunAsUserContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) (string, []byte) {
	var uidV runtimeapi.Int64Value
	uidV.Value = 1001
	userID := strconv.FormatInt(uidV.Value, 10)
	expectedLogMessage := []byte(userID + "\n")

	By("create a container with RunAsUser")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: defaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				RunAsUser: &uidV,
			},
		},
	}

	return createContainer(rc, ic, containerConfig, podID, podConfig), expectedLogMessage
}

// createRunAsUserNameContainer creates the container with specified RunAsUserName in ContainerConfig.
func createRunAsUserNameContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) (string, []byte) {
	userName := "nobody"
	expectedLogMessage := []byte(userName + "\n")

	By("create a container with RunAsUserName")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: defaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				RunAsUsername: userName,
			},
		},
	}
	return createContainer(rc, ic, containerConfig, podID, podConfig), expectedLogMessage
}

// createNamespacePodSandbox creates a PodSandbox for creating containers.
func createNamespacePodSandbox(rc internalapi.RuntimeService, hostNetwork bool, hostPID bool, hostIPC bool) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "NamespaceOption-PodSandbox-" + framework.NewUUID()
	uid := defaultUIDPrefix + framework.NewUUID()
	namespace := defaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: buildPodSandboxMetadata(podSandboxName, uid, namespace, defaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
				NamespaceOptions: &runtimeapi.NamespaceOption{
					HostNetwork: hostNetwork,
					HostPid:     hostPID,
					HostIpc:     hostIPC,
				},
			},
		},
	}

	podID := runPodSandbox(rc, config)
	return podID, config
}

// createNamespaceContainer creates container with different NamespaceOption config.
func createNamespaceContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string, image string, hostPID bool, command []string) (string, string) {
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: image},
		Command:  command,
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				NamespaceOptions: &runtimeapi.NamespaceOption{
					HostPid: hostPID,
				},
			},
		},
	}

	return createContainer(rc, ic, containerConfig, podID, podConfig), containerName

}

// createReadOnlyRootfsContainer creates creates the container with specified ReadOnlyRootfs in ContainerConfig.
func createReadOnlyRootfsContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string, readonly bool) (string, string) {
	containerName := prefix + framework.NewUUID()
	path := fmt.Sprintf("%s.log", containerName)
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: defaultContainerImage},
		Command:  []string{"sh", "-c", "touch test.go && [ -f test.go ] && echo 'Found'"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				ReadonlyRootfs: readonly,
			},
		},
		LogPath: path,
	}

	return containerConfig.LogPath, createContainer(rc, ic, containerConfig, podID, podConfig)
}

// checkRootfs checks whether the rootfs parameter of the ContainerConfig is working properly.
func checkRootfs(podConfig *runtimeapi.PodSandboxConfig, logpath string, readOnlyRootfs bool) {
	if readOnlyRootfs {
		failLog := "touch: test.go: Read-only file system"
		expectedLogMessage := &logMessage{
			log:    []byte(failLog + "\n"),
			stream: stderrType,
		}
		verifyLogContents(podConfig, logpath, expectedLogMessage)
	} else {
		successLog := "Found"
		expectedLogMessage := &logMessage{
			log:    []byte(successLog + "\n"),
			stream: stdoutType,
		}
		verifyLogContents(podConfig, logpath, expectedLogMessage)
	}
}
