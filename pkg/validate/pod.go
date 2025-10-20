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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/common"
	"sigs.k8s.io/cri-tools/pkg/framework"
)

// expectedMetricDescriptorNames contains all expected metric descriptor names
// based on metrics returned by kubelet with CRI-O and cadvisor on the legacy cadvisor stats provider
// on kubernetes 1.35.
var expectedMetricDescriptorNames = []string{
	"container_blkio_device_usage_total",
	"container_cpu_load_average_10s",
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
	"container_tasks_state",
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

		AfterEach(func() {
			if podID != "" {
				By("stop PodSandbox")
				Expect(rc.StopPodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
				By("delete PodSandbox")
				Expect(rc.RemovePodSandbox(context.TODO(), podID)).NotTo(HaveOccurred())
			}
		})

		It("runtime should support running PodSandbox [Conformance]", func() {
			By("test run a default PodSandbox")
			podID = testRunDefaultPodSandbox(rc)

			By("test list PodSandbox")
			pods := listPodSandboxForID(rc, podID)
			Expect(podSandboxFound(pods, podID)).To(BeTrue(), "PodSandbox should be listed")
		})

		It("runtime should support stopping PodSandbox [Conformance]", func() {
			By("run PodSandbox")
			podID = framework.RunDefaultPodSandbox(rc, "PodSandbox-for-test-stop-")

			By("test stop PodSandbox")
			testStopPodSandbox(rc, podID)
		})

		It("runtime should support removing PodSandbox [Conformance]", func() {
			By("run PodSandbox")
			podID = framework.RunDefaultPodSandbox(rc, "PodSandbox-for-test-remove-")

			By("stop PodSandbox")
			stopPodSandbox(rc, podID)

			By("test remove PodSandbox")
			testRemovePodSandbox(rc, podID)
			podID = "" // no need to cleanup pod
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
			containerID := framework.CreatePauseContainer(rc, ic, podID, podConfig, "container-for-metrics-")

			By("start container")
			startContainer(rc, containerID)

			By("list pod sandbox metrics")
			metrics := listPodSandboxMetrics(rc)

			By("verify pod metrics are present")
			testPodSandboxMetrics(metrics, podID)
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
func verifyPodSandboxStatus(c internalapi.RuntimeService, podID string, expectedStatus runtimeapi.PodSandboxState, statusName string) {
	status := getPodSandboxStatus(c, podID)
	Expect(status.GetState()).To(Equal(expectedStatus), "PodSandbox state should be "+statusName)
}

// testRunDefaultPodSandbox runs a PodSandbox and make sure it is ready.
func testRunDefaultPodSandbox(c internalapi.RuntimeService) string {
	podID := framework.RunDefaultPodSandbox(c, "PodSandbox-for-create-test-")
	verifyPodSandboxStatus(c, podID, runtimeapi.PodSandboxState_SANDBOX_READY, "ready")

	return podID
}

// getPodSandboxStatus gets PodSandboxStatus for podID.
func getPodSandboxStatus(c internalapi.RuntimeService, podID string) *runtimeapi.PodSandboxStatus {
	By("Get PodSandbox status for podID: " + podID)
	status, err := c.PodSandboxStatus(context.TODO(), podID, false)
	framework.ExpectNoError(err, "failed to get PodSandbox %q status: %v", podID, err)

	return status.GetStatus()
}

// stopPodSandbox stops the PodSandbox for podID.
func stopPodSandbox(c internalapi.RuntimeService, podID string) {
	By("Stop PodSandbox for podID: " + podID)
	err := c.StopPodSandbox(context.TODO(), podID)
	framework.ExpectNoError(err, "Failed to stop PodSandbox: %v", err)
	framework.Logf("Stopped PodSandbox %q\n", podID)
}

// testStopPodSandbox stops the PodSandbox for podID and make sure it's not ready.
func testStopPodSandbox(c internalapi.RuntimeService, podID string) {
	stopPodSandbox(c, podID)
	verifyPodSandboxStatus(c, podID, runtimeapi.PodSandboxState_SANDBOX_NOTREADY, "not ready")
}

// removePodSandbox removes the PodSandbox for podID.
func removePodSandbox(c internalapi.RuntimeService, podID string) {
	By("Remove PodSandbox for podID: " + podID)
	err := c.RemovePodSandbox(context.TODO(), podID)
	framework.ExpectNoError(err, "failed to remove PodSandbox: %v", err)
	framework.Logf("Removed PodSandbox %q\n", podID)
}

// testRemovePodSandbox removes a PodSandbox and make sure it is removed.
func testRemovePodSandbox(c internalapi.RuntimeService, podID string) {
	removePodSandbox(c, podID)
	pods := listPodSandboxForID(c, podID)
	Expect(podSandboxFound(pods, podID)).To(BeFalse(), "PodSandbox should be removed")
}

// listPodSandboxForID lists PodSandbox for podID.
func listPodSandboxForID(c internalapi.RuntimeService, podID string) []*runtimeapi.PodSandbox {
	By("List PodSandbox for podID: " + podID)
	filter := &runtimeapi.PodSandboxFilter{
		Id: podID,
	}

	return listPodSandbox(c, filter)
}

// listPodSandbox lists PodSandbox.
func listPodSandbox(c internalapi.RuntimeService, filter *runtimeapi.PodSandboxFilter) []*runtimeapi.PodSandbox {
	By("List PodSandbox.")

	pods, err := c.ListPodSandbox(context.TODO(), filter)
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
func createPodSandboxWithLogDirectory(c internalapi.RuntimeService) (sandboxID string, podConfig *runtimeapi.PodSandboxConfig, hostPath string) {
	By("create a PodSandbox with log directory")

	podSandboxName := "PodSandbox-with-log-directory-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

	hostPath, podLogPath := createLogTempDir(podSandboxName)
	podConfig = &runtimeapi.PodSandboxConfig{
		Metadata:     framework.BuildPodSandboxMetadata(podSandboxName, uid, namespace, framework.DefaultAttempt),
		LogDirectory: podLogPath,
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			CgroupParent: common.GetCgroupParent(context.TODO(), c),
		},
	}

	return framework.RunPodSandbox(c, podConfig), podConfig, hostPath
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
func testPodSandboxMetrics(allMetrics []*runtimeapi.PodSandboxMetrics, podID string) {
	var podMetrics *runtimeapi.PodSandboxMetrics

	for _, m := range allMetrics {
		if m.GetPodSandboxId() == podID {
			podMetrics = m

			break
		}
	}

	Expect(podMetrics).NotTo(BeNil(), "Metrics for pod %q should be present", podID)

	metricNamesFound := make(map[string]bool)

	for _, metric := range podMetrics.GetMetrics() {
		if !metricNamesFound[metric.GetName()] {
			metricNamesFound[metric.GetName()] = true
		}
	}

	for _, containerMetric := range podMetrics.GetContainerMetrics() {
		for _, metric := range containerMetric.GetMetrics() {
			if !metricNamesFound[metric.GetName()] {
				metricNamesFound[metric.GetName()] = true
			}
		}
	}

	missingMetrics := []string{}

	for _, expectedName := range expectedMetricDescriptorNames {
		if !metricNamesFound[expectedName] {
			missingMetrics = append(missingMetrics, expectedName)
		}
	}

	Expect(missingMetrics).To(BeEmpty(), "Expected %s metrics to be present and they were not", strings.Join(missingMetrics, " "))
}
