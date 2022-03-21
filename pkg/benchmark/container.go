/*
Copyright 2022 The Kubernetes Authors.

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
	"path"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/cri-tools/pkg/framework"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/gmeasure"
)

const (
	defaultContainerBenchmarkTimeoutSeconds = 60
)

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
			timeout := defaultContainerBenchmarkTimeoutSeconds
			if framework.TestContext.BenchmarkingParams.ContainerBenchmarkTimeoutSeconds > 0 {
				timeout = framework.TestContext.BenchmarkingParams.ContainerBenchmarkTimeoutSeconds
			}

			// Setup sampling config from TestContext:
			samplingConfig := gmeasure.SamplingConfig{
				N:           framework.TestContext.BenchmarkingParams.ContainersNumber,
				NumParallel: framework.TestContext.BenchmarkingParams.ContainersNumberParallel,
			}
			if samplingConfig.N < 1 {
				samplingConfig.N = 1
			}
			if samplingConfig.NumParallel < 1 {
				samplingConfig.NumParallel = 1
			}

			// Setup results reporting channel:
			resultsSet := LifecycleBenchmarksResultsSet{
				OperationsNames: []string{"CreateContainer", "StartContainer", "StatusContainer", "StopContainer", "RemoveContainer"},
				NumParallel:     samplingConfig.NumParallel,
				Datapoints:      make([]LifecycleBenchmarkDatapoint, 0),
			}
			resultsManager := NewLifecycleBenchmarksResultsManager(
				resultsSet,
				timeout,
			)
			resultsChannel := resultsManager.StartResultsConsumer()

			experiment := gmeasure.NewExperiment("ContainerOps")
			experiment.Sample(func(idx int) {
				var podID string
				var podConfig *runtimeapi.PodSandboxConfig
				var containerID string
				var lastStartTime, lastEndTime int64
				var err error
				durations := make([]int64, len(resultsSet.OperationsNames))

				podID, podConfig = framework.CreatePodSandboxForContainer(rc)

				By("CreatingContainer")
				startTime := time.Now().UnixNano()
				lastStartTime = startTime
				containerID = framework.CreateDefaultContainer(rc, ic, podID, podConfig, "Benchmark-container-")
				lastEndTime = time.Now().UnixNano()
				durations[0] = lastEndTime - lastStartTime

				By("StartingContainer")
				lastStartTime = time.Now().UnixNano()
				err = rc.StartContainer(containerID)
				lastEndTime = time.Now().UnixNano()
				durations[1] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to start Container: %v", err)

				By("ContainerStatus")
				lastStartTime = time.Now().UnixNano()
				_, err = rc.ContainerStatus(containerID, true)
				lastEndTime = time.Now().UnixNano()
				durations[2] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to get Container status: %v", err)

				By("ContainerStop")
				lastStartTime = time.Now().UnixNano()
				err = rc.StopContainer(containerID, framework.DefaultStopContainerTimeout)
				lastEndTime = time.Now().UnixNano()
				durations[3] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to stop Container: %v", err)

				By("ContainerRemove")
				lastStartTime = time.Now().UnixNano()
				err = rc.RemoveContainer(containerID)
				lastEndTime = time.Now().UnixNano()
				durations[4] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to remove Container: %v", err)

				res := LifecycleBenchmarkDatapoint{
					SampleIndex:           idx,
					StartTime:             startTime,
					EndTime:               lastEndTime,
					OperationsDurationsNs: durations,
					MetaInfo:              map[string]string{"podId": podID, "containerId": containerID},
				}
				resultsChannel <- &res

				By("stop PodSandbox")
				rc.StopPodSandbox(podID)
				By("delete PodSandbox")
				rc.RemovePodSandbox(podID)

			}, samplingConfig)

			// Send nil and give the manager a minute to process any already-queued results:
			resultsChannel <- nil
			err := resultsManager.AwaitAllResults(60)
			if err != nil {
				glog.Errorf("Results manager failed to await all results: %s", err)
			}

			if framework.TestContext.BenchmarkingOutputDir != "" {
				filepath := path.Join(framework.TestContext.BenchmarkingOutputDir, "container_benchmark_data.json")
				err = resultsManager.WriteResultsFile(filepath)
				if err != nil {
					glog.Errorf("Error occurred while writing benchmark results to file %s: %s", filepath, err)
				}
			} else {
				glog.Infof("No benchmarking output dir provided, skipping writing benchmarking results file.")
				glog.Infof("Benchmark results were: %+v", resultsManager.resultsSet)
			}
		})
	})
})
