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
	defaultAPIVersion      string = "0.1.0"
	defaultUIDPrefix       string = "e2e-cri-uid"
	defaultNamespacePrefix string = "e2e-cri-namespace"
	defaultAttempt         uint32 = 2
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

// TestGetVersion test if we can get runtime name.
func TestGetVersion(c internalapi.RuntimeService) {
	version, err := c.Version(defaultAPIVersion)
	ExpectNoError(err, "failed to get version: %v", err)
	Expect(version.Version).To(Not(BeNil()), "Version should not be nil")
	Expect(version.RuntimeName).To(Not(BeNil()), "RuntimeName should not be nil")
	Expect(version.RuntimeVersion).To(Not(BeNil()), "RuntimeVersion should not be nil")
	Expect(version.RuntimeApiVersion).To(Not(BeNil()), "RuntimeApiVersion should not be nil")
	Logf("Get version info succeed")
}

// PodSandboxFound returns whether PodSandbox is found.
func PodSandboxFound(podSandboxs []*runtimeapi.PodSandbox, podID string) bool {
	if len(podSandboxs) == 1 && podSandboxs[0].Id == podID {
		return true
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
	By("run PodSandbox.")
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
	By("Sto PodSandbox for podID: " + podID)
	err := c.StopPodSandbox(podID)
	ExpectNoError(err, "Failed to stop PodSandbox: %v", err)
	Logf("Stopped PodSandbox %q\n", podID)
}

// TestStopPodSandbox stops the PodSandbox for podID and make sure it is not ready.
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
	By("List PodSandbox")
	pods, err := c.ListPodSandbox(filter)
	ExpectNoError(err, "failed to list PodSandbox status: %v", err)
	Logf("List PodSandbox succeed")
	return pods
}
