/*
Copyright The Kubernetes Authors.

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
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

const (
	listStreamingLabel = "critest.kubernetes.io/list-streaming"
	// cri-client's normal receive limit for a single gRPC message.
	listStreamingMaxMsgSize = 16 * 1024 * 1024
)

var _ = framework.KubeDescribe("CRI list streaming", func() {
	f := framework.NewDefaultCRIFramework()

	It(
		"should list more than 5000 containers beyond the unary message size limit [Conformance]",
		Serial,
		func(ctx SpecContext) {
			const (
				payloadSize      = 2 * 1024
				containerCount   = listStreamingMaxMsgSize/payloadSize + 1
				containersPerPod = 128
				podCount         = (containerCount + containersPerPod - 1) / containersPerPod
				payloadKey       = "critest.kubernetes.io/list-streaming-payload"
				workers          = 8
			)

			runtimeClient := f.CRIClient.CRIRuntimeClient
			pods := make([]string, podCount)
			podConfigs := make([]*runtimeapi.PodSandboxConfig, podCount)
			// Container events include pod-wide status snapshots. Keep each pod small
			// so fixture setup does not require quadratic work in containerCount.
			for index := range pods {
				pods[index], podConfigs[index] = framework.CreatePodSandboxForContainer(
					ctx,
					runtimeClient,
				)
				DeferCleanup(framework.CleanupPodSandbox, runtimeClient, pods[index])
			}

			imageID := framework.PullPublicImage(ctx, f.CRIClient.CRIImageClient,
				framework.TestContext.TestImageList.DefaultTestContainerImage, podConfigs[0])
			imageName := framework.PrepareImageName(
				framework.TestContext.TestImageList.DefaultTestContainerImage,
			)
			labels := map[string]string{listStreamingLabel: framework.NewUUID()}
			payload := strings.Repeat("x", payloadSize)
			ids := make([]string, containerCount)

			var completed atomic.Int32

			setupCtx, setupCancel := context.WithCancel(ctx)
			defer setupCancel()

			By(fmt.Sprintf("retain %d exited containers with %d-byte annotations",
				containerCount, payloadSize))

			createExited := func(index int) error {
				podIndex := index / containersPerPod

				id, err := runtimeClient.CreateContainer(setupCtx, pods[podIndex],
					&runtimeapi.ContainerConfig{
						Metadata: framework.BuildContainerMetadata(
							"list-streaming-large-"+framework.NewUUID(), 0,
						),
						Image: &runtimeapi.ImageSpec{
							Image:              imageID,
							UserSpecifiedImage: imageName,
						},
						Command:     echoHelloCmd,
						Labels:      labels,
						Annotations: map[string]string{payloadKey: payload},
					}, podConfigs[podIndex])
				if err != nil {
					return fmt.Errorf("create fixture container %d: %w", index, err)
				}

				if err := runtimeClient.StartContainer(setupCtx, id); err != nil {
					return fmt.Errorf("start fixture container %q: %w", id, err)
				}

				if err := runtimeClient.StopContainer(setupCtx, id, 0); err != nil {
					return fmt.Errorf("stop fixture container %q: %w", id, err)
				}

				ids[index] = id

				if count := completed.Add(1); count%1000 == 0 {
					framework.Logf(
						"Retained %d of %d exited fixture containers",
						count,
						containerCount,
					)
				}

				return nil
			}

			var (
				wg       sync.WaitGroup
				firstErr sync.Once
				setupErr error
			)

			for worker := range workers {
				wg.Go(func() {
					for podIndex := worker; podIndex < podCount; podIndex += workers {
						if setupCtx.Err() != nil {
							return
						}

						var err error

						start := podIndex * containersPerPod
						for index := start; index < min(start+containersPerPod, containerCount); index++ {
							if err = createExited(index); err != nil {
								break
							}
						}

						if err == nil {
							err = runtimeClient.StopPodSandbox(setupCtx, pods[podIndex])
						}

						if err != nil {
							firstErr.Do(func() {
								setupErr = fmt.Errorf(
									"populate sandbox %q: %w",
									pods[podIndex],
									err,
								)

								setupCancel()
							})

							return
						}
					}
				})
			}

			wg.Wait()
			framework.ExpectNoError(setupErr)
			framework.ExpectNoError(setupCtx.Err())

			By("list every container and its complete annotation through the CRI client")

			containers, err := runtimeClient.ListContainers(ctx, &runtimeapi.ContainerFilter{
				LabelSelector: labels,
			})
			framework.ExpectNoError(err)
			Expect(containers).To(HaveLen(containerCount))

			actual := make(map[string]*runtimeapi.Container, len(containers))
			for _, container := range containers {
				Expect(container).NotTo(BeNil())
				actual[container.GetId()] = container
			}

			actualIDs := slices.Collect(maps.Keys(actual))
			expectedIDs := slices.Clone(ids)

			slices.Sort(actualIDs)
			slices.Sort(expectedIDs)
			Expect(actualIDs).To(HaveExactElements(expectedIDs))

			for index, id := range ids {
				container := actual[id]
				Expect(container.GetPodSandboxId()).To(Equal(pods[index/containersPerPod]))
				Expect(container.GetState()).To(Equal(runtimeapi.ContainerState_CONTAINER_EXITED))
				Expect(
					container.GetAnnotations()[payloadKey],
				).To(Equal(payload), "container %q did not preserve its complete annotation payload", id)
			}

			responseSize := proto.Size(&runtimeapi.ListContainersResponse{
				Containers: containers,
			})
			Expect(responseSize).To(BeNumerically(">", listStreamingMaxMsgSize))
			framework.Logf(
				"ListContainers returned %d containers (%d bytes as a unary response)",
				len(containers),
				responseSize,
			)
		},
	)
})
