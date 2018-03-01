REM Copyright 2018 The Kubernetes Authors.
REM
REM Licensed under the Apache License, Version 2.0 (the "License");
REM you may not use this file except in compliance with the License.
REM You may obtain a copy of the License at
REM
REM     http://www.apache.org/licenses/LICENSE-2.0
REM
REM Unless required by applicable law or agreed to in writing, software
REM distributed under the License is distributed on an "AS IS" BASIS,
REM WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
REM See the License for the specific language governing permissions and
REM limitations under the License.

REM Clone kubernetes codes
git clone https://github.com/kubernetes/kubernetes %GOPATH%\src\k8s.io\kubernetes

REM recreat symbol links
cd %GOPATH%\src\k8s.io\kubernetes\vendor\k8s.io
del api  apiextensions-apiserver  apimachinery  apiserver  client-go  code-generator kube-aggregator  metrics  sample-apiserver  sample-controller
mklink /d api ..\..\staging\src\k8s.io\api
mklink /d apiextensions-apiserver ..\..\staging\src\k8s.io\apiextensions-apiserver
mklink /d apimachinery ..\..\staging\src\k8s.io\apimachinery
mklink /d apiserver ..\..\staging\src\k8s.io\apiserver
mklink /d client-go ..\..\staging\src\k8s.io\client-go
mklink /d code-generator ..\..\staging\src\k8s.io\code-generator
mklink /d kube-aggregator ..\..\staging\src\k8s.io\kube-aggregator
mklink /d metrics ..\..\staging\src\k8s.io\metrics
mklink /d sample-apiserver ..\..\staging\src\k8s.io\sample-apiserver
mklink /d sample-controller ..\..\staging\src\k8s.io\sample-controller

REM build k8s binaries on windows
cd ..\..
go build cmd\kubelet\kubelet.go
mkdir %GOPATH%\bin
copy kubelet.exe %GOPATH%\bin
