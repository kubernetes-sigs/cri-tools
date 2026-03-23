/*
Copyright 2025 The Kubernetes Authors.

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
	"fmt"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	remoteclient "k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
)

const (
	TransportWebsocket = "websocket"
	TransportSpdy      = "spdy"
)

func GetExecutor(transport string, parsedURL *url.URL, tlsConfig *rest.TLSClientConfig) (exec remoteclient.Executor, err error) {
	config := &rest.Config{TLSClientConfig: *tlsConfig}

	switch transport {
	case TransportSpdy:
		return remoteclient.NewSPDYExecutor(config, "POST", parsedURL)

	case TransportWebsocket:
		return remoteclient.NewWebSocketExecutor(config, "GET", parsedURL.String())

	default:
		return nil, fmt.Errorf("unknown transport: %s", transport)
	}
}

func GetDialer(transport string, parsedURL *url.URL, tlsConfig *rest.TLSClientConfig) (exec httpstream.Dialer, err error) {
	config := &rest.Config{TLSClientConfig: *tlsConfig}

	switch transport {
	case TransportSpdy:
		tr, upgrader, err := spdy.RoundTripperFor(config)
		if err != nil {
			return nil, fmt.Errorf("get SPDY round tripper: %w", err)
		}

		return spdy.NewDialer(upgrader, &http.Client{Transport: tr}, "POST", parsedURL), nil

	case TransportWebsocket:
		return portforward.NewSPDYOverWebsocketDialer(parsedURL, config)

	default:
		return nil, fmt.Errorf("unknown transport: %s", transport)
	}
}
