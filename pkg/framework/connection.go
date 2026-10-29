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

package framework

import (
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	criutil "k8s.io/cri-client/pkg/util"
)

// RuntimeServiceMaxMsgSize is cri-client's receive limit for a single gRPC message.
const RuntimeServiceMaxMsgSize = 16 * 1024 * 1024

// NewRuntimeServiceConnection creates a connection for direct CRI RPCs using
// cri-client's target selection, credentials, authority, receive limit and backoff.
// Keep this in sync with RemoteRuntimeServiceBuilder.Build and clientTargetForAddress
// in vendor/k8s.io/cri-client/pkg/{remote_runtime,connection_target}.go (upstream:
// https://github.com/kubernetes/cri-client/tree/master/pkg).
// The caller must close the connection and set RPC deadlines; this does not wait
// for the connection to be established or validate the runtime service.
func NewRuntimeServiceConnection(endpoint string) (*grpc.ClientConn, error) {
	addr, dialer, err := criutil.GetAddressAndDialer(endpoint)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(addr, "unix:///") {
		addr = strings.TrimPrefix(addr, "unix://")
	}

	if strings.HasPrefix(addr, "/") {
		addr = "passthrough:///" + addr
	}

	params := grpc.ConnectParams{
		Backoff:           backoff.DefaultConfig,
		MinConnectTimeout: 5 * time.Second,
	}
	params.Backoff.BaseDelay = 100 * time.Millisecond
	params.Backoff.MaxDelay = 3 * time.Second

	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithAuthority("localhost"),
		grpc.WithContextDialer(dialer),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(RuntimeServiceMaxMsgSize)),
		grpc.WithConnectParams(params),
	)
}
