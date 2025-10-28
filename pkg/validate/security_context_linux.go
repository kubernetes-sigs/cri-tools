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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

const (
	nginxContainerImage string = framework.DefaultRegistryE2ETestImagesPrefix + "nginx:1.14-2"
	noNewPrivsImage     string = framework.DefaultRegistryE2ETestImagesPrefix + "nonewprivs:1.3"
	usernsSize          int    = 65536
	usernsHostID        int    = 65536
)

var _ = framework.KubeDescribe("Security Context", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService
	var podID, podLogDir string
	var dirToCleanup []string
	var podConfig *runtimeapi.PodSandboxConfig

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	AfterEach(func() {
		if podID != "" {
			By("stop PodSandbox")
			Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
		}
		if podLogDir != "" {
			os.RemoveAll(podLogDir)
		}
		for _, dir := range dirToCleanup {
			os.RemoveAll(dir)
		}
	})

	Context("NamespaceOption", func() {
		podSandboxName := "NamespaceOption-PodSandbox-" + framework.NewUUID()

		It("runtime should support HostPID", func() {
			By("create podSandbox for security context HostPID")
			namespaceOption := &runtimeapi.NamespaceOption{
				Pid:     runtimeapi.NamespaceMode_NODE,
				Ipc:     runtimeapi.NamespaceMode_POD,
				Network: runtimeapi.NamespaceMode_POD,
			}
			podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podSandboxName, "")

			By("create nginx container")
			prefix := "nginx-container-hostpid-"
			nginxContainerName := prefix + framework.NewUUID()
			containerID, _ := createNamespaceContainer(rc, ic, podID, podConfig, nginxContainerName, nginxContainerImage, namespaceOption, nil, "")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("get nginx container pid")
			command := []string{"sh", "-c", "while [ ! -f /var/run/nginx.pid ]; do sleep 1; done && cat /var/run/nginx.pid"}
			output := execSyncContainer(rc, containerID, command)
			nginxPid := strings.TrimSpace(output)
			framework.Logf("Nginx's pid is %q", nginxPid)

			By("create busybox container with hostPID")
			command = []string{"sh", "-c", "sleep 1000"}
			prefix = "container-with-HostPID-test-"
			containerName := prefix + framework.NewUUID()
			containerID, _ = createNamespaceContainer(rc, ic, podID, podConfig, containerName, framework.TestContext.TestImageList.DefaultTestContainerImage, namespaceOption, command, "")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("should show its pid in the hostPID namespace container")
			cmd := []string{"pidof", "nginx", "||", "true"}
			output = execSyncContainer(rc, containerID, cmd)
			pids := strings.TrimSpace(output)
			framework.Logf("Got nginx's pid %q from pod %q", pids, nginxContainerName)

			if pids == "" {
				framework.Failf("nginx's pid should be seen by hostpid containers")
			}

			if !strings.Contains(pids, nginxPid) {
				framework.Failf("nginx's pid should be seen by hostpid containers")
			}
		})

		testHostIPC := func(mode runtimeapi.NamespaceMode) {
			By("create shared memory segment on the host")
			out, err := exec.Command("ipcmk", "-M", "1048576").Output()
			framework.ExpectNoError(err, "failed to execute ipcmk -M 1048576")
			rawID := strings.TrimSpace(string(out))
			segmentID := strings.TrimPrefix(rawID, "Shared memory id: ")

			By("create podSandbox for security context HostIPC is " + mode.String())
			namespaceOption := &runtimeapi.NamespaceOption{
				Pid:     runtimeapi.NamespaceMode_POD,
				Ipc:     mode,
				Network: runtimeapi.NamespaceMode_POD,
			}
			podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podSandboxName, "")

			By("create a default container with namespace")
			prefix := "namespace-container-"
			containerName := prefix + framework.NewUUID()
			containerID, _ := createNamespaceContainer(rc, ic, podID, podConfig, containerName, framework.TestContext.TestImageList.DefaultTestContainerImage, namespaceOption, pauseCmd, "")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("check if the shared memory segment is (not) included in the container")
			command := []string{"ipcs", "-m"}
			o := execSyncContainer(rc, containerID, command)

			const substr = "The shared memory segment should be included in the container"
			switch mode {
			case runtimeapi.NamespaceMode_NODE:
				Expect(o).To(ContainSubstring(segmentID), substr)
			case runtimeapi.NamespaceMode_POD:
				Expect(o).NotTo(ContainSubstring(segmentID), substr)
			case runtimeapi.NamespaceMode_CONTAINER, runtimeapi.NamespaceMode_TARGET:
				framework.Failf("Unexpected namespace mode: %v", mode)
			}
		}

		It("runtime should support HostIpc is true", func() {
			testHostIPC(runtimeapi.NamespaceMode_NODE)
		})

		It("runtime should support HostIpc is false", func() {
			testHostIPC(runtimeapi.NamespaceMode_POD)
		})

		It("runtime should support PodPID", func() {
			By("create podSandbox for sharing process namespace")
			namespaceOption := &runtimeapi.NamespaceOption{
				Pid: runtimeapi.NamespaceMode_POD,
			}
			framework.Logf("Pid namespace is %q", namespaceOption.GetPid())
			podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podSandboxName, "")

			By("create nginx container")
			prefix := "nginx-container-process-namespace-"
			containerName := prefix + framework.NewUUID()
			containerID, _ := createNamespaceContainer(rc, ic, podID, podConfig, containerName, nginxContainerImage, namespaceOption, nil, "")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("get nginx container pid")
			command := []string{"cat", "/proc/1/cmdline"}
			time.Sleep(time.Second) // waits for nginx to be up-and-running
			o := execSyncContainer(rc, containerID, command)
			Expect(o).ToNot(ContainSubstring("master process"))
		})

		It("runtime should support ContainerPID", func() {
			By("create podSandbox with PID set to container")
			namespaceOption := &runtimeapi.NamespaceOption{
				Pid: runtimeapi.NamespaceMode_CONTAINER,
			}
			framework.Logf("Pid namespace is %q", namespaceOption.GetPid())
			podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podSandboxName, "")

			By("create nginx container")
			prefix := "nginx-container-pid-"
			containerName := prefix + framework.NewUUID()
			containerID, _ := createNamespaceContainer(rc, ic, podID, podConfig, containerName, nginxContainerImage, namespaceOption, nil, "")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("get nginx container pid")
			command := []string{"sh", "-c", `while ! cat /proc/1/cmdline | grep "master process"; do sleep 1; done`}
			execSyncContainer(rc, containerID, command)
		})

		It("runtime should support HostNetwork is true", func() {
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
					_, err = conn.Write([]byte("hello"))
					Expect(err).NotTo(HaveOccurred())
				}
			}()
			defer srv.Close()

			ports := strings.Split(srv.Addr().String(), ":")
			podID, podLogDir = createAndCheckHostNetwork(rc, ic, podSandboxName, ports[len(ports)-1], true)
		})

		It("runtime should support HostNetwork is false", func() {
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
					_, err = conn.Write([]byte("hello"))
					Expect(err).NotTo(HaveOccurred())
				}
			}()
			defer srv.Close()

			ports := strings.Split(srv.Addr().String(), ":")
			podID, podLogDir = createAndCheckHostNetwork(rc, ic, podSandboxName, ports[len(ports)-1], false)
		})
	})

	Context("bucket", func() {
		It("runtime should support SupplementalGroups", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			supplementalGroups := []int64{1234, 5678}
			By("create container for security context SupplementalGroups")
			containerName := "container-with-SupplementalGroups-test-" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  pauseCmd,
				Linux: &runtimeapi.LinuxContainerConfig{
					SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
						SupplementalGroups: supplementalGroups,
					},
				},
			}
			containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("verify SupplementalGroups for container")
			command := []string{"id", "-G"}
			o := execSyncContainer(rc, containerID, command)
			groups := strings.Split(strings.TrimSpace(o), " ")
			Expect(groups).To(ContainElement("1234"))
			Expect(groups).To(ContainElement("5678"))
		})

		It("runtime should support RunAsUser", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container for security context RunAsUser")
			containerID, expectedLogMessage := createRunAsUserContainer(rc, ic, podID, podConfig, "container-with-RunAsUser-test-")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("verify RunAsUser for container")
			command := []string{"id", "-u"}
			verifyExecSyncOutput(rc, containerID, command, expectedLogMessage)
		})

		It("runtime should support RunAsUserName", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container for security context RunAsUser")
			containerID, expectedLogMessage := createRunAsUserNameContainer(rc, ic, podID, podConfig, "container-with-RunAsUserName-test-")

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("verify RunAsUserName for container")
			command := []string{"id", "-nu"}
			verifyExecSyncOutput(rc, containerID, command, expectedLogMessage)
		})

		It("runtime should support RunAsGroup", func() {
			By("create pod")
			podID, podConfig, podLogDir = createPodSandboxWithLogDirectory(rc)

			By("create container for security context RunAsGroup")
			containerName := "container-with-RunAsGroup-test-" + framework.NewUUID()
			containerID, expectedLogMessage := createRunAsGroupContainer(rc, ic, podID, podConfig, containerName)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

			By("verify RunAsGroup for container")
			matchContainerOutput(podConfig, containerName, expectedLogMessage)
		})

		It("runtime should return error if RunAsGroup is set without RunAsUser", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container with invalid RunAsGroup")
			containerName := "container-with-RunAsGroup-without-RunAsUser-test-" + framework.NewUUID()
			createInvalidRunAsGroupContainer(rc, ic, podID, podConfig, containerName)
		})

		It("runtime should support that ReadOnlyRootfs is false", func() {
			By("create pod with log")
			podID, podConfig, podLogDir = createPodSandboxWithLogDirectory(rc)

			By("create container with ReadOnlyRootfs_false")
			readOnlyRootfs := false
			containerID, logPath := createReadOnlyRootfsContainer(rc, ic, podID, podConfig, "container-with-ReadOnlyRootfs-false-test-", readOnlyRootfs)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

			By("Check whether rootfs is writable")
			checkRootfs(podConfig, logPath, readOnlyRootfs)
		})

		It("runtime should support that ReadOnlyRootfs is true", func() {
			By("create pod with log")
			podID, podConfig, podLogDir = createPodSandboxWithLogDirectory(rc)

			By("create container with ReadOnlyRootfs_true")
			readOnlyRootfs := true
			containerID, logPath := createReadOnlyRootfsContainer(rc, ic, podID, podConfig, "container-with-ReadOnlyRootfs-true-test-", readOnlyRootfs)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

			By("Check whether rootfs is read-only")
			checkRootfs(podConfig, logPath, readOnlyRootfs)
		})

		It("runtime should support Privileged is true", func() {
			By("create pod")
			isPrivileged := true
			podID, podConfig = createPrivilegedPodSandbox(rc, isPrivileged)

			By("create container for security context Privileged is true")
			containerID := createPrivilegedContainer(rc, ic, podID, podConfig, "container-with-isPrivileged-test-", isPrivileged)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("check the Privileged container")
			checkNetworkManagement(rc, containerID, isPrivileged)
		})

		It("runtime should support Privileged is false", func() {
			By("create pod")
			notPrivileged := false
			podID, podConfig = createPrivilegedPodSandbox(rc, notPrivileged)

			By("create container for security context Privileged is true")
			containerID := createPrivilegedContainer(rc, ic, podID, podConfig, "container-with-notPrivileged-test-", notPrivileged)

			By("start container")
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			By("check the Privileged container")
			checkNetworkManagement(rc, containerID, notPrivileged)
		})

		It("runtime should support adding capability", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container with security context Capability and test")
			containerID := createCapabilityContainer(rc, ic, podID, podConfig, "container-with-added-capability-test-", []string{"NET_ADMIN"}, nil)

			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			checkNetworkManagement(rc, containerID, true)

			By("create container without security context Capability and test")
			containerID = framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-with-notCapability-test-")

			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			checkNetworkManagement(rc, containerID, false)
		})

		It("runtime should support dropping capability", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container with security context Capability and test")
			containerID := createCapabilityContainer(rc, ic, podID, podConfig, "container-with-dropped-capability-test-", nil, []string{"NET_RAW"})

			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			stdout, stderr, err := rc.ExecSync(
				context.TODO(), containerID, []string{"ping", "127.0.0.1"},
				time.Duration(defaultExecSyncTimeout)*time.Second,
			)
			Expect(err).To(HaveOccurred())
			Expect(string(stdout)).NotTo(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("permission denied"))
		})

		It("runtime should support adding ALL capabilities", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container with security context Capability and test")
			containerID := createCapabilityContainer(rc, ic, podID, podConfig, "container-with-added-all-capability-test-", []string{"ALL"}, nil)

			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			stdout, stderr, err := rc.ExecSync(
				context.TODO(), containerID, []string{"cat", "/proc/self/status"},
				time.Duration(defaultExecSyncTimeout)*time.Second,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(stderr)).To(BeEmpty())
			Expect(string(stdout)).NotTo(MatchRegexp(`CapBnd:\s0000000000000000`))
		})

		It("runtime should support dropping ALL capabilities", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container with security context Capability and test")
			containerID := createCapabilityContainer(rc, ic, podID, podConfig, "container-with-dropped-all-capability-test-", nil, []string{"ALL"})

			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			stdout, stderr, err := rc.ExecSync(
				context.TODO(), containerID, []string{"cat", "/proc/self/status"},
				time.Duration(defaultExecSyncTimeout)*time.Second,
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(stderr)).To(BeEmpty())
			Expect(string(stdout)).To(MatchRegexp(`CapBnd:\s0000000000000000`))
		})

		It("runtime should support MaskedPaths", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container with MaskedPaths")
			containerName := "container-with-maskedpaths" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  pauseCmd,
				Linux: &runtimeapi.LinuxContainerConfig{
					SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
						MaskedPaths: []string{"/bin/ls"},
					},
				},
			}

			containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			cmd := []string{"/bin/sh", "-c", "ls"}
			_, stderr, err := rc.ExecSync(context.TODO(), containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
			Expect(err).To(HaveOccurred())
			Expect(string(stderr)).To(Equal("/bin/sh: ls: Permission denied\n"))
		})

		It("runtime should support ReadonlyPaths", func() {
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container with ReadonlyPaths")
			containerName := "container-with-readonlypaths" + framework.NewUUID()
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
				Command:  pauseCmd,
				Linux: &runtimeapi.LinuxContainerConfig{
					SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
						ReadonlyPaths: []string{"/tmp"},
					},
				},
			}

			containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

			cmd := []string{"touch", "/tmp/test"}
			_, stderr, err := rc.ExecSync(context.TODO(), containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
			Expect(err).To(HaveOccurred())
			Expect(string(stderr)).To(Equal("touch: /tmp/test: Read-only file system\n"))
		})
	})

	Context("SupplementalGroupsPolicy", func() {
		BeforeEach(func(ctx context.Context) {
			By("skip if the runtime does not support SupplementalGroupsPolicy")
			statusResponse, err := rc.Status(ctx, false)
			Expect(err).NotTo(HaveOccurred())
			if statusResponse.GetFeatures() == nil || !statusResponse.GetFeatures().GetSupplementalGroupsPolicy() {
				Skip("The runtime does not support SupplementalGroupsPolicy feature")
			}
		})

		When("SupplementalGroupsPolicy=Merge (Default)", func() {
			It("if the container's primary UID belongs to some groups in the image, runtime should add SupplementalGroups to them", func() {
				By("create pod")
				podID, podConfig, podLogDir = createPodSandboxWithLogDirectory(rc)

				By("create container for security context SupplementalGroups")
				supplementalGroup := int64(1234)
				containerName := "container-with-SupplementalGroupsPolicyMerge-" + framework.NewUUID()
				logPath := containerName + ".log"
				containerConfig := &runtimeapi.ContainerConfig{
					Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
					Image:    &runtimeapi.ImageSpec{Image: testImagePreDefinedGroup},
					Command:  []string{"sh", "-c", "id -G; sleep infinity"},
					Linux: &runtimeapi.LinuxContainerConfig{
						SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
							RunAsUser:          &runtimeapi.Int64Value{Value: imagePredefinedGroupUID},
							SupplementalGroups: []int64{supplementalGroup},
						},
					},
					LogPath: logPath,
				}
				containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)

				By("start container")
				startContainer(rc, containerID)

				Eventually(func(g Gomega) {
					containerStatus := getContainerStatus(rc, containerID)
					g.Expect(containerStatus.GetState()).To(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))
					// In testImagePreDefinedGroup,
					// - its default user is default-user(uid=1000)
					// - default-user belongs to group-defined-in-image(gid=50000) in /etc/group
					// And, SupplementalGroupsPolicy is Merge(default)
					//
					// Thus, firstly attached process identity of the first container processes should be
					// - uid: 1000 (RunAsUser)
					// - gid: 1000 (default group for uid=1000)
					// - supplementary groups
					//   - 1000: self
					//   - 1234: SupplementalGroups
					//   - 50000: groups defined in the container image (/etc/group)
					if containerStatus.GetUser() != nil && containerStatus.GetUser().GetLinux() != nil {
						slices.Sort(containerStatus.GetUser().GetLinux().GetSupplementalGroups())
					}
					g.Expect(containerStatus.GetUser()).To(BeEquivalentTo(&runtimeapi.ContainerUser{
						Linux: &runtimeapi.LinuxContainerUser{
							Uid:                imagePredefinedGroupUID,
							Gid:                imagePredefinedGroupUID,
							SupplementalGroups: []int64{imagePredefinedGroupUID, supplementalGroup, imagePredefinedGroupGID},
						},
					}))
					g.Expect(parseLogLine(podConfig, logPath)).NotTo(BeEmpty())
				}, time.Minute, time.Second*4).Should(Succeed())

				// $ id -G
				// 1000 1234 50000
				expectedOutput := fmt.Sprintf("%d %d %d\n", imagePredefinedGroupUID, supplementalGroup, imagePredefinedGroupGID)

				By("verify groups for the first process of the container")
				verifyLogContents(podConfig, logPath, expectedOutput, stdoutType)

				By("verify groups for 'exec'-ed process of container")
				command := []string{"id", "-G"}
				o := execSyncContainer(rc, containerID, command)
				Expect(o).To(BeEquivalentTo(expectedOutput))
			})
		})
		When("SupplementalGroupsPolicy=Strict", func() {
			It("even if the container's primary UID belongs to some groups in the image, runtime should not add SupplementalGroups to them", func() {
				By("create pod")
				podID, podConfig, podLogDir = createPodSandboxWithLogDirectory(rc)

				By("create container for security context SupplementalGroups")
				supplementalGroup := int64(1234)
				containerName := "container-with-SupplementalGroupsPolicyMerge-" + framework.NewUUID()
				logPath := containerName + ".log"
				containerConfig := &runtimeapi.ContainerConfig{
					Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
					Image:    &runtimeapi.ImageSpec{Image: testImagePreDefinedGroup},
					Command:  []string{"sh", "-c", "id -G; sleep infinity"},
					Linux: &runtimeapi.LinuxContainerConfig{
						SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
							RunAsUser:                &runtimeapi.Int64Value{Value: imagePredefinedGroupUID},
							SupplementalGroups:       []int64{supplementalGroup},
							SupplementalGroupsPolicy: runtimeapi.SupplementalGroupsPolicy_Strict,
						},
					},
					LogPath: logPath,
				}
				containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)

				By("start container")
				startContainer(rc, containerID)

				Eventually(func(g Gomega) {
					containerStatus := getContainerStatus(rc, containerID)
					g.Expect(containerStatus.GetState()).To(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))
					// In testImagePreDefinedGroup,
					// - its default user is default-user(uid=1000)
					// - default-user belongs to group-defined-in-image(gid=50000) in /etc/group
					// And, SupplementalGroupsPolicy is Strict
					//
					// Thus, firstly attached process identity of the first container processes should be
					// (5000(defined in /etc/group) is not appended to supplementary groups)
					// - uid: 1000 (RunAsUser)
					// - gid: 1000 (default group for uid=1000)
					// - supplementary groups
					//   - 1000: self
					//   - 1234: SupplementalGroups
					if containerStatus.GetUser() != nil && containerStatus.GetUser().GetLinux() != nil {
						slices.Sort(containerStatus.GetUser().GetLinux().GetSupplementalGroups())
					}
					g.Expect(containerStatus.GetUser()).To(BeEquivalentTo(&runtimeapi.ContainerUser{
						Linux: &runtimeapi.LinuxContainerUser{
							Uid:                imagePredefinedGroupUID,
							Gid:                imagePredefinedGroupUID,
							SupplementalGroups: []int64{imagePredefinedGroupUID, supplementalGroup},
						},
					}))
					g.Expect(parseLogLine(podConfig, logPath)).NotTo(BeEmpty())
				}, time.Minute, time.Second*4).Should(Succeed())

				// $ id -G
				// 1000 1234
				expectedOutput := fmt.Sprintf("%d %d\n", imagePredefinedGroupUID, supplementalGroup)

				By("verify groups for the first process of the container")
				verifyLogContents(podConfig, logPath, expectedOutput, stdoutType)

				By("verify groups for 'exec'-ed process of container")
				command := []string{"id", "-G"}
				o := execSyncContainer(rc, containerID, command)
				Expect(o).To(BeEquivalentTo(expectedOutput))
			})
		})
	})

	// TODO(random-liu): We should set apparmor to unconfined in seccomp test to prevent
	// them from interfering with each other.
	Context("SeccompProfilePath", func() {
		const (
			// profile which denies sethostname syscall
			seccompBlockHostNameProfile = `{
     "defaultAction": "SCMP_ACT_ALLOW",
     "syscalls": [
         {
             "names": ["sethostname"],
             "action": "SCMP_ACT_ERRNO"
         }
     ]
}`
			// profile which denies chmod syscall
			seccompBlockChmodProfile = `{
     "defaultAction": "SCMP_ACT_ALLOW",
     "syscalls": [
         {
             "names": ["chmod", "fchmodat"],
             "action": "SCMP_ACT_ERRNO"
         }
     ]
}`

			// seccompProcStatusField is the field of /proc/$PID/status referencing the seccomp filter type.
			seccompProcStatusField = "Seccomp:"

			// procSelfStatusPath is the path to /proc/self/status.
			procSelfStatusPath = "/proc/self/status"
		)

		var (
			profileDir, blockHostNameProfilePath, blockchmodProfilePath string
			err                                                         error

			sysAdminCap = []string{"SYS_ADMIN"}

			// seccompProcSelfStatusGrepCommand is the command to grep the seccomp status of the current process.
			seccompProcSelfStatusGrepCommand = []string{"grep", seccompProcStatusField, procSelfStatusPath}
		)

		BeforeEach(func() {
			profileDir, err = createSeccompProfileDir()
			if err != nil {
				Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed creating seccomp profile directory: %v", err))

				return
			}
			dirToCleanup = append(dirToCleanup, profileDir)
			blockHostNameProfilePath, err = createSeccompProfile(seccompBlockHostNameProfile, "block-host-name.json", profileDir)
			if err != nil {
				Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed creating seccomp block hostname profile: %v", err))

				return
			}
			blockchmodProfilePath, err = createSeccompProfile(seccompBlockChmodProfile, "block-chmod.json", profileDir)
			if err != nil {
				Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed creating seccomp block chmod profile: %v", err))

				return
			}
		})

		It("should support seccomp unconfined on the container", func() {
			var containerID string
			seccompProfile := &runtimeapi.SecurityProfile{ProfileType: runtimeapi.SecurityProfile_Unconfined}

			By("create seccomp sandbox and container")
			podID, containerID = seccompTestContainer(rc, ic, seccompProfile)

			By("verify seccomp profile")
			verifySeccomp(rc, containerID, seccompProcSelfStatusGrepCommand, false, "0") // seccomp disabled
		})

		It("should support seccomp localhost profile on the container", func() {
			var containerID string

			By("create seccomp sandbox and container")
			seccompProfile := &runtimeapi.SecurityProfile{
				ProfileType:  runtimeapi.SecurityProfile_Localhost,
				LocalhostRef: blockchmodProfilePath,
			}
			podID, containerID = seccompTestContainer(rc, ic, seccompProfile)

			By("verify seccomp profile")
			verifySeccomp(rc, containerID, []string{"chmod", "400", "/"}, true, "Operation not permitted") // seccomp denied
		})

		It("should support seccomp default on the container", func() {
			var containerID string
			seccompProfile := &runtimeapi.SecurityProfile{}

			By("create seccomp sandbox and container")
			podID, containerID = seccompTestContainer(rc, ic, seccompProfile)

			By("verify seccomp profile")
			verifySeccomp(rc, containerID, seccompProcSelfStatusGrepCommand, false, "2") // seccomp enabled
		})

		It("should support nil profile, which is unconfined", func() {
			var containerID string

			By("create seccomp sandbox and container")
			podID, containerID = seccompTestContainer(rc, ic, nil)

			By("verify seccomp profile")
			verifySeccomp(rc, containerID, seccompProcSelfStatusGrepCommand, false, "0") // seccomp disabled
		})

		// SYS_ADMIN capability allows sethostname, and seccomp is unconfined. sethostname should work.
		It("runtime should not block setting host name with unconfined seccomp and SYS_ADMIN", func() {
			privileged := false
			expectContainerCreateToPass := true
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)
			By("create container with seccompBlockHostNameProfile and test")
			seccompProfile := &runtimeapi.SecurityProfile{ProfileType: runtimeapi.SecurityProfile_Unconfined}
			containerID := createSeccompContainer(rc, ic, podID, podConfig,
				"container-with-block-hostname-seccomp-profile-test-",
				seccompProfile, sysAdminCap, privileged, expectContainerCreateToPass)
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))
			checkSetHostname(rc, containerID, true)
		})

		// SYS_ADMIN capability allows sethostname, but seccomp profile should be able to block it.
		It("runtime should support an seccomp profile that blocks setting hostname with SYS_ADMIN", func() {
			privileged := false
			expectContainerCreateToPass := true
			By("create pod")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)
			By("create container with seccompBlockHostNameProfile and test")
			seccompProfile := &runtimeapi.SecurityProfile{
				ProfileType:  runtimeapi.SecurityProfile_Localhost,
				LocalhostRef: blockHostNameProfilePath,
			}
			containerID := createSeccompContainer(rc, ic, podID, podConfig,
				"container-with-block-hostname-seccomp-profile-test-",
				seccompProfile, sysAdminCap, privileged, expectContainerCreateToPass)
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))
			checkSetHostname(rc, containerID, false)
		})

		It("runtime should ignore a seccomp profile that blocks setting hostname when privileged", func() {
			privileged := true
			expectContainerCreateToPass := true
			By("create privileged pod")
			podID, podConfig = createPrivilegedPodSandbox(rc, true)
			By("create privileged container with seccompBlockHostNameProfile and test")
			seccompProfile := &runtimeapi.SecurityProfile{
				ProfileType:  runtimeapi.SecurityProfile_Localhost,
				LocalhostRef: blockHostNameProfilePath,
			}
			containerID := createSeccompContainer(rc, ic, podID, podConfig,
				"container-with-block-hostname-seccomp-profile-test-",
				seccompProfile, nil, privileged, expectContainerCreateToPass)
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))
			checkSetHostname(rc, containerID, true)
		})
	})

	Context("NoNewPrivs", func() {
		BeforeEach(func() {
			podID, podConfig, podLogDir = createPodSandboxWithLogDirectory(rc)
		})

		createContainerWithNoNewPrivs := func(name string, noNewPrivs bool, uid int64) string {
			By("create container " + name)
			containerConfig := &runtimeapi.ContainerConfig{
				Metadata: framework.BuildContainerMetadata(name, framework.DefaultAttempt),
				Image:    &runtimeapi.ImageSpec{Image: noNewPrivsImage},
				Linux: &runtimeapi.LinuxContainerConfig{
					SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
						NoNewPrivs: noNewPrivs,
						RunAsUser: &runtimeapi.Int64Value{
							Value: uid,
						},
					},
				},
				LogPath: name + ".log",
			}
			containerID := framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)

			// wait container started and check the status.
			startContainer(rc, containerID)
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).GetState()
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

			return containerID
		}
		It("should not allow privilege escalation when true", func() {
			containerName := "alpine-nnp-true-" + framework.NewUUID()
			createContainerWithNoNewPrivs(containerName, true, 1000)
			matchContainerOutput(podConfig, containerName, "Effective uid: 1000\n")
		})

		It("should allow privilege escalation when false", func() {
			containerName := "alpine-nnp-false-" + framework.NewUUID()
			createContainerWithNoNewPrivs(containerName, false, 1000)
			matchContainerOutput(podConfig, containerName, "Effective uid: 0\n")
		})
	})

	Context("UserNamespaces", func() {
		var (
			podName string

			// We call rc.Status() once and save the result in statusResp.
			statusOnce sync.Once
			statusResp *runtimeapi.StatusResponse

			defaultMapping = []*runtimeapi.IDMapping{{
				ContainerId: 0,
				HostId:      1000,
				Length:      100000,
			}}
		)

		BeforeEach(func() {
			podName = "user-namespaces-pod-" + framework.NewUUID()

			// Find a working runtime handler if none provided
			By("searching for runtime handler which supports user namespaces")
			statusOnce.Do(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				// Set verbose to true, other BeforeEach calls need the info field
				// populated.
				// XXX: Do NOT use ":=" here, it breaks the closure reference to
				// statusResp.
				var err error
				statusResp, err = rc.Status(ctx, true)
				framework.ExpectNoError(err, "failed to get runtime config: %v", err)
				_ = statusResp // Avoid unused variable error
			})

			var supportsUserNamespaces bool
			for _, rh := range statusResp.GetRuntimeHandlers() {
				if rh.GetName() == framework.TestContext.RuntimeHandler {
					if rh.GetFeatures().GetUserNamespaces() {
						supportsUserNamespaces = true

						break
					}
				}
			}
			if !supportsUserNamespaces {
				Skip("no runtime handler found which supports user namespaces")
			}
		})

		When("Host idmap mount support is needed", func() {
			BeforeEach(func() {
				pathIDMap := rootfsPath(statusResp.GetInfo())
				if err := supportsIDMap(pathIDMap); err != nil {
					Skip("ID mapping is not supported" + " with path: " + pathIDMap + ": " + err.Error())
				}
			})

			It("runtime should support NamespaceMode_POD", func() {
				namespaceOption := &runtimeapi.NamespaceOption{
					UsernsOptions: &runtimeapi.UserNamespace{
						Mode: runtimeapi.NamespaceMode_POD,
						Uids: defaultMapping,
						Gids: defaultMapping,
					},
				}

				hostLogPath, podLogPath := createLogTempDir(podName)
				defer os.RemoveAll(hostLogPath)
				podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podName, podLogPath)
				containerName := runUserNamespaceContainer(rc, ic, podID, podConfig)

				matchContainerOutputRe(podConfig, containerName, `\s+0\s+1000\s+100000\n`)
			})
		})

		When("Host idmap mount support is not needed", func() {
			It("runtime should support NamespaceMode_NODE", func() {
				namespaceOption := &runtimeapi.NamespaceOption{
					UsernsOptions: &runtimeapi.UserNamespace{
						Mode: runtimeapi.NamespaceMode_NODE,
					},
				}

				hostLogPath, podLogPath := createLogTempDir(podName)
				defer os.RemoveAll(hostLogPath)
				podID, podConfig = createNamespacePodSandbox(rc, namespaceOption, podName, podLogPath)
				containerName := runUserNamespaceContainer(rc, ic, podID, podConfig)

				// If this test is run inside a userns, we need to check the
				// container userns is the same as the one we see outside.
				expectedOutput := hostUsernsContent()
				if expectedOutput == "" {
					Fail("failed to get host userns content")
				}
				// The userns mapping can have several lines, we match each of them.
				for line := range strings.SplitSeq(expectedOutput, "\n") {
					if line == "" {
						continue
					}
					mapping := parseUsernsMappingLine(line)
					if len(mapping) != 3 {
						msg := fmt.Sprintf("slice: %#v, len: %v", mapping, len(mapping))
						Fail("Unexpected host mapping line: " + msg)
					}

					// The container outputs the content of its /proc/self/uid_map.
					// That output should match the regex of the host userns content.
					containerID, hostID, length := mapping[0], mapping[1], mapping[2]
					regex := fmt.Sprintf(`\s+%v\s+%v\s+%v`, containerID, hostID, length)
					matchContainerOutputRe(podConfig, containerName, regex)
				}
			})

			It("runtime should fail if more than one mapping provided", func() {
				wrongMapping := []*runtimeapi.IDMapping{{
					ContainerId: 0,
					HostId:      1000,
					Length:      100000,
				}, {
					ContainerId: 0,
					HostId:      2000,
					Length:      100000,
				}}
				usernsOptions := &runtimeapi.UserNamespace{
					Mode: runtimeapi.NamespaceMode_POD,
					Uids: wrongMapping,
					Gids: wrongMapping,
				}

				runUserNamespacePodWithError(rc, podName, usernsOptions)
				podID = "" // no need to cleanup the pod
			})

			It("runtime should fail if container ID 0 is not mapped", func() {
				mapping := []*runtimeapi.IDMapping{{
					ContainerId: 1,
					HostId:      1000,
					Length:      100000,
				}}
				usernsOptions := &runtimeapi.UserNamespace{
					Mode: runtimeapi.NamespaceMode_POD,
					Uids: mapping,
					Gids: mapping,
				}

				runUserNamespacePodWithError(rc, podName, usernsOptions)
				podID = "" // no need to cleanup the pod
			})

			It("runtime should fail with NamespaceMode_CONTAINER", func() {
				usernsOptions := &runtimeapi.UserNamespace{Mode: runtimeapi.NamespaceMode_CONTAINER}

				runUserNamespacePodWithError(rc, podName, usernsOptions)
				podID = "" // no need to cleanup the pod
			})

			It("runtime should fail with NamespaceMode_TARGET", func() {
				usernsOptions := &runtimeapi.UserNamespace{Mode: runtimeapi.NamespaceMode_TARGET}

				runUserNamespacePodWithError(rc, podName, usernsOptions)
				podID = "" // no need to cleanup the pod
			})
		})
	})
})

// matchContainerOutput matches log line in container logs.
func matchContainerOutput(podConfig *runtimeapi.PodSandboxConfig, name, output string) {
	By("check container output")
	verifyLogContents(podConfig, name+".log", output, stdoutType)
}

// matchContainerOutputRe matches log line in container logs using the provided regular expression pattern.
func matchContainerOutputRe(podConfig *runtimeapi.PodSandboxConfig, name, pattern string) {
	By("check container output")
	verifyLogContentsRe(podConfig, name+".log", pattern, stdoutType)
}

// createRunAsUserContainer creates the container with specified RunAsUser in ContainerConfig.
func createRunAsUserContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) (containerID, expectedLogMessage string) {
	By("create RunAsUser container")

	var uidV runtimeapi.Int64Value

	uidV.Value = 1001
	expectedLogMessage = "1001\n"

	By("create a container with RunAsUser")

	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  pauseCmd,
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				RunAsUser: &uidV,
			},
		},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig), expectedLogMessage
}

// createRunAsUserNameContainer creates the container with specified RunAsUserName in ContainerConfig.
func createRunAsUserNameContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) (containerID, expectedLogMessage string) {
	By("create RunAsUserName container")

	userName := "nobody"
	expectedLogMessage = userName + "\n"

	By("create a container with RunAsUserName")

	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  pauseCmd,
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				RunAsUsername: userName,
			},
		},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig), expectedLogMessage
}

// createRunAsGroupContainer creates the container with specified RunAsGroup in ContainerConfig.
func createRunAsGroupContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, containerName string) (containerID, expectedLogMessage string) {
	By("create RunAsGroup container")

	var uidV, gidV runtimeapi.Int64Value

	uidV.Value = 1001
	gidV.Value = 1002
	expectedLogMessage = "1001:1002\n"

	By("create a container with RunAsUser and RunAsGroup")

	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  []string{"sh", "-c", "echo $(id -u):$(id -g)"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				RunAsUser:  &uidV,
				RunAsGroup: &gidV,
			},
		},
		LogPath: containerName + ".log",
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig), expectedLogMessage
}

// createInvalidRunAsGroupContainer creates the container with specified RunAsGroup without
// RunAsUser specified in ContainerConfig.
func createInvalidRunAsGroupContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, containerName string) {
	By("create invalid RunAsGroup container")

	var gidV runtimeapi.Int64Value

	gidV.Value = 1002

	By("create a container with RunAsGroup without RunAsUser")

	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  []string{"sh", "-c", "echo $(id -u):$(id -g)"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				RunAsGroup: &gidV,
			},
		},
	}
	_, err := framework.CreateContainerWithError(rc, ic, containerConfig, podID, podConfig)
	Expect(err).To(HaveOccurred())
}

// createNamespacePodSandbox creates a PodSandbox with different NamespaceOption config for creating containers.
func createNamespacePodSandbox(rc internalapi.RuntimeService, podSandboxNamespace *runtimeapi.NamespaceOption, podSandboxName, podLogPath string) (string, *runtimeapi.PodSandboxConfig) {
	By("create NamespaceOption podSandbox")

	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata:  framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		DnsConfig: &runtimeapi.DNSConfig{},
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
				NamespaceOptions: podSandboxNamespace,
			},
			CgroupParent: common.GetCgroupParent(context.TODO(), rc),
		},
		LogDirectory: podLogPath,
		Labels:       framework.DefaultPodLabels,
	}

	return framework.RunPodSandbox(rc, config), config
}

// createNamespaceContainer creates container with different NamespaceOption config.
func createNamespaceContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, containerName, image string, containerNamespace *runtimeapi.NamespaceOption, command []string, path string) (containerID, logPath string) {
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

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig), containerConfig.GetLogPath()
}

// createReadOnlyRootfsContainer creates the container with specified ReadOnlyRootfs in ContainerConfig.
func createReadOnlyRootfsContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string, readonly bool) (containerID, logPath string) {
	By("create ReadOnlyRootfs container")

	containerName := prefix + framework.NewUUID()
	path := containerName + ".log"
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  []string{"sh", "-c", "touch test.go && [ -f test.go ] && echo 'Found'"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				ReadonlyRootfs: readonly,
			},
		},
		LogPath: path,
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig), containerConfig.GetLogPath()
}

// checkRootfs checks whether the rootfs parameter of the ContainerConfig is working properly.
func checkRootfs(podConfig *runtimeapi.PodSandboxConfig, logpath string, readOnlyRootfs bool) {
	if readOnlyRootfs {
		failLog := "touch: test.go: Read-only file system"
		expectedLogMessage := failLog + "\n"
		verifyLogContents(podConfig, logpath, expectedLogMessage, stderrType)
	} else {
		successLog := "Found"
		expectedLogMessage := successLog + "\n"
		verifyLogContents(podConfig, logpath, expectedLogMessage, stdoutType)
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
			CgroupParent: common.GetCgroupParent(context.TODO(), rc),
		},
		Labels: framework.DefaultPodLabels,
	}

	return framework.RunPodSandbox(rc, config), config
}

// createPrivilegedContainer creates container with specified Privileged in ContainerConfig.
func createPrivilegedContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string, privileged bool) string {
	By("create Privileged container")

	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  pauseCmd,
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
	cmd := []string{"brctl", "addbr", "foobar"}

	stdout, stderr, err := rc.ExecSync(context.TODO(), containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	msg := fmt.Sprintf("cmd %v, stdout %q, stderr %q", cmd, stdout, stderr)

	if manageable {
		Expect(err).NotTo(HaveOccurred(), msg)
	} else {
		Expect(err).To(HaveOccurred(), msg)
	}
}

// createCapabilityContainer creates container with specified Capability in ContainerConfig.
func createCapabilityContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string, add, drop []string) string {
	By("create Capability container")

	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  pauseCmd,
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				Capabilities: &runtimeapi.Capability{
					AddCapabilities:  add,
					DropCapabilities: drop,
				},
			},
		},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

func createAndCheckHostNetwork(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podSandboxName, hostNetworkPort string, hostNetwork bool) (podID, podLogDir string) {
	By(fmt.Sprintf("creating a podSandbox with hostNetwork %v", hostNetwork))

	netNSMode := runtimeapi.NamespaceMode_POD
	if hostNetwork {
		netNSMode = runtimeapi.NamespaceMode_NODE
	}

	namespaceOptions := &runtimeapi.NamespaceOption{
		Pid:     runtimeapi.NamespaceMode_POD,
		Ipc:     runtimeapi.NamespaceMode_POD,
		Network: netNSMode,
	}
	podLogDir, podLogPath := createLogTempDir(podSandboxName)
	podID, podConfig := createNamespacePodSandbox(rc, namespaceOptions, podSandboxName, podLogPath)

	By("create a container in the sandbox")

	command := []string{"sh", "-c", "netstat -ln"}
	containerName := "container-with-HostNetwork-test-" + framework.NewUUID()
	path := containerName + ".log"
	containerID, logPath := createNamespaceContainer(rc, ic, podID, podConfig, containerName, framework.TestContext.TestImageList.DefaultTestContainerImage, namespaceOptions, command, path)

	By("start container")
	startContainer(rc, containerID)

	By("checking host http service port in the container")
	Eventually(func() error {
		log := parseLogLine(podConfig, logPath)
		for _, msg := range log {
			if strings.Contains(msg.log, ":"+hostNetworkPort) {
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
	}, time.Minute, time.Second).Should(Succeed())

	return podID, podLogDir
}

// createSeccompProfileDir creates a seccomp test profile directory.
func createSeccompProfileDir() (string, error) {
	hostPath, err := os.MkdirTemp("", "seccomp-tests")
	if err != nil {
		return "", fmt.Errorf("create tempdir %q: %w", hostPath, err)
	}

	return hostPath, nil
}

// createSeccompProfile creates a seccomp test profile with profileContents.
func createSeccompProfile(profileContents, profileName, hostPath string) (string, error) {
	profilePath := filepath.Join(hostPath, profileName)
	if err := os.WriteFile(profilePath, []byte(profileContents), 0o644); err != nil {
		return "", fmt.Errorf("create %s: %w", profilePath, err)
	}

	return profilePath, nil
}

// seccompTestContainer creates and starts a seccomp sandbox and a container.
func seccompTestContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, profile *runtimeapi.SecurityProfile) (podID, containerID string) {
	By("create seccomp sandbox")

	podSandboxName := "seccomp-sandbox-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	podConfig := &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
				Seccomp: profile,
			},
			CgroupParent: common.GetCgroupParent(context.TODO(), rc),
		},
		Labels: framework.DefaultPodLabels,
	}
	podID = framework.RunPodSandbox(rc, podConfig)

	By("create container")

	containerNamePrefix := "seccomp-container-" + framework.NewUUID()
	containerName := containerNamePrefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.TestContext.TestImageList.DefaultTestContainerImage},
		Command:  pauseCmd,
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				Seccomp: profile,
			},
		},
	}
	containerID = framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)

	By("start container")
	startContainer(rc, containerID)
	Eventually(func() runtimeapi.ContainerState {
		return getContainerStatus(rc, containerID).GetState()
	}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))

	return podID, containerID
}

func verifySeccomp(rc internalapi.RuntimeService, containerID string, command []string, expectError bool, output string) {
	stdout, stderr, err := rc.ExecSync(context.TODO(), containerID, command, time.Duration(defaultExecSyncTimeout)*time.Second)
	msg := fmt.Sprintf("cmd %v, stdout %q, stderr %q, with err: %v", command, stdout, stderr, err)

	if expectError {
		Expect(err).To(HaveOccurred(), msg)
		Expect(string(stderr)).To(ContainSubstring(output))
	} else {
		Expect(err).NotTo(HaveOccurred(), msg)
		Expect(string(stdout)).To(ContainSubstring(output))
	}
}

// createSeccompContainer creates container with the specified seccomp profile.
func createSeccompContainer(rc internalapi.RuntimeService,
	ic internalapi.ImageManagerService,
	podID string,
	podConfig *runtimeapi.PodSandboxConfig,
	prefix string,
	profile *runtimeapi.SecurityProfile,
	caps []string,
	privileged bool,
	expectContainerCreateToPass bool,
) string {
	By("create " + profile.GetProfileType().String() + " Seccomp container")

	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image: &runtimeapi.ImageSpec{
			Image:              framework.TestContext.TestImageList.DefaultTestContainerImage,
			UserSpecifiedImage: framework.TestContext.TestImageList.DefaultTestContainerImage,
		},
		Command: []string{"sleep", "60"},
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				Privileged: privileged,
				Capabilities: &runtimeapi.Capability{
					AddCapabilities: caps,
				},
				Seccomp: profile,
			},
		},
	}

	return createContainerWithExpectation(rc, ic, containerConfig, podID, podConfig, expectContainerCreateToPass)
}

// createContainerWithExpectation creates a container with the prefix of containerName
// and expectation of failure or success, depending on parameter, in the create step.
func createContainerWithExpectation(rc internalapi.RuntimeService,
	ic internalapi.ImageManagerService,
	config *runtimeapi.ContainerConfig,
	podID string,
	podConfig *runtimeapi.PodSandboxConfig,
	expectContainerCreateToPass bool,
) string {
	// Pull the image if it does not exist. (don't fail for inability to pull image)
	imageName := config.GetImage().GetImage()
	if !strings.Contains(imageName, ":") {
		imageName += ":latest"
	}

	status := framework.ImageStatus(ic, imageName)
	if status == nil {
		framework.PullPublicImage(ic, imageName, nil)
	}

	By("Create container.")

	containerID, err := rc.CreateContainer(context.TODO(), podID, config, podConfig)

	if !expectContainerCreateToPass {
		msg := fmt.Sprintf("create should fail with err %v", err)
		Expect(err).To(HaveOccurred(), msg)
	} else {
		framework.ExpectNoError(err, "failed to create container: %v", err)
		framework.Logf("Created container %q\n", containerID)
	}

	return containerID
}

// checkSetHostname checks if the hostname can be set in the container.
func checkSetHostname(rc internalapi.RuntimeService, containerID string, settable bool) {
	By("set hostname in container to determine whether sethostname is blocked")

	cmd := []string{"hostname", "ANewHostName"}
	stdout, stderr, err := rc.ExecSync(context.TODO(), containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	msg := fmt.Sprintf("cmd %v, stdout %q, stderr %q", cmd, stdout, stderr)

	if settable {
		Expect(err).NotTo(HaveOccurred(), msg)
	} else {
		Expect(err).To(HaveOccurred(), msg)
	}
}

func runUserNamespaceContainer(
	rc internalapi.RuntimeService,
	ic internalapi.ImageManagerService,
	podID string,
	podConfig *runtimeapi.PodSandboxConfig,
) string {
	By("create user namespaces container")

	containerName := "user-namespaces-container-" + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image: &runtimeapi.ImageSpec{
			Image:              framework.TestContext.TestImageList.DefaultTestContainerImage,
			UserSpecifiedImage: framework.TestContext.TestImageList.DefaultTestContainerImage,
		},
		Command: []string{"cat", "/proc/self/uid_map"},
		LogPath: containerName + ".log",
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				NamespaceOptions: podConfig.GetLinux().GetSecurityContext().GetNamespaceOptions(),
			},
		},
	}

	containerID := createContainerWithExpectation(rc, ic, containerConfig, podID, podConfig, true)
	startContainer(rc, containerID)

	Eventually(func() runtimeapi.ContainerState {
		return getContainerStatus(rc, containerID).GetState()
	}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

	return containerName
}

func runUserNamespacePodWithError(
	rc internalapi.RuntimeService,
	podName string,
	usernsOptions *runtimeapi.UserNamespace,
) {
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(podName, uid, namespace, framework.DefaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
				NamespaceOptions: &runtimeapi.NamespaceOption{
					UsernsOptions: usernsOptions,
				},
			},
		},
		Labels: framework.DefaultPodLabels,
	}

	framework.RunPodSandboxError(rc, config)
}

func supportsIDMap(path string) error {
	treeFD, err := unix.OpenTree(-1, path, uint(unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC))
	if err != nil {
		return err
	}
	defer unix.Close(treeFD)

	// We want to test if idmap mounts are supported.
	// So we use just some random mapping, it doesn't really matter which one.
	// For the helper command, we just need something that is alive while we
	// test this, a sleep 5 will do it.
	cmd := exec.CommandContext(context.TODO(), "sleep", "5")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: usernsHostID, Size: usernsSize}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: usernsHostID, Size: usernsSize}},
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	usernsPath := fmt.Sprintf("/proc/%d/ns/user", cmd.Process.Pid)

	var usernsFile *os.File

	if usernsFile, err = os.Open(usernsPath); err != nil {
		return err
	}

	defer usernsFile.Close()

	attr := unix.MountAttr{
		Attr_set:  unix.MOUNT_ATTR_IDMAP,
		Userns_fd: uint64(usernsFile.Fd()),
	}
	if err := unix.MountSetattr(treeFD, "", unix.AT_EMPTY_PATH, &attr); err != nil {
		return err
	}

	return nil
}

// rootfsPath returns the parent path used for containerd stateDir (the container rootfs lives
// inside there). If the object can't be parsed, it returns the "/var/lib".
// Usually the rootfs is inside /var/lib and it's the same filesystem. In the end, to see if a path
// supports idmap, we only care about its fs so this is a good fallback.
func rootfsPath(info map[string]string) string {
	defaultPath := "/var/lib"
	jsonCfg, ok := info["config"]

	if !ok {
		return defaultPath
	}

	// Get only the StateDir from the json.
	type containerdConfig struct {
		StateDir string `json:"stateDir"`
	}

	cfg := containerdConfig{}
	if err := json.Unmarshal([]byte(jsonCfg), &cfg); err != nil {
		return defaultPath
	}

	if cfg.StateDir == "" {
		return defaultPath
	}

	// The stateDir might have not been created yet. Let's use the parent directory that should
	// always exist.
	return filepath.Dir(cfg.StateDir)
}

func hostUsernsContent() string {
	uidMapPath := "/proc/self/uid_map"

	uidMapContent, err := os.ReadFile(uidMapPath)
	if err != nil {
		return ""
	}

	return string(uidMapContent)
}

func parseUsernsMappingLine(line string) []string {
	// The line format is:
	// <container-id> <host-id> <length>
	// But there could be a lot of spaces between the fields.
	line = strings.TrimSpace(line)
	m := strings.Split(line, " ")
	m = slices.DeleteFunc(m, func(s string) bool {
		return s == ""
	})

	return m
}
