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
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
// on kubernetes 1.37.
var expectedMetricDescriptorNames = []string{
	"container_blkio_device_usage_total",
	"container_cpu_cfs_periods_total",
	"container_cpu_cfs_throttled_periods_total",
	"container_cpu_cfs_throttled_seconds_total",
	"container_cpu_system_seconds_total",
	"container_cpu_usage_seconds_total",
	"container_cpu_user_seconds_total",
	"container_file_descriptors",
	"container_fs_inodes_free",
	"container_fs_inodes_total",
	"container_fs_limit_bytes",
	"container_fs_reads_bytes_total",
	"container_fs_reads_total",
	"container_fs_usage_bytes",
	"container_fs_writes_bytes_total",
	"container_fs_writes_total",
	"container_hugetlb_max_usage_bytes",
	"container_hugetlb_usage_bytes",
	"container_last_seen",
	"container_memory_cache",
	"container_memory_failcnt",
	"container_memory_failures_total",
	"container_memory_kernel_usage",
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
	"container_pressure_cpu_stalled_seconds_total",
	"container_pressure_cpu_waiting_seconds_total",
	"container_pressure_io_stalled_seconds_total",
	"container_pressure_io_waiting_seconds_total",
	"container_pressure_memory_stalled_seconds_total",
	"container_pressure_memory_waiting_seconds_total",
	"container_processes",
	"container_sockets",
	"container_spec_cpu_period",
	"container_spec_cpu_quota",
	"container_spec_cpu_shares",
	"container_spec_memory_limit_bytes",
	"container_spec_memory_reservation_limit_bytes",
	"container_spec_memory_swap_limit_bytes",
	"container_start_time_seconds",
	"container_threads",
	"container_threads_max",
	"container_ulimits_soft",
}

// unimplementedMetricDescriptors contains metric descriptors not yet
// implemented by all container runtimes. Remove entries as runtimes add support.
// TODO(containerd): https://github.com/containerd/containerd/issues/13532
var unimplementedMetricDescriptors = []string{
	"container_hugetlb_max_usage_bytes",
	"container_hugetlb_usage_bytes",
	"container_pressure_cpu_stalled_seconds_total",
	"container_pressure_cpu_waiting_seconds_total",
	"container_pressure_io_stalled_seconds_total",
	"container_pressure_io_waiting_seconds_total",
	"container_pressure_memory_stalled_seconds_total",
	"container_pressure_memory_waiting_seconds_total",
	"container_spec_cpu_quota",
	"container_spec_memory_reservation_limit_bytes",
	"container_ulimits_soft",
}

// optionalValuesForMetricDescriptors contains the metric descriptors that have
// optional values due to test environment limitations.
var optionalValuesForMetricDescriptors = []string{
	// All of these depend on blkio cgroup accounting, which doesn't work on
	// GitHub actions runners because the overlay filesystem is typically backed
	// by a virtual disk that doesn't report per-device I/O stats.
	"container_blkio_device_usage_total",
	"container_fs_reads_bytes_total",
	"container_fs_reads_total",
	"container_fs_writes_bytes_total",
	"container_fs_writes_total",
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
				framework.CleanupPodSandbox(ctx, rc, podID)
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

		It(
			"runtime should support preserving PodSandbox attributes [Conformance]",
			func(ctx SpecContext) {
				By("test run a PodSandbox with attributes")

				podSandboxName := "PodSandbox-with-attributes-" + framework.NewUUID()
				uid := framework.DefaultUIDPrefix + framework.NewUUID()
				namespace := framework.DefaultNamespacePrefix + framework.NewUUID()
				metadata := framework.BuildPodSandboxMetadata(
					podSandboxName,
					uid,
					namespace,
					framework.DefaultAttempt,
				)
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
			},
		)
	})

	// The kubelet derives the sandbox attempt number from what the runtime
	// lists and retries RunPodSandbox with the same metadata until a sandbox
	// shows up. It relies on the runtime never creating two sandboxes with the
	// same (name, uid, namespace, attempt) between RunPodSandbox and
	// RemovePodSandbox. A duplicate request must either fail or return the ID
	// of the existing sandbox.
	Context("runtime should enforce PodSandbox metadata uniqueness", func() {
		var (
			podSandboxName string
			uid            string
			namespace      string
			testLabel      map[string]string
		)

		BeforeEach(func() {
			podSandboxName = "PodSandbox-for-metadata-uniqueness-test-" + framework.NewUUID()
			uid = framework.DefaultUIDPrefix + framework.NewUUID()
			namespace = framework.DefaultNamespacePrefix + framework.NewUUID()
			testLabel = map[string]string{
				"cri-tools-metadata-uniqueness-test": framework.NewUUID(),
			}
		})

		// Clean up by label so that sandboxes whose IDs the test never saw
		// (timed out or concurrent requests) are removed too.
		AfterEach(func(ctx SpecContext) {
			for _, pod := range listTestPodSandboxes(ctx, rc, testLabel) {
				framework.CleanupPodSandbox(ctx, rc, pod.GetId())
			}
		})

		newConfig := func(ctx context.Context, sandboxUID string, attempt uint32) *runtimeapi.PodSandboxConfig {
			return &runtimeapi.PodSandboxConfig{
				Metadata: framework.BuildPodSandboxMetadata(
					podSandboxName,
					sandboxUID,
					namespace,
					attempt,
				),
				Labels: testLabel,
				Linux: &runtimeapi.LinuxPodSandboxConfig{
					CgroupParent: common.GetCgroupParent(ctx, rc),
				},
			}
		}

		// runWithSameMetadata runs a PodSandbox with the metadata shared by
		// the tests below, using a dedicated timeout to avoid blocking on an
		// in-flight request with the same metadata. A request that does not
		// return in time simply did not succeed, which the contract allows.
		const sameMetadataTimeout = 10 * time.Second

		runWithSameMetadata := func(ctx context.Context) (string, error) {
			config := newConfig(ctx, uid, 0)

			ctx, cancel := context.WithTimeout(ctx, sameMetadataTimeout)
			defer cancel()

			return rc.RunPodSandbox(ctx, config, framework.TestContext.RuntimeHandler)
		}

		// runDuplicate runs a PodSandbox with metadata already used by
		// existingID. It must fail or return existingID.
		runDuplicate := func(ctx context.Context, existingID, explain string) {
			id, err := runWithSameMetadata(ctx)
			if err == nil {
				Expect(id).To(Equal(existingID), explain)
			}

			Expect(listTestPodSandboxes(ctx, rc, testLabel)).To(HaveLen(1), explain)
		}

		It(
			"runtime should not create a second PodSandbox with the same metadata [Conformance]",
			func(ctx SpecContext) {
				By("run the first PodSandbox with attempt 0")

				firstID := framework.RunPodSandbox(ctx, rc, newConfig(ctx, uid, 0))

				By("run a PodSandbox with the same metadata while the first one is ready")
				runDuplicate(ctx, firstID, "metadata of a ready PodSandbox must not be reused")

				By("run a PodSandbox with the same metadata while the first one is stopped")
				stopPodSandbox(ctx, rc, firstID)
				runDuplicate(ctx, firstID, "metadata of a stopped PodSandbox must not be reused")

				By("run a PodSandbox with attempt 1")

				nextID := framework.RunPodSandbox(ctx, rc, newConfig(ctx, uid, 1))
				Expect(nextID).NotTo(Equal(firstID))
				Expect(listTestPodSandboxes(ctx, rc, testLabel)).To(HaveLen(2))

				By("run a PodSandbox with attempt 0 again after the first one is removed")
				removePodSandbox(ctx, rc, firstID)

				reusedID := framework.RunPodSandbox(ctx, rc, newConfig(ctx, uid, 0))
				Expect(reusedID).NotTo(Equal(firstID))
				Expect(listTestPodSandboxes(ctx, rc, testLabel)).To(HaveLen(2))
			},
		)

		It(
			"runtime should allow a PodSandbox with the same name and namespace but a new UID [Conformance]",
			func(ctx SpecContext) {
				By("run the first PodSandbox")

				firstID := framework.RunPodSandbox(ctx, rc, newConfig(ctx, uid, 0))

				// A pod deleted and recreated under the same name (for
				// example by a StatefulSet) gets a new UID, while the old
				// pod's sandbox may still exist.
				By("run a PodSandbox for a recreated pod while the first one is ready")

				newUID := framework.DefaultUIDPrefix + framework.NewUUID()
				secondID := framework.RunPodSandbox(ctx, rc, newConfig(ctx, newUID, 0))
				Expect(secondID).NotTo(Equal(firstID))
				Expect(listTestPodSandboxes(ctx, rc, testLabel)).To(HaveLen(2))
			},
		)

		It(
			"runtime should create at most one PodSandbox for concurrent requests with the same metadata [Conformance]",
			func(ctx SpecContext) {
				const requests = 5

				var (
					wg   sync.WaitGroup
					ids  = make([]string, requests)
					errs = make([]error, requests)
				)

				By("run PodSandboxes with the same metadata concurrently")

				for i := range requests {
					wg.Go(func() {
						defer GinkgoRecover()

						ids[i], errs[i] = runWithSameMetadata(ctx)
					})
				}

				wg.Wait()

				By("verify that exactly one PodSandbox was created")

				var succeeded []string

				for i := range requests {
					if errs[i] == nil {
						succeeded = append(succeeded, ids[i])
					} else {
						framework.Logf("Concurrent RunPodSandbox %d failed: %v", i, errs[i])
					}
				}

				Expect(succeeded).NotTo(BeEmpty(), "at least one request should succeed")
				Expect(slices.Compact(slices.Sorted(slices.Values(succeeded)))).To(HaveLen(1),
					"successful requests must all return the same PodSandbox ID")

				pods := listTestPodSandboxes(ctx, rc, testLabel)
				Expect(pods).To(HaveLen(1))
				Expect(pods[0].GetId()).To(Equal(succeeded[0]))
			},
		)

		It(
			"runtime should create at most one PodSandbox when retrying a timed out request [Conformance]",
			func(ctx SpecContext) {
				By("run a PodSandbox with a client timeout shorter than sandbox creation")

				shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
				_, err := rc.RunPodSandbox(
					shortCtx,
					newConfig(ctx, uid, 0),
					framework.TestContext.RuntimeHandler,
				)

				cancel()
				framework.Logf("RunPodSandbox with a short timeout returned: %v", err)

				// Mirror the kubelet: retry with the same metadata until the
				// runtime lists a sandbox for the pod. Poll tightly, because
				// the interesting window is the one where the cancelled
				// request is still being served: a retry landing in it is what
				// would make a duplicate appear.
				By("retry with the same metadata until a PodSandbox is listed")
				Eventually(ctx, func() error {
					pods := listTestPodSandboxes(ctx, rc, testLabel)
					if len(pods) > 1 {
						return StopTrying("runtime created more than one PodSandbox").
							Attach("PodSandboxes", pods)
					}

					if len(pods) == 1 {
						return nil
					}

					_, err := runWithSameMetadata(ctx)
					framework.Logf("Retried RunPodSandbox returned: %v", err)

					return errors.New("no PodSandbox listed yet")
				}).WithTimeout(time.Minute).WithPolling(100 * time.Millisecond).Should(Succeed())

				By("verify that no second PodSandbox appears later")
				Consistently(ctx, func() []*runtimeapi.PodSandbox {
					return listTestPodSandboxes(ctx, rc, testLabel)
				}).WithTimeout(5 * time.Second).WithPolling(500 * time.Millisecond).Should(HaveLen(1))
			},
		)
	})

	Context("runtime should support metrics operations", func() {
		var (
			podID     string
			podConfig *runtimeapi.PodSandboxConfig
		)

		BeforeEach(func(ctx SpecContext) {
			_, err := rc.ListMetricDescriptors(ctx)
			if err == nil {
				return
			}

			s, ok := grpcstatus.FromError(err)
			if ok && s.Code() == codes.Unimplemented {
				Skip("CRI Metrics endpoints not supported by this runtime version")
			}

			Expect(err).NotTo(HaveOccurred(), "failed to list MetricDescriptors")
		})

		AfterEach(func(ctx SpecContext) {
			if podID != "" {
				By("stop PodSandbox")
				Expect(rc.StopPodSandbox(ctx, podID)).NotTo(HaveOccurred())
				By("delete PodSandbox")
				Expect(rc.RemovePodSandbox(ctx, podID)).NotTo(HaveOccurred())
			}
		})

		It(
			"runtime should support returning metrics descriptors [Conformance]",
			func(ctx SpecContext) {
				By("list metric descriptors")

				descs := listMetricDescriptors(ctx, rc)

				By("verify expected metric descriptors are present")
				testMetricDescriptors(descs)
			},
		)

		It(
			"runtime should support listing pod sandbox metrics [Conformance]",
			func(ctx SpecContext) {
				By("create pod sandbox")

				podID, podConfig = framework.CreatePodSandboxForContainer(ctx, rc)

				By("create container in pod")

				ic := f.CRIClient.CRIImageClient
				containerID := createContainerForMetrics(ctx, rc, ic, podID, podConfig)

				By("start container")
				startContainer(ctx, rc, containerID)

				_, _, err := rc.ExecSync(
					ctx,
					containerID,
					[]string{
						"/bin/sh",
						"-c",
						"mkdir -p /var/lib/mydisktest && for i in $(seq 1 10); do echo hi >> /var/lib/mydisktest/inode_test_file_$i; done; sync",
					},
					time.Duration(defaultExecSyncTimeout)*time.Second,
				)
				Expect(err).ToNot(HaveOccurred())

				By("list metric descriptors")

				descs := listMetricDescriptors(ctx, rc)

				By("list pod sandbox metrics")

				metrics := listPodSandboxMetrics(ctx, rc)

				By("verify pod metrics are present")
				testPodSandboxMetrics(metrics, descs, podID)
			},
		)
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
func verifyPodSandboxStatus(
	ctx context.Context,
	c internalapi.RuntimeService,
	podID string,
	expectedStatus runtimeapi.PodSandboxState,
	statusName string,
) {
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
func getPodSandboxStatus(
	ctx context.Context,
	c internalapi.RuntimeService,
	podID string,
) *runtimeapi.PodSandboxStatus {
	By("Get PodSandbox status for podID: " + podID)
	status, err := c.PodSandboxStatus(ctx, podID, false)
	framework.ExpectNoError(err, "failed to get PodSandbox %q status", podID)

	return status.GetStatus()
}

// stopPodSandbox stops the PodSandbox for podID.
func stopPodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	By("Stop PodSandbox for podID: " + podID)
	err := c.StopPodSandbox(ctx, podID)
	framework.ExpectNoError(err, "Failed to stop PodSandbox")
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
	framework.ExpectNoError(err, "failed to remove PodSandbox")
	framework.Logf("Removed PodSandbox %q\n", podID)
}

// testRemovePodSandbox removes a PodSandbox and make sure it is removed.
func testRemovePodSandbox(ctx context.Context, c internalapi.RuntimeService, podID string) {
	removePodSandbox(ctx, c, podID)
	pods := listPodSandboxForID(ctx, c, podID)
	Expect(podSandboxFound(pods, podID)).To(BeFalse(), "PodSandbox should be removed")
}

// listPodSandboxForID lists PodSandbox for podID.
func listPodSandboxForID(
	ctx context.Context,
	c internalapi.RuntimeService,
	podID string,
) []*runtimeapi.PodSandbox {
	By("List PodSandbox for podID: " + podID)
	filter := &runtimeapi.PodSandboxFilter{
		Id: podID,
	}

	return listPodSandbox(ctx, c, filter)
}

// listPodSandbox lists PodSandbox.
func listPodSandbox(
	ctx context.Context,
	c internalapi.RuntimeService,
	filter *runtimeapi.PodSandboxFilter,
) []*runtimeapi.PodSandbox {
	By("List PodSandbox.")

	pods, err := c.ListPodSandbox(ctx, filter)
	framework.ExpectNoError(err, "failed to list PodSandbox status")
	framework.Logf("List PodSandbox succeed")

	return pods
}

// listTestPodSandboxes lists the PodSandboxes that carry the given labels.
func listTestPodSandboxes(
	ctx context.Context,
	c internalapi.RuntimeService,
	labels map[string]string,
) []*runtimeapi.PodSandbox {
	pods, err := c.ListPodSandbox(ctx, &runtimeapi.PodSandboxFilter{LabelSelector: labels})
	framework.ExpectNoError(err, "failed to list PodSandboxes")

	return pods
}

// listMetricDescriptors lists MetricDescriptors.
func listMetricDescriptors(
	ctx context.Context,
	c internalapi.RuntimeService,
) []*runtimeapi.MetricDescriptor {
	By("List MetricDescriptors.")

	descs, err := c.ListMetricDescriptors(ctx)
	framework.ExpectNoError(err, "failed to list MetricDescriptors: %v", err)
	framework.Logf("List MetricDescriptors succeed")

	return descs
}

// createLogTempDir creates the log temp directory for podSandbox.
func createLogTempDir(podSandboxName string) (hostPath, podLogPath string) {
	hostPath, err := os.MkdirTemp("", "podLogTest")
	framework.ExpectNoError(err, "failed to create TempDir %q", hostPath)
	podLogPath = filepath.Join(hostPath, podSandboxName)
	err = os.MkdirAll(podLogPath, 0o777)
	framework.ExpectNoError(err, "failed to create host path %s", podLogPath)

	return hostPath, podLogPath
}

// createPodSandboxWithLogDirectory creates a PodSandbox with log directory.
func createPodSandboxWithLogDirectory(
	ctx context.Context,
	c internalapi.RuntimeService,
) (sandboxID string, podConfig *runtimeapi.PodSandboxConfig, hostPath string) {
	By("create a PodSandbox with log directory")

	podSandboxName := "PodSandbox-with-log-directory-" + framework.NewUUID()
	uid := framework.DefaultUIDPrefix + framework.NewUUID()
	namespace := framework.DefaultNamespacePrefix + framework.NewUUID()

	hostPath, podLogPath := createLogTempDir(podSandboxName)
	podConfig = &runtimeapi.PodSandboxConfig{
		Metadata: framework.BuildPodSandboxMetadata(
			podSandboxName,
			uid,
			namespace,
			framework.DefaultAttempt,
		),
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
		Expect(
			desc.GetHelp(),
		).NotTo(BeEmpty(), "Metric descriptor %q should have help text", desc.GetName())
		Expect(
			desc.GetLabelKeys(),
		).NotTo(BeEmpty(), "Metric descriptor %q should have label keys", desc.GetName())
	}

	missingMetrics := []string{}

	for _, expectedName := range expectedMetricDescriptorNames {
		_, found := returnedDescriptors[expectedName]
		if !found {
			if slices.Contains(unimplementedMetricDescriptors, expectedName) {
				continue
			}

			missingMetrics = append(missingMetrics, expectedName)
		}
	}

	Expect(
		missingMetrics,
	).To(BeEmpty(), "Expected %s metrics to be present and they were not", strings.Join(missingMetrics, " "))
}

// listPodSandboxMetrics lists PodSandboxMetrics.
func listPodSandboxMetrics(
	ctx context.Context,
	c internalapi.RuntimeService,
) []*runtimeapi.PodSandboxMetrics {
	By("List PodSandboxMetrics.")

	metrics, err := c.ListPodSandboxMetrics(ctx)
	framework.ExpectNoError(err, "failed to list PodSandboxMetrics: %v", err)
	framework.Logf("List PodSandboxMetrics succeed")

	return metrics
}

// testPodSandboxMetrics verifies that metrics are present for the specified pod.
func testPodSandboxMetrics(
	allMetrics []*runtimeapi.PodSandboxMetrics,
	descs []*runtimeapi.MetricDescriptor,
	podID string,
) {
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
			if slices.Contains(optionalValuesForMetricDescriptors, expectedName) ||
				slices.Contains(unimplementedMetricDescriptors, expectedName) {
				continue
			}

			missingMetrics = append(missingMetrics, expectedName)
		}
	}

	Expect(
		missingMetrics,
	).To(BeEmpty(), "Expected %s metrics to be present and they were not", strings.Join(missingMetrics, " "))

	mismatchedLabels := []string{}

	for _, desc := range descs {
		values, found := metricNamesFound[desc.GetName()]
		if !found {
			if slices.Contains(optionalValuesForMetricDescriptors, desc.GetName()) ||
				slices.Contains(unimplementedMetricDescriptors, desc.GetName()) {
				continue
			}

			mismatchedLabels = append(mismatchedLabels, desc.GetName())

			continue
		}

		if len(values) != len(desc.GetLabelKeys()) {
			mismatchedLabels = append(mismatchedLabels, desc.GetName())
		}
	}

	Expect(
		mismatchedLabels,
	).To(BeEmpty(), "Expected %s metrics to have same set of labels in ListMetricDescriptors and ListPodSandboxMetrics", strings.Join(mismatchedLabels, ","))
}

// createContainerForMetrics creates a container for metrics.
func createContainerForMetrics(
	ctx context.Context,
	rc internalapi.RuntimeService,
	ic internalapi.ImageManagerService,
	podID string,
	podConfig *runtimeapi.PodSandboxConfig,
) string {
	containerName := "container-for-metrics-" + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image: &runtimeapi.ImageSpec{
			Image: framework.TestContext.TestImageList.DefaultTestContainerImage,
		},
		Command: framework.DefaultContainerCommand,
		Linux: &runtimeapi.LinuxContainerConfig{
			Resources: &runtimeapi.LinuxContainerResources{
				CpuPeriod:              100000,
				CpuQuota:               20000,
				CpuShares:              1024,
				MemoryLimitInBytes:     64 * 1024 * 1024,
				MemorySwapLimitInBytes: 64 * 1024 * 1024,
			},
		},
	}

	return framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
}
