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

	"github.com/pborman/uuid"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/remote"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	//lock for uuid
	uuidLock sync.Mutex

	// lastUUID record last generated uuid from NewUUID()
	lastUUID uuid.UUID
)

const (
	// DefaultUIDPrefix is a default UID prefix of PodSandbox
	DefaultUIDPrefix string = "cri-test-uid"

	// DefaultNamespacePrefix is a default namespace prefix of PodSandbox
	DefaultNamespacePrefix string = "cri-test-namespace"

	// DefaultAttempt is a default attempt prefix of PodSandbox or container
	DefaultAttempt uint32 = 2
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

// RunDefaultPodSandbox runs a PodSandbox with default options.
func RunDefaultPodSandbox(c internalapi.RuntimeService, prefix string) string {
	podSandboxName := prefix + NewUUID()
	uid := DefaultUIDPrefix + NewUUID()
	namespace := DefaultNamespacePrefix + NewUUID()

	config := &runtimeapi.PodSandboxConfig{
		Metadata: BuildPodSandboxMetadata(podSandboxName, uid, namespace, DefaultAttempt),
		Linux:    &runtimeapi.LinuxPodSandboxConfig{},
	}
	return RunPodSandbox(c, config)
}

// BuildPodSandboxMetadata builds PodSandboxMetadata.
func BuildPodSandboxMetadata(podSandboxName, uid, namespace string, attempt uint32) *runtimeapi.PodSandboxMetadata {
	return &runtimeapi.PodSandboxMetadata{
		Name:      podSandboxName,
		Uid:       uid,
		Namespace: namespace,
		Attempt:   attempt,
	}
}

// RunPodSandbox runs a PodSandbox.
func RunPodSandbox(c internalapi.RuntimeService, config *runtimeapi.PodSandboxConfig) string {
	podID, err := c.RunPodSandbox(config)
	ExpectNoError(err, "failed to create PodSandbox: %v", err)
	return podID
}
