# Copyright The Kubernetes Authors.
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

FROM ubuntu:24.04

# Install dependencies and containerd
RUN apt-get update && apt-get install -y \
    apparmor-utils \
    ca-certificates \
    containerd \
    curl \
    iptables \
    libseccomp2 \
    socat \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Install CNI plugins
ARG CNI_PLUGINS_VERSION=v1.4.0
RUN mkdir -p /opt/cni/bin && \
    curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-$(dpkg --print-architecture)-${CNI_PLUGINS_VERSION}.tgz" | \
    tar -C /opt/cni/bin -xz

# Copy setup scripts
COPY hack/setup-containerd.sh /usr/local/bin/setup-containerd.sh
COPY hack/wait-for-containerd.sh /usr/local/bin/wait-for-containerd.sh
RUN chmod +x /usr/local/bin/setup-containerd.sh /usr/local/bin/wait-for-containerd.sh

# Copy entrypoint
COPY images/containerd-local-test/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV PATH="/usr/local/bin/critest-tools:${PATH}"

ENTRYPOINT ["/entrypoint.sh"]
