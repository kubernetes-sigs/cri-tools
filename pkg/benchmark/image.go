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

package benchmark

import (
	"context"
	"fmt"
	"path"
	"runtime"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/cri-tools/pkg/framework"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega/gmeasure"
)

const (
	defaultImageBenchmarkTimeoutSeconds = 10
)

var defaultImageListingBenchmarkImagesAmd64 = []string{
	"busybox:1.26.2-glibc",
	"busybox:1-uclibc",
	"busybox:1",
	"busybox:1-glibc",
	"busybox:1-musl",
}
var defaultImageListingBenchmarkImages = []string{
	"busybox:1",
	"busybox:1-glibc",
	"busybox:1-musl",
}

var _ = framework.KubeDescribe("Image", func() {
	var ic internalapi.ImageManagerService
	f := framework.NewDefaultCRIFramework()

	var testImageList []string = framework.TestContext.BenchmarkingParams.ImageListingBenchmarkImages
	if len(testImageList) == 0 {
		if runtime.GOARCH == "amd64" {
			testImageList = defaultImageListingBenchmarkImagesAmd64
		} else {
			testImageList = defaultImageListingBenchmarkImages
		}
	}

	BeforeEach(func() {
		ic = f.CRIClient.CRIImageClient
	})

	AfterEach(func() {
		for _, imageName := range testImageList {
			imageSpec := &runtimeapi.ImageSpec{
				Image: imageName,
			}
			ic.RemoveImage(context.TODO(), imageSpec)
		}
	})

	Context("benchmark about operations on Image", func() {
		It("benchmark about basic operations on Image", func() {
			var err error

			imageBenchmarkTimeoutSeconds := defaultImageBenchmarkTimeoutSeconds
			if framework.TestContext.BenchmarkingParams.ImageBenchmarkTimeoutSeconds > 0 {
				imageBenchmarkTimeoutSeconds = framework.TestContext.BenchmarkingParams.ImageBenchmarkTimeoutSeconds
			}

			imagePullingBenchmarkImage := framework.TestContext.BenchmarkingParams.ImagePullingBenchmarkImage
			// NOTE(aznashwan): default to using first test image from listing benchmark images:
			if imagePullingBenchmarkImage == "" {
				imagePullingBenchmarkImage = testImageList[0]
				glog.Infof("Defaulting to using following image: %s", imagePullingBenchmarkImage)
			}

			// Setup shared sampling config from TestContext:
			samplingConfig := gmeasure.SamplingConfig{
				N:           framework.TestContext.BenchmarkingParams.ImagesNumber,
				NumParallel: framework.TestContext.BenchmarkingParams.ImagesNumberParallel,
			}
			if samplingConfig.N <= 0 {
				Skip("skipping image lifecycle benchmarks since image number option was not set")
			}
			if samplingConfig.NumParallel < 1 {
				samplingConfig.NumParallel = 1
			}

			// Setup image lifecycle results reporting channel:
			lifecycleResultsSet := LifecycleBenchmarksResultsSet{
				OperationsNames: []string{"PullImage", "StatusImage", "RemoveImage"},
				NumParallel:     samplingConfig.NumParallel,
				Datapoints:      make([]LifecycleBenchmarkDatapoint, 0),
			}
			lifecycleResultsManager := NewLifecycleBenchmarksResultsManager(
				lifecycleResultsSet,
				imageBenchmarkTimeoutSeconds,
			)
			lifecycleResultsChannel := lifecycleResultsManager.StartResultsConsumer()

			// Image lifecycle benchmark experiment:
			experiment := gmeasure.NewExperiment("ImageLifecycle")
			experiment.Sample(func(idx int) {
				var err error
				var lastStartTime, lastEndTime int64
				durations := make([]int64, len(lifecycleResultsSet.OperationsNames))

				imageSpec := &runtimeapi.ImageSpec{
					Image: imagePullingBenchmarkImage,
				}

				By(fmt.Sprintf("Pull Image %d", idx))
				startTime := time.Now().UnixNano()
				lastStartTime = startTime
				imageId := framework.PullPublicImage(ic, imagePullingBenchmarkImage, nil)
				lastEndTime = time.Now().UnixNano()
				durations[0] = lastEndTime - lastStartTime

				By(fmt.Sprintf("Status Image %d", idx))
				lastStartTime = time.Now().UnixNano()
				_, err = ic.ImageStatus(context.TODO(), imageSpec, false)
				lastEndTime = time.Now().UnixNano()
				durations[1] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to status Image: %v", err)

				By(fmt.Sprintf("Remove Image %d", idx))
				lastStartTime = time.Now().UnixNano()
				err = ic.RemoveImage(context.TODO(), imageSpec)
				lastEndTime = time.Now().UnixNano()
				durations[2] = lastEndTime - lastStartTime
				framework.ExpectNoError(err, "failed to remove Image: %v", err)

				res := LifecycleBenchmarkDatapoint{
					SampleIndex:           idx,
					StartTime:             startTime,
					EndTime:               lastEndTime,
					OperationsDurationsNs: durations,
					MetaInfo:              map[string]string{"imageId": imageId},
				}
				lifecycleResultsChannel <- &res

			}, samplingConfig)

			// Send nil and give the manager a minute to process any already-queued results:
			lifecycleResultsChannel <- nil
			err = lifecycleResultsManager.AwaitAllResults(60)
			if err != nil {
				glog.Errorf("Results manager failed to await all results: %s", err)
			}

			if framework.TestContext.BenchmarkingOutputDir != "" {
				filepath := path.Join(framework.TestContext.BenchmarkingOutputDir, "image_lifecycle_benchmark_data.json")
				err = lifecycleResultsManager.WriteResultsFile(filepath)
				if err != nil {
					glog.Errorf("Error occurred while writing benchmark results to file %s: %s", filepath, err)
				}
			} else {
				glog.Infof("No benchmarking out dir provided, skipping writing benchmarking results.")
				glog.Infof("Image lifecycle results were: %+v", lifecycleResultsManager.resultsSet)
			}
		})

		It("benchmark about listing Image", func() {
			var err error

			imageBenchmarkTimeoutSeconds := defaultImageBenchmarkTimeoutSeconds
			if framework.TestContext.BenchmarkingParams.ImageBenchmarkTimeoutSeconds > 0 {
				imageBenchmarkTimeoutSeconds = framework.TestContext.BenchmarkingParams.ImageBenchmarkTimeoutSeconds
			}

			// Setup shared sampling config from TestContext:
			samplingConfig := gmeasure.SamplingConfig{
				N:           framework.TestContext.BenchmarkingParams.ImagesNumber,
				NumParallel: framework.TestContext.BenchmarkingParams.ImagesNumberParallel,
			}
			if samplingConfig.N <= 0 {
				Skip("skipping image listing benchmarks since image listing number option was not set")
			}
			if samplingConfig.NumParallel < 1 {
				samplingConfig.NumParallel = 1
			}
			// Setup image lifecycle results reporting channel:
			imageListResultsSet := LifecycleBenchmarksResultsSet{
				OperationsNames: []string{"ListImages"},
				NumParallel:     samplingConfig.NumParallel,
				Datapoints:      make([]LifecycleBenchmarkDatapoint, 0),
			}
			imageListResultsManager := NewLifecycleBenchmarksResultsManager(
				imageListResultsSet,
				imageBenchmarkTimeoutSeconds,
			)
			imagesResultsChannel := imageListResultsManager.StartResultsConsumer()

			// Image listing benchmark experiment:
			experiment := gmeasure.NewExperiment("ImageListing")
			experiment.Sample(func(idx int) {
				var err error
				durations := make([]int64, len(imageListResultsSet.OperationsNames))

				By(fmt.Sprintf("List Images %d", idx))
				startTime := time.Now().UnixNano()
				_, err = ic.ListImages(context.TODO(), nil)
				endTime := time.Now().UnixNano()
				durations[0] = endTime - startTime
				framework.ExpectNoError(err, "failed to List images: %v", err)

				res := LifecycleBenchmarkDatapoint{
					SampleIndex:           idx,
					StartTime:             startTime,
					EndTime:               endTime,
					OperationsDurationsNs: durations,
					MetaInfo:              nil,
				}
				imagesResultsChannel <- &res

			}, samplingConfig)

			// Send nil and give the manager a minute to process any already-queued results:
			imagesResultsChannel <- nil
			err = imageListResultsManager.AwaitAllResults(60)
			if err != nil {
				glog.Errorf("Results manager failed to await all results: %s", err)
			}

			if framework.TestContext.BenchmarkingOutputDir != "" {
				filepath := path.Join(framework.TestContext.BenchmarkingOutputDir, "image_listing_benchmark_data.json")
				err = imageListResultsManager.WriteResultsFile(filepath)
				if err != nil {
					glog.Errorf("Error occurred while writing benchmark results to file %s: %s", filepath, err)
				}
			} else {
				glog.Infof("No benchmarking out dir provided, skipping writing benchmarking results.")
				glog.Infof("Image listing results were: %+v", imageListResultsManager.resultsSet)
			}
		})
	})
})
