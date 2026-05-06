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

package validate

import (
	"context"

	internalapi "k8s.io/cri-api/pkg/apis"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"sigs.k8s.io/cri-tools/pkg/framework"
)

// createHostNetWebServerContainer creates a web server container using hostNetWebServerImage.
func createHostNetWebServerContainer(ctx context.Context, rc internalapi.RuntimeService, ic internalapi.ImageManagerService, podID string, podConfig *runtimeapi.PodSandboxConfig, prefix string) string {
	containerName := prefix + framework.NewUUID()
	containerConfig := &runtimeapi.ContainerConfig{
		Metadata: framework.BuildContainerMetadata(containerName, framework.DefaultAttempt),
		Image:    &runtimeapi.ImageSpec{Image: hostNetWebServerImage},
		Linux:    &runtimeapi.LinuxContainerConfig{},
	}

	return framework.CreateContainer(ctx, rc, ic, containerConfig, podID, podConfig)
}
