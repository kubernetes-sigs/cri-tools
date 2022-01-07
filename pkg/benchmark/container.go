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
	"io/ioutil"
	"path"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/cri-tools/pkg/framework"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/gmeasure"
)

type ContainerExperimentData struct {
	CreateContainer, StatusContainer, StopContainer, RemoveContainer, StartContainer []int64
}

var _ = framework.KubeDescribe("Container", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService
	var ic internalapi.ImageManagerService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
		ic = f.CRIClient.CRIImageClient
	})

	Context("benchmark about operations on Container", func() {
		It("benchmark about basic operations on Container", func() {
			experiment := gmeasure.NewExperiment("ContainerOps")
			experiment.Sample(func(idx int) {
				var podID string
				var podConfig *runtimeapi.PodSandboxConfig
				var containerID string
				var err error

				podID, podConfig = framework.CreatePodSandboxForContainer(rc)

				By("CreatingContainer")
				stopwatch := experiment.NewStopwatch()
				stopwatch.Reset()
				containerID = framework.CreateDefaultContainer(rc, ic, podID, podConfig, "Benchmark-container-")
				stopwatch.Record("CreateContainer")

				By("StartingContainer")
				stopwatch.Reset()
				err = rc.StartContainer(containerID)
				stopwatch.Record("StartContainer")
				framework.ExpectNoError(err, "failed to start Container: %v", err)

				By("ContainerStatus")
				stopwatch.Reset()
				_, err = rc.ContainerStatus(containerID)
				stopwatch.Record("StatusContainer")
				framework.ExpectNoError(err, "failed to get Container status: %v", err)

				By("ContainerStop")
				stopwatch.Reset()
				err = rc.StopContainer(containerID, framework.DefaultStopContainerTimeout)
				stopwatch.Record("StopContainer")
				framework.ExpectNoError(err, "failed to stop Container: %v", err)

				By("ContainerRemove")
				stopwatch.Reset()
				err = rc.RemoveContainer(containerID)
				stopwatch.Record("RemoveContainer")
				framework.ExpectNoError(err, "failed to remove Container: %v", err)

				By("stop PodSandbox")
				rc.StopPodSandbox(podID)
				By("delete PodSandbox")
				rc.RemovePodSandbox(podID)

			}, gmeasure.SamplingConfig{N: framework.TestContext.BenchmarkingParams.ContainersNumber, NumParallel: framework.TestContext.BenchmarkingParams.ContainersNumberParallel})

			data := ContainerExperimentData{
				CreateContainer: getNanosecondsForDurations(experiment.Get("CreateContainer").Durations),
				StartContainer:  getNanosecondsForDurations(experiment.Get("StartContainer").Durations),
				StatusContainer: getNanosecondsForDurations(experiment.Get("StatusContainer").Durations),
				StopContainer:   getNanosecondsForDurations(experiment.Get("StopContainer").Durations),
				RemoveContainer: getNanosecondsForDurations(experiment.Get("RemoveContainer").Durations),
			}

			if framework.TestContext.BenchmarkingOutputDir != "" {
				filepath := path.Join(framework.TestContext.BenchmarkingOutputDir, "container_benchmark_data.json")
				data, err := json.MarshalIndent(data, "", " ")
				if err == nil {
					err = ioutil.WriteFile(filepath, data, 0644)
					if err != nil {
						glog.Errorf("Failed to write container benchmark data: %v", filepath)
					}
				} else {
					glog.Errorf("Failed to serialize benchmark data: %v", err)
				}
			} else {
				glog.Infof("No benchmarking output dir provided, skipping writing benchmarking resulsts.")
			}
		})

	})
})
