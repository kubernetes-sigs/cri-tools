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


# Install docker.
sudo sh -c 'echo "deb https://apt.dockerproject.org/repo ubuntu-$(lsb_release -cs) main" > /etc/apt/sources.list.d/docker.list'
curl -fsSL https://apt.dockerproject.org/gpg | sudo apt-key add -
sudo apt-key fingerprint 58118E89F3A912897C070ADBF76221572C52609D
sudo apt-get update
sudo apt-get -y install "docker-engine=17.03.1~ce-0~ubuntu-$(lsb_release -cs)"

# docker debs don't support seccomp, so we install a static version instead.
curl -sSL -o docker.tgz https://download.docker.com/linux/static/stable/x86_64/docker-17.03.1-ce.tgz
tar xzvf docker.tgz
sudo cp -f docker/* /usr/bin
rm -f docker.tgz

# Restart docker daemon.
sudo service docker restart

