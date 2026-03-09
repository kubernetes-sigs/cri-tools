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

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const checkpointTestImage = "registry.k8s.io/e2e-test-images/busybox:1.29-2"

func writeCheckpointConfig(pattern, contents string) string {
	file, err := os.CreateTemp("", pattern)
	Expect(err).NotTo(HaveOccurred())
	_, err = fmt.Fprint(file, contents)
	Expect(err).NotTo(HaveOccurred())
	Expect(file.Close()).To(Succeed())

	return file.Name()
}

func checkpointPodConfigs(
	name, uid, namespace, cgroupParent string,
) (podConfig, containerConfig string) {
	podConfig = writeCheckpointConfig("pod-config-", fmt.Sprintf(`{
		"metadata": {"name": %q, "uid": %q, "namespace": %q},
		"linux": {
			"cgroup_parent": %q,
			"security_context": {"namespace_options": {"network": 2}}
		}
	}`, name, uid, namespace, cgroupParent))
	// The token and counter live in shell variables. A fresh process would
	// overwrite the restored file with a new token and restart its counter.
	program := `token=$(cat /proc/sys/kernel/random/uuid); counter=0; ` +
		`while :; do counter=$((counter+1)); ` +
		`printf '%s %s\n' "$token" "$counter" > /checkpoint-state.tmp; ` +
		`mv /checkpoint-state.tmp /checkpoint-state; sleep 1; done`
	containerConfig = writeCheckpointConfig("container-config-", fmt.Sprintf(`{
		"metadata": {"name": "checkpoint-container"},
		"image": {"image": %q},
		"command": ["sh"],
		"args": ["-c", %q]
	}`, checkpointTestImage, program))

	return podConfig, containerConfig
}

func checkpointPodCgroup() string {
	result := t.Crictl("runtime-config")
	if result.ExitCode() != 0 &&
		bytes.Contains(result.Err.Contents(), []byte("code = Unimplemented")) {
		Skip("RuntimeConfig is not supported by this runtime")
	}

	Expect(result).To(Exit(0))
	fields := strings.Fields(string(result.Out.Contents()))
	Expect(fields).NotTo(BeEmpty())

	parent := "/crictlcheckpoint" + strings.ReplaceAll(uuid.NewString(), "-", "")

	switch fields[len(fields)-1] {
	case "CGROUPFS":
	case "SYSTEMD":
		// A slice name without hyphens has no expanded ancestor slices, so
		// the configured parent is also its path in the cgroup filesystem.
		parent += ".slice"
	default:
		Fail("runtime-config did not report a supported cgroup driver")
	}

	DeferCleanup(func(ctx SpecContext) {
		if strings.HasSuffix(parent, ".slice") {
			// This UUID-named slice belongs only to this test. Stop its unit
			// after removing the Pods so systemd releases the parent cgroup.
			unitName := strings.TrimPrefix(parent, "/")
			output, err := exec.CommandContext(ctx, "systemctl", "stop", unitName).CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "stop test slice: %s", output)
		}

		paths := []string{filepath.Join(string(os.PathSeparator), "sys", "fs", "cgroup", parent)}
		if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); os.IsNotExist(err) {
			paths, err = filepath.Glob("/sys/fs/cgroup/*" + parent)
			Expect(err).NotTo(HaveOccurred())
		}

		for _, path := range paths {
			err := os.Remove(path)
			if !os.IsNotExist(err) {
				Expect(err).NotTo(HaveOccurred(), "remove empty test pod cgroup %s", path)
			}
		}
	}, NodeTimeout(time.Minute))

	return parent
}

func checkpointProcessState(containerID string) (token string, counter int, err error) {
	result := t.Crictl("exec --sync " + containerID + " cat /checkpoint-state")
	if result.ExitCode() != 0 {
		return "", 0, fmt.Errorf("read checkpoint process state: %s", result.Err.Contents())
	}

	fields := strings.Fields(string(result.Out.Contents()))
	if len(fields) != 2 {
		return "", 0, fmt.Errorf("unexpected checkpoint process state %q", result.Out.Contents())
	}

	counter, err = strconv.Atoi(fields[1])

	return fields[0], counter, err
}

func expectCheckpointContainerState(containerID, state string) {
	result := t.Crictl("inspect " + containerID)
	Expect(result).To(Exit(0))

	var response struct {
		Status struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"status"`
	}
	Expect(json.Unmarshal(result.Out.Contents(), &response)).To(Succeed())
	Expect(response.Status.ID).To(Equal(containerID))
	Expect(response.Status.State).To(Equal(state))
}

var _ = t.Describe("checkpointp", func() {
	It("should validate its arguments", func() {
		t.CrictlExpectFailure("checkpointp", "", "POD-ID cannot be empty")
		t.CrictlExpectFailure("checkpointp pod-id", "", "without specifying the checkpoint path")
		t.CrictlExpectFailure("checkpointp --path /tmp/checkpoint pod1 pod2", "",
			"only one pod sandbox can be checkpointed at a time")
	})

	It("should reject a pod without running containers", func() {
		podConfig, containerConfig := checkpointPodConfigs(
			"checkpoint-empty", "checkpoint-empty", "default", "",
		)
		defer os.Remove(podConfig)
		defer os.Remove(containerConfig)

		result := t.Crictl("runp " + podConfig)
		Expect(result).To(Exit(0))

		podID := string(bytes.TrimSpace(result.Out.Contents()))
		defer func() { Expect(t.Crictl("rmp -f " + podID)).To(Exit(0)) }()

		checkpointPath, err := os.MkdirTemp("", "checkpoint-")
		Expect(err).NotTo(HaveOccurred())

		defer os.RemoveAll(checkpointPath)

		t.CrictlExpectFailure(fmt.Sprintf("checkpointp --path %s %s", checkpointPath, podID), "",
			"has no running containers to checkpoint")
	})

	It("should checkpoint and restore a pod", func() {
		if runtime.GOOS != "linux" {
			Skip("Pod checkpoint and restore requires Linux")
		}

		parent := checkpointPodCgroup()
		uid := strings.TrimSuffix(strings.TrimPrefix(parent, "/"), ".slice")
		podConfig, containerConfig := checkpointPodConfigs(
			uid, uid, "default", parent)
		DeferCleanup(os.Remove, podConfig)
		DeferCleanup(os.Remove, containerConfig)

		podID := ""
		restoredPodID := ""

		DeferCleanup(func() {
			for _, id := range []string{restoredPodID, podID} {
				if id != "" {
					Expect(t.Crictl("rmp -f " + id)).To(Exit(0))
				}
			}

			t.CrictlRemovePauseImages()
		})

		result := t.Crictl("runp " + podConfig)
		Expect(result).To(Exit(0))
		podID = string(bytes.TrimSpace(result.Out.Contents()))
		Expect(podID).NotTo(BeEmpty())
		result = t.Crictl(
			fmt.Sprintf("create --with-pull %s %s %s", podID, containerConfig, podConfig),
		)
		Expect(result).To(Exit(0))
		containerID := string(bytes.TrimSpace(result.Out.Contents()))
		Expect(containerID).NotTo(BeEmpty())
		Expect(t.Crictl("start " + containerID)).To(Exit(0))
		Eventually(func() (int, error) {
			_, counter, err := checkpointProcessState(containerID)

			return counter, err
		}, "15s", "100ms").Should(BeNumerically(">", 0))

		token, counter, err := checkpointProcessState(containerID)
		Expect(err).NotTo(HaveOccurred())

		checkpointPath, err := os.MkdirTemp("", "checkpoint-")
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(os.RemoveAll, checkpointPath)

		result = t.Crictl(fmt.Sprintf("checkpointp --path %s %s", checkpointPath, podID))
		if result.ExitCode() != 0 &&
			bytes.Contains(result.Err.Contents(), []byte("code = Unimplemented")) {
			Skip("CheckpointPod is not supported by this runtime")
		}

		Expect(result).To(Exit(0))
		Expect(result.Out.Contents()).To(ContainSubstring("checkpointed"))
		Eventually(func() (int, error) {
			_, current, err := checkpointProcessState(containerID)

			return current, err
		}, "15s", "100ms").Should(BeNumerically(">", counter), "checkpoint must resume the source process")

		entries, err := os.ReadDir(checkpointPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).NotTo(BeEmpty(), "the runtime should write checkpoint data")

		Expect(t.Crictl("rmp -f " + podID)).To(Exit(0))
		podID = ""
		result = t.Crictl(fmt.Sprintf(
			"restorep -o json --pod-config %s --container-config %s %s",
			podConfig, containerConfig, checkpointPath,
		))
		Expect(result).To(Exit(0))

		var response struct {
			PodSandboxID string `json:"podSandboxId"`
			Containers   []struct {
				Name string `json:"name"`
				ID   string `json:"containerId"`
			} `json:"restoredContainers"`
		}
		Expect(json.Unmarshal(result.Out.Contents(), &response)).To(Succeed())
		restoredPodID = response.PodSandboxID
		Expect(restoredPodID).NotTo(BeEmpty())
		Expect(response.Containers).To(HaveLen(1))
		Expect(response.Containers[0].Name).To(Equal("checkpoint-container"))
		restoredID := response.Containers[0].ID
		Expect(restoredID).NotTo(BeEmpty())
		Expect(restoredID).NotTo(Equal(containerID))

		Expect(t.Crictl("inspectp " + restoredPodID)).To(Exit(0))
		result = t.Crictl("ps -a --pod " + restoredPodID + " -q")
		Expect(result).To(Exit(0))
		Expect(strings.Fields(string(result.Out.Contents()))).To(Equal([]string{restoredID}))
		expectCheckpointContainerState(restoredID, "CONTAINER_CREATED")

		Expect(t.Crictl("start " + restoredID)).To(Exit(0))
		expectCheckpointContainerState(restoredID, "CONTAINER_RUNNING")
		restoredToken, restoredCounter, err := checkpointProcessState(restoredID)
		Expect(err).NotTo(HaveOccurred())
		Expect(restoredToken).To(Equal(token), "restore must preserve process memory")
		Expect(restoredCounter).To(BeNumerically(">=", counter))
		Eventually(func() (int, error) {
			currentToken, current, err := checkpointProcessState(restoredID)
			if err == nil && currentToken != token {
				err = fmt.Errorf("process token changed from %q to %q", token, currentToken)
			}

			return current, err
		}, "15s", "100ms").Should(BeNumerically(">", restoredCounter), "restored process must continue its counter")
	})
})

var _ = t.Describe("restorep", func() {
	It("should validate its arguments and required configs", func() {
		t.CrictlExpectFailure("restorep", "", "CHECKPOINT-PATH cannot be empty")
		t.CrictlExpectFailure("restorep /path1 /path2", "",
			"only one checkpoint path can be restored at a time")
		t.CrictlExpectFailure("restorep /checkpoint", "", "--pod-config is required")

		podConfig, containerConfig := checkpointPodConfigs(
			"restore-validation", "restore-validation", "default", "",
		)
		defer os.Remove(podConfig)
		defer os.Remove(containerConfig)

		t.CrictlExpectFailure("restorep --pod-config "+podConfig+" /checkpoint", "",
			"at least one --container-config is required")
		t.CrictlExpectFailure(fmt.Sprintf(
			"restorep --pod-config %s --container-config %s %s",
			podConfig, containerConfig, filepath.Join(os.TempDir(), "missing-checkpoint"),
		), "", "restoring pod")
	})
})
