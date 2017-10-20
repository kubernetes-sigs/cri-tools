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
	"net/http"
	"strconv"
	"time"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	defaultDNSServer   string = "10.10.10.10"
	defaultDNSSearch   string = "google.com"
	defaultDNSOption   string = "ndots:8"
	resolvConfigPath   string = "/etc/resolv.conf"
	nginxImage         string = "nginx"
	nginxContainerPort int32  = 80
	nginxHostPort      int32  = 8000
)

var _ = framework.KubeDescribe("Networking", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support networking", func() {
		var podID string

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support DNS config [Conformance]", func() {
			By("create a PodSandbox with DNS config")
			var podConfig *runtimeapi.PodSandboxConfig
			podID, podConfig = createPodSandWithDNSConfig(rc)

			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-DNS-config-test-")

			By("start container")
			startContainer(rc, containerID)

			By("check DNS config")
			expectedContent := []string{
				"nameserver " + defaultDNSServer,
				"search " + defaultDNSSearch,
				"options " + defaultDNSOption,
			}
			checkDNSConfig(rc, containerID, expectedContent)
		})

		It("runtime should support port mapping with only container port [Conformance]", func() {
			By("create a PodSandbox with container port port mapping")
			var podConfig *runtimeapi.PodSandboxConfig
			portMappings := []*runtimeapi.PortMapping{
				{
					ContainerPort: nginxContainerPort,
				},
			}
			podID, podConfig = createPodSandboxWithPortMapping(rc, portMappings)

			By("create a nginx container")
			containerID := createNginxContainer(rc, ic, podID, podConfig, "container-for-container-port")

			By("start the nginx container")
			startContainer(rc, containerID)

			By("check the port mapping with only container port")
			checkNginxMainPage(rc, podID, false)
		})

		It("runtime should support port mapping with host port and container port [Conformance]", func() {
			By("create a PodSandbox with host port and container port port mapping")
			var podConfig *runtimeapi.PodSandboxConfig
			portMappings := []*runtimeapi.PortMapping{
				{
					ContainerPort: nginxContainerPort,
					HostPort:      nginxHostPort,
				},
			}
			podID, podConfig = createPodSandboxWithPortMapping(rc, portMappings)

			By("create a nginx container")
			containerID := createNginxContainer(rc, ic, podID, podConfig, "container-for-host-port")

			By("start the nginx container")
			startContainer(rc, containerID)

			By("check the port mapping with host port and container port")
			checkNginxMainPage(rc, "", true)
		})
	})
})

// createPodSandWithDNSConfig create a PodSandbox with DNS config.
func createPodSandWithDNSConfig(c internalapi.RuntimeService) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-with-DNS-config" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		DnsConfig: &runtimeapi.DNSConfig{
			Servers:  []string{defaultDNSServer},
			Searches: []string{defaultDNSSearch},
			Options:  []string{defaultDNSOption},
		},
		Linux: &runtimeapi.LinuxPodSandboxConfig{},
	}

	podID := framework.RunPodSandbox(c, config)
	return podID, config
}

// createPodSandboxWithPortMapping create a PodSandbox with port mapping.
func createPodSandboxWithPortMapping(c internalapi.RuntimeService, portMappings []*runtimeapi.PortMapping) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-with-port-mapping" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata:     framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		PortMappings: portMappings,
		Linux:        &runtimeapi.LinuxPodSandboxConfig{},
	}

	podID := framework.RunPodSandbox(c, config)
	return podID, config
}

// checkDNSConfig checks the content of /etc/resolv.conf.
func checkDNSConfig(c internalapi.RuntimeService, containerID string, expectedContent []string) {
	By("get the content of /etc/resolv.conf via execSync")
	cmd := []string{"cat", resolvConfigPath}
	stdout, stderr, err := c.ExecSync(containerID, cmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	framework.ExpectNoError(err, "failed to execSync in container %q", containerID)
	for _, content := range expectedContent {
		Expect(string(stdout)).To(ContainSubstring(content), "The stdout output of execSync should contain %q", content)
	}
	Expect(stderr).To(BeNil(), "The stderr should be nil.")
	framework.Logf("check DNS config succeed")
}

// createNginxContainer creates a  nginx container.
func createNginxContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: nginxImage},
		Linux:    &runtimeapi.LinuxContainerConfig{},
	}
	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// checkNginxMainPage check if the we can get the main page of nginx via given IP:port.
func checkNginxMainPage(c internalapi.RuntimeService, podID string, localHost bool) {
	By("get the IP:port needed to be checked")
	var err error
	var resp *http.Response

	url := "http://"
	if localHost {
		url += "127.0.0.1:" + strconv.Itoa(int(nginxHostPort))
	} else {
		status := getPodSandboxStatus(c, podID)
		Expect(status.GetNetwork()).NotTo(BeNil(), "The network in status should not be nil.")
		Expect(status.GetNetwork().Ip).NotTo(BeNil(), "The IP should not be nil.")
		url += status.GetNetwork().Ip + ":" + strconv.Itoa(int(nginxContainerPort))
	}
	framework.Logf("the IP:port is " + url)

	By("check the content of " + url)

	Eventually(func() error {
		resp, err = http.Get(url)
		return err
	}, time.Minute, time.Second).Should(BeNil())

	Expect(resp.StatusCode).To(Equal(200), "The status code of response should be 200.")
	framework.Logf("check port mapping succeed")
}
