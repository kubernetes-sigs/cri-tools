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
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1"

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

	Context("runtime should support NamespaceOption in the Security Context", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig
		podSandboxName := "NamespaceOption-PodSandbox-" + framework.NewUUID()

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support HostPID [security context]", func() {
			By("create podSandbox for security context HostPID")
			namespaceOption := &runtimeapi.NamespaceOption{
				HostPid:     true,
				HostIpc:     false,
				HostNetwork: false,
			}
			podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podSandboxName, "")

			By("create nginx container")
			prefix := "nginx-container-"
			containerName := prefix + framework.NewUUID()
			containerID, nginxContainerName, _ := createNamespaceContainer(rc, ic, podID, podConfig, containerName, nginxContainerImage, namespaceOption, nil, "")

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
			prefix = "container-with-HostPID-test-"
			containerName = prefix + framework.NewUUID()
			containerID, _, _ = createNamespaceContainer(rc, ic, podID, podConfig, containerName, framework.DefaultContainerImage, namespaceOption, command, "")

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

		It("runtime should support HostIpc is true [security context]", func() {
			By("create shared memory segment on the host")
			out, err := exec.Command("ipcmk", "-M", "1M").Output()
			framework.ExpectNoError(err, "failed to execute ipcmk -M 1M")
			rawID := strings.TrimSpace(string(out))
			segmentID := strings.TrimPrefix(rawID, "Shared memory id: ")

			By("create podSandbox for security context HostIPC is true")
			namespaceOption := &runtimeapi.NamespaceOption{
				HostPid:     false,
				HostIpc:     true,
				HostNetwork: false,
			}
			podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podSandboxName, "")

			By("create a default container with namespace")
			prefix := "namespace-container-"
			command := []string{"top"}
			containerName := prefix + framework.NewUUID()
			containerID, _, _ := createNamespaceContainer(rc, ic, podID, podConfig, containerName, framework.DefaultContainerImage, namespaceOption, command, "")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("check if the shared memory segment is included in the container")
			command = []string{"ipcs", "-m"}
			out = execSyncContainer(rc, containerID, command)
			Expect(string(out)).To(ContainSubstring(segmentID), "The shared memory segment should be included in the container")
		})

		It("runtime should support HostIpc is false [security context]", func() {
			By("create shared memory segment on the host")
			out, err := exec.Command("ipcmk", "-M", "1M").Output()
			framework.ExpectNoError(err, "failed to execute ipcmk -M 1M")
			rawID := strings.TrimSpace(string(out))
			segmentID := strings.TrimPrefix(rawID, "Shared memory id: ")

			By("create podSandbox for security context HostIpc is false")
			namespaceOption := &runtimeapi.NamespaceOption{
				HostPid:     false,
				HostIpc:     false,
				HostNetwork: false,
			}
			podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podSandboxName, "")

			By("create a default container with namespace")
			prefix := "namespace-container-"
			command := []string{"top"}
			containerName := prefix + framework.NewUUID()
			containerID, _, _ := createNamespaceContainer(rc, ic, podID, podConfig, containerName, framework.DefaultContainerImage, namespaceOption, command, "")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("check if the shared memory segment is not included in the container")
			command = []string{"ipcs", "-m"}
			out = execSyncContainer(rc, containerID, command)
			Expect(string(out)).NotTo(ContainSubstring(segmentID), "The shared memory segment should be included in the container")
		})

		It("runtime should support HostNetwork is true [security context]", func() {
			srv, err := net.Listen("tcp", ":0")
			if err != nil {
				framework.Failf("Failed to listen a tcp port: %v", err)
			}
			go func() {
				defer GinkgoRecover()
				for {
					conn, err := srv.Accept()
					if err != nil {
						return
					}
					conn.Write([]byte("hello"))
				}
			}()
			defer srv.Close()

			ports := strings.Split(srv.Addr().String(), ":")
			podID = createAndCheckHostNetwork(rc, ic, podSandboxName, ports[len(ports)-1], true)
		})

		It("runtime should support HostNetwork is false [security context]", func() {
			srv, err := net.Listen("tcp", ":0")
			if err != nil {
				framework.Failf("Failed to listen a tcp port: %v", err)
			}
			go func() {
				defer GinkgoRecover()
				for {
					conn, err := srv.Accept()
					if err != nil {
						return
					}
					conn.Write([]byte("hello"))
				}
			}()
			defer srv.Close()

			ports := strings.Split(srv.Addr().String(), ":")
			podID = createAndCheckHostNetwork(rc, ic, podSandboxName, ports[len(ports)-1], false)
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

		It("runtime should support RunAsUser [security context]", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

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

		It("runtime should support RunAsUserName [security context]", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

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

		It("runtime should support that ReadOnlyRootfs is false [security context]", func() {
			By("create pod with log")
			podID, podConfig, hostPath = createPodSandboxWithLogDirectory(rc)

			defer os.RemoveAll(hostPath) //clean up the TempDir

			By("create container with ReadOnlyRootfs_false")
			readOnlyRootfs := false
			logPath, containerID := createReadOnlyRootfsContainer(rc, ic, podID, podConfig, "container-with-ReadOnlyRootfs-false-test-", readOnlyRootfs)

			By("start container")
			startContainer(rc, containerID)

			By("Check whether rootfs is writable")
			checkRootfs(podConfig, logPath, readOnlyRootfs)
		})

		It("runtime should support that ReadOnlyRootfs is true [security context]", func() {
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

		It("runtime should support Privileged is true [security context]", func() {
			By("create pod")
			isPrivileged := true
			podID, podConfig = createPrivilegedPodSandbox(rc, isPrivileged)

			By("create container for security context Privileged is true")
			containerID := createPrivilegedContainer(rc, ic, podID, podConfig, "container-with-isPrivileged-test-", isPrivileged)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("check the Privileged container")
			checkNetworkManagement(rc, containerID, isPrivileged)
		})

		It("runtime should support Privileged is false [security context]", func() {
			By("create pod")
			notPrivileged := false
			podID, podConfig = createPrivilegedPodSandbox(rc, notPrivileged)

			By("create container for security context Privileged is true")
			containerID := createPrivilegedContainer(rc, ic, podID, podConfig, "container-with-notPrivileged-test-", notPrivileged)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("check the Privileged container")
			checkNetworkManagement(rc, containerID, notPrivileged)
		})

		It("runtime should support setting Capability", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container with security context Capability and test")
			containerID := createCapabilityContainer(rc, ic, podID, podConfig, "container-with-Capability-test-")

			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			checkNetworkManagement(rc, containerID, true)

			By("create container without security context Capability and test")
			containerID = framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-with-notCapability-test-")

			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			checkNetworkManagement(rc, containerID, false)
		})
	})

})

// createRunAsUserContainer creates the container with specified RunAsUser in ContainerConfig.
func createRunAsUserContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) (string, []byte) {
	By("create RunAsUser container")
	var uidV runtimeapi.Int64Value
	uidV.Value = 1001
	userID := strconv.FormatInt(uidV.Value, 10)
	expectedLogMessage := []byte(userID + "\n")

	By("create a container with RunAsUser")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				RunAsUser: &uidV,
			},
		},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig), expectedLogMessage
}

// createRunAsUserNameContainer creates the container with specified RunAsUserName in ContainerConfig.
func createRunAsUserNameContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) (string, []byte) {
	By("create RunAsUserName container")
	userName := "nobody"
	expectedLogMessage := []byte(userName + "\n")

	By("create a container with RunAsUserName")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				RunAsUsername: userName,
			},
		},
	}
	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig), expectedLogMessage
}

// createNamespacePodSandbox creates a PodSandbox with different NamespaceOption config for creating containers.
func createNamespacePodSandbox(rc internalapi.RuntimeService, podSandboxNamespace *runtimeapi.NamespaceOption, podSandboxName string, podLogPath string) (string, *runtimeapi.PodSandboxConfig) {
	By("create NamespaceOption podSandbox")
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
				NamespaceOptions: podSandboxNamespace,
			},
		},
		LogDirectory: podLogPath,
	}

	return framework.RunPodSandbox(rc, config), config
}

// createNamespaceContainer creates container with different NamespaceOption config.
func createNamespaceContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, containerName string, image string, containerNamespace *runtimeapi.NamespaceOption, command []string, path string) (string, string, string) {
	By("create NamespaceOption container")
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: image},
		Command:  command,
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				NamespaceOptions: containerNamespace,
			},
		},
		LogPath: path,
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig), containerName, containerConfig.LogPath

}

// createReadOnlyRootfsContainer creates the container with specified ReadOnlyRootfs in ContainerConfig.
func createReadOnlyRootfsContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string, readonly bool) (string, string) {
	By("create ReadOnlyRootfs container")
	containerName := prefix + framework.NewUUID()
	path := fmt.Sprintf("%s.log", containerName)
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"sh", "-c", "touch test.go && [ -f test.go ] && echo 'Found'"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				ReadonlyRootfs: readonly,
			},
		},
		LogPath: path,
	}

	return containerConfig.LogPath, framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
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

// createPrivilegedPodSandbox creates a PodSandbox with Privileged of SecurityContext config.
func createPrivilegedPodSandbox(rc internalapi.RuntimeService, privileged bool) (string, *runtimeapi.PodSandboxConfig) {
	By("create Privileged podSandbox")
	podSandboxName := "create-Privileged-PodSandbox-for-container-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
				Privileged: privileged,
			},
		},
	}

	return framework.RunPodSandbox(rc, config), config
}

// createPrivilegedContainer creates container with specified Privileged in ContainerConfig.
func createPrivilegedContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string, privileged bool) string {
	By("create Privileged container")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"top"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				Privileged: privileged,
			},
		},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// checkNetworkManagement checks the container's network management works fine.
func checkNetworkManagement(rc internalapi.RuntimeService, containerID string, manageable bool) {
	cmd := []string{"ip", "link", "add", "dummy0", "type", "dummy"}

	stdout, stderr, err := rc.ExecSync(containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	msg := fmt.Sprintf("cmd %v, stdout %q, stderr %q", cmd, stdout, stderr)

	if manageable {
		Expect(err).NotTo(HaveOccurred(), msg)
	} else {
		Expect(err).To(HaveOccurred(), msg)
	}
}

// createCapabilityContainer creates container with specified Capability in ContainerConfig.
func createCapabilityContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	By("create Capability container")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"top"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				Capabilities: &runtimeapi.Capability{
					AddCapabilities: []string{"NET_ADMIN"},
				},
			},
		},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

func createAndCheckHostNetwork(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podSandboxName, hostNetworkPort string, hostNetwork bool) (podID string) {
	By(fmt.Sprintf("creating a podSandbox with hostNetwork %v", hostNetwork))
	namespaceOptions := &runtimeapi.NamespaceOption{
		HostPid:     false,
		HostIpc:     false,
		HostNetwork: hostNetwork,
	}
	hostPath, podLogPath := createLogTempDir(podSandboxName)
	podID, podConfig := createNamespacePodSandbox(rc, namespaceOptions, podSandboxName, podLogPath)

	defer os.RemoveAll(hostPath) //clean up the TempDir

	By("create a container in the sandbox")
	command := []string{"sh", "-c", "netstat -ln"}
	containerName := "container-with-HostNetwork-test-" + framework.NewUUID()
	path := fmt.Sprintf("%s.log", containerName)
	containerID, _, logPath := createNamespaceContainer(rc, ic, podID, podConfig, containerName, framework.DefaultContainerImage, namespaceOptions, command, path)

	By("start container")
	startContainer(rc, containerID)

	By("checking host http service port in the container")
	Eventually(func() error {
		log := parseLogLine(podConfig, logPath)
		for _, msg := range log {
			if strings.Contains(string(msg.log), fmt.Sprintf(":%s", hostNetworkPort)) {
				if hostNetwork {
					return nil
				}

				return fmt.Errorf("host port %s should not in container's port list", hostNetworkPort)
			}
		}

		if hostNetwork {
			return fmt.Errorf("host port %s should be in container's port list", hostNetworkPort)
		}
		return nil
	}, time.Minute, time.Second).Should(BeNil())

	return podID
}
