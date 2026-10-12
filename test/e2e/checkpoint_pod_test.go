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
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

const checkpointTestImage = "registry.k8s.io/busybox:1.36.1"

func writeCheckpointConfig(pattern, contents string) string {
	file, err := os.CreateTemp("", pattern)
	Expect(err).NotTo(HaveOccurred())
	_, err = fmt.Fprint(file, contents)
	Expect(err).NotTo(HaveOccurred())
	Expect(file.Close()).To(Succeed())

	return file.Name()
}

func checkpointPodConfigs(name, uid, namespace string) (podConfig, containerConfig string) {
	podConfig = writeCheckpointConfig("pod-config-", fmt.Sprintf(`{
		"metadata": {"name": %q, "uid": %q, "namespace": %q},
		"linux": {"security_context": {"namespace_options": {"network": 2}}}
	}`, name, uid, namespace))
	containerConfig = writeCheckpointConfig("container-config-", fmt.Sprintf(`{
		"metadata": {"name": "checkpoint-container"},
		"image": {"image": %q},
		"command": ["sh"],
		"args": ["-c", "sleep 300"]
	}`, checkpointTestImage))

	return podConfig, containerConfig
}

func createCheckpointPod(name, uid, namespace string) (podID, podConfig, containerConfig string) {
	podConfig, containerConfig = checkpointPodConfigs(name, uid, namespace)
	Expect(t.Crictl(fmt.Sprintf("run --with-pull %s %s", containerConfig, podConfig))).To(Exit(0))
	result := t.Crictl("pods --name " + name + " -q")
	Expect(result).To(Exit(0))
	podID = string(bytes.TrimSpace(result.Out.Contents()))
	Expect(podID).NotTo(BeEmpty())

	return podID, podConfig, containerConfig
}

func cleanupCheckpointPod(podID, podConfig, containerConfig string) {
	if podID != "" {
		Expect(t.Crictl("rmp -f " + podID)).To(Exit(0))
	}

	Expect(os.Remove(podConfig)).To(Succeed())
	Expect(os.Remove(containerConfig)).To(Succeed())
	t.CrictlRemovePauseImages()
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
			"checkpoint-empty", "checkpoint-empty", "default",
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
		podID, podConfig, containerConfig := createCheckpointPod(
			"checkpoint-roundtrip", "checkpoint-roundtrip", "default")
		defer func() { cleanupCheckpointPod(podID, podConfig, containerConfig) }()

		checkpointPath, err := os.MkdirTemp("", "checkpoint-")
		Expect(err).NotTo(HaveOccurred())

		defer os.RemoveAll(checkpointPath)

		result := t.Crictl(fmt.Sprintf("checkpointp --path %s %s", checkpointPath, podID))
		Expect(result).To(Exit(0))
		Expect(result.Out.Contents()).To(ContainSubstring("checkpointed"))

		entries, err := os.ReadDir(checkpointPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).NotTo(BeEmpty(), "the runtime should write checkpoint data")

		Expect(t.Crictl("rmp -f " + podID)).To(Exit(0))
		podID = ""
		result = t.Crictl(fmt.Sprintf(
			"restorep --pod-config %s --container-config %s %s",
			podConfig, containerConfig, checkpointPath,
		))
		Expect(result).To(Exit(0))
		fields := bytes.Fields(result.Out.Contents())
		Expect(fields).NotTo(BeEmpty())
		podID = string(fields[len(fields)-1])
		Expect(podID).NotTo(BeEmpty())
		Expect(t.Crictl("inspectp " + podID)).To(Exit(0))
	})
})

var _ = t.Describe("restorep", func() {
	It("should validate its arguments and required configs", func() {
		t.CrictlExpectFailure("restorep", "", "CHECKPOINT-PATH cannot be empty")
		t.CrictlExpectFailure("restorep /path1 /path2", "",
			"only one checkpoint path can be restored at a time")
		t.CrictlExpectFailure("restorep /checkpoint", "", "--pod-config is required")

		podConfig, containerConfig := checkpointPodConfigs(
			"restore-validation", "restore-validation", "default",
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
