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
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	remoteclient "k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	internalapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	defaultStreamServerAddress string = "127.0.0.1:10250"
	defaultStreamServerScheme  string = "http"
)

var _ = framework.KubeDescribe("Streaming", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("runtime should support streaming interfaces", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support exec [Conformance]", func() {
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create a default container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-exec-test")

			By("start container")
			startContainer(rc, containerID)

			req := createDefaultExec(rc, containerID)

			By("check the output of exec")
			checkExec(rc, req)
		})

		It("runtime should support attach [Conformance]", func() {
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create a default container")
			containerID := createShellContainer(rc, ic, podID, podConfig, "container-for-attach-test")

			By("start container")
			startContainer(rc, containerID)

			req := createDefaultAttach(rc, containerID)

			By("check the output of attach")
			checkAttach(rc, req)
		})

		It("runtime should support portforward [Conformance]", func() {
			By("create a PodSandbox with host port and container port port mapping")
			var podConfig *runtimeapi.PodSandboxConfig
			portMappings := []*runtimeapi.PortMapping{
				{
					ContainerPort: nginxContainerPort,
				},
			}
			podID, podConfig = createPodSandboxWithPortMapping(rc, portMappings)

			By("create a nginx container")
			containerID := createNginxContainer(rc, ic, podID, podConfig, "container-for-portforward-test")

			By("start the nginx container")
			startContainer(rc, containerID)

			req := createDefaultPortForward(rc, podID)

			By("check the output of portforward")
			checkPortForward(rc, req)
		})
	})
})

func createDefaultExec(c internalapi.RuntimeService, containerID string) string {
	By("exec default command in container: " + containerID)
	req := &runtimeapi.ExecRequest{
		ContainerId: containerID,
		Cmd:         []string{"echo", "hello"},
		Stdout:      true,
		Stderr:      true,
	}

	resp, err := c.Exec(req)
	framework.ExpectNoError(err, "failed to exec in container %q", containerID)
	framework.Logf("Get exec url: " + resp.Url)
	return resp.Url
}

func checkExec(c internalapi.RuntimeService, execServerURL string) {
	localOut := &bytes.Buffer{}
	localErr := &bytes.Buffer{}

	// Only http is supported now.
	// TODO: support streaming APIs via tls.
	url := parseURL(c, execServerURL)
	e, err := remoteclient.NewSPDYExecutor(&rest.Config{}, "POST", url)
	framework.ExpectNoError(err, "failed to create executor for %q", execServerURL)

	err = e.Stream(remoteclient.StreamOptions{
		Stdout: localOut,
		Stderr: localErr,
		Tty:    false,
	})
	framework.ExpectNoError(err, "failed to open streamer for %q", execServerURL)

	Expect(localOut.String()).To(Equal("hello\n"), "The stdout of exec should be hello")
	Expect(localErr.String()).To(BeEmpty(), "The stderr of exec should be empty")
	framework.Logf("Check exec url %q succeed", execServerURL)
}

func parseURL(c internalapi.RuntimeService, serverURL string) *url.URL {
	url, err := url.Parse(serverURL)
	framework.ExpectNoError(err, "failed to parse url:  %q", serverURL)

	version := getVersion(c)
	if version.RuntimeName == "docker" {
		if url.Host == "" {
			url.Host = defaultStreamServerAddress
		}
		if url.Scheme == "" {
			url.Scheme = defaultStreamServerScheme
		}
	}

	Expect(url.Host).NotTo(BeEmpty(), "The host of url should not be empty")
	framework.Logf("Parse url %q succeed", serverURL)
	return url
}

func createDefaultAttach(c internalapi.RuntimeService, containerID string) string {
	By("attach container: " + containerID)
	req := &runtimeapi.AttachRequest{
		ContainerId: containerID,
		Stdin:       true,
		Stdout:      true,
		Stderr:      true,
		Tty:         false,
	}

	resp, err := c.Attach(req)
	framework.ExpectNoError(err, "failed to attach in container %q", containerID)
	framework.Logf("Get attach url: " + resp.Url)
	return resp.Url
}

func checkAttach(c internalapi.RuntimeService, attachServerURL string) {
	localOut := &bytes.Buffer{}
	localErr := &bytes.Buffer{}
	reader, writer := io.Pipe()
	var out string

	go func() {
		defer GinkgoRecover()
		writer.Write([]byte("echo hello\n"))
		Eventually(func() string {
			out = localOut.String()
			return out
		}, time.Minute, time.Second).ShouldNot(BeEmpty())
		writer.Close()
	}()

	// Only http is supported now.
	// TODO: support streaming APIs via tls.
	url := parseURL(c, attachServerURL)
	e, err := remoteclient.NewSPDYExecutor(&rest.Config{}, "POST", url)
	framework.ExpectNoError(err, "failed to create executor for %q", attachServerURL)

	err = e.Stream(remoteclient.StreamOptions{
		Stdin:  reader,
		Stdout: localOut,
		Stderr: localErr,
		Tty:    false,
	})
	framework.ExpectNoError(err, "failed to open streamer for %q", attachServerURL)

	Expect(out).To(Equal("hello\n"), "The stdout of exec should be hello")
	Expect(localErr.String()).To(BeEmpty(), "The stderr of attach should be empty")
	framework.Logf("Check attach url %q succeed", attachServerURL)
}

func createDefaultPortForward(c internalapi.RuntimeService, podID string) string {
	By("port forward PodSandbox: " + podID)
	req := &runtimeapi.PortForwardRequest{
		PodSandboxId: podID,
	}

	resp, err := c.PortForward(req)
	framework.ExpectNoError(err, "failed to port forward PodSandbox %q", podID)
	framework.Logf("Get port forward url: " + resp.Url)
	return resp.Url
}

func checkPortForward(c internalapi.RuntimeService, portForwardSeverURL string) {
	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})
	defer close(stopChan)

	transport, upgrader, err := spdy.RoundTripperFor(&rest.Config{})
	framework.ExpectNoError(err, "failed to create spdy round tripper")
	url := parseURL(c, portForwardSeverURL)
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)
	pf, err := portforward.New(dialer, []string{"8000:80"}, stopChan, readyChan, os.Stdout, os.Stderr)
	framework.ExpectNoError(err, "failed to create port forward for %q", portForwardSeverURL)

	go func() {
		defer GinkgoRecover()
		By("start port forward")
		err = pf.ForwardPorts()
		framework.ExpectNoError(err, "failed to start port forward for %q", portForwardSeverURL)
	}()

	By("check if we can get nginx main page via localhost:8000")
	checkNginxMainPage(c, "", true)
	framework.Logf("Check port forward url %q succeed", portForwardSeverURL)
}
