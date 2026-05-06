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
	"runtime"
)

const (
	// The port used in hostNetNginxImage (See images/hostnet-nginx/).
	webServerHostNetContainerPort int32 = 12003

	// The host port for hostnet port-forward tests; only used on Linux.
	webServerHostPortForHostNetPortForward int32 = 12002
)

const hostNetWebServerImage = registry + "hostnet-nginx-" + runtime.GOARCH
