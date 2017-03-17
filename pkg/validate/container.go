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
	"strings"
	"time"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	defaultContainerImage       string = "gcr.io/google_containers/busybox:1.24"
	defaultStopContainerTimeout int64  = 60
	defaultExecSyncTimeout      int64  = 5
)

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
			testExecSync(rc, containerID)
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
func verifyContainerStatus(c internalapi.RuntimeService, containerID string, expectedStatus runtimeapi.ContainerState, stateName string) {
	status := getContainerStatus(c, containerID)
	Expect(status.State).To(Equal(expectedStatus), "Container state should be %s", stateName)
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
	}

	return createContainer(rc, ic, containerConfig, podID, podConfig)
}

// testCreateDefaultContainer creates a container in the pod which ID is podID and make sure it's ready.
func testCreateDefaultContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	containerID := createDefaultContainer(rc, ic, podID, podConfig, "container-for-create-test-")
	verifyContainerStatus(rc, containerID, runtimeapi.ContainerState_CONTAINER_CREATED, "created")
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
	verifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_RUNNING, "running")
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
	verifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_EXITED, "exited")
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

// testExecSync test execSync for containerID and make sure the response is right.
func testExecSync(c internalapi.RuntimeService, containerID string) {
	By("Test execSync for containerID: " + containerID)
	cmd := []string{"echo", "hello"}
	stdout, stderr, err := c.ExecSync(containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	framework.ExpectNoError(err, "failed to execSync in container %q", containerID)
	Expect(string(stdout)).To(Equal("hello\n"), "The stdout output of execSync should be hello")
	Expect(stderr).To(BeNil(), "The stderr should be nil.")
	framework.Logf("Execsync succeed")
}
