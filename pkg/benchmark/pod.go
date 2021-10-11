//go:build pod
// +build pod

/*
Copyright 2021 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/kubernetes-sigs/cri-tools/pkg/framework"
	"github.com/onsi/gomega/gmeasure"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	defaultOperationTimes int = 20
)

type ExperimentData struct {
	CreatePod, StatusPod, StopPod, RemovePod string
}

var _ = framework.KubeDescribe("PodSandbox", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.RuntimeService

	BeforeEach(func() {
		c = f.CRIClient.CRIRuntimeClient
	})

	Context("benchmark about operations on PodSandbox", func() {
		It("benchmark about lifecycle of PodSandbox", func() {

			experiment := gmeasure.NewExperiment("PodLifecycle")
			experiment.Sample(func(idx int) {
				var podID string
				var err error

				podSandboxName := "PodSandbox-for-creating-performance-test-" + framework.NewUUID()
				uid := framework.DefaultUIDPrefix + framework.NewUUID()
				namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

				config := &runtimeapi.PodSandboxConfig{
					Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
					Linux:    &runtimeapi.LinuxPodSandboxConfig{},
					Labels:   framework.DefaultPodLabels,
				}

				By("Creating a pod")
				stopwatch := experiment.NewStopwatch()

				podID, err = c.RunPodSandbox(config, framework.TestContext.RuntimeHandler)
				stopwatch.Record("CreatePod")
				framework.ExpectNoError(err, "failed to create PodSandbox: %v", err)

				By("Get Pod status")
				stopwatch.Reset()
				_, err = c.PodSandboxStatus(podID)
				stopwatch.Record("StatusPod")
				framework.ExpectNoError(err, "failed to get PodStatus: %v", err)

				By("Stop PodSandbox")
				stopwatch.Reset()
				err = c.StopPodSandbox(podID)
				stopwatch.Record("StopPod")
				framework.ExpectNoError(err, "failed to stop PodSandbox: %v", err)

				By("Remove PodSandbox")
				stopwatch.Reset()
				err = c.RemovePodSandbox(podID)
				stopwatch.Record("RemovePod")
				framework.ExpectNoError(err, "failed to remove PodSandbox: %v", err)

			}, gmeasure.SamplingConfig{N: 1000, NumParallel: 1})

			data := ExperimentData{
				CreatePod: fmt.Sprintf("%v", experiment.Get("CreatePod").Durations),
				StatusPod: fmt.Sprintf("%v", experiment.Get("StatusPod").Durations),
				StopPod:   fmt.Sprintf("%v", experiment.Get("StopPod").Durations),
				RemovePod: fmt.Sprintf("%v", experiment.Get("RemovePod").Durations),
			}

			file, _ := json.MarshalIndent(data, "", " ")
			_ = ioutil.WriteFile("c:/experiment_old_hcsshim.json", file, 0644)
		})
	})

})
