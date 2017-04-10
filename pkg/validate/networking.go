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
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

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
			By("create a PodSandbxo with DNS config")
			var podConfig *runtimeapi.PodSandboxConfig
			podID, podConfig = createPodSandWithDNSConfig(rc)

			By("create container")
			containerID := createDefaultContainer(rc, ic, podID, podConfig, "container-for-DNS-config-test-")

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
			By("create a PodSandbxo with container port port mapping")
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
			checkPortMapping(rc, podID, true)
		})

		It("runtime should support port mapping with host port and container port [Conformance]", func() {
			By("create a PodSandbxo with host port and container port port mapping")
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
			checkPortMapping(rc, podID, false)
		})
	})
})

// createPodSandWithDNSConfig create a PodSandbxo with DNS config.
func createPodSandWithDNSConfig(c internalapi.RuntimeService) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-with-DNS-config" + framework.NewUUID()
	uid := defaultUIDPrefix + framework.NewUUID()
	namespace := defaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: buildPodSandboxMetadata(podSandboxName, uid, namespace, defaultAttempt),
		DnsConfig: &runtimeapi.DNSConfig{
			Servers:  []string{defaultDNSServer},
			Searches: []string{defaultDNSSearch},
			Options:  []string{defaultDNSOption},
		},
		Linux: &runtimeapi.LinuxPodSandboxConfig{},
	}

	podID := runPodSandbox(c, config)
	return podID, config
}

// createPodSandboxWithPortMapping create a PodSandbxo with port mapping.
func createPodSandboxWithPortMapping(c internalapi.RuntimeService, portMappings []*runtimeapi.PortMapping) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-with-port-mapping" + framework.NewUUID()
	uid := defaultUIDPrefix + framework.NewUUID()
	namespace := defaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata:     buildPodSandboxMetadata(podSandboxName, uid, namespace, defaultAttempt),
		PortMappings: portMappings,
		Linux:        &runtimeapi.LinuxPodSandboxConfig{},
	}

	podID := runPodSandbox(c, config)
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
		Metadata: buildContainerMetadata(containerName, defaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: nginxImage},
		Linux:    &runtimeapi.LinuxContainerConfig{},
	}
	return createContainer(rc, ic, containerConfig, podID, podConfig)
}

// checkPortMapping check if the given IP:port works fine.
func checkPortMapping(c internalapi.RuntimeService, podID string, containerPortOnly bool) {
	By("get the IP:port needed to be checked")
	url := "http://"
	if containerPortOnly {
		status := getPodSandboxStatus(c, podID)
		Expect(status.GetNetwork()).NotTo(BeNil(), "The network in status should not be nil.")
		Expect(status.GetNetwork().Ip).NotTo(BeNil(), "The IP should not be nil.")
		url += status.GetNetwork().Ip + ":" + strconv.Itoa(int(nginxContainerPort))
	} else {
		url += "localhost:" + strconv.Itoa(int(nginxHostPort))
	}
	framework.Logf("the IP:port is " + url)

	By("check the content of " + url)
	resp, err := http.Get(url)
	framework.ExpectNoError(err, "failed to get the content of %q", url)
	Expect(resp.StatusCode).To(Equal(200), "The status code of response should be 200.")
	framework.Logf("check port mapping succeed")
}
