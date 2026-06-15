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

# Trigger the default sandbox creation by listing pods, then capture the mount
# baseline. containerd lazily creates its sandbox shim on the first CRI call,
# so we need to trigger it before baselining.
crictl --runtime-endpoint unix:///run/containerd/containerd.sock pods > /dev/null 2>&1 || true
sleep 1
MOUNT_BASELINE=$(mount | grep -E "containerd.*(shm|rootfs)" || true)

# Execute the command passed to the container. Capture the exit code without
# tripping `set -e` so the containerd cleanup below always runs even when the
# test command fails (mirrors CI's `|| TEST_RC=$?` pattern).
echo "Executing: $@"
EXIT_CODE=0
"$@" || EXIT_CODE=$?

# Post-test leak detection.
# Leaked sandboxes or orphaned mounts indicate a test bug — every test must
# clean up its sandboxes and containers. If a leak is detected here, the
# test run is marked as failed even if all tests passed, because leaked
# resources cause "Device or resource busy" errors in CI cleanup.
LEAK_DETECTED=0

echo "Checking for leaked sandboxes..."
LEAKED_SANDBOXES=$(crictl --runtime-endpoint unix:///run/containerd/containerd.sock pods 2>/dev/null || true)
SANDBOX_COUNT=0
if echo "$LEAKED_SANDBOXES" | grep -q "POD ID"; then
    SANDBOX_COUNT=$(echo "$LEAKED_SANDBOXES" | tail -n +2 | grep -c . || true)
fi

if [ "$SANDBOX_COUNT" -gt 0 ]; then
    echo "FAIL: $SANDBOX_COUNT sandbox(es) leaked after tests — test cleanup is broken"
    echo "$LEAKED_SANDBOXES"
    LEAK_DETECTED=1
else
    echo "No leaked sandboxes."
fi

echo "Checking for orphaned mounts..."
CURRENT_MOUNTS=$(mount | grep -E "containerd.*(shm|rootfs)" || true)
# Compare against baseline to find mounts created during the test run.
NEW_MOUNTS=$(diff <(echo "$MOUNT_BASELINE") <(echo "$CURRENT_MOUNTS") | grep "^>" | sed 's/^> //' || true)
if [ -n "$NEW_MOUNTS" ]; then
    echo "FAIL: new containerd mounts appeared during tests — sandbox teardown is incomplete"
    echo "$NEW_MOUNTS"
    echo "Dumping containerd log for mount correlation:"
    cat /var/log/containerd.log
    LEAK_DETECTED=1
else
    echo "No orphaned mounts."
fi

# Cleanup containerd
echo "Cleaning up containerd (PID: $CONTAINERD_PID)..."
kill $CONTAINERD_PID
wait $CONTAINERD_PID || true

# Fail the run if tests passed but resources leaked.
if [ "$EXIT_CODE" -eq 0 ] && [ "$LEAK_DETECTED" -eq 1 ]; then
    echo "Tests passed but leaked resources detected — marking run as failed."
    exit 1
fi

exit $EXIT_CODE
