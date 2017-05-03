# cri-tools

CLI and validation tools for Kubelet Container Runtime Interface (CRI) .

[![Build Status](https://travis-ci.org/kubernetes-incubator/cri-tools.svg?branch=master&maxAge=2592000)](https://travis-ci.org/kubernetes-incubator/cri-tools)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-incubator/cri-tools)](https://goreportcard.com/report/github.com/kubernetes-incubator/cri-tools)

## What is the scope of this project?

cri-tools aims to provide a series of debugging and validation tools for Kubelet CRI, which includes:

- crictl: CLI for kubelet CRI.
- critest: validation test suites for kubelet CRI.

## What is not in scope for this project?

* Building a new kubelet container runtime based on CRI.
* Managing pods/containers for CRI-compatible runtimes by end-users, e.g. pods created by crictl may be removed automatically by kubelet because of non-exist on the kube-apiserver.

## Roadmap

* Basic sandbox/image/container lifecycle operations
* Handle streaming APIs (exec/attach/port-forwarding)
* Reading logs of containers.
* Validation test suites.

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- Slack: #sig-node
- Mailing List: <https://groups.google.com/forum/#!forum/kubernetes-sig-node>

## Kubernetes Incubator

This is a [Kubernetes Incubator project](https://github.com/kubernetes/community/blob/master/incubator.md). The incubator team for the project is:

- Sponsor: Dawn Chen (@dchen1107)
- Champion: Yu-Ju Hong (@yujuhong)
- SIG: sig-node

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

## Getting started

### Build cri-tools

```bash
go get github.com/kubernetes-incubator/cri-tools/cmd/critest
go get github.com/kubernetes-incubator/cri-tools/cmd/critctl
```
### critest

 Validation test suites for kubelet CRI

#### Flags

- `--runtime-endpoint`, `-r`:CRI runtime server endpoint. Default is /var/run/dockershim.sock.

- `--image-endpoint`, `-i`:CRI server image endpoint, default same as runtime endpoint.

- `--compile`, `-c`:If we don't need to build dependencies, we can add this flag `--compile=false` or `-c=false`.

- `--ginkgo-flags`,`-g`:Space-separated list of arguments to pass to Ginkgo test runner.

- `--focus`,`-f`:CRI e2e test will only run the test that match the focus regular expression.

- `--benchmark`,`-b`:If set, critest will only run benchmark.

### crictl

CLI for kubelet CRI

#### Commands

- `version`: Get version information of runtime

- `sandbox`, `sb`: Manage lifecycle of podsandbox

- `container`, `ctr`:Manage lifecycle of container

- `status`:Get the status information of runtime

- `attach`:Attach a running container

- `image`:Manage image

- `exec`:Exec(exec, syncexec) a command in a running container

- `portforward`:Forword ports(localport:remoteport) from a sandbox

- `help`, `h`:Shows a list of commands or help for one command

#### Global flags

- `--runtime-endpoint`:CRI server runtime endpoint (default: "/var/run/dockershim.sock").The default server is dockershim. If we want to debug other CRI server such as frakti, we can add flag `--runtime-endpoint=/var/run/frakti.sock`

- `--image-endpoint`:CRI server image endpoint, default same as runtime endpoint.

- `--timeout`:Timeout of connecting to server (default: 10s)

- `--debug`:Enable debug output

- `--help`, `-h`:show help

- `--version`, `-v`:print the version information of crictl

#### Examples

- Run podsandbox with config file

`crictl sandbox run pod.json`

example of config file for podsandbox `pod.json`:

```
{
    "metadata": {
        "name": "podsandbox-1",
        "namespace": "default",
        "attempt": "2",
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "linux": {
    }
}
```
- Create container in a sandbox with container config file

`crictl container create --pod podid --connfig container.json` (replace `podid` with id of a ready sandbox)

example of config file for container `container.json`:

```
    {
    "metadata": {
        "name": "container-1"
    },
    "image":{
        "image": "busybox"
    },
    "command": [
        "sh",
        "-c"
    ],
    "linux": {
    }
}
```
