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
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/jsonlog"
	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	"golang.org/x/sys/unix"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// streamType is the type of the stream.
type streamType string

const (
	defaultStopContainerTimeout int64      = 60
	defaultExecSyncTimeout      int64      = 5
	defaultLog                  string     = "hello World"
	stdoutType                  streamType = "stdout"
	stderrType                  streamType = "stderr"
)

// logMessage is the internal log type.
type logMessage struct {
	timestamp time.Time
	stream    streamType
	log       []byte
}

var _ = framework.KubeDescribe("Container", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support basic operations on container", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support creating container [Conformance]", func() {
			By("test create a default container")
			containerID := testCreateDefaultContainer(rc, ic, podID, podConfig)

			By("test list container")
			containers := listContainerForID(rc, containerID)
			Expect(containerFound(containers, containerID)).To(BeTrue(), "Container should be created")
		})

		It("runtime should support starting container [Conformance]", func() {
			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-start-test-")

			By("test start container")
			testStartContainer(rc, containerID)
		})

		It("runtime should support stopping container [Conformance]", func() {
			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-stop-test-")

			By("start container")
			startContainer(rc, containerID)

			By("test stop container")
			testStopContainer(rc, containerID)
		})

		It("runtime should support removing container [Conformance]", func() {
			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-remove-test-")

			By("test remove container")
			removeContainer(rc, containerID)
			containers := listContainerForID(rc, containerID)
			Expect(containerFound(containers, containerID)).To(BeFalse(), "Container should be removed")
		})

		It("runtime should support execSync [Conformance]", func() {
			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-execSync-test-")

			By("start container")
			startContainer(rc, containerID)

			By("test execSync")
			cmd := []string{"echo", "hello"}
			expectedLogMessage := []byte("hello" + "\n")
			verifyExecSyncOutput(rc, containerID, cmd, expectedLogMessage)
		})
	})

	Context("runtime should support adding volume and device [Conformance]", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support starting container with volume [Conformance]", func() {
			By("create host path and flag file")
			hostPath, _ := createHostPath(podID)

			defer os.RemoveAll(hostPath) // clean up the TempDir

			By("create container with volume")
			containerID := createVolumeContainer(rc, ic, "container-with-volume-test-", podID, podConfig, hostPath)

			By("test start container with volume")
			testStartContainer(rc, containerID)

			By("check whether 'hostPath' contains file or dir in container")
			command := []string{"ls", "-A", hostPath}
			output := execSyncContainer(rc, containerID, command)
			Expect(len(output)).NotTo(BeZero(), "len(output) should not be zero.")
		})

		It("runtime should support starting container with volume when host path doesn't exist", func() {
			// TODO: revisit this after https://github.com/kubernetes/kubernetes/issues/52318.
			Skip("Skip the test since the behavior is not decided yet in CRI")
			By("get a host path that doesn't exist")
			hostPath, _ := createHostPath(podID)
			os.RemoveAll(hostPath) // make sure the host path doesn't exist

			defer os.RemoveAll(hostPath) // clean up the TempDir

			By("create container with volume")
			containerID := createVolumeContainer(rc, ic, "container-with-volume-test-", podID, podConfig, hostPath)

			By("test start container with volume")
			testStartContainer(rc, containerID)

			By("check whether 'hostPath' does exist or not in host")
			Eventually(func() bool {
				return pathExists(hostPath)
			}, time.Minute, time.Second*4).Should(Equal(true))
		})
	})

	Context("runtime should support log", func() {
		var podID, hostPath string
		var podConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig, hostPath = createPodSandboxWithLogDirectory(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
			By("clean up the TempDir")
			os.RemoveAll(hostPath)
		})

		It("runtime should support starting container with log [Conformance]", func() {
			By("create container with log")
			logPath, containerID := createLogContainer(rc, ic, "container-with-log-test-", podID, podConfig)

			By("start container with log")
			startContainer(rc, containerID)
			// wait container exited and check the status.
			Eventually(func() runtimeapi.ContainerState {
				return getContainerStatus(rc, containerID).State
			}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))

			By("check the log context")
			expectedLogMessage := &logMessage{
				log:    []byte(defaultLog + "\n"),
				stream: stdoutType,
			}
			verifyLogContents(podConfig, logPath, expectedLogMessage)
		})
	})

	Context("runtime should support mount propagation", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		BeforeEach(func() {
			podID, podConfig = createPrivilegedPodSandbox(rc, true)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("mount with 'rprivate' should not support propagation", func() {
			By("create host path and flag file")
			mntSource, propagationSrcDir, propagationMntPoint, clearHostPath := createHostPathForMountPropagation(podID, runtimeapi.MountPropagation_PROPAGATION_PRIVATE)
			defer clearHostPath() // clean up the TempDir

			By("create container with volume")
			containerID := createMountPropagationContainer(rc, ic, "mount-propagation-test-", podID, podConfig, mntSource, runtimeapi.MountPropagation_PROPAGATION_PRIVATE)

			By("test start container with volume")
			testStartContainer(rc, containerID)

			By("create a propatation mount point in host")
			createPropagationMountPoint(propagationSrcDir, propagationMntPoint)

			By("check whether propagationMntPoint contains file or dir in container")
			command := []string{"ls", "-A", propagationMntPoint}
			output := execSyncContainer(rc, containerID, command)
			Expect(len(output)).To(BeZero(), "len(output) should be zero.")

			By("create a directory named containerMntPoint as a mount point in container")
			containerMntPoint := path.Join(mntSource, "containerMntPoint")
			command = []string{"sh", "-c", "mkdir -p " + containerMntPoint}
			execSyncContainer(rc, containerID, command)

			By("mount /etc to the mount point in container")
			command = []string{"sh", "-c", "mount --bind /etc " + containerMntPoint}
			execSyncContainer(rc, containerID, command)

			By("check whether containerMntPoint contains file or dir in host")
			fileInfo, err := ioutil.ReadDir(containerMntPoint)
			framework.ExpectNoError(err, "failed to ReadDir %q in Host", containerMntPoint)
			Expect(len(fileInfo)).To(BeZero(), "len(fileInfo) should be zero.")
		})

		It("mount with 'rshared' should support propagation from host to container and vice versa", func() {
			By("create host path and flag file")
			mntSource, propagationSrcDir, propagationMntPoint, clearHostPath := createHostPathForMountPropagation(podID, runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL)
			defer clearHostPath() // clean up the TempDir

			By("create container with volume")
			containerID := createMountPropagationContainer(rc, ic, "mount-propagation-test-", podID, podConfig, mntSource, runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL)

			By("test start container with volume")
			testStartContainer(rc, containerID)

			By("create a propatation mount point in host")
			createPropagationMountPoint(propagationSrcDir, propagationMntPoint)

			By("check whether propagationMntPoint contains file or dir in container")
			command := []string{"ls", "-A", propagationMntPoint}
			output := execSyncContainer(rc, containerID, command)
			Expect(len(output)).NotTo(BeZero(), "len(output) should not be zero.")

			By("create a directory named containerMntPoint as a mount point in container")
			containerMntPoint := path.Join(mntSource, "containerMntPoint")
			command = []string{"sh", "-c", "mkdir -p " + containerMntPoint}
			execSyncContainer(rc, containerID, command)

			By("mount /etc to the mount point in container")
			command = []string{"sh", "-c", "mount --bind /etc " + containerMntPoint}
			execSyncContainer(rc, containerID, command)

			By("check whether containerMntPoint contains file or dir in host")
			fileInfo, err := ioutil.ReadDir(containerMntPoint)
			framework.ExpectNoError(err, "failed to ReadDir %q in Host", containerMntPoint)
			Expect(len(fileInfo)).NotTo(BeZero(), "len(fileInfo) should not be zero.")
		})

		It("mount with 'rslave' should support propagation from host to container", func() {
			By("create host path and flag file")
			mntSource, propagationSrcDir, propagationMntPoint, clearHostPath := createHostPathForMountPropagation(podID, runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER)
			defer clearHostPath() // clean up the TempDir

			By("create container with volume")
			containerID := createMountPropagationContainer(rc, ic, "mount-propagation-test-", podID, podConfig, mntSource, runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER)

			By("test start container with volume")
			testStartContainer(rc, containerID)

			By("create a propatation mount point in host")
			createPropagationMountPoint(propagationSrcDir, propagationMntPoint)

			By("check whether propagationMntPoint contains file or dir in container")
			command := []string{"ls", "-A", propagationMntPoint}
			output := execSyncContainer(rc, containerID, command)
			Expect(len(output)).NotTo(BeZero(), "len(output) should not be zero.")

			By("create a directory named containerMntPoint as a mount point in container")
			containerMntPoint := path.Join(mntSource, "containerMntPoint")
			command = []string{"sh", "-c", "mkdir -p " + containerMntPoint}
			execSyncContainer(rc, containerID, command)

			By("mount /etc to the mount point in container")
			command = []string{"sh", "-c", "mount --bind /etc " + containerMntPoint}
			execSyncContainer(rc, containerID, command)

			By("check whether containerMntPoint contains file or dir in host")
			fileInfo, err := ioutil.ReadDir(containerMntPoint)
			framework.ExpectNoError(err, "failed to ReadDir %q in Host", containerMntPoint)
			Expect(len(fileInfo)).To(BeZero(), "len(fileInfo) should be zero.")
		})
	})
})

// containerFound returns whether containers is found.
func containerFound(containers []*runtimeapi.Container, containerID string) bool {
	for _, container := range containers {
		if container.Id == containerID {
			return true
		}
	}
	return false
}

// getContainerStatus gets ContainerState for containerID and fails if it gets error.
func getContainerStatus(c internalapi.RuntimeService, containerID string) *runtimeapi.ContainerStatus {
	By("Get container status for containerID: " + containerID)
	status, err := c.ContainerStatus(containerID)
	framework.ExpectNoError(err, "failed to get container %q status: %v", containerID, err)
	return status
}

// createShellContainer creates a container to run /bin/sh.
func createShellContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"/bin/sh"},
		Linux:    &runtimeapi.LinuxContainerConfig{},
		Stdin:    true,
		Tty:      false,
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// testCreateDefaultContainer creates a container in the pod which ID is podID and make sure it's ready.
func testCreateDefaultContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-create-test-")
	Eventually(func() runtimeapi.ContainerState {
		return getContainerStatus(rc, containerID).State
	}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_CREATED))
	return containerID
}

// startContainer start the container for containerID.
func startContainer(c internalapi.RuntimeService, containerID string) {
	By("Start container for containerID: " + containerID)
	err := c.StartContainer(containerID)
	framework.ExpectNoError(err, "failed to start container: %v", err)
	framework.Logf("Started container %q\n", containerID)
}

// testStartContainer starts the container for containerID and make sure it's running.
func testStartContainer(rc internalapi.RuntimeService, containerID string) {
	startContainer(rc, containerID)
	Eventually(func() runtimeapi.ContainerState {
		return getContainerStatus(rc, containerID).State
	}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_RUNNING))
}

// stopContainer stops the container for containerID.
func stopContainer(c internalapi.RuntimeService, containerID string, timeout int64) {
	By("Stop container for containerID: " + containerID)
	stopped := make(chan bool, 1)

	go func() {
		defer GinkgoRecover()
		err := c.StopContainer(containerID, timeout)
		framework.ExpectNoError(err, "failed to stop container: %v", err)
		stopped <- true
	}()

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		framework.Failf("stop container %q timeout.\n", containerID)
	case <-stopped:
		framework.Logf("Stopped container %q\n", containerID)
	}
}

// testStopContainer stops the container for containerID and make sure it's exited.
func testStopContainer(c internalapi.RuntimeService, containerID string) {
	stopContainer(c, containerID, defaultStopContainerTimeout)
	Eventually(func() runtimeapi.ContainerState {
		return getContainerStatus(c, containerID).State
	}, time.Minute, time.Second*4).Should(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))
}

// removeContainer removes the container for containerID.
func removeContainer(c internalapi.RuntimeService, containerID string) {
	By("Remove container for containerID: " + containerID)
	err := c.RemoveContainer(containerID)
	framework.ExpectNoError(err, "failed to remove container: %v", err)
	framework.Logf("Removed container %q\n", containerID)
}

// listContainerForID lists container for containerID.
func listContainerForID(c internalapi.RuntimeService, containerID string) []*runtimeapi.Container {
	By("List containers for containerID: " + containerID)
	filter := &runtimeapi.ContainerFilter{
		Id: containerID,
	}
	containers, err := c.ListContainers(filter)
	framework.ExpectNoError(err, "failed to list containers %q status: %v", containerID, err)
	return containers
}

// execSyncContainer test execSync for containerID and make sure the response is right.
func execSyncContainer(c internalapi.RuntimeService, containerID string, command []string) []byte {
	By("execSync for containerID: " + containerID)
	stdout, stderr, err := c.ExecSync(containerID, command, time.Duration(defaultExecSyncTimeout)*time.Second)
	framework.ExpectNoError(err, "failed to execSync in container %q", containerID)
	Expect(stderr).To(BeNil(), "The stderr should be nil.")
	framework.Logf("Execsync succeed")

	return stdout
}

// execSyncContainer test execSync for containerID and make sure the response is right.
func verifyExecSyncOutput(c internalapi.RuntimeService, containerID string, command []string, expectedLogMessage []byte) {
	By("verify execSync output")
	stdout := execSyncContainer(c, containerID, command)
	Expect(string(stdout)).To(Equal(string(expectedLogMessage)), "The stdout output of execSync should be %s", string(expectedLogMessage))
	framework.Logf("verfiy Execsync output succeed")
}

// createHostPath creates the hostPath and flagFile for volume.
func createHostPath(podID string) (string, string) {
	hostPath, err := ioutil.TempDir("", "/test"+podID)
	framework.ExpectNoError(err, "failed to create TempDir %q: %v", hostPath, err)

	flagFile := "testVolume.file"
	_, err = os.Create(filepath.Join(hostPath, flagFile))
	framework.ExpectNoError(err, "failed to create volume file %q: %v", flagFile, err)

	return hostPath, flagFile
}

// createVolumeContainer creates a container with volume and the prefix of containerName and fails if it gets error.
func createVolumeContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig, hostPath string) string {
	By("create a container with volume and name")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
		// mount host path to the same directory in container, and will check if hostPath isn't empty
		Mounts: []*runtimeapi.Mount{
			{
				HostPath:      hostPath,
				ContainerPath: hostPath,
			},
		},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// createLogContainer creates a container with log and the prefix of containerName.
func createLogContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig) (string, string) {
	By("create a container with log and name")
	containerName := prefix + framework.NewUUID()
	path := fmt.Sprintf("%s.log", containerName)
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"echo", defaultLog},
		LogPath:  path,
	}
	return containerConfig.LogPath, framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// pathExists check whether 'path' does exist or not
func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	framework.ExpectNoError(err, "failed to check whether %q Exists: %v", path, err)
	return false
}

// parseDockerJSONLog parses logs in Docker JSON log format.
// Docker JSON log format example:
//   {"log":"content 1","stream":"stdout","time":"2016-10-20T18:39:20.57606443Z"}
//   {"log":"content 2","stream":"stderr","time":"2016-10-20T18:39:20.57606444Z"}
func parseDockerJSONLog(log []byte, msg *logMessage) {
	var l jsonlog.JSONLog

	err := json.Unmarshal(log, &l)
	framework.ExpectNoError(err, "failed with %v to unmarshal log %q", err, l)

	msg.timestamp = l.Created
	msg.stream = streamType(l.Stream)
	msg.log = []byte(l.Log)
}

// parseCRILog parses logs in CRI log format.
// CRI log format example :
//   2016-10-06T00:17:09.669794202Z stdout P The content of the log entry 1
//   2016-10-06T00:17:10.113242941Z stderr F The content of the log entry 2
func parseCRILog(log string, msg *logMessage) {
	timeStamp, err := time.Parse(time.RFC3339Nano, strings.Fields(log)[0])
	framework.ExpectNoError(err, "failed to parse timeStamp: %v", err)
	stream := strings.Fields(log)[1]
	// Skip the tag field.
	logMessage := strings.Fields(log)[3:]

	msg.timestamp = timeStamp
	msg.stream = streamType(stream)
	msg.log = []byte(strings.Join(logMessage, " ") + "\n")
}

// parseLogLine parses log by row.
func parseLogLine(podConfig *runtimeapi.PodSandboxConfig, logPath string) []logMessage {
	path := filepath.Join(podConfig.LogDirectory, logPath)
	f, err := os.Open(path)
	framework.ExpectNoError(err, "failed to open log file: %v", err)
	framework.Logf("Open log file %s", path)
	defer f.Close()

	var msg logMessage
	var msgLog []logMessage

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// to determine whether the log is Docker format or CRI format.
		if strings.HasPrefix(line, "{") {
			parseDockerJSONLog([]byte(line), &msg)
		} else {
			parseCRILog(line, &msg)
		}

		msgLog = append(msgLog, msg)
	}

	if err := scanner.Err(); err != nil {
		framework.ExpectNoError(err, "failed to read log by row: %v", err)
	}
	framework.Logf("Parse container log succeed")

	return msgLog
}

// verifyLogContents verifies the contents of container log.
func verifyLogContents(podConfig *runtimeapi.PodSandboxConfig, logPath string, expectedLogMessage *logMessage) {
	By("verify log contents")
	log := parseLogLine(podConfig, logPath)

	for _, msg := range log {
		Expect(string(msg.log)).To(Equal(string(expectedLogMessage.log)), "Log should be %s", string(expectedLogMessage.log))
		Expect(string(msg.stream)).To(Equal(string(expectedLogMessage.stream)), "Stream should be %s", string(expectedLogMessage.stream))
	}
}

// createHostPath creates the hostPath for mount propagation test.
func createHostPathForMountPropagation(podID string, propagationOpt runtimeapi.MountPropagation) (string, string, string, func()) {
	hostPath, err := ioutil.TempDir("", "/test"+podID)
	framework.ExpectNoError(err, "failed to create TempDir %q: %v", hostPath, err)

	mntSource := filepath.Join(hostPath, "mnt")
	propagationMntPoint := filepath.Join(mntSource, "propagationMnt")
	err = os.MkdirAll(propagationMntPoint, 0700)
	framework.ExpectNoError(err, "failed to create volume dir %q: %v", propagationMntPoint, err)

	propagationSrcDir := filepath.Join(hostPath, "propagationSrcDir")
	err = os.MkdirAll(propagationSrcDir, 0700)
	framework.ExpectNoError(err, "failed to create volume dir %q: %v", propagationSrcDir, err)

	_, err = os.Create(filepath.Join(propagationSrcDir, "flagFile"))
	framework.ExpectNoError(err, "failed to create volume file \"flagFile\": %v", err)

	switch propagationOpt {
	case runtimeapi.MountPropagation_PROPAGATION_PRIVATE:
		err := unix.Mount(mntSource, mntSource, "bind", unix.MS_BIND|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to mount \"mntSource\": %v", err)
		err = unix.Mount("", mntSource, "", unix.MS_PRIVATE|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to set \"mntSource\" to \"rprivate\": %v", err)
	case runtimeapi.MountPropagation_PROPAGATION_HOST_TO_CONTAINER,
		runtimeapi.MountPropagation_PROPAGATION_BIDIRECTIONAL:
		err := unix.Mount(mntSource, mntSource, "bind", unix.MS_BIND|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to mount \"mntSource\": %v", err)
		err = unix.Mount("", mntSource, "", unix.MS_SHARED|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to set \"mntSource\" to \"rprivate\": %v", err)
	default:
		err := unix.Mount(mntSource, mntSource, "bind", unix.MS_BIND|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to mount \"mntSource\": %v", err)
		err = unix.Mount("", mntSource, "", unix.MS_PRIVATE|unix.MS_REC, "")
		framework.ExpectNoError(err, "failed to set \"mntSource\" to \"rprivate\": %v", err)
	}

	clearHostPath := func() {
		By("clean up the TempDir")
		err := unix.Unmount(propagationMntPoint, unix.MNT_DETACH)
		framework.ExpectNoError(err, "failed to unmount \"propagationMntPoint\": %v", err)
		err = unix.Unmount(mntSource, unix.MNT_DETACH)
		framework.ExpectNoError(err, "failed to unmount \"mntSource\": %v", err)

		os.RemoveAll(hostPath)
		framework.ExpectNoError(err, "failed to remove \"hostPath\": %v", err)
	}

	return mntSource, propagationSrcDir, propagationMntPoint, clearHostPath
}

// createMountPropagationContainer creates a container with volume and Privileged and fails if it gets error.
func createMountPropagationContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, prefix string, podID string,
	podConfig *runtimeapi.PodSandboxConfig, hostPath string, PropagationOpt runtimeapi.MountPropagation) string {
	By("create a container with volume and name")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: framework.DefaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
		// Set Privileged in order to executing mount command in container
		Linux: &runtimeapi.LinuxContainerConfig{
			SecurityContext: &runtimeapi.LinuxContainerSecurityContext{
				Privileged: true,
			},
		},
		Mounts: []*runtimeapi.Mount{
			{
				HostPath:      hostPath,
				ContainerPath: hostPath,
				Propagation:   PropagationOpt,
			},
		},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// createPropagationMountPoint mount "propagationSrcDir" at "propagationMntPoint",
// this will be used to check whether mount can be propagated from host to container or not.
func createPropagationMountPoint(propagationSrcDir, propagationMntPoint string) {
	err := unix.Mount(propagationSrcDir, propagationMntPoint, "bind", unix.MS_BIND|unix.MS_REC, "")
	framework.ExpectNoError(err, "failed to mount \"propagationMntPoint\": %v", err)
}
