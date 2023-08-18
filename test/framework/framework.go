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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"github.com/sirupsen/logrus"
)

const timeout = 10 * time.Minute

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

// Run crictl on the specified endpoint and return the resulting session
func (t *TestFramework) CrictlWithEndpoint(endpoint, args string) *Session {
	return lcmd("crictl --runtime-endpoint=%s %s", endpoint, args).Wait(time.Minute)
}

// Run crictl on the specified endpoint and return the resulting session without wait
func (t *TestFramework) CrictlWithEndpointNoWait(endpoint, args string) *Session {
	return lcmd("crictl --runtime-endpoint=%s %s", endpoint, args)
}

// Run crictl and expect exit, expectedOut, expectedErr
func (t *TestFramework) CrictlExpect(
	endpoint, args string, exit int, expectedOut, expectedErr string,
) {
	// When
	res := t.CrictlWithEndpoint(endpoint, args)

	// Then
	Expect(res).To(Exit(exit))
	if expectedOut == "" {
		Expect(string(res.Out.Contents())).To(BeEmpty())
	} else {
		Expect(res.Out).To(Say(expectedOut))
	}
	if expectedErr == "" {
		Expect(string(res.Err.Contents())).To(BeEmpty())
	} else {
		Expect(res.Err).To(Say(expectedErr))
	}
}

// Run crictl and expect success containing the specified output
func (t *TestFramework) CrictlExpectSuccess(args, expectedOut string) {
	t.CrictlExpect("", args, 0, expectedOut, "")
}

// Run crictl and expect success containing the specified output
func (t *TestFramework) CrictlExpectSuccessWithEndpoint(endpoint, args, expectedOut string) {
	t.CrictlExpect(endpoint, args, 0, expectedOut, "")
}

// Run crictl and expect error containing the specified outputs
func (t *TestFramework) CrictlExpectFailure(
	args string, expectedOut, expectedErr string,
) {
	t.CrictlExpect("", args, 1, expectedOut, expectedErr)
}

// Run crictl and expect failure containing the specified output
func (t *TestFramework) CrictlExpectFailureWithEndpoint(
	endpoint, args, expectedOut, expectedErr string,
) {
	t.CrictlExpect(endpoint, args, 1, expectedOut, expectedErr)
}

func SetupCrio() string {
	const (
		crioURL       = "https://github.com/cri-o/cri-o"
		crioVersion   = "v1.26.4"
		conmonURL     = "https://github.com/containers/conmon"
		conmonVersion = "v2.1.7"
	)
	tmpDir := filepath.Join(os.TempDir(), "crio-tmp")

	if _, err := os.Stat(tmpDir); errors.Is(err, os.ErrNotExist) {
		logrus.Info("cloning and building CRI-O")

		Expect(checkoutAndBuild(tmpDir, crioURL, crioVersion)).To(BeNil())

		conmonTmp := filepath.Join(tmpDir, "conmon")
		checkoutAndBuild(conmonTmp, conmonURL, conmonVersion)
	}

	return tmpDir
}

func checkoutAndBuild(dir, url, rev string) error {
	// A much faster approach than just cloning the whole repository
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	cmd(dir, "git init").Wait(timeout)
	cmd(dir, "git remote add origin %s", url).Wait(timeout)
	cmd(dir, "git fetch --depth=1 origin %s", rev).Wait(timeout)
	cmd(dir, "git checkout -f FETCH_HEAD").Wait(timeout)
	cmd(dir, "make").Wait(timeout)
	return nil
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

	for _, d := range []string{
		"cni-config", "root", "runroot", "log", "exits", "attach",
	} {
		Expect(os.MkdirAll(filepath.Join(tmpDir, d), 0755)).To(BeNil())
	}

	lcmd("cp %s %s", filepath.Join(t.crioDir, "contrib", "cni",
		"10-crio-bridge.conf"),
		filepath.Join(tmpDir, "cni-config")).Wait()

	endpoint := filepath.Join(tmpDir, "crio.sock")

	session := cmd(tmpDir, "%s"+
		" --config=%s"+
		" --listen=%s"+
		" --conmon=%s"+
		" --container-exits-dir=%s"+
		" --container-attach-socket-dir=%s"+
		" --log-dir=%s"+
		" --signature-policy=%s"+
		" --cni-config-dir=%s"+
		" --root=%s"+
		" --runroot=%s"+
		" --pinns-path=%s"+
		" --enable-pod-events",
		filepath.Join(tmpDir, "bin", "crio"),
		filepath.Join(t.crioDir, "crio.conf"),
		endpoint,
		filepath.Join(t.crioDir, "conmon", "bin", "conmon"),
		filepath.Join(tmpDir, "exits"),
		filepath.Join(tmpDir, "attach"),
		filepath.Join(tmpDir, "log"),
		filepath.Join(tmpDir, "policy.json"),
		filepath.Join(tmpDir, "cni-config"),
		filepath.Join(tmpDir, "root"),
		filepath.Join(tmpDir, "runroot"),
		filepath.Join(tmpDir, "bin", "pinns"),
	)

	endpoint = "unix://" + endpoint

	// Wait for the connection to be available
	for i := 0; i < 100; i++ {
		res := t.CrictlWithEndpoint(endpoint, "--timeout=200ms info")
		if res.ExitCode() == 0 {
			break
		}
		logrus.Info("Waiting for CRI-O to become ready")
		time.Sleep(3 * time.Second)
	}
	return endpoint, tmpDir, session
}

// Stop the container runtime process
func (t *TestFramework) StopCrio(testDir string, session *Session) {
	Expect(session.Interrupt().Wait()).To(Exit(0))
}
