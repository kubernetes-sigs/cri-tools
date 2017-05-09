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
go get github.com/kubernetes-incubator/cri-tools/cmd/crictl
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

- `info`: Get version information of runtime

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

- Run sandbox with config file

```
# cat sandbox-config.json
{
    "metadata": {
        "name": "nginx-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "linux": {
    }
}
# crictl sandbox run sandbox-config.json
9b542bfe8f93eb2d726d0f7b619f253c18858006aa53023e392e138b0be6301c
```

- Create container in a sandbox with config file

```
# cat sandbox-config.json
{
    "metadata": {
        "name": "nginx-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "linux": {
    }
}
# cat container-config.json
{
  "metadata": {
      "name": "busybox"
  },
  "image":{
      "image": "busybox"
  },
  "command": [
      "top"
  ],
  "linux": {
  }
}
# crictl container create 9b542bfe8f93eb2d726d0f7b619f253c18858006aa53023e392e138b0be6301c container-config.json sandbox-config.json
bf642f55ecf54345354a86a42c08fb0d66e55e90c855973495f31e991c2bf725
```

* Start container

```
# crictl container start bf642f55ecf54345354a86a42c08fb0d66e55e90c855973495f31e991c2bf725
bf642f55ecf54345354a86a42c08fb0d66e55e90c855973495f31e991c2bf725
```

* Exec a command in container

```
# crictl exec -i -t bf642f55ecf54345354a86a42c08fb0d66e55e90c855973495f31e991c2bf725 sh
```
