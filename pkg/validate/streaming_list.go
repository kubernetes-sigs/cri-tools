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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

var _ = framework.KubeDescribe("Streaming List", func() {
	var (
		rc internalapi.RuntimeService
		ic internalapi.ImageManagerService
	)

	BeforeEach(func(ctx SpecContext) {
		client, err := framework.LoadCRIClientWithStreaming()
		Expect(err).NotTo(HaveOccurred())

		rc = client.CRIRuntimeClient
		ic = client.CRIImageClient

		version, err := rc.Version(ctx, "")
		Expect(err).NotTo(HaveOccurred())
		// TODO: remove once containerd supports streaming list APIs
		if strings.Contains(strings.ToLower(version.GetRuntimeName()), "containerd") {
			Skip("streaming list is not supported on containerd")
		}
	})

	Context("runtime should support streaming list for pods", func() {
		var podID string

		AfterEach(func(ctx SpecContext) {
			if podID != "" {
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
			}
		})

		It("should list a created pod via streaming", func(ctx SpecContext) {
			podID = framework.RunDefaultPodSandbox(ctx, rc, "stream-list-pod-")

			pods, err := rc.ListPodSandbox(ctx, &runtimeapi.PodSandboxFilter{Id: podID})
			Expect(err).NotTo(HaveOccurred())
			Expect(podSandboxFound(pods, podID)).To(BeTrue(), "PodSandbox should be listed via streaming")
		})

		It("should list pods filtered by state via streaming", func(ctx SpecContext) {
			podID = framework.RunDefaultPodSandbox(ctx, rc, "stream-list-pod-state-")

			readyFilter := &runtimeapi.PodSandboxFilter{
				State: &runtimeapi.PodSandboxStateValue{
					State: runtimeapi.PodSandboxState_SANDBOX_READY,
				},
			}
			pods, err := rc.ListPodSandbox(ctx, readyFilter)
			Expect(err).NotTo(HaveOccurred())
			Expect(podSandboxFound(pods, podID)).To(BeTrue(), "Running pod should appear with READY filter")
		})

		It("should list pods filtered by labels via streaming", func(ctx SpecContext) {
			podSandboxName := "stream-list-pod-labels-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			labels := map[string]string{"stream-test": "yes"}

			podConfig := &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Labels:   labels,
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
			}
			podID = framework.RunPodSandbox(ctx, rc, podConfig)

			pods, err := rc.ListPodSandbox(ctx, &runtimeapi.PodSandboxFilter{LabelSelector: labels})
			Expect(err).NotTo(HaveOccurred())
			Expect(podSandboxFound(pods, podID)).To(BeTrue(), "Pod should be found with label filter")

			pods, err = rc.ListPodSandbox(ctx, &runtimeapi.PodSandboxFilter{
				LabelSelector: map[string]string{"stream-test": "no"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(podSandboxFound(pods, podID)).To(BeFalse(), "Pod should not match wrong label")
		})

		It("should return empty list for non-existent pod via streaming", func(ctx SpecContext) {
			pods, err := rc.ListPodSandbox(ctx, &runtimeapi.PodSandboxFilter{Id: "non-existent-id"})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods).To(BeEmpty())
		})
	})

	Context("runtime should support streaming list for containers", func() {
		var (
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
		)

		BeforeEach(func(ctx SpecContext) {
			podID, podConfig = framework.CreatePodSandboxForContainer(ctx, rc)
		})

		AfterEach(func(ctx SpecContext) {
			Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
			Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
		})

		It("should list a created container via streaming", func(ctx SpecContext) {
			containerID := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "stream-list-ctr-")

			containers, err := rc.ListContainers(ctx, &runtimeapi.ContainerFilter{Id: containerID})
			Expect(err).NotTo(HaveOccurred())
			Expect(containerFound(containers, containerID)).To(BeTrue(), "Container should be listed via streaming")
		})

		It("should list containers filtered by pod via streaming", func(ctx SpecContext) {
			containerID := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "stream-list-ctr-pod-")

			containers, err := rc.ListContainers(ctx, &runtimeapi.ContainerFilter{PodSandboxId: podID})
			Expect(err).NotTo(HaveOccurred())
			Expect(containerFound(containers, containerID)).To(BeTrue(), "Container should be found by pod filter")
		})

		It("should list containers filtered by state via streaming", func(ctx SpecContext) {
			containerID := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "stream-list-ctr-state-")

			startContainer(ctx, rc, containerID)

			runningFilter := &runtimeapi.ContainerFilter{
				State: &runtimeapi.ContainerStateValue{
					State: runtimeapi.ContainerState_CONTAINER_RUNNING,
				},
			}
			containers, err := rc.ListContainers(ctx, runningFilter)
			Expect(err).NotTo(HaveOccurred())
			Expect(containerFound(containers, containerID)).To(BeTrue(), "Running container should appear with RUNNING filter")
		})

		It("should list multiple containers via streaming", func(ctx SpecContext) {
			id1 := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "stream-list-multi-1-")
			id2 := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "stream-list-multi-2-")
			id3 := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "stream-list-multi-3-")

			containers, err := rc.ListContainers(ctx, &runtimeapi.ContainerFilter{PodSandboxId: podID})
			Expect(err).NotTo(HaveOccurred())
			Expect(containerFound(containers, id1)).To(BeTrue(), "First container should be listed")
			Expect(containerFound(containers, id2)).To(BeTrue(), "Second container should be listed")
			Expect(containerFound(containers, id3)).To(BeTrue(), "Third container should be listed")
		})

		It("should not list removed container via streaming", func(ctx SpecContext) {
			containerID := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "stream-list-ctr-remove-")

			Expect(rc.RemoveContainer(ctx, containerID)).NotTo(HaveOccurred())

			containers, err := rc.ListContainers(ctx, &runtimeapi.ContainerFilter{Id: containerID})
			Expect(err).NotTo(HaveOccurred())
			Expect(containerFound(containers, containerID)).To(BeFalse(), "Removed container should not be listed")
		})

		It("should list container stats via streaming", func(ctx SpecContext) {
			containerID := framework.CreateDefaultContainer(ctx, rc, ic, podID, podConfig, "stream-list-stats-")
			startContainer(ctx, rc, containerID)

			stats, err := rc.ListContainerStats(ctx, &runtimeapi.ContainerStatsFilter{Id: containerID})
			Expect(err).NotTo(HaveOccurred())
			Expect(statFound(stats, containerID)).To(BeTrue(), "Container stats should be listed via streaming")
		})

		It("should list container stats filtered by labels via streaming", func(ctx SpecContext) {
			labels := map[string]string{"stream-stats": "yes"}
			containerID := framework.CreateDefaultContainerWithLabels(ctx, rc, ic, podID, podConfig, "stream-list-stats-labels-", labels)
			startContainer(ctx, rc, containerID)

			stats, err := rc.ListContainerStats(ctx, &runtimeapi.ContainerStatsFilter{LabelSelector: labels})
			Expect(err).NotTo(HaveOccurred())
			Expect(statFound(stats, containerID)).To(BeTrue(), "Container stats should match label filter")

			stats, err = rc.ListContainerStats(ctx, &runtimeapi.ContainerStatsFilter{
				LabelSelector: map[string]string{"stream-stats": "no"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(statFound(stats, containerID)).To(BeFalse(), "Container stats should not match wrong label")
		})
	})

	Context("runtime should support streaming list for pod stats", func() {
		var podID string

		AfterEach(func(ctx SpecContext) {
			if podID != "" {
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
			}
		})

		It("should list pod sandbox stats by ID via streaming", func(ctx SpecContext) {
			podID = framework.RunDefaultPodSandbox(ctx, rc, "stream-list-pod-stats-")

			stats, err := rc.ListPodSandboxStats(ctx, &runtimeapi.PodSandboxStatsFilter{Id: podID})
			Expect(err).NotTo(HaveOccurred())
			Expect(stats).To(HaveLen(1), "Should return stats for the created pod")
			Expect(stats[0].GetAttributes().GetId()).To(Equal(podID))
		})

		It("should list pod sandbox stats with nil filter via streaming", func(ctx SpecContext) {
			podID = framework.RunDefaultPodSandbox(ctx, rc, "stream-list-pod-stats-nil-")

			stats, err := rc.ListPodSandboxStats(ctx, nil)
			Expect(err).NotTo(HaveOccurred())

			found := false

			for _, s := range stats {
				if s.GetAttributes().GetId() == podID {
					found = true

					break
				}
			}

			Expect(found).To(BeTrue(), "Pod stats should appear in unfiltered list")
		})

		It("should list pod sandbox stats filtered by labels via streaming", func(ctx SpecContext) {
			labels := map[string]string{"stream-pod-stats": "yes"}
			podSandboxName := "stream-list-pod-stats-labels-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

			podConfig := &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
				Labels:   labels,
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
			}
			podID = framework.RunPodSandbox(ctx, rc, podConfig)

			stats, err := rc.ListPodSandboxStats(ctx, &runtimeapi.PodSandboxStatsFilter{LabelSelector: labels})
			Expect(err).NotTo(HaveOccurred())

			found := false

			for _, s := range stats {
				if s.GetAttributes().GetId() == podID {
					found = true

					break
				}
			}

			Expect(found).To(BeTrue(), "Pod stats should match label filter")
		})
	})

	Context("runtime should support streaming list for pod metrics", func() {
		var podID string

		AfterEach(func(ctx SpecContext) {
			if podID != "" {
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
			}
		})

		It("should list pod sandbox metrics via streaming", func(ctx SpecContext) {
			podID = framework.RunDefaultPodSandbox(ctx, rc, "stream-list-pod-metrics-")

			metrics, err := rc.ListPodSandboxMetrics(ctx)
			Expect(err).NotTo(HaveOccurred())

			found := false

			for _, m := range metrics {
				if m.GetPodSandboxId() == podID {
					found = true

					break
				}
			}

			Expect(found).To(BeTrue(), "Pod metrics should include the created pod")
		})
	})

	Context("runtime should support streaming list for images", func() {
		It("should list images via streaming", func(ctx SpecContext) {
			imageName := framework.TestContext.TestImageList.DefaultTestContainerImage
			framework.PullPublicImage(ctx, ic, imageName, nil)

			images, err := ic.ListImages(ctx, &runtimeapi.ImageFilter{})
			Expect(err).NotTo(HaveOccurred())
			Expect(images).NotTo(BeEmpty(), "Image list should not be empty via streaming")
			Expect(imageFound(images, imageName)).To(BeTrue(), "Pulled image should appear in streaming list")
		})

		It("should list images filtered by spec via streaming", func(ctx SpecContext) {
			imageName := framework.TestContext.TestImageList.DefaultTestContainerImage
			framework.PullPublicImage(ctx, ic, imageName, nil)

			filter := &runtimeapi.ImageFilter{
				Image: &runtimeapi.ImageSpec{Image: imageName},
			}
			images, err := ic.ListImages(ctx, filter)
			Expect(err).NotTo(HaveOccurred())
			Expect(images).NotTo(BeEmpty(), "Filtered image list should not be empty")
			Expect(imageFound(images, imageName)).To(BeTrue(), "Image should match filter")
		})
	})
})

func imageFound(images []*runtimeapi.Image, imageName string) bool {
	prepared := framework.PrepareImageName(imageName)

	for _, img := range images {
		for _, tag := range img.GetRepoTags() {
			if tag == imageName || tag == prepared {
				return true
			}
		}
	}

	return false
}
