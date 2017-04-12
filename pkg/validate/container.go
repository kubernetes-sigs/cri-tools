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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/jsonlog"
	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// streamType is the type of the stream.
type streamType string

const (
	defaultContainerImage       string        = "gcr.io/google_containers/busybox:1.24"
	defaultStopContainerTimeout int64         = 60
	defaultExecSyncTimeout      int64         = 5
	defaultLog                  string        = "hello World"
	stdoutType                  streamType    = "stdout"
	stderrType                  streamType    = "stderr"
	pollTIMEOUT                 time.Duration = time.Minute
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
			podID, podConfig = createPodSandboxForContainer(rc)
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
			containerID := createDefaultContainer(rc, ic, podID, podConfig, "container-for-start-test-")

			By("test start container")
			testStartContainer(rc, containerID)
		})

		It("runtime should support stopping container [Conformance]", func() {
			By("create container")
			containerID := createDefaultContainer(rc, ic, podID, podConfig, "container-for-stop-test-")

			By("start container")
			startContainer(rc, containerID)

			By("test stop container")
			testStopContainer(rc, containerID)
		})

		It("runtime should support removing container [Conformance]", func() {
			By("create container")
			containerID := createDefaultContainer(rc, ic, podID, podConfig, "container-for-remove-test-")

			By("test remove container")
			removeContainer(rc, containerID)
			containers := listContainerForID(rc, containerID)
			Expect(containerFound(containers, containerID)).To(BeFalse(), "Container should be removed")
		})

		It("runtime should support execSync [Conformance]", func() {
			By("create container")
			containerID := createDefaultContainer(rc, ic, podID, podConfig, "container-for-execSync-test-")

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
			podID, podConfig = createPodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support starting container with volume [Conformance]", func() {
			By("create host path and flag file")
			hostPath, flagFile := createHostPath(podID)

			defer os.RemoveAll(hostPath) // clean up the TempDir

			By("create container with volume")
			containerID := createVolumeContainer(rc, ic, "container-with-volume-test-", podID, podConfig, hostPath, flagFile)

			By("test start container with volume")
			testStartContainer(rc, containerID)

			By("test container exit code")
			Eventually(getContainerStatus(rc, containerID).ExitCode).Should(Equal(int32(0)))
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
			// wait container started and check the status.
			Eventually(verifyContainerStatus(rc, containerID, runtimeapi.ContainerState_CONTAINER_RUNNING), pollTIMEOUT).Should(BeTrue())

			By("check the log context")
			expectedLogMessage := &logMessage{
				log:    []byte(defaultLog + "\n"),
				stream: stdoutType,
			}
			verifyLogContents(podConfig, logPath, expectedLogMessage)
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

// buildContainerMetadata builds containerMetadata.
func buildContainerMetadata(containerName string, attempt uint32) *runtimeapi.ContainerMetadata {
	return &runtimeapi.ContainerMetadata{
		Name:    containerName,
		Attempt: attempt,
	}
}

// getContainerStatus gets ContainerState for containerID and fails if it gets error.
func getContainerStatus(c internalapi.RuntimeService, containerID string) *runtimeapi.ContainerStatus {
	By("Get container status for containerID: " + containerID)
	status, err := c.ContainerStatus(containerID)
	framework.ExpectNoError(err, "failed to get container %q status: %v", containerID, err)
	return status
}

// verifyContainerStatus verifies whether container status for given containerID matches.
func verifyContainerStatus(c internalapi.RuntimeService, containerID string, expectedStatus runtimeapi.ContainerState) bool {
	status := getContainerStatus(c, containerID)
	return status.State == expectedStatus
}

// createContainer creates a container with the prefix of containerName.
func createContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, config *runtimeapi.ContainerConfig, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	// Pull the image if it does not exist.
	imageName := config.Image.Image
	if !strings.Contains(imageName, ":") {
		imageName = imageName + ":latest"
		framework.Logf("Use latest as default image tag.")
	}

	images := listImageForImageName(ic, imageName)
	if len(images) == 0 {
		pullPublicImage(ic, imageName)
	}

	By("Create container.")
	containerID, err := rc.CreateContainer(podID, config, podConfig)
	framework.ExpectNoError(err, "failed to create container: %v", err)
	framework.Logf("Created container %q\n", containerID)
	return containerID
}

// createDefaultContainer creates a  default container with default options.
func createDefaultContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: defaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
		Linux:    &runtimeapi.LinuxContainerConfig{},
	}

	return createContainer(rc, ic, containerConfig, podID, podConfig)
}

// testCreateDefaultContainer creates a container in the pod which ID is podID and make sure it's ready.
func testCreateDefaultContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	containerID := createDefaultContainer(rc, ic, podID, podConfig, "container-for-create-test-")
	Eventually(verifyContainerStatus(rc, containerID, runtimeapi.ContainerState_CONTAINER_CREATED), pollTIMEOUT).Should(BeTrue())
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
func testStartContainer(c internalapi.RuntimeService, containerID string) {
	startContainer(c, containerID)
	Eventually(verifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_RUNNING), pollTIMEOUT).Should(BeTrue())
}

// stopContainer stops the container for containerID.
func stopContainer(c internalapi.RuntimeService, containerID string, timeout int64) {
	By("Stop container for containerID: " + containerID)
	stopped := make(chan bool, 1)

	go func() {
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
	Eventually(verifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_EXITED), pollTIMEOUT).Should(BeTrue())
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

// createVolContainerOrFail creates a container with volume and the prefix of containerName and fails if it gets error.
func createVolumeContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig, hostPath, flagFile string) string {
	By("create a container with volume and name")
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: defaultContainerImage},
		// mount host path to the same directory in container, and check if flag file exists
		Command: []string{"sh", "-c", "test -f " + filepath.Join(hostPath, flagFile)},
		Mounts: []*runtimeapi.Mount{
			{
				HostPath:      hostPath,
				ContainerPath: hostPath,
			},
		},
	}

	return createContainer(rc, ic, containerConfig, podID, podConfig)
}

// createLogContainer creates a container with log and the prefix of containerName.
func createLogContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, prefix string, podID string, podConfig *runtimeapi.PodSandboxConfig) (string, string) {
	By("create a container with log and name")
	containerName := prefix + framework.NewUUID()
	path := fmt.Sprintf("%s.log", containerName)
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: defaultContainerImage},
		Command:  []string{"echo", defaultLog},
		LogPath:  path,
	}
	return containerConfig.LogPath, createContainer(rc, ic, containerConfig, podID, podConfig)
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

// parseCRILog parses logs in Docker JSON log format.
// CRI log format example :
//   2016-10-06T00:17:09.669794202Z stdout The content of the log entry 1
//   2016-10-06T00:17:10.113242941Z stderr The content of the log entry 2
func parseCRILog(log string, msg *logMessage) {
	timeStamp, err := time.Parse(time.RFC3339Nano, strings.Fields(log)[0])
	framework.ExpectNoError(err, "failed to parse timeStamp: %v", err)
	stream := strings.Fields(log)[1]
	logMessage := strings.Fields(log)[2:]

	msg.timestamp = timeStamp
	msg.stream = streamType(stream)
	msg.log = []byte(strings.Join(logMessage, " ") + "\n")
}

// verifyLogContents verifies the contents of container log.
func verifyLogContents(podConfig *runtimeapi.PodSandboxConfig, logPath string, expectedLogMessage *logMessage) {
	path := filepath.Join(podConfig.LogDirectory, logPath)
	f, err := os.Open(path)
	framework.ExpectNoError(err, "failed to open log file: %v", err)
	framework.Logf("Open log file %s", path)
	defer f.Close()

	log, err := ioutil.ReadAll(f)
	framework.ExpectNoError(err, "failed to read log file: %v", err)
	framework.Logf("Log file context is %s", log)

	var msg logMessage

	// to determine whether the log is Docker format or CRI format.
	if strings.Contains(string(log), "{") {
		parseDockerJSONLog(log, &msg)
	} else {
		parseCRILog(string(log), &msg)
	}
	framework.Logf("Parse json log succeed")

	Expect(string(msg.log)).To(Equal(string(expectedLogMessage.log)), "Log should be %s", string(expectedLogMessage.log))
	Expect(string(msg.stream)).To(Equal(string(expectedLogMessage.stream)), "Stream should be %s", string(expectedLogMessage.stream))
}
