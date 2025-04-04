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
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"github.com/sirupsen/logrus"
)

var (
	crictlBinaryPath      string
	crictlRuntimeEndpoint string
)

func init() {
	flag.StringVar(&crictlBinaryPath, "crictl-binary-path", "", "`crictl` binary path to be used")
	flag.StringVar(&crictlRuntimeEndpoint, "crictl-runtime-endpoint", "", "`crictl --runtime-endpoint` to be used")
}

// TestFramework is used to support commonly used test features.
type TestFramework struct{}

// NewTestFramework creates a new test framework instance.
func NewTestFramework() *TestFramework {
	return &TestFramework{}
}

// Setup is the global initialization function which runs before each test
// suite.
func (t *TestFramework) Setup() {
	// Global initialization for the whole framework goes in here
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(GinkgoWriter)
}

// Teardown is the global deinitialization function which runs after each test
// suite.
func (t *TestFramework) Teardown() {
}

// Describe is a convenience wrapper around the `ginkgo.Describe` function.
func (t *TestFramework) Describe(text string, body func()) bool {
	return Describe("crictl: "+text, body)
}

// Convenience method for command creation.
func cmd(workDir, format string, args ...interface{}) *Session {
	c := strings.Split(fmt.Sprintf(format, args...), " ")

	command := exec.Command(c[0], c[1:]...)
	if workDir != "" {
		command.Dir = workDir
	}

	session, err := Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	return session
}

func crictlBinaryPathFlag() (path string) {
	if crictlBinaryPath != "" {
		return crictlBinaryPath
	}

	return "crictl"
}

func crictlRuntimeEndpointFlag() string {
	if crictlRuntimeEndpoint != "" {
		return " --runtime-endpoint=" + crictlRuntimeEndpoint
	}

	return ""
}

// Convenience method for command creation in the current working directory.
func lcmd(format string, args ...interface{}) *Session {
	return cmd("", format, args...)
}

// Run crictl on the specified endpoint and return the resulting session.
func (t *TestFramework) Crictl(args string) *Session {
	return lcmd("%s%s %s", crictlBinaryPathFlag(), crictlRuntimeEndpointFlag(), args).Wait(time.Minute)
}

// Run crictl on the specified endpoint and return the resulting session without wait.
func (t *TestFramework) CrictlNoWait(args string) *Session {
	return lcmd("%s%s %s", crictlBinaryPathFlag(), crictlRuntimeEndpointFlag(), args)
}

// Run crictl and expect exit, expectedOut, expectedErr.
func (t *TestFramework) CrictlExpect(
	args string, exit int, expectedOut, expectedErr string,
) {
	// When
	res := t.Crictl(args)

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

// Run crictl and expect success containing the specified output.
func (t *TestFramework) CrictlExpectSuccess(args, expectedOut string) {
	t.CrictlExpect(args, 0, expectedOut, "")
}

// Run crictl and expect error containing the specified outputs.
func (t *TestFramework) CrictlExpectFailure(
	args string, expectedOut, expectedErr string,
) {
	t.CrictlExpect(args, 1, expectedOut, expectedErr)
}

// CrictlRemovePause can be uased to cleanup the pause images.
func (t *TestFramework) CrictlRemovePauseImages() {
	res := t.Crictl("images --filter reference=registry.k8s.io/pause -q")
	Expect(res).To(Exit(0))

	contents := res.Out.Contents()
	if len(contents) > 0 {
		output := strings.Split(string(contents), "\n")
		t.CrictlExpectSuccess("rmi "+strings.TrimSpace(strings.Join(output, " ")), "Deleted")
	}
}

func (t *TestFramework) IsContainerd() bool {
	return strings.Contains(crictlRuntimeEndpoint, "containerd")
}
