/*
Copyright 2023 The Kubernetes Authors.

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

package common

import (
	"context"

	internalapi "k8s.io/cri-api/pkg/apis"
	runtimev1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	DefaultSystemdCgroupSlice = "/test.slice"
)

func GetCgroupParent(ctx context.Context, c internalapi.RuntimeService) string {
	runtimeConfig, err := c.RuntimeConfig(ctx)
	if err != nil {
		return DefaultSystemdCgroupSlice
	}

	if runtimeConfig == nil || runtimeConfig.GetLinux() == nil {
		return DefaultSystemdCgroupSlice
	}

	cgroupDriver := runtimeConfig.GetLinux().GetCgroupDriver()
	if cgroupDriver == runtimev1.CgroupDriver_CGROUPFS {
		return ""
	}

	return DefaultSystemdCgroupSlice
}
