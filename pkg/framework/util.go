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

package framework

import (
	"fmt"
	"strings"
	"sync"
	"time"

	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/remote"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
)

const (
	defaultUIDPrefix            string = "e2e-cri-uid"
	defaultNamespacePrefix      string = "e2e-cri-namespace"
	defaultAttempt              uint32 = 2
	defaultContainerImage       string = "gcr.io/google_containers/busybox:1.24"
	defaultStopContainerTimeout int64  = 60
	defaultExecSyncTimeout      int64  = 5
)

var (
	//lock for uuid
	uuidLock sync.Mutex

	// lastUUID record last generated uuid from NewUUID()
	lastUUID uuid.UUID
)

// LoadCRIClient creates a InternalAPIClient.
func LoadCRIClient() (*InternalAPIClient, error) {
	rService, err := remote.NewRemoteRuntimeService(TestContext.RuntimeServiceAddr, TestContext.RuntimeServiceTimeout)
	if err != nil {
		return nil, err
	}

	iService, err := remote.NewRemoteImageService(TestContext.ImageServiceAddr, TestContext.ImageServiceTimeout)
	if err != nil {
		return nil, err
	}

	return &InternalAPIClient{
		CRIRuntimeClient: rService,
		CRIImageClient:   iService,
	}, nil
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func log(level string, format string, args ...interface{}) {
	fmt.Fprintf(GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf prints a info message.
func Logf(format string, args ...interface{}) {
	log("INFO", format, args...)
}

// Failf prints an error message.
func Failf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log("INFO", msg)
	Fail(nowStamp()+": "+msg, 1)
}

// ExpectNoError reports error if err is not nil.
func ExpectNoError(err error, explain ...interface{}) {
	if err != nil {
		Logf("Unexpected error occurred: %v", err)
	}
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), explain...)
}

// NewUUID creates a new UUID string.
func NewUUID() string {
	uuidLock.Lock()
	defer uuidLock.Unlock()
	result := uuid.NewUUID()
	// The UUID package is naive and can generate identical UUIDs if the
	// time interval is quick enough.
	// The UUID uses 100 ns increments so it's short enough to actively
	// wait for a new value.
	for uuid.Equal(lastUUID, result) == true {
		result = uuid.NewUUID()
	}
	lastUUID = result
	return result.String()
}

// PodSandboxFound returns whether PodSandbox is found.
func PodSandboxFound(podSandboxs []*runtimeapi.PodSandbox, podID string) bool {
	for _, podSandbox := range podSandboxs {
		if podSandbox.Id == podID {
			return true
		}
	}
	return false
}

// buildPodSandboxMetadata builds PodSandboxMetadata.
func buildPodSandboxMetadata(podSandboxName, uid, namespace string, attempt uint32) *runtimeapi.PodSandboxMetadata {
	return &runtimeapi.PodSandboxMetadata{
		Name:      podSandboxName,
		Uid:       uid,
		Namespace: namespace,
		Attempt:   attempt,
	}
}

// VerifyPodSandboxStatus verifies whether PodSandbox status for given podID matches.
func VerifyPodSandboxStatus(c internalapi.RuntimeService, podID string, expectedStatus runtimeapi.PodSandboxState, statusName string) {
	status := GetPodSandboxStatus(c, podID)
	Expect(status.State).To(Equal(expectedStatus), "PodSandbox state should be "+statusName)
}

// RunPodSandbox runs a PodSandbox.
func RunPodSandbox(c internalapi.RuntimeService, config *runtimeapi.PodSandboxConfig) string {
	By("Run PodSandbox.")
	podID, err := c.RunPodSandbox(config)
	ExpectNoError(err, "failed to create PodSandbox: %v", err)
	Logf("Created PodSandbox %q\n", podID)
	return podID
}

// RunDefaultPodSandbox runs a PodSandbox with default options.
func RunDefaultPodSandbox(c internalapi.RuntimeService, prefix string) string {
	podSandboxName := prefix + NewUUID()
	uid := defaultUIDPrefix + NewUUID()
	namespace := defaultNamespacePrefix + NewUUID()

	config := &runtimeapi.PodSandboxConfig{
		Metadata: buildPodSandboxMetadata(podSandboxName, uid, namespace, defaultAttempt),
	}
	return RunPodSandbox(c, config)
}

// TestRunDefaultPodSandbox runs a PodSandbox and make sure it is ready.
func TestRunDefaultPodSandbox(c internalapi.RuntimeService) string {
	podID := RunDefaultPodSandbox(c, "PodSandbox-for-create-test-")
	VerifyPodSandboxStatus(c, podID, runtimeapi.PodSandboxState_SANDBOX_READY, "ready")
	return podID
}

// GetPodSandboxStatus gets PodSandboxStatus for podID.
func GetPodSandboxStatus(c internalapi.RuntimeService, podID string) *runtimeapi.PodSandboxStatus {
	By("Get PodSandbox status for podID: " + podID)
	status, err := c.PodSandboxStatus(podID)
	ExpectNoError(err, "failed to get PodSandbox %q status: %v", podID, err)
	return status
}

// StopPodSandbox stops the PodSandbox for podID.
func StopPodSandbox(c internalapi.RuntimeService, podID string) {
	By("Stop PodSandbox for podID: " + podID)
	err := c.StopPodSandbox(podID)
	ExpectNoError(err, "Failed to stop PodSandbox: %v", err)
	Logf("Stopped PodSandbox %q\n", podID)
}

// TestStopPodSandbox stops the PodSandbox for podID and make sure it's not ready.
func TestStopPodSandbox(c internalapi.RuntimeService, podID string) {
	StopPodSandbox(c, podID)
	VerifyPodSandboxStatus(c, podID, runtimeapi.PodSandboxState_SANDBOX_NOTREADY, "not ready")
}

// RemovePodSandbox removes the PodSandbox for podID.
func RemovePodSandbox(c internalapi.RuntimeService, podID string) {
	By("Remove PodSandbox for podID: " + podID)
	err := c.RemovePodSandbox(podID)
	ExpectNoError(err, "failed to remove PodSandbox: %v", err)
	Logf("Removed PodSandbox %q\n", podID)
}

// TestRemovePodSandbox removes a PodSandbox and make sure it is removed.
func TestRemovePodSandbox(c internalapi.RuntimeService, podID string) {
	RemovePodSandbox(c, podID)
	pods := ListPodSanboxForID(c, podID)
	Expect(PodSandboxFound(pods, podID)).To(BeFalse(), "PodSandbox should be removed")
}

// ListPodSanboxForID lists PodSandbox for podID.
func ListPodSanboxForID(c internalapi.RuntimeService, podID string) []*runtimeapi.PodSandbox {
	By("List PodSandbox for podID: " + podID)
	filter := &runtimeapi.PodSandboxFilter{
		Id: podID,
	}
	return ListPodSandbox(c, filter)
}

// ListPodSandbox lists PodSandbox.
func ListPodSandbox(c internalapi.RuntimeService, filter *runtimeapi.PodSandboxFilter) []*runtimeapi.PodSandbox {
	By("List PodSandbox.")
	pods, err := c.ListPodSandbox(filter)
	ExpectNoError(err, "failed to list PodSandbox status: %v", err)
	Logf("List PodSandbox succeed")
	return pods
}

// ContainerFound returns whether containers is found.
func ContainerFound(containers []*runtimeapi.Container, containerID string) bool {
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

// GetContainerStatus gets ContainerState for containerID and fails if it gets error.
func GetContainerStatus(c internalapi.RuntimeService, containerID string) *runtimeapi.ContainerStatus {
	By("Get container status for containerID: " + containerID)
	status, err := c.ContainerStatus(containerID)
	ExpectNoError(err, "failed to get container %q status: %v", containerID, err)
	return status
}

// VerifyContainerStatus verifies whether container status for given containerID matches.
func VerifyContainerStatus(c internalapi.RuntimeService, containerID string, expectedStatus runtimeapi.ContainerState, stateName string) {
	status := GetContainerStatus(c, containerID)
	Expect(status.State).To(Equal(expectedStatus), "Container state should be %s", stateName)
}

// CreateContainer creates a container with the prefix of containerName.
func CreateContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, config *runtimeapi.ContainerConfig, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	// Pull the image if it does not exist.
	imageName := config.Image.Image
	if !strings.Contains(imageName, ":") {
		imageName = imageName + ":latest"
		Logf("Use latest as default image tag.")
	}

	images := ListImageForImageName(ic, imageName)
	if len(images) == 0 {
		PullPublicImage(ic, imageName)
	}

	By("Create container.")
	containerID, err := rc.CreateContainer(podID, config, podConfig)
	ExpectNoError(err, "failed to create container: %v", err)
	Logf("Created container %q\n", containerID)
	return containerID
}

// CreateDefaultContainer creates a  default container with default options.
func CreateDefaultContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	containerName := prefix + NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: defaultContainerImage},
		Command:  []string{"sh", "-c", "top"},
	}

	return CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// TestCreateDefaultContainer creates a container in the pod which ID is podID and make sure it's ready.
func TestCreateDefaultContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	containerID := CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-create-test-")
	VerifyContainerStatus(rc, containerID, runtimeapi.ContainerState_CONTAINER_CREATED, "created")
	return containerID
}

// StartContainer start the container for containerID.
func StartContainer(c internalapi.RuntimeService, containerID string) {
	By("Start container for containerID: " + containerID)
	err := c.StartContainer(containerID)
	ExpectNoError(err, "failed to start container: %v", err)
	Logf("Started container %q\n", containerID)
}

// TestStartContainer starts the container for containerID and make sure it's running.
func TestStartContainer(c internalapi.RuntimeService, containerID string) {
	StartContainer(c, containerID)
	VerifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_RUNNING, "running")
}

// StopContainer stops the container for containerID.
func StopContainer(c internalapi.RuntimeService, containerID string, timeout int64) {
	By("Stop container for containerID: " + containerID)
	stopped := make(chan bool, 1)

	go func() {
		err := c.StopContainer(containerID, timeout)
		ExpectNoError(err, "failed to stop container: %v", err)
		stopped <- true
	}()

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		Failf("stop container %q timeout.\n", containerID)
	case <-stopped:
		Logf("Stopped container %q\n", containerID)
	}
}

// TestStopContainer stops the container for containerID and make sure it's exited.
func TestStopContainer(c internalapi.RuntimeService, containerID string) {
	StopContainer(c, containerID, defaultStopContainerTimeout)
	VerifyContainerStatus(c, containerID, runtimeapi.ContainerState_CONTAINER_EXITED, "exited")
}

// RemoveContainer removes the container for containerID.
func RemoveContainer(c internalapi.RuntimeService, containerID string) {
	By("Remove container for containerID: " + containerID)
	err := c.RemoveContainer(containerID)
	ExpectNoError(err, "failed to remove container: %v", err)
	Logf("Removed container %q\n", containerID)
}

// ListContainerForID lists container for containerID.
func ListContainerForID(c internalapi.RuntimeService, containerID string) []*runtimeapi.Container {
	By("List containers for containerID: " + containerID)
	filter := &runtimeapi.ContainerFilter{
		Id: containerID,
	}
	containers, err := c.ListContainers(filter)
	ExpectNoError(err, "failed to list containers %q status: %v", containerID, err)
	return containers
}

// CreatePodSandboxForContainer creates a PodSandbox for creating containers.
func CreatePodSandboxForContainer(c internalapi.RuntimeService) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-for-container-" + NewUUID()
	uid := defaultUIDPrefix + NewUUID()
	namespace := defaultNamespacePrefix + NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: buildPodSandboxMetadata(podSandboxName, uid, namespace, defaultAttempt),
	}

	podID := RunPodSandbox(c, config)
	return podID, config
}

// TestExecSync test execSync for containerID and make sure the response is right.
func TestExecSync(c internalapi.RuntimeService, containerID string) {
	By("Test execSync for containerID: " + containerID)
	cmd := []string{"echo", "hello"}
	stdout, stderr, err := c.ExecSync(containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	ExpectNoError(err, "failed to execSync in container %q", containerID)
	Expect(string(stdout)).To(Equal("hello\n"), "The stdout output of execSync should be %q", stdout)
	Expect(stderr).To(BeNil(), "The stderr should be nil.")
	Logf("Execsync succeed")
}

// PullPublicImage pulls the public image named imageName.
func PullPublicImage(c internalapi.ImageManagerService, imageName string) {
	if !strings.Contains(imageName, ":") {
		imageName = imageName + ":latest"
		Logf("Use latest as default image tag.")
	}

	By("Pull image : " + imageName)
	imageSpec := &runtimeapi.ImageSpec{
		Image: imageName,
	}
	_, err := c.PullImage(imageSpec, nil)
	ExpectNoError(err, "failed to pull image: %v", err)
}

// RemoveImage removes the image named imagesName.
func RemoveImage(c internalapi.ImageManagerService, imageName string) {
	By("Remove image : " + imageName)
	imageSpec := &runtimeapi.ImageSpec{
		Image: imageName,
	}
	err := c.RemoveImage(imageSpec)
	ExpectNoError(err, "failed to remove image: %v", err)
}

// ListImageForImageName lists the images named imageName.
func ListImageForImageName(c internalapi.ImageManagerService, imageName string) []*runtimeapi.Image {
	By("Get image list for imageName : " + imageName)
	filter := &runtimeapi.ImageFilter{
		Image: &runtimeapi.ImageSpec{Image: imageName},
	}
	images := ListImage(c, filter)
	return images
}

// ListImage list the image filtered by the image filter.
func ListImage(c internalapi.ImageManagerService, filter *runtimeapi.ImageFilter) []*runtimeapi.Image {
	images, err := c.ListImages(filter)
	ExpectNoError(err, "Failed to get image list: %v", err)
	return images
}
