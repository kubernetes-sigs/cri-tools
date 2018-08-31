// +build windows

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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	defaultRuntimeEndpoint = "tcp://localhost:3735"
)

var defaultConfigPath string

func init() {
	defaultConfigPath = filepath.Join(os.Getenv("USERPROFILE"), ".crictl", "crictl.yaml")
}

// GetAddressAndDialer returns the address and a dialer for the endpoint
// protocol.
//
// On Windows supported protocols are Windows named pipes and tcp.
//
// Examples:
//
// An endpoint of "tcp://localhost:3735" returns address "localhost:3735" and a
// tcp socket dialer for this address.
//
// An endpoint of "\\.\pipe\name" returns an address of "\\.\pipe\name" and a
// Windows named pipe dialer for this address.
func GetAddressAndDialer(endpoint string) (string, func(addr string, timeout time.Duration) (net.Conn, error), error) {
	if strings.HasPrefix(endpoint, "\\\\.\\pipe") {
		return endpoint, dial, nil
	}
	return util.GetAddressAndDialer(endpoint)
}

func dial(addr string, timeout time.Duration) (net.Conn, error) {
	return winio.DialPipe(addr, &timeout)
}
