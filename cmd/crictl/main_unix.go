// +build !windows

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

package main

import (
	"net"
	"time"

	"k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	defaultConfigPath      = "/etc/crictl.yaml"
	defaultRuntimeEndpoint = "unix:///var/run/dockershim.sock"
)

// GetAddressAndDialer returns the address and a dialer for the endpoint
// protocol.
//
// On Unix supported protocols are unix sockets.
//
// Examples:
//
// An endpoint of "unix:///var/run/dockershim.sock" returns address
// "/var/run/dockershim.sock" and a unix socket dialer for this address.
//
// An endpoint of "/var/run/dockershim.sock" returns address
// "/var/run/dockershim.sock" and a unix socket dialer for this address.
func GetAddressAndDialer(endpoint string) (string, func(addr string, timeout time.Duration) (net.Conn, error), error) {
	return util.GetAddressAndDialer(endpoint)
}
