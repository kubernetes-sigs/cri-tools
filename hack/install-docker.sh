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

arch=$(dpkg --print-architecture)

# readiness_check checks readiness of a daemon with specified command.
readiness_check() {
  local command=$1
  local MAX_ATTEMPTS=5
  local attempt_num=1
  until ${command} &> /dev/null || (( attempt_num == MAX_ATTEMPTS ))
  do
      echo "$attempt_num attempt \"$command\"! Trying again in $attempt_num seconds..."
      sleep $(( attempt_num++ ))
  done
}

# Workarounds for error "Failed to fetch https://packagecloud.io/github/git-lfs/ubuntu/dists/trusty/InRelease"
# TODO: remove it after the issue fixed in git-lfs.
sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 6B05F25D762E3157

# Install docker.
sudo apt-get update
sudo apt-get install -y\
    apt-transport-https \
    ca-certificates \
    curl socat \
    gnupg-agent \
    software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo add-apt-repository \
   "deb [arch=$arch] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
sudo apt-get update
# Docker is downgraded because exec process in 18.x doesn't inherit additional group id from the init process.
# See more details at https://github.com/moby/moby/issues/38865.
if [ "$arch" == amd64 ]; then
	sudo apt-get -y --allow-downgrades install docker-ce=5:18.09.5~3-0~ubuntu-xenial
elif [ "$arch" == ppc64el ]; then
	sudo apt-get -y --allow-downgrades install docker-ce=18.06.0~ce~3-0~ubuntu
fi
# Restart docker daemon.
sudo service docker restart

# Check for when docker daemon is ready
readiness_check "sudo ls -al /var/run/docker.sock"
