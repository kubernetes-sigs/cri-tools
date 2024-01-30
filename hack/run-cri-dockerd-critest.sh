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

# Run critest with default cri-dockerd.

set -o errexit
set -o nounset
set -o pipefail
set -x
export LANG=C
export LC_ALL=C
SKIP="runtime should support apparmor|runtime should support reopening container log|runtime should support execSync with timeout"

arch=$(uname -m)

# Install nsenter
if [ "$arch" == x86_64 ]; then
	docker run --rm -v /usr/local/bin:/target jpetazzo/nsenter
else
	sudo apt-get update
	sudo apt-get install -y util-linux
	SKIP="${SKIP}|runtime should support SupplementalGroups"
fi

# regex of SKIP
SKIP=$(echo $SKIP|sed "s/ /\\\s/g" -)

# Start cri-dockerd first
logs_dir="$GOPATH/logs"
mkdir -p $logs_dir
ep="unix:///var/run/cri-dockerd.sock"
sudo /usr/local/bin/cri-dockerd --network-plugin="" --container-runtime-endpoint=${ep} >$logs_dir/cri-dockerd.log 2>&1 &

# Wait a while for cri-dockerd starting.
sleep 10

# Run e2e test cases
# Skip reopen container log test because docker doesn't support it.
# Skip runtime should support execSync with timeout because docker doesn't
# support it.
# Skip apparmor test as we don't enable apparmor yet in this CI job.
sudo critest -ginkgo.skip=${SKIP}

# Run benchmark test cases
sudo critest -benchmark
