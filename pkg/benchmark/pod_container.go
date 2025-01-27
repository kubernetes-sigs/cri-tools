/*
Copyright 2018 The Kubernetes Authors.

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

package benchmark

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

const (
	defaultPodContainerBenchmarkTimeoutSeconds = 5
)

func getPodContainerBenchmarkTimeoutSeconds() int {
	timeout := defaultPodContainerBenchmarkTimeoutSeconds
	if framework.TestContext.BenchmarkingParams.PodContainerStartBenchmarkTimeoutSeconds > 0 {
		timeout = framework.TestContext.BenchmarkingParams.PodContainerStartBenchmarkTimeoutSeconds
	}

	return timeout
}

var _ = framework.KubeDescribe("PodSandbox", func() {
	f := framework.NewDefaultCRIFramework()

	var (
		experiment *gmeasure.Experiment
		rc         internalapi.RuntimeService
		ic         internalapi.ImageManagerService
	)

	BeforeEach(func() {
		experiment = gmeasure.NewExperiment("start-container-benchmark")
		AddReportEntry(experiment.Name, experiment)

		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("benchmark about start a container from scratch", func() {
		It("benchmark about start a container from scratch", func() {
			podSandboxName := "PodSandbox-for-creating-pod-and-container-performance-test-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			config := &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(context.TODO(), rc),
				},
			}

			benchmark := func() {
				By("run PodSandbox")
				podID, err := rc.RunPodSandbox(context.TODO(), config, framework.TestContext.RuntimeHandler)
				framework.ExpectNoError(err, "failed to create PodSandbox: %v", err)

				By("create container in PodSandbox")
				containerID := framework.CreateDefaultContainer(rc, ic, podID, config, "Pod-Container-for-creating-benchmark-")

				By("start container in PodSandbox")
				err = rc.StartContainer(context.TODO(), containerID)
				framework.ExpectNoError(err, "failed to start Container: %v", err)

				By("stop PodSandbox")
				Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
				By("delete PodSandbox")
				Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			}

			// Run a single test to ensure images are available and everything works
			benchmark()

			// Do the benchmark
			operation := experiment.MeasureDuration("create PodSandbox and container", benchmark)

			Expect(operation.Seconds()).Should(BeNumerically("<", getPodContainerBenchmarkTimeoutSeconds()), "create PodSandbox shouldn't take too long.")
		})
	})
})
