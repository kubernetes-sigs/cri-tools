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
	defaultPodBenchmarkTimeoutSeconds = 60
)

var _ = framework.KubeDescribe("PodSandbox", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.RuntimeService

	BeforeEach(func() {
		c = f.CRIClient.CRIRuntimeClient
	})

	Context("benchmark about operations on PodSandbox", func() {
		It("benchmark about lifecycle of PodSandbox", func() {
			timeout := defaultPodBenchmarkTimeoutSeconds
			if framework.TestContext.BenchmarkingParams.ContainerBenchmarkTimeoutSeconds > 0 {
				timeout = framework.TestContext.BenchmarkingParams.ContainerBenchmarkTimeoutSeconds
			}

			// Setup sampling config from TestContext:
			samplingConfig := gmeasure.SamplingConfig{
				N:           framework.TestContext.BenchmarkingParams.PodsNumber,
				NumParallel: framework.TestContext.BenchmarkingParams.PodsNumberParallel,
			}
			if samplingConfig.N < 1 {
				samplingConfig.N = 1
			}
			if samplingConfig.NumParallel < 1 {
				samplingConfig.NumParallel = 1
			}

			// Setup results reporting channel:
			resultsSet := LifecycleBenchmarksResultsSet{
				OperationsNames: []string{"CreatePod", "StatusPod", "StopPod", "RemovePod"},
				NumParallel:     samplingConfig.NumParallel,
				Datapoints:      make([]LifecycleBenchmarkDatapoint, 0),
			}
			resultsManager := NewLifecycleBenchmarksResultsManager(
				resultsSet,
				timeout,
			)
			resultsChannel := resultsManager.StartResultsConsumer()

			experiment := gmeasure.NewExperiment("PodLifecycle")
			experiment.Sample(func(idx int) {
				var lastStartTime, lastEndTime int64
				var podID string
				var err error
				durations := make([]int64, len(resultsSet.OperationsNames))

				podSandboxName := "PodSandbox-for-creating-performance-test-" + framework.NewUUID()
				uid := framework.DefaultUIDPrefix + framework.NewUUID()
				namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

				config := &runtimeapi.PodSandboxConfig{
					Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
					Linux:    &runtimeapi.LinuxPodSandboxConfig{},
					Labels:   framework.DefaultPodLabels,
				}

				By("Creating a pod")
				startTime := time.Now().UnixNano()
				lastStartTime = startTime
				podID, err = c.RunPodSandbox(config, framework.TestContext.RuntimeHandler)
				lastEndTime = time.Now().UnixNano()
				durations[0] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to create PodSandbox: %v", err)

				By("Get Pod status")
				lastStartTime = time.Now().UnixNano()
				_, err = c.PodSandboxStatus(podID, true)
				lastEndTime = time.Now().UnixNano()
				durations[1] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to get PodStatus: %v", err)

				By("Stop PodSandbox")
				lastStartTime = time.Now().UnixNano()
				err = c.StopPodSandbox(podID)
				lastEndTime = time.Now().UnixNano()
				durations[2] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to stop PodSandbox: %v", err)

				By("Remove PodSandbox")
				lastStartTime = time.Now().UnixNano()
				err = c.RemovePodSandbox(podID)
				lastEndTime = time.Now().UnixNano()
				durations[3] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to remove PodSandbox: %v", err)

				res := LifecycleBenchmarkDatapoint{
					SampleIndex:           idx,
					StartTime:             startTime,
					EndTime:               lastEndTime,
					OperationsDurationsNs: durations,
					MetaInfo:              map[string]string{"podId": podID, "podSandboxName": podSandboxName},
				}
				resultsChannel <- &res

			}, samplingConfig)

			// Send nil and give the manager a minute to process any already-queued results:
			resultsChannel <- nil
			err := resultsManager.AwaitAllResults(60)
			if err != nil {
				glog.Errorf("Results manager failed to await all results: %s", err)
			}

			if framework.TestContext.BenchmarkingOutputDir != "" {
				filepath := path.Join(framework.TestContext.BenchmarkingOutputDir, "pod_benchmark_data.json")
				err = resultsManager.WriteResultsFile(filepath)
				if err != nil {
					glog.Errorf("Error occurred while writing benchmark results to file %s: %s", filepath, err)
				}
			} else {
				glog.Infof("No benchmarking out dir provided, skipping writing benchmarking results.")
				glog.Infof("Benchmark results were: %+v", resultsManager.resultsSet)
			}
		})
	})
})
