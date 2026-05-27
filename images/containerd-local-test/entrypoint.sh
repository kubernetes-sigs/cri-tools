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

# In cgroup v2 environments, the root cgroup (the container's root)
# cannot have processes if we want to enable subtree_control.
# Move our shell to a sub-cgroup immediately.
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
    mkdir -p /sys/fs/cgroup/init
    echo 0 > /sys/fs/cgroup/init/cgroup.procs
fi

# Enable verbose logging for debugging if needed
# set -x

echo "Initializing container environment..."
# setup-containerd.sh creates the NRI socket directory itself, but only when
# NRI is enabled (containerd main); do not create it unconditionally here.
setup-containerd.sh

echo "Starting containerd..."
mkdir -p /run/containerd

# Start containerd in the background
containerd > /var/log/containerd.log 2>&1 &
CONTAINERD_PID=$!

# Wait for containerd to be ready
wait-for-containerd.sh

# Execute the command passed to the container. Capture the exit code without
# tripping `set -e` so the containerd cleanup below always runs even when the
# test command fails (mirrors CI's `|| TEST_RC=$?` pattern).
echo "Executing: $@"
EXIT_CODE=0
"$@" || EXIT_CODE=$?

# Cleanup containerd
echo "Cleaning up containerd (PID: $CONTAINERD_PID)..."
kill $CONTAINERD_PID
wait $CONTAINERD_PID || true

exit $EXIT_CODE
