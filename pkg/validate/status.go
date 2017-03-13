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
	"github.com/kubernetes-incubator/cri-tools/pkg/framework"
	internalapi "k8s.io/kubernetes/pkg/kubelet/api"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.KubeDescribe("Status", func() {
	f := framework.NewDefaultCRIFramework()

	var c internalapi.RuntimeService
	var count int

	BeforeEach(func() {
		c = f.CRIClient.CRIRuntimeClient
	})

	It("runtime should return runtime conditions [Conformance]", func() {
		By("test runtime status")
		status, err := c.Status()
		framework.ExpectNoError(err, "failed to get runtime conditions: %v", err)

		for _, condition := range status.Conditions {
			if condition.Type == "RuntimeReady" && condition.Status == true {
				count = count + 1
			}
			if condition.Type == "NetworkReady" && condition.Status == true {
				count = count + 1
			}
		}
		Expect(count >= 2).To(BeTrue(), "should return all the required runtime conditions")
	})
})
