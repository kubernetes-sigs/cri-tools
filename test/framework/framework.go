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

package framework

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"github.com/sirupsen/logrus"
)

// TestFramework is used to support commonly used test features
type TestFramework struct {
	crioDir string
}

// NewTestFramework creates a new test framework instance
func NewTestFramework() *TestFramework {
	return &TestFramework{""}
}

// Setup is the global initialization function which runs before each test
// suite
func (t *TestFramework) Setup(dir string) {
	// Global initialization for the whole framework goes in here
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(GinkgoWriter)
	t.crioDir = dir
}

// Teardown is the global deinitialization function which runs after each test
// suite
func (t *TestFramework) Teardown() {
}

// Describe is a convenience wrapper around the `ginkgo.Describe` function
func (t *TestFramework) Describe(text string, body func()) bool {
	return Describe("crictl: "+text, body)
}

// Convenience method for command creation
func cmd(workDir, format string, args ...interface{}) *Session {
	c := strings.Split(fmt.Sprintf(format, args...), " ")
	command := exec.Command(c[0], c[1:]...)
	if workDir != "" {
		command.Dir = workDir
	}

	session, err := Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).To(BeNil())

	return session
}

// Convenience method for command creation in the current working directory
func lcmd(format string, args ...interface{}) *Session {
	return cmd("", format, args...)
}

// Run crictl and return the resulting session
func (t *TestFramework) Crictl(args string) *Session {
	return lcmd("crictl %s", args).Wait()
}

// Run crictl on the specified endpoint and return the resulting session
func (t *TestFramework) CrictlWithEndpoint(endpoint, args string) *Session {
	return lcmd("crictl --runtime-endpoint=%s %s", endpoint, args).Wait()
}

// Run crictl and expect success containing the specified output
func (t *TestFramework) CrictlExpectSuccess(args, expectedOut string) {
	t.CrictlExpectSuccessWithEndpoint("", args, expectedOut)
}

// Run crictl and expect success containing the specified output
func (t *TestFramework) CrictlExpectSuccessWithEndpoint(endpoint, args, expectedOut string) {
	// When
	res := t.CrictlWithEndpoint(endpoint, args)

	// Then
	Expect(res).To(Exit(0))
	Expect(res.Out).To(Say(expectedOut))
	Expect(res.Err.Contents()).To(BeEmpty())
}

// Run crictl and expect error containing the specified outputs
func (t *TestFramework) CrictlExpectFailure(
	args string, expectedOut, expectedErr string,
) {
	// When
	res := t.Crictl(args)

	// Then
	Expect(res).To(Exit(1))
	Expect(res.Out).To(Say(expectedOut))
	Expect(res.Err).To(Say(expectedErr))
}

func SetupCrio() string {
	const (
		crioURL = "https://github.com/cri-o/cri-o"
		timeout = 10 * time.Minute
	)
	tmpDir := filepath.Join(os.TempDir(), "crio-tmp")

	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		logrus.Info("cloning and building CRI-O")
		lcmd("git clone --depth=1 %s %s", crioURL, tmpDir).Wait(timeout)
		cmd(tmpDir, "make").Wait(timeout)
	}

	return tmpDir
}

// Start the container runtime process
func (t *TestFramework) StartCrio() (string, string, *Session) {
	// Create a new sandbox directory
	tmpDir, err := ioutil.TempDir("", "crictl-e2e-")
	Expect(err).To(BeNil())

	// Copy everything together
	lcmd("cp -R %s %s", filepath.Join(t.crioDir, "bin"), tmpDir).Wait()

	lcmd("cp %s %s", filepath.Join(t.crioDir, "test", "policy.json"),
		tmpDir).Wait()

	lcmd("cp %s %s", filepath.Join(t.crioDir, "contrib", "cni",
		"10-crio-bridge.conf"), tmpDir).Wait()

	for _, d := range []string{
		"cni-config", "root", "runroot", "log", "exits", "attach",
	} {
		Expect(os.MkdirAll(filepath.Join(tmpDir, d), 0755)).To(BeNil())
	}

	endpoint := filepath.Join(tmpDir, "crio.sock")

	session := cmd(tmpDir, "%s"+
		" --listen=%s"+
		" --conmon=%s"+
		" --container-exits-dir=%s"+
		" --container-attach-socket-dir=%s"+
		" --log-dir=%s"+
		" --signature-policy=%s"+
		" --cni-config-dir=%s"+
		" --root=%s"+
		" --runroot=%s"+
		" --storage-driver=vfs",
		filepath.Join(tmpDir, "bin", "crio"),
		endpoint,
		filepath.Join(tmpDir, "bin", "conmon"),
		filepath.Join(tmpDir, "exits"),
		filepath.Join(tmpDir, "attach"),
		filepath.Join(tmpDir, "log"),
		filepath.Join(tmpDir, "policy.json"),
		filepath.Join(tmpDir, "cni-config"),
		filepath.Join(tmpDir, "root"),
		filepath.Join(tmpDir, "runroot"),
	)

	// Wait for the connection to be available
	for i := 0; i < 100; i++ {
		res := t.CrictlWithEndpoint(endpoint, "--timeout=200ms info")
		if res.ExitCode() == 0 {
			break
		}
		logrus.Info("waiting for CRI-O to become ready")
	}
	return endpoint, tmpDir, session
}

// Stop the container runtime process
func (t *TestFramework) StopCrio(testDir string, session *Session) {
	Expect(session.Interrupt().Wait()).To(Exit(0))
	Expect(os.RemoveAll(testDir)).To(BeNil())
}
