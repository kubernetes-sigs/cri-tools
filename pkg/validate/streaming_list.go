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
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

const listStreamingLabel = "critest.kubernetes.io/list-streaming"

var _ = framework.KubeDescribe("CRI list streaming", func() {
	f := framework.NewDefaultCRIFramework()

	It(
		"should list more than 5000 containers beyond the unary message size limit [Conformance]",
		Serial,
		Label("Slow"),
		// Override CLI flake retries: rebuilding this fixture can exhaust the suite timeout.
		MustPassRepeatedly(1),
		func(ctx SpecContext) {
			const (
				unaryContainerCount = 5000
				containerCount      = 5100
				containersPerPod    = 128
				podCount            = (containerCount + containersPerPod - 1) / containersPerPod
				payloadKey          = "critest.kubernetes.io/list-streaming-payload"
				workers             = 8
			)

			skipIfStreamContainersUnsupported(ctx)

			runtimeClient := f.CRIClient.CRIRuntimeClient
			pods := make([]string, podCount)
			podConfigs := make([]*runtimeapi.PodSandboxConfig, podCount)

			DeferCleanup(func(ctx SpecContext) error {
				By("stop and remove fixture sandboxes in parallel")

				cleanupErrors := make([]error, len(pods))

				var wg sync.WaitGroup
				for worker := range workers {
					wg.Go(func() {
						for index := worker; index < len(pods); index += workers {
							podID := pods[index]
							if podID == "" {
								continue
							}

							if err := runtimeClient.StopPodSandbox(ctx, podID); err != nil {
								cleanupErrors[index] = fmt.Errorf(
									"stop fixture sandbox %q: %w",
									podID,
									err,
								)

								continue
							}

							if err := runtimeClient.RemovePodSandbox(ctx, podID); err != nil {
								cleanupErrors[index] = fmt.Errorf(
									"remove fixture sandbox %q: %w",
									podID,
									err,
								)
							}
						}
					})
				}

				wg.Wait()

				return errors.Join(cleanupErrors...)
			})

			// Container events include pod-wide status snapshots. Keep each pod small
			// so fixture setup does not require quadratic work in containerCount.
			for index := range pods {
				pods[index], podConfigs[index] = framework.CreatePodSandboxForContainer(
					ctx,
					runtimeClient,
				)
			}

			imageID := framework.PullPublicImage(ctx, f.CRIClient.CRIImageClient,
				framework.TestContext.TestImageList.DefaultTestContainerImage, podConfigs[0])
			imageName := framework.PrepareImageName(
				framework.TestContext.TestImageList.DefaultTestContainerImage,
			)
			labels := map[string]string{listStreamingLabel: framework.NewUUID()}
			createExited := func(ctx context.Context, podIndex int, payload string) (string, error) {
				id, err := runtimeClient.CreateContainer(ctx, pods[podIndex],
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
					return "", fmt.Errorf("create fixture container: %w", err)
				}

				if err := runtimeClient.StartContainer(ctx, id); err != nil {
					return "", fmt.Errorf("start fixture container %q: %w", id, err)
				}

				if err := runtimeClient.StopContainer(ctx, id, 0); err != nil {
					return "", fmt.Errorf("stop fixture container %q: %w", id, err)
				}

				return id, nil
			}

			By("calibrate the annotation size from an exited sample container")

			samplePayload := strings.Repeat("x", 1024)
			sampleID, err := createExited(ctx, 0, samplePayload)
			framework.ExpectNoError(err)
			samples, err := runtimeClient.ListContainers(
				ctx,
				&runtimeapi.ContainerFilter{Id: sampleID},
			)
			framework.ExpectNoError(err)
			Expect(samples).To(HaveLen(1))
			// Target the midpoint so 5,000 entries fit below 16 MiB while 5,100 exceed it.
			targetSize := framework.RuntimeServiceMaxMsgSize / ((unaryContainerCount + containerCount) / 2)
			sampleSize := proto.Size(&runtimeapi.ListContainersResponse{Containers: samples})
			payloadSize := len(samplePayload) + targetSize - sampleSize
			Expect(payloadSize).To(BeNumerically(">", 0))

			framework.ExpectNoError(runtimeClient.RemoveContainer(ctx, sampleID))
			framework.Logf(
				"Calibrated %d-byte annotation for a %d-byte listed entry",
				payloadSize,
				targetSize,
			)

			payload := strings.Repeat("x", payloadSize)
			ids := make([]string, containerCount)

			var completed atomic.Int32

			setupCtx, setupCancel := context.WithCancel(ctx)
			defer setupCancel()

			By(fmt.Sprintf("retain %d exited containers with %d-byte annotations",
				containerCount, payloadSize))

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
							if ids[index], err = createExited(
								setupCtx,
								podIndex,
								payload,
							); err != nil {
								break
							}

							if count := completed.Add(1); count%1000 == 0 {
								framework.Logf(
									"Retained %d of %d exited fixture containers",
									count,
									containerCount,
								)
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
			// Select by recorded IDs: list ordering is runtime-dependent.
			firstBatch := make([]*runtimeapi.Container, unaryContainerCount)
			for index, id := range ids[:unaryContainerCount] {
				firstBatch[index] = actual[id]
			}

			firstBatchSize := proto.Size(&runtimeapi.ListContainersResponse{Containers: firstBatch})
			framework.Logf(
				"ListContainers unary response sizes: first %d containers = %d bytes; all %d = %d bytes",
				unaryContainerCount,
				firstBatchSize,
				containerCount,
				responseSize,
			)
			Expect(firstBatchSize).To(BeNumerically("<", framework.RuntimeServiceMaxMsgSize))
			Expect(responseSize).To(BeNumerically(">", framework.RuntimeServiceMaxMsgSize))
		},
	)
})

func skipIfStreamContainersUnsupported(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, framework.TestContext.RuntimeServiceTimeout)
	defer cancel()

	conn, err := framework.NewRuntimeServiceConnection(framework.TestContext.RuntimeServiceAddr)
	framework.ExpectNoError(err)

	defer func() { framework.ExpectNoError(conn.Close()) }()

	// Probe the RPC directly because the standard CRI client hides Unimplemented
	// by falling back to ListContainers. A fresh label avoids listing existing containers.
	stream, err := runtimeapi.NewRuntimeServiceClient(conn).StreamContainers(ctx,
		&runtimeapi.StreamContainersRequest{
			Filter: &runtimeapi.ContainerFilter{
				LabelSelector: map[string]string{listStreamingLabel: framework.NewUUID()},
			},
		})
	if err == nil {
		// Server-side errors, including Unimplemented, arrive on Recv.
		_, err = stream.Recv()
	}

	if status.Code(err) == codes.Unimplemented {
		Skip("StreamContainers is not implemented by the runtime")
	}

	if !errors.Is(err, io.EOF) {
		framework.ExpectNoError(err, "failed to probe StreamContainers support")
	}
}
