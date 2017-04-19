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
$GOPATH/bin/critest
```

### Flags

`--runtime-service-address`, `-r`:The default server is dockershim. If we want to test other CRI server such as frakti, we can add flag `--runtime-service-address=/var/run/frakti.sock`. And we can run this test against frakti.

`--build-dependencies`, `-b`:If we don't need to build dependencies, we can add this flag `--build-dependencies=false` or `-b=false`.

`--ginkgo-flags`,`-g`:Space-separated list of arguments to pass to Ginkgo test runner. If we want to run the test that match the focus regular expression, we can add flag like `-g=-focus="attach"`.