#!/usr/bin/env bash

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

set -euo pipefail

# This script sets up containerd configuration and CNI networking.
# It is used by both the local containerized tests and the CI.

CONTD_CONFIG_DIR="${CONTD_CONFIG_DIR:-/etc/containerd}"
CNI_CONFIG_DIR="${CNI_CONFIG_DIR:-/etc/cni/net.d}"
RUNTIME="${RUNTIME:-io.containerd.runc.v2}"

echo "Setting up containerd configuration in ${CONTD_CONFIG_DIR}..."
mkdir -p "${CONTD_CONFIG_DIR}"
cat <<EOF > "${CONTD_CONFIG_DIR}/config.toml"
version = 2
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
        runtime_type = "${RUNTIME}"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
          SystemdCgroup = false
  [plugins."io.containerd.nri.v1.nri"]
    disable = false
    disable_connections = false
    plugin_path = "/opt/nri/plugins"
    socket_path = "/var/run/nri/nri.sock"
EOF

echo "Setting up NRI socket directory..."
mkdir -p /var/run/nri
mkdir -p /opt/nri/plugins

echo "Setting up CNI networking in ${CNI_CONFIG_DIR}..."
mkdir -p "${CNI_CONFIG_DIR}"
cat <<EOF > "${CNI_CONFIG_DIR}/10-containerd-net.conflist"
{
  "cniVersion": "1.0.0",
  "name": "containerd-net",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "cni0",
      "isGateway": true,
      "ipMasq": true,
      "promiscMode": true,
      "ipam": {
        "type": "host-local",
        "ranges": [
          [{ "subnet": "10.88.0.0/16" }]
        ],
        "routes": [{ "dst": "0.0.0.0/0" }]
      }
    },
    {
      "type": "portmap",
      "capabilities": { "portMappings": true }
    },
    { "type": "firewall" },
    { "type": "tuning" }
  ]
}
EOF

echo "Containerd and CNI setup complete."

# Create empty crictl config to suppress warnings
touch /etc/crictl.yaml

# Pre-create cgroup for sandboxes if we have write access to /sys/fs/cgroup
if [ -w /sys/fs/cgroup ]; then
    # Enable controllers in cgroup v2 if present
    if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
        # Enable all available controllers on the root
        for c in $(cat /sys/fs/cgroup/cgroup.controllers); do
            echo "+$c" > /sys/fs/cgroup/cgroup.subtree_control || true
        done
        # Create k8s.io cgroup and enable controllers on it
        mkdir -p /sys/fs/cgroup/k8s.io
        if [ -f /sys/fs/cgroup/k8s.io/cgroup.controllers ]; then
            for c in $(cat /sys/fs/cgroup/k8s.io/cgroup.controllers); do
                echo "+$c" > /sys/fs/cgroup/k8s.io/cgroup.subtree_control || true
            done
        fi
    fi
fi
