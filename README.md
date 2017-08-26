# cri-tools

CLI and validation tools for Kubelet Container Runtime Interface (CRI) .

[![Build Status](https://travis-ci.org/kubernetes-incubator/cri-tools.svg?branch=master)](https://travis-ci.org/kubernetes-incubator/cri-tools)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-incubator/cri-tools)](https://goreportcard.com/report/github.com/kubernetes-incubator/cri-tools)

## What is the scope of this project?

cri-tools aims to provide a series of debugging and validation tools for Kubelet CRI, which includes:

- crictl: CLI for kubelet CRI.
- critest: validation test suites for kubelet CRI.

## What is not in scope for this project?

* Building a new kubelet container runtime based on CRI.
* Managing pods/containers for CRI-compatible runtimes by end-users, e.g. pods created by crictl may be removed automatically by kubelet because of non-exist on the kube-apiserver.

## Current Status

We are currently working toward an alpha version of CRI validation tests to be used in conjunction with Kubernetes 1.7. See the [roadmap](docs/roadmap.md) for information about current and future milestones.

## Documentation

- [CRI validation test suite](docs/validation.md)
- [CRI performance benchmarking](docs/benchmark.md)
- [CRI CLI](docs/crictl.md)

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

## Contributing

Interested in contributing? Check out the [documentation](CONTRIBUTING.md).

## Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
