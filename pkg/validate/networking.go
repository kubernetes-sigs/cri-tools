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
	"net/http"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
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
			Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
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
			expectedContent := getDNSConfigContent
			checkDNSConfig(rc, containerID, expectedContent)
		})

		It("runtime should support set hostname [Conformance]", func() {
			By("create a PodSandbox with hostname")
			var podConfig *runtimeapi.PodSandboxConfig
			const testHostname = "test-hostname"
			podID, podConfig = createPodSandWithHostname(rc, testHostname)

			By("create container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-hostname-test-")

			By("start container")
			startContainer(rc, containerID)

			By("check hostname")
			checkHostname(rc, containerID, testHostname)
		})

		It("runtime should support port mapping with only container port [Conformance]", func() {
			By("create a PodSandbox with container port mapping")
			var podConfig *runtimeapi.PodSandboxConfig
			portMappings := []*runtimeapi.PortMapping{
				{
					ContainerPort: webServerContainerPort,
				},
			}
			podID, podConfig = createPodSandboxWithPortMapping(rc, portMappings, false)

			By("create a web server container")
			containerID := createWebServerContainer(rc, ic, podID, podConfig, "container-for-container-port")

			By("start the web server container")
			startContainer(rc, containerID)

			By("check the port mapping with only container port")
			checkMainPage(rc, podID, 0, webServerContainerPort)
		})

		It("runtime should support port mapping with host port and container port [Conformance]", func() {
			By("create a PodSandbox with host port and container port mapping")
			var podConfig *runtimeapi.PodSandboxConfig
			portMappings := []*runtimeapi.PortMapping{
				{
					ContainerPort: webServerContainerPort,
					HostPort:      webServerHostPortForPortMapping,
				},
			}
			podID, podConfig = createPodSandboxWithPortMapping(rc, portMappings, false)

			By("create a web server container")
			containerID := createWebServerContainer(rc, ic, podID, podConfig, "container-for-host-port")

			By("start the web server container")
			startContainer(rc, containerID)

			By("check the port mapping with host port and container port")
			checkMainPage(rc, "", webServerHostPortForPortMapping, 0)
		})
	})
})

// createPodSandWithHostname create a PodSandbox with hostname.
func createPodSandWithHostname(c internalapi.RuntimeService, hostname string) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-with-hostname" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
	config := &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		Hostname: hostname,
		Labels:   framework.DefaultPodLabels,
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(context.TODO(), c),
		},
	}

	podID := framework.RunPodSandbox(c, config)

	return podID, config
}

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
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(context.TODO(), c),
		},
		Labels: framework.DefaultPodLabels,
	}

	podID := framework.RunPodSandbox(c, config)

	return podID, config
}

// createPodSandboxWithPortMapping create a PodSandbox with port mapping.
func createPodSandboxWithPortMapping(c internalapi.RuntimeService, portMappings []*runtimeapi.PortMapping, hostNet bool) (string, *runtimeapi.PodSandboxConfig) {
	podSandboxName := "create-PodSandbox-with-port-mapping" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

	config := &runtimeapi.PodSandboxConfig{
		Metadata:     framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		PortMappings: portMappings,
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(context.TODO(), c),
		},
		Labels: framework.DefaultPodLabels,
	}
	if hostNet {
		config.Linux.SecurityContext = &runtimeapi.LinuxSandboxSecurityContext{
			NamespaceOptions: &runtimeapi.NamespaceOption{
				Network: runtimeapi.NamespaceMode_NODE,
			},
		}
	}

	podID := framework.RunPodSandbox(c, config)

	return podID, config
}

// checkHostname checks the container hostname.
func checkHostname(c internalapi.RuntimeService, containerID, hostname string) {
	By("get the current hostname via execSync")

	stdout, stderr, err := c.ExecSync(context.TODO(), containerID, getHostnameCmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	framework.ExpectNoError(err, "failed to execSync in container %q", containerID)
	Expect(strings.EqualFold(strings.TrimSpace(string(stdout)), hostname)).To(BeTrue())
	Expect(string(stderr)).To(BeEmpty(), "The stderr should be empty.")
	framework.Logf("check hostname succeed")
}

// checkDNSConfig checks the content of /etc/resolv.conf.
func checkDNSConfig(c internalapi.RuntimeService, containerID string, expectedContent []string) {
	By("get the current dns config via execSync")

	stdout, stderr, err := c.ExecSync(context.TODO(), containerID, getDNSConfigCmd, time.Duration(defaultExecSyncTimeout)*time.Second)
	framework.ExpectNoError(err, "failed to execSync in container %q", containerID)

	for _, content := range expectedContent {
		Expect(string(stdout)).To(ContainSubstring(content), "The stdout output of execSync should contain %q", content)
	}

	Expect(string(stderr)).To(BeEmpty(), "The stderr should be empty.")
	framework.Logf("check DNS config succeed")
}

// createWebServerContainer creates a container running a web server.
func createWebServerContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: webServerImage},
		Linux:    &runtimeapi.LinuxContainerConfig{},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// createHostNetWebServerContainer creates a web server container using webServerHostNetContainerPort.
func createHostNetWebServerContainer(rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: hostNetWebServerImage},
		Linux:    &runtimeapi.LinuxContainerConfig{},
	}

	return framework.CreateContainer(rc, ic, containerConfig, podID, podConfig)
}

// checkMainPage check if the we can get the main page of the pod via given IP:port.
func checkMainPage(c internalapi.RuntimeService, podID string, hostPort, containerPort int32) {
	By("get the IP:port needed to be checked")

	url := "http://"
	if hostPort != 0 {
		url += "127.0.0.1:" + strconv.Itoa(int(hostPort))
	} else {
		status := getPodSandboxStatus(c, podID)
		Expect(status.GetNetwork()).NotTo(BeNil(), "The network in status should not be nil.")
		Expect(status.GetNetwork().GetIp()).NotTo(BeNil(), "The IP should not be nil.")
		url += status.GetNetwork().GetIp() + ":" + strconv.Itoa(int(containerPort))
	}

	framework.Logf("the IP:port is " + url)

	By("check the content of " + url)

	respChan := make(chan *http.Response, 1)
	defer close(respChan)

	Eventually(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		respChan <- resp

		return nil
	}, time.Minute, time.Second).Should(Succeed())

	resp := <-respChan
	Expect(resp.StatusCode).To(Equal(200), "The status code of response should be 200.")
	framework.Logf("check port mapping succeed")
}
