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
	"net/url"

	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	remotecommandconsts "k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

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

		BeforeEach(func() {
			podID, podConfig = createPodSandboxForContainer(rc)
		})

		AfterEach(func() {
			By("stop PodSandbox")
			rc.StopPodSandbox(podID)
			By("delete PodSandbox")
			rc.RemovePodSandbox(podID)
		})

		It("runtime should support exec [Conformance]", func() {
			By("test create a default container")
			containerID := createDefaultContainer(rc, ic, podID, podConfig, "container-for-exec-test")

			By("start container")
			startContainer(rc, containerID)

			req := createDefaultExec(rc, containerID)

			By("check the output of exec")
			checkExec(rc, req)
		})
	})
})

func createDefaultExec(c internalapi.RuntimeService, containerID string) string {
	By("exec default command in container: " + containerID)
	req := &runtimeapi.ExecRequest{
		ContainerId: containerID,
		Cmd:         []string{"echo", "hello"},
	}

	resp, err := c.Exec(req)
	framework.ExpectNoError(err, "failed to exec in container %q", containerID)
	framework.Logf("Get exec url: " + resp.Url)
	return resp.Url
}

func checkExec(c internalapi.RuntimeService, execServerURL string) {
	localOut := &bytes.Buffer{}
	localErr := &bytes.Buffer{}

	url := parseURL(c, execServerURL)
	conf := &rest.Config{
		// Only http is supported now.
		// TODO: support streaming APIs via tls.
		Host: url.Host,
	}
	e, err := remotecommand.NewExecutor(conf, "POST", url)
	framework.ExpectNoError(err, "failed to create executor for %q", execServerURL)

	err = e.Stream(remotecommand.StreamOptions{
		SupportedProtocols: []string{remotecommandconsts.StreamProtocolV1Name},
		Stdin:              nil,
		Stdout:             localOut,
		Stderr:             localErr,
		Tty:                false,
	})
	framework.ExpectNoError(err, "failed to open streamer for %q", execServerURL)

	Expect(localOut.String()).To(Equal("hello\n"), "The stdout of exec should be hello")
	Expect(localErr.String()).To(BeEmpty(), "The stderr of exec should be empty")
	framework.Logf("Check exec url %q succeed", execServerURL)
}

func parseURL(c internalapi.RuntimeService, execServerURL string) *url.URL {
	url, err := url.Parse(execServerURL)
	framework.ExpectNoError(err, "failed to parse url:  %q", execServerURL)

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
	framework.Logf("Parse url %q succeed", execServerURL)
	return url
}
