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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

const (
	defaultAPIVersion string = "0.1.0"
)

var _ = framework.KubeDescribe("Runtime info", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.RuntimeService

	BeforeEach(func() {
		c = f.CRIClient.CRIRuntimeClient
	})
	Context("runtime should support returning runtime info", func() {
		It("runtime should return version info [Conformance]", func() {
			TestGetVersion(c)
		})

		It("runtime should return runtime conditions [Conformance]", func() {
			By("test runtime status")
			TestGetRuntimeStatus(c)
		})
	})
})

// TestGetVersion test if we can get runtime version info.
func TestGetVersion(c internalapi.RuntimeService) {
	version := getVersion(c)
	Expect(version.GetVersion()).To(Not(BeNil()), "Version should not be nil")
	Expect(version.GetRuntimeName()).To(Not(BeNil()), "RuntimeName should not be nil")
	Expect(version.GetRuntimeVersion()).To(Not(BeNil()), "RuntimeVersion should not be nil")
	Expect(version.GetRuntimeApiVersion()).To(Not(BeNil()), "RuntimeApiVersion should not be nil")
	framework.Logf("Get version info succeed")
}

// TestGetRuntimeStatus test if we can get runtime status.
func TestGetRuntimeStatus(c internalapi.RuntimeService) {
	var count int

	status, err := c.Status(context.TODO(), false)
	framework.ExpectNoError(err, "failed to get runtime conditions: %v", err)

	for _, condition := range status.GetStatus().GetConditions() {
		if condition.GetType() == "RuntimeReady" && condition.GetStatus() {
			count++
		}

		if condition.GetType() == "NetworkReady" && condition.GetStatus() {
			count++
		}
	}

	Expect(count).To(BeNumerically(">=", 2), "should return all the required runtime conditions")
}

// getVersion gets runtime version info.
func getVersion(c internalapi.RuntimeService) *runtimeapi.VersionResponse {
	version, err := c.Version(context.TODO(), defaultAPIVersion)
	framework.ExpectNoError(err, "failed to get version: %v", err)

	return version
}
