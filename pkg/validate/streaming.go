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
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	remoteclient "k8s.io/client-go/tools/remotecommand"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

const defaultExecStdinCloseTimeout = 20 * time.Second

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
			Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			By("delete PodSandbox")
			Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
		})

		It("runtime should support exec with tty=false and stdin=false [Conformance]", func() {
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create a default container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-exec-test")

			By("start container")
			startContainer(rc, containerID)

			execReq := &runtimeapi.ExecRequest{
				ContainerId: containerID,
				Cmd:         echoHelloCmd,
				Stdout:      true,
				Stderr:      true,
			}
			req := createExec(rc, execReq)

			By("check the output of exec")
			checkExec(rc, req, echoHelloOutput, true, false)
		})

		It("runtime should support exec with tty=true and stdin=true [Conformance]", func() {
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create a default container")
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-exec-test")

			By("start container")
			startContainer(rc, containerID)

			execReq := &runtimeapi.ExecRequest{
				ContainerId: containerID,
				Cmd:         echoHelloCmd,
				Stdout:      true,
				Tty:         true,
				Stdin:       true,
			}
			req := createExec(rc, execReq)

			By("check the output of exec")
			checkExec(rc, req, "hello", false, true)
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
			By("create a PodSandbox with container port mapping")
			var podConfig *runtimeapi.PodSandboxConfig
			portMappings := []*runtimeapi.PortMapping{
				{
					ContainerPort: webServerContainerPort,
				},
			}
			podID, podConfig = createPodSandboxWithPortMapping(rc, portMappings, false)

			By("create a web server container")
			containerID := createWebServerContainer(rc, ic, podID, podConfig, "container-for-portforward-test")

			By("start the web server container")
			startContainer(rc, containerID)

			By("ensure the web server container is serving")
			checkMainPage(rc, podID, 0, webServerContainerPort)

			req := createDefaultPortForward(rc, podID)

			By("check the output of portforward")
			checkPortForward(rc, req, webServerHostPortForPortForward, webServerContainerPort)
		})
	})
})

func createExec(c internalapi.RuntimeService, execReq *runtimeapi.ExecRequest) string {
	By("exec given command in container: " + execReq.GetContainerId())
	resp, err := c.Exec(context.TODO(), execReq)
	framework.ExpectNoError(err, "failed to exec in container %q", execReq.GetContainerId())
	framework.Logf("Get exec URL: " + resp.GetUrl())

	return resp.GetUrl()
}

func checkExec(c internalapi.RuntimeService, execServerURL, stdout string, stdoutExactMatch, isTty bool) {
	var (
		localOut                  = &safeBuffer{buffer: bytes.Buffer{}}
		localErr                  = &safeBuffer{buffer: bytes.Buffer{}}
		localInRead, localInWrite = io.Pipe()
		testDone                  = make(chan struct{})
		wg                        sync.WaitGroup
	)

	// Wait until output read and then shutdown localIn pipe.
	wg.Go(func() {
		defer localInWrite.Close()

		ticker := time.NewTicker(defaultExecStdinCloseTimeout)
		select {
		case <-testDone:
		case <-ticker.C:
		}
	})

	defer func() {
		close(testDone)
		wg.Wait()
	}()

	// Only http is supported now.
	// TODO: support streaming APIs via tls.
	parsedURL := parseURL(c, execServerURL)

	transport := common.TransportSpdy
	if framework.TestContext.UseWebsocketForExec {
		transport = common.TransportWebsocket
	}

	e, err := common.GetExecutor(transport, parsedURL, &rest.TLSClientConfig{Insecure: true})
	framework.ExpectNoError(err, "failed to create executor for %q", execServerURL)

	streamOptions := remoteclient.StreamOptions{
		Stdout: localOut,
		Stderr: localErr,
		Tty:    false,
	}
	if isTty {
		streamOptions.Stdin = localInRead
		streamOptions.Tty = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = e.StreamWithContext(ctx, streamOptions)
	framework.ExpectNoError(err, "failed to open streamer for %q", execServerURL)

	if stdoutExactMatch {
		Expect(localOut.String()).To(Equal(stdout), "The stdout of exec should be "+stdout)
	} else {
		Expect(localOut.String()).To(ContainSubstring(stdout), "The stdout of exec should contain "+stdout)
	}

	Expect(localErr.String()).To(BeEmpty(), "The stderr of exec should be empty")
	framework.Logf("Check exec URL %q succeed", execServerURL)
}

func parseURL(c internalapi.RuntimeService, serverURL string) *url.URL {
	parsedURL, err := url.Parse(serverURL)
	framework.ExpectNoError(err, "failed to parse URL:  %q", serverURL)

	version := getVersion(c)
	if version.GetRuntimeName() == "docker" {
		if parsedURL.Host == "" {
			parsedURL.Host = defaultStreamServerAddress
		}

		if parsedURL.Scheme == "" {
			parsedURL.Scheme = defaultStreamServerScheme
		}
	}

	Expect(parsedURL.Host).NotTo(BeEmpty(), "The host of URL should not be empty")
	framework.Logf("Parse URL %q succeed", serverURL)

	return parsedURL
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

	resp, err := c.Attach(context.TODO(), req)
	framework.ExpectNoError(err, "failed to attach in container %q", containerID)
	framework.Logf("Get attach URL: " + resp.GetUrl())

	return resp.GetUrl()
}

// safeBuffer is a goroutine safe bytes.Buffer.
type safeBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

// Write appends the contents of p to the buffer, growing the buffer as needed. It returns
// the number of bytes written.
func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.buffer.Write(p)
}

// String returns the contents of the unread portion of the buffer
// as a string.  If the Buffer is a nil pointer, it returns "<nil>".
func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.buffer.String()
}

func checkAttach(c internalapi.RuntimeService, attachServerURL string) {
	localOut := &safeBuffer{buffer: bytes.Buffer{}}
	localErr := &safeBuffer{buffer: bytes.Buffer{}}
	reader, writer := io.Pipe()

	go func() {
		defer GinkgoRecover()
		defer writer.Close()

		header := localOut.String()

		time.Sleep(1 * time.Second)
		Eventually(func() bool {
			oldHeader := header
			header = localOut.String()

			return len(header) == len(oldHeader)
		}, 10*time.Second, time.Second).Should(BeTrue(), "The container should stop output when there is no input")

		_, err := writer.Write([]byte(strings.Join(echoHelloCmd, " ") + "\n"))
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() string {
			return strings.TrimPrefix(localOut.String(), header)
		}, time.Minute, time.Second).Should(Equal(attachEchoHelloOutput), "The stdout of attach should be hello")
		Consistently(func() string {
			return strings.TrimPrefix(localOut.String(), header)
		}, 3*time.Second, time.Second).Should(Equal(attachEchoHelloOutput), "The stdout of attach should not contain other things")
	}()

	// Only http is supported now.
	// TODO: support streaming APIs via tls.
	parsedURL := parseURL(c, attachServerURL)

	transport := common.TransportSpdy
	if framework.TestContext.UseWebsocketForAttach {
		transport = common.TransportWebsocket
	}

	e, err := common.GetExecutor(transport, parsedURL, &rest.TLSClientConfig{Insecure: true})
	framework.ExpectNoError(err, "failed to create executor for %q", attachServerURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = e.StreamWithContext(ctx, remoteclient.StreamOptions{
		Stdin:  reader,
		Stdout: localOut,
		Stderr: localErr,
		Tty:    false,
	})
	framework.ExpectNoError(err, "failed to open streamer for %q", attachServerURL)

	Expect(localErr.String()).To(BeEmpty(), "The stderr of attach should be empty")
	framework.Logf("Check attach URL %q succeed", attachServerURL)
}

func createDefaultPortForward(c internalapi.RuntimeService, podID string) string {
	By("port forward PodSandbox: " + podID)
	req := &runtimeapi.PortForwardRequest{
		PodSandboxId: podID,
	}

	resp, err := c.PortForward(context.TODO(), req)
	framework.ExpectNoError(err, "failed to port forward PodSandbox %q", podID)
	framework.Logf("Get port forward URL: " + resp.GetUrl())

	return resp.GetUrl()
}

func checkPortForward(c internalapi.RuntimeService, portForwardSeverURL string, hostPort, containerPort int32) {
	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	defer close(stopChan)

	parsedURL := parseURL(c, portForwardSeverURL)

	transport := common.TransportSpdy
	if framework.TestContext.UseWebsocketForPortForward {
		transport = common.TransportWebsocket
	}

	dialer, err := common.GetDialer(transport, parsedURL, &rest.TLSClientConfig{Insecure: true})
	framework.ExpectNoError(err, "failed to create port forward dialer for %q", portForwardSeverURL)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	pf, err := portforward.New(dialer, []string{fmt.Sprintf("%d:%d", hostPort, containerPort)}, stopChan, readyChan, stdout, stderr)
	framework.ExpectNoError(err, "failed to create port forward for %q", portForwardSeverURL)

	go func() {
		defer GinkgoRecover()

		By("start port forward")

		err := pf.ForwardPorts()
		framework.ExpectNoError(err, "failed to start port forward for %q, stdout: %s, stderr: %s", portForwardSeverURL, stdout.String(), stderr.String())
	}()

	By(fmt.Sprintf("check if we can get nginx main page via localhost:%d", hostPort))
	checkMainPage(c, "", hostPort, 0)
	framework.Logf("Check port forward URL %q succeed", portForwardSeverURL)
}
