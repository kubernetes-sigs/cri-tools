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
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	remote "k8s.io/cri-client/pkg"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cri-tools/pkg/common"
)

var (
	// the callbacks to run during BeforeSuite.
	beforeSuiteCallbacks []func()

	// DefaultPodLabels are labels used by default in pods.
	DefaultPodLabels map[string]string

	// DefaultContainerCommand is the default command used for containers.
	DefaultContainerCommand []string

	// DefaultPauseCommand is the default command used for containers.
	DefaultPauseCommand []string

	// DefaultLinuxPodLabels default pod labels for Linux.
	DefaultLinuxPodLabels = map[string]string{}

	// DefaultLinuxContainerCommand default container command for Linux.
	DefaultLinuxContainerCommand = []string{"top"}

	// DefaultLinuxPauseCommand default container command for Linux pause.
	DefaultLinuxPauseCommand = []string{"sh", "-c", "top"}

	// DefaultLcowPodLabels default pod labels for Linux containers on Windows.
	DefaultLcowPodLabels = map[string]string{
		"sandbox-platform": "linux/amd64",
	}

	// DefaultWindowsPodLabels default pod labels for Windows.
	DefaultWindowsPodLabels = map[string]string{}

	// DefaultWindowsContainerCommand default container command for Windows.
	DefaultWindowsContainerCommand = []string{"cmd", "/c", "ping -t localhost"}

	// DefaultWindowsPauseCommand default container pause command for Windows.
	DefaultWindowsPauseCommand = []string{"powershell", "-c", "ping -t localhost"}
)

const (
	// DefaultUIDPrefix is a default UID prefix of PodSandbox.
	DefaultUIDPrefix string = "cri-test-uid"

	// DefaultNamespacePrefix is a default namespace prefix of PodSandbox.
	DefaultNamespacePrefix string = "cri-test-namespace"

	// DefaultAttempt is a default attempt prefix of PodSandbox or container.
	DefaultAttempt uint32 = 2

	// DefaultStopContainerTimeout is the default timeout for stopping container.
	DefaultStopContainerTimeout int64 = 60

	// DefaultLinuxContainerImage default container image for Linux.
	DefaultLinuxContainerImage string = DefaultRegistryE2ETestImagesPrefix + "busybox:1.29-2"

	// DefaultWindowsContainerImage default container image for Windows.
	DefaultWindowsContainerImage string = DefaultLinuxContainerImage
)

// Set the constants based on operating system and flags.
var _ = BeforeSuite(func() {
	if runtime.GOOS != OSWindows || TestContext.IsLcow {
		DefaultPodLabels = DefaultLinuxPodLabels
		DefaultContainerCommand = DefaultLinuxContainerCommand
		DefaultPauseCommand = DefaultLinuxPauseCommand
		TestContext.TestImageList.DefaultTestContainerImage = DefaultLinuxContainerImage

		if TestContext.IsLcow {
			DefaultPodLabels = DefaultLcowPodLabels
		}
	} else {
		DefaultPodLabels = DefaultWindowsPodLabels
		DefaultContainerCommand = DefaultWindowsContainerCommand
		DefaultPauseCommand = DefaultWindowsPauseCommand
		TestContext.TestImageList.DefaultTestContainerImage = DefaultWindowsContainerImage
	}

	// Load any custom image definitions:
	err := TestContext.LoadYamlConfigFiles()
	if err != nil {
		panic(err)
	}

	for _, callback := range beforeSuiteCallbacks {
		callback()
	}
})

// AddBeforeSuiteCallback adds a callback to run during BeforeSuite.
func AddBeforeSuiteCallback(callback func()) bool {
	beforeSuiteCallbacks = append(beforeSuiteCallbacks, callback)

	return true
}

// LoadCRIClient creates a InternalAPIClient.
func LoadCRIClient() (*InternalAPIClient, error) {
	rService, err := remote.NewRemoteRuntimeService(
		TestContext.RuntimeServiceAddr,
		TestContext.RuntimeServiceTimeout,
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	imageServiceAddr := TestContext.ImageServiceAddr
	if imageServiceAddr == "" {
		// Fallback to runtime service endpoint
		imageServiceAddr = TestContext.RuntimeServiceAddr
	}

	iService, err := remote.NewRemoteImageService(imageServiceAddr, TestContext.ImageServiceTimeout, nil, nil)
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

func logf(level, format string, args ...any) {
	fmt.Fprintf(GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

// Logf prints a info message.
func Logf(format string, args ...any) {
	logf("INFO", format, args...)
}

// Failf prints an error message.
func Failf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logf("INFO", msg)
	Fail(nowStamp()+": "+msg, 1)
}

// ExpectNoError reports error if err is not nil.
func ExpectNoError(err error, explain ...any) {
	if err != nil {
		Logf("Unexpected error occurred: %v", err)
	}

	ExpectWithOffset(1, err).NotTo(HaveOccurred(), explain...)
}

// NewUUID creates a new UUID string.
func NewUUID() string {
	return uuid.New().String()
}

// RunDefaultPodSandbox runs a PodSandbox with default options.
func RunDefaultPodSandbox(c internalapi.RuntimeService, prefix string) string {
	podSandboxName := prefix + NewUUID()
	uid := DefaultUIDPrefix + NewUUID()
	namespace := DefaultNamespacePrefix + NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: BuildPodSandboxMetadata(podSandboxName, uid, namespace, DefaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(context.TODO(), c),
		},
		Labels: DefaultPodLabels,
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
	podID, err := c.RunPodSandbox(context.TODO(), config, TestContext.RuntimeHandler)
	ExpectNoError(err, "failed to create PodSandbox: %v", err)

	return podID
}

// RunPodSandboxError runs a PodSandbox and expects an error.
func RunPodSandboxError(c internalapi.RuntimeService, config *runtimeapi.PodSandboxConfig) string {
	podID, err := c.RunPodSandbox(context.TODO(), config, TestContext.RuntimeHandler)
	Expect(err).To(HaveOccurred())

	return podID
}

// CreatePodSandboxForContainer creates a PodSandbox for creating containers.
func CreatePodSandboxForContainer(c internalapi.RuntimeService) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-for-container-" + NewUUID()
	uid := DefaultUIDPrefix + NewUUID()
	namespace := DefaultNamespacePrefix + NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: BuildPodSandboxMetadata(podSandboxName, uid, namespace, DefaultAttempt),
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(context.TODO(), c),
		},
		Labels: DefaultPodLabels,
	}

	podID := RunPodSandbox(c, config)

	return podID, config
}

// BuildContainerMetadata builds containerMetadata.
func BuildContainerMetadata(containerName string, attempt uint32) *runtimeapi.ContainerMetadata {
	return &runtimeapi.ContainerMetadata{
		Name:    containerName,
		Attempt: attempt,
	}
}

// CreateDefaultContainer creates a default container with default options.
func CreateDefaultContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	return CreateDefaultContainerWithLabels(rc, ic, podID, podConfig, prefix, nil)
}

// CreateDefaultContainerWithLabels creates a default container with default options.
func CreateDefaultContainerWithLabels(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string, labels map[string]string) string {
	containerName := prefix + NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: BuildContainerMetadata(containerName, DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: TestContext.TestImageList.DefaultTestContainerImage},
		Command:  DefaultContainerCommand,
		Linux:    &runtimeapi.LinuxContainerConfig{},
		Labels:   labels,
	}

	return CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// CreatePauseContainer creates a container with default pause options.
func CreatePauseContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	containerName := prefix + NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: BuildContainerMetadata(containerName, DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: TestContext.TestImageList.DefaultTestContainerImage},
		Command:  DefaultPauseCommand,
		Linux:    &runtimeapi.LinuxContainerConfig{},
	}

	return CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// CreateContainerWithError creates a container but leave error check to caller.
func CreateContainerWithError(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, config *runtimeapi.ContainerConfig, podID string, podConfig *runtimeapi.PodSandboxConfig) (string, error) {
	// Pull the image if it does not exist.
	imageName := config.GetImage().GetImage()
	if !strings.Contains(imageName, ":") {
		imageName += ":latest"

		Logf("Use latest as default image tag.")
	}

	status := ImageStatus(ic, imageName)
	if status == nil {
		PullPublicImage(ic, imageName, podConfig)
	}

	if config.GetImage().GetUserSpecifiedImage() == "" {
		config.Image.UserSpecifiedImage = imageName
	}

	By("Create container.")

	containerID, err := rc.CreateContainer(context.TODO(), podID, config, podConfig)

	return containerID, err
}

// CreateContainer creates a container with the prefix of containerName.
func CreateContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, config *runtimeapi.ContainerConfig, podID string, podConfig *runtimeapi.PodSandboxConfig) string {
	containerID, err := CreateContainerWithError(rc, ic, config, podID, podConfig)
	ExpectNoError(err, "failed to create container: %v", err)
	Logf("Created container %q\n", containerID)

	return containerID
}

// ImageStatus gets the status of the image named imageName.
func ImageStatus(c internalapi.ImageManagerService, imageName string) *runtimeapi.Image {
	By("Get image status for image: " + imageName)
	imageSpec := &runtimeapi.ImageSpec{
		Image: imageName,
	}
	status, err := c.ImageStatus(context.TODO(), imageSpec, false)
	ExpectNoError(err, "failed to get image status: %v", err)

	return status.GetImage()
}

// ListImage list the image filtered by the image filter.
func ListImage(c internalapi.ImageManagerService, filter *runtimeapi.ImageFilter) []*runtimeapi.Image {
	images, err := c.ListImages(context.TODO(), filter)
	ExpectNoError(err, "Failed to get image list: %v", err)

	return images
}

// PrepareImageName builds a pullable image name for the provided one.
func PrepareImageName(imageName string) string {
	ref, err := reference.ParseNamed(imageName)
	if err != nil {
		if errors.Is(err, reference.ErrNameNotCanonical) {
			// Non canonical images can simply be prefixed
			return fmt.Sprintf("%s/%s", TestContext.RegistryPrefix, imageName)
		}

		Failf("Unable to parse imageName: %v", err)
	}

	// Modify the image if it's a fully qualified image name
	if TestContext.RegistryPrefix != DefaultRegistryPrefix {
		r := fmt.Sprintf("%s/%s", TestContext.RegistryPrefix, reference.Path(ref))
		ref, err = reference.ParseNamed(r)
		ExpectNoError(err, "failed to parse new image name: %v", err)
	}

	imageName = ref.String()

	if !strings.Contains(imageName, ":") {
		imageName += ":latest"

		Logf("Use latest as default image tag.")
	}

	return imageName
}

// PullPublicImage pulls the public image named imageName.
func PullPublicImage(c internalapi.ImageManagerService, imageName string, podConfig *runtimeapi.PodSandboxConfig) string {
	imageName = PrepareImageName(imageName)

	By("Pull image : " + imageName)
	imageSpec := &runtimeapi.ImageSpec{
		Image: imageName,
	}
	id, err := c.PullImage(context.TODO(), imageSpec, nil, podConfig)
	ExpectNoError(err, "failed to pull image: %v", err)

	return id
}

// LoadYamlFile attempts to load the given YAML file into the given struct.
func LoadYamlFile(filepath string, obj any) error {
	Logf("Attempting to load YAML file %q into %+v", filepath, obj)

	fileContent, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("error reading %q file contents: %w", filepath, err)
	}

	err = yaml.Unmarshal(fileContent, obj)
	if err != nil {
		return fmt.Errorf("error unmarshalling %q YAML file: %w", filepath, err)
	}

	Logf("Successfully loaded YAML file %q into %+v", filepath, obj)

	return nil
}
