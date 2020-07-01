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

# Workarounds for error "Failed to fetch https://packagecloud.io/github/git-lfs/ubuntu/dists/bionic/InRelease"
# TODO: remove it after the issue fixed in git-lfs.
sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 6B05F25D762E3157
sudo apt-get update

# Install prereqs
sudo apt-get install -y \
  conntrack iptables iproute2 ethtool socat util-linux mount ebtables udev kmod \
  libseccomp2

# Install docker.
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
sudo apt-cache madison docker-ce
sudo apt-get install docker-ce docker-ce-cli containerd.io

# Restart docker daemon.
sudo service docker restart
