#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

set -x

CNI_VERSION="v0.8.6"
echo "Installing CNI binaries ..." \
    && export ARCH=$(dpkg --print-architecture | sed 's/ppc64el/ppc64le/' | sed 's/armhf/arm/') \
    && export CNI_TARBALL="${CNI_VERSION}/cni-plugins-linux-${ARCH}-${CNI_VERSION}.tgz" \
    && export CNI_URL="https://github.com/containernetworking/plugins/releases/download/${CNI_TARBALL}" \
    && curl -sSL --retry 5 --output /tmp/cni.tgz "${CNI_URL}" \
    && mkdir -p /opt/cni/bin \
    && tar -C /opt/cni/bin -xzf /tmp/cni.tgz \
    && rm -rf /tmp/cni.tgz

# Install kubelet
cd $GOPATH/src/k8s.io/kubernetes

make WHAT='cmd/kubelet'
sudo cp _output/bin/kubelet /usr/local/bin
