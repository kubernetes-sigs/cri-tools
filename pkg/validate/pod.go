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
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubelet/pkg/types"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

// expectedMetricDescriptorNames contains all expected metric descriptor names
// based on metrics returned by kubelet with CRI-O and cadvisor on the legacy cadvisor stats provider
// on kubernetes 1.35.
var expectedMetricDescriptorNames = []string{
	"container_blkio_device_usage_total",
	"container_cpu_system_seconds_total",
	"container_cpu_usage_seconds_total",
	"container_cpu_user_seconds_total",
	"container_file_descriptors",
	"container_fs_reads_bytes_total",
	"container_fs_reads_total",
	"container_fs_usage_bytes",
	"container_fs_writes_bytes_total",
	"container_fs_writes_total",
	"container_last_seen",
	"container_memory_cache",
	"container_memory_failcnt",
	"container_memory_failures_total",
	"container_memory_mapped_file",
	"container_memory_max_usage_bytes",
	"container_memory_rss",
	"container_memory_swap",
	"container_memory_usage_bytes",
	"container_memory_working_set_bytes",
	"container_network_receive_bytes_total",
	"container_network_receive_errors_total",
	"container_network_receive_packets_dropped_total",
	"container_network_receive_packets_total",
	"container_network_transmit_bytes_total",
	"container_network_transmit_errors_total",
	"container_network_transmit_packets_dropped_total",
	"container_network_transmit_packets_total",
	"container_oom_events_total",
	"container_processes",
	"container_sockets",
	"container_spec_cpu_period",
	"container_spec_cpu_shares",
	"container_spec_memory_limit_bytes",
	"container_spec_memory_reservation_limit_bytes",
	"container_spec_memory_swap_limit_bytes",
	"container_start_time_seconds",
	"container_threads",
	"container_threads_max",
	"container_ulimits_soft",
}

var _ = framework.KubeDescribe("PodSandbox", func() {
	f := framework.NewDefaultCRIFramework()

	var rc internalapi.RuntimeService

	BeforeEach(func() {
		rc = f.CRIClient.CRIRuntimeClient
	})

	Context("runtime should support basic operations on PodSandbox", func() {
		var podID string

		AfterEach(func(ctx SpecContext) {
			if podID != "" {
				By("stop PodSandbox")
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
				By("delete PodSandbox")
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
			}
		})

		It("runtime should support running PodSandbox [Conformance]", func(ctx SpecContext) {
			By("test run a default PodSandbox")

			podID = testRunDefaultPodSandbox(ctx, rc)

			By("test list PodSandbox")

			pods := listPodSandboxForID(ctx, rc, podID)
			Expect(podSandboxFound(pods, podID)).To(BeTrue(), "PodSandbox should be listed")
		})

		It("runtime should support stopping PodSandbox [Conformance]", func(ctx SpecContext) {
			By("run PodSandbox")

			podID = framework.RunDefaultPodSandbox(ctx, rc, "PodSandbox-for-test-stop-")

			By("test stop PodSandbox")
			testStopPodSandbox(ctx, rc, podID)
		})

		It("runtime should support removing PodSandbox [Conformance]", func(ctx SpecContext) {
			By("run PodSandbox")

			podID = framework.RunDefaultPodSandbox(ctx, rc, "PodSandbox-for-test-remove-")

			By("stop PodSandbox")
			stopPodSandbox(ctx, rc, podID)

			By("test remove PodSandbox")
			testRemovePodSandbox(ctx, rc, podID)
			podID = "" // no need to cleanup pod
		})

		It("runtime should support preserving PodSandbox attributes [Conformance]", func(ctx SpecContext) {
			By("test run a PodSandbox with attributes")

			podSandboxName := "PodSandbox-with-attributes-" + framework.NewUUID()
			uid := framework.DefaultUIDPrefix + framework.NewUUID()
			namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
			metadata := framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt)
			labels := map[string]string{
				"foo":                             "bar",
				types.KubernetesPodNameLabel:      podSandboxName,
				types.KubernetesPodNamespaceLabel: namespace,
				types.KubernetesPodUIDLabel:       uid,
			}
			annotations := map[string]string{"abc": "def"}

			podConfig := &runtimeapi.PodSandboxConfig{
				Metadata:    metadata,
				Labels:      labels,
				Annotations: annotations,
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
			}
			podID = framework.RunPodSandbox(ctx, rc, podConfig)

			By("test get PodSandbox status")

			status := getPodSandboxStatus(ctx, rc, podID)
			Expect(status.GetMetadata().GetName()).To(Equal(metadata.GetName()))
			Expect(status.GetMetadata().GetUid()).To(Equal(metadata.GetUid()))
			Expect(status.GetMetadata().GetNamespace()).To(Equal(metadata.GetNamespace()))
			Expect(status.GetMetadata().GetAttempt()).To(Equal(metadata.GetAttempt()))
			framework.ExpectSubset(status.GetLabels(), labels, "labels")
			framework.ExpectSubset(status.GetAnnotations(), annotations, "annotations")

			By("test list PodSandbox")

			pods := listPodSandbox(ctx, rc, &runtimeapi.PodSandboxFilter{Id: podID})
			Expect(pods).To(HaveLen(1))
			pod := pods[0]
			Expect(pod.GetMetadata().GetName()).To(Equal(metadata.GetName()))
			Expect(pod.GetMetadata().GetUid()).To(Equal(metadata.GetUid()))
			Expect(pod.GetMetadata().GetNamespace()).To(Equal(metadata.GetNamespace()))
			Expect(pod.GetMetadata().GetAttempt()).To(Equal(metadata.GetAttempt()))
			framework.ExpectSubset(pod.GetLabels(), labels, "labels")
			framework.ExpectSubset(pod.GetAnnotations(), annotations, "annotations")
		})
	})
	Context("runtime should support metrics operations", func() {
		var podID string
		var podConfig *runtimeapi.PodSandboxConfig
		BeforeEach(func() {
			_, err := rc.ListMetricDescriptors(context.TODO())
			if err != nil {
				s, ok := grpcstatus.FromError(err)
				Expect(ok && s.Code() == codes.Unimplemented).To(BeTrue(), "Expected CRI metric descriptors call to either be not supported, or not error")
				if s.Code() == codes.Unimplemented {
					Skip("CRI Metrics endpoints not supported by this runtime version")
				}
			}
		})

		AfterEach(func() {
			if podID != "" {
				By("stop PodSandbox")
				Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
				By("delete PodSandbox")
				Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			}
		})

		It("runtime should support returning metrics descriptors [Conformance]", func() {
			By("list metric descriptors")
			descs := listMetricDescriptors(rc)

			By("verify expected metric descriptors are present")
			testMetricDescriptors(descs)
		})

		It("runtime should support listing pod sandbox metrics [Conformance]", func() {
			By("create pod sandbox")
			podID, podConfig = framework.CreatePodSandboxForContainer(rc)

			By("create container in pod")
			ic := f.CRIClient.CRIImageClient
			containerID := framework.CreateDefaultContainer(rc, ic, podID, podConfig, "container-for-metrics-")

			By("start container")
			startContainer(rc, containerID)

			_, _, err := rc.ExecSync(
				context.TODO(), containerID, []string{"/bin/sh", "-c", "for i in $(seq 1 10); do echo hi >> /var/lib/mydisktest/inode_test_file_$i; done; sync"},
				time.Duration(defaultExecSyncTimeout)*time.Second,
			)

			Expect(err).ToNot(HaveOccurred())

			By("list metric descriptors")
			descs := listMetricDescriptors(rc)

			By("list pod sandbox metrics")
			metrics := listPodSandboxMetrics(rc)

			By("verify pod metrics are present")
			testPodSandboxMetrics(metrics, descs, podID)
		})
	})
})

// podSandboxFound returns whether PodSandbox is found.
func podSandboxFound(podSandboxs []*runtimeapi.PodSandbox, podID string) bool {
	for _, podSandbox := range podSandboxs {
		if podSandbox.GetId() == podID {
			return true
		}
	}

	return false
}

// verifyPodSandboxStatus verifies whether PodSandbox status for given podID matches.
func verifyPodSandboxStatus(ctx context.Context, c internalapi.RuntimeService, podID string, expectedStatus runtimeapi.PodSandboxState, statusName string) {
	status := getPodSandboxStatus(ctx, c, podID)
	Expect(status.GetState()).To(Equal(expectedStatus), "PodSandbox state should be "+statusName)
}

// testRunDefaultPodSandbox runs a PodSandbox and make sure it is ready.
func testRunDefaultPodSandbox(ctx context.Context, c internalapi.RuntimeService) string {
	podID := framework.RunDefaultPodSandbox(ctx, c, "PodSandbox-for-create-test-")
	verifyPodSandboxStatus(ctx, c, podID, runtimeapi.PodSandboxState_SANDBOX_READY, "ready")

	return podID
}

// getPodSandboxStatus gets PodSandboxStatus for podID.
func getPodSandboxStatus(ctx context.Context, c internalapi.RuntimeService, podID string) *runtimeapi.PodSandboxStatus {
	By("Get PodSandbox status for podID: " + podID)
	status, err := c.PodSandboxStatus(ctx, podID, false)
	framework.ExpectNoError(err, "failed to get PodSandbox %q status: %v", podID, err)

	return status.GetStatus()
}

// stopPodSandbox stops the PodSandbox for podID.
func stopPodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	By("Stop PodSandbox for podID: " + podID)
	err := c.StopPodSandbox(ctx, podID)
	framework.ExpectNoError(err, "Failed to stop PodSandbox: %v", err)
	framework.Logf("Stopped PodSandbox %q\n", podID)
}

// testStopPodSandbox stops the PodSandbox for podID and make sure it's not ready.
func testStopPodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	stopPodSandbox(ctx, c, podID)
	verifyPodSandboxStatus(ctx, c, podID, runtimeapi.PodSandboxState_SANDBOX_NOTREADY, "not ready")
}

// removePodSandbox removes the PodSandbox for podID.
func removePodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	By("Remove PodSandbox for podID: " + podID)
	err := c.RemovePodSandbox(ctx, podID)
	framework.ExpectNoError(err, "failed to remove PodSandbox: %v", err)
	framework.Logf("Removed PodSandbox %q\n", podID)
}

// testRemovePodSandbox removes a PodSandbox and make sure it is removed.
func testRemovePodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	removePodSandbox(ctx, c, podID)
	pods := listPodSandboxForID(ctx, c, podID)
	Expect(podSandboxFound(pods, podID)).To(BeFalse(), "PodSandbox should be removed")
}

// listPodSandboxForID lists PodSandbox for podID.
func listPodSandboxForID(ctx context.Context, c internalapi.RuntimeService, podID string) []*runtimeapi.PodSandbox {
	By("List PodSandbox for podID: " + podID)
	filter := &runtimeapi.PodSandboxFilter{
		Id: podID,
	}

	return listPodSandbox(ctx, c, filter)
}

// listPodSandbox lists PodSandbox.
func listPodSandbox(ctx context.Context, c internalapi.RuntimeService, filter *runtimeapi.PodSandboxFilter) []*runtimeapi.PodSandbox {
	By("List PodSandbox.")

	pods, err := c.ListPodSandbox(ctx, filter)
	framework.ExpectNoError(err, "failed to list PodSandbox status: %v", err)
	framework.Logf("List PodSandbox succeed")

	return pods
}

// listMetricDescriptors lists MetricDescriptors.
func listMetricDescriptors(c internalapi.RuntimeService) []*runtimeapi.MetricDescriptor {
	By("List MetricDescriptors.")

	descs, err := c.ListMetricDescriptors(context.TODO())
	framework.ExpectNoError(err, "failed to list MetricDescriptors status: %v", err)
	framework.Logf("List MetricDescriptors succeed")

	return descs
}

// createLogTempDir creates the log temp directory for podSandbox.
func createLogTempDir(podSandboxName string) (hostPath, podLogPath string) {
	hostPath, err := os.MkdirTemp("", "podLogTest")
	framework.ExpectNoError(err, "failed to create TempDir %q: %v", hostPath, err)
	podLogPath = filepath.Join(hostPath, podSandboxName)
	err = os.MkdirAll(podLogPath, 0o777)
	framework.ExpectNoError(err, "failed to create host path %s: %v", podLogPath, err)

	return hostPath, podLogPath
}

// createPodSandboxWithLogDirectory creates a PodSandbox with log directory.
func createPodSandboxWithLogDirectory(ctx context.Context, c internalapi.RuntimeService) (sandboxID string, podConfig *runtimeapi.PodSandboxConfig, hostPath string) {
	By("create a PodSandbox with log directory")

	podSandboxName := "PodSandbox-with-log-directory-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

	hostPath, podLogPath := createLogTempDir(podSandboxName)
	podConfig = &runtimeapi.PodSandboxConfig{
		Metadata:     framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		LogDirectory: podLogPath,
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(ctx, c),
		},
	}

	return framework.RunPodSandbox(ctx, c, podConfig), podConfig, hostPath
}

// testMetricDescriptors verifies that all expected metric descriptors are present.
func testMetricDescriptors(descs []*runtimeapi.MetricDescriptor) {
	returnedDescriptors := make(map[string]*runtimeapi.MetricDescriptor)

	for _, desc := range descs {
		returnedDescriptors[desc.GetName()] = desc
		Expect(desc.GetHelp()).NotTo(BeEmpty(), "Metric descriptor %q should have help text", desc.GetName())
		Expect(desc.GetLabelKeys()).NotTo(BeEmpty(), "Metric descriptor %q should have label keys", desc.GetName())
	}

	missingMetrics := []string{}

	for _, expectedName := range expectedMetricDescriptorNames {
		_, found := returnedDescriptors[expectedName]
		if !found {
			missingMetrics = append(missingMetrics, expectedName)
		}
	}

	Expect(missingMetrics).To(BeEmpty(), "Expected %s metrics to be present and they were not", strings.Join(missingMetrics, " "))
}

// listPodSandboxMetrics lists PodSandboxMetrics.
func listPodSandboxMetrics(c internalapi.RuntimeService) []*runtimeapi.PodSandboxMetrics {
	By("List PodSandboxMetrics.")

	metrics, err := c.ListPodSandboxMetrics(context.TODO())
	framework.ExpectNoError(err, "failed to list PodSandboxMetrics: %v", err)
	framework.Logf("List PodSandboxMetrics succeed")

	return metrics
}

// testPodSandboxMetrics verifies that metrics are present for the specified pod.
func testPodSandboxMetrics(allMetrics []*runtimeapi.PodSandboxMetrics, descs []*runtimeapi.MetricDescriptor, podID string) {
	var podMetrics *runtimeapi.PodSandboxMetrics

	for _, m := range allMetrics {
		if m.GetPodSandboxId() == podID {
			podMetrics = m

			break
		}
	}

	Expect(podMetrics).NotTo(BeNil(), "Metrics for pod %q should be present", podID)

	metricNamesFound := make(map[string][]string)

	for _, metric := range podMetrics.GetMetrics() {
		if len(metricNamesFound[metric.GetName()]) == 0 {
			metricNamesFound[metric.GetName()] = metric.GetLabelValues()
		}
	}

	for _, containerMetric := range podMetrics.GetContainerMetrics() {
		for _, metric := range containerMetric.GetMetrics() {
			if len(metricNamesFound[metric.GetName()]) == 0 {
				metricNamesFound[metric.GetName()] = metric.GetLabelValues()
			}
		}
	}

	missingMetrics := []string{}

	for _, expectedName := range expectedMetricDescriptorNames {
		if len(metricNamesFound[expectedName]) == 0 {
			missingMetrics = append(missingMetrics, expectedName)
		}
	}

	Expect(missingMetrics).To(BeEmpty(), "Expected %s metrics to be present and they were not", strings.Join(missingMetrics, " "))

	mismatchedLabels := []string{}
	for _, desc := range descs {
		if len(metricNamesFound[desc.GetName()]) != len(desc.GetLabelKeys()) {
			mismatchedLabels = append(mismatchedLabels, desc.GetName())
		}
	}

	Expect(mismatchedLabels).To(BeEmpty(), "Expected %s metrics to have same set of labels in ListMetricDescriptors and ListPodSandboxMetrics", strings.Join(mismatchedLabels, ","))
}
