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

# Stop on any error.
$ErrorActionPreference = "Stop"
$VerbosePreference = "Continue"

# Install kubelet
$gopath = [System.Environment]::GetEnvironmentVariable("GOPATH")
$k8siopath = $gopath + "/src/k8s.io"
$kubernetespath = $k8siopath + "/kubernetes"
mkdir -p $k8siopath
cd $k8siopath
git clone -c core.symlinks=true https://github.com/kubernetes/kubernetes

cd $kubernetespath

$branch = [System.Environment]::GetEnvironmentVariable("TRAVIS_BRANCH")
if ( ! "$branch".Equals("master") ) {
  # We can do this because cri-tools have the same branch name with kubernetes.
  git checkout "$branch"
}

# Build kubelet
$version = git describe --tags --dirty --always
go build -ldflags "-X k8s.io/kubernetes/vendor/k8s.io/client-go/pkg/version.gitVersion=$version -X k8s.io/kubernetes/pkg/version.gitVersion=$version" ./cmd/kubelet/kubelet.go
mkdir -p "$gopath/bin"
cp ./kubelet.exe "$gopath/bin/"

# Dump version
echo "Kubelet version:"
kubelet.exe --version
echo "Docker version:"
docker version
