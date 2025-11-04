# cri-tools

CLI and validation tools for Kubelet Container Runtime Interface (CRI) .

[![Build Status](https://github.com/kubernetes-sigs/cri-tools/workflows/build/badge.svg)](https://github.com/kubernetes-sigs/cri-tools/actions/workflows/build.yml)
[![critest containerd](https://github.com/kubernetes-sigs/cri-tools/workflows/critest%20containerd/badge.svg)](https://github.com/kubernetes-sigs/cri-tools/actions/workflows/containerd.yml)
[![CRI-O](https://github.com/kubernetes-sigs/cri-tools/workflows/CRI-O/badge.svg)](https://github.com/kubernetes-sigs/cri-tools/actions/workflows/crio.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/cri-tools)](https://goreportcard.com/report/github.com/kubernetes-sigs/cri-tools)

## What is the scope of this project?

cri-tools aims to provide a series of debugging and validation tools for Kubelet CRI, which includes:

- crictl: CLI for kubelet CRI.
- critest: validation test suites for kubelet CRI.

## What is not in scope for this project?

- Building a new kubelet container runtime based on CRI.
- Managing pods/containers for CRI-compatible runtimes by end-users, e.g. pods created by crictl may be removed automatically by kubelet because of non-exist on the kube-apiserver.

## Compatibility matrix: cri-tools ⬄ Kubernetes

| Kubernetes Version | cri-tools Version | cri-tools branch |
| ------------------ | ----------------- | ---------------- |
| ≥ 1.27.x           | ≥ 1.27.x          | master           |
| ≥ 1.16.x ≤ 1.26.x  | ≥ 1.16.x ≤ 1.26.x | master           |
| 1.15.X             | v1.15.0           | release-1.15     |
| 1.14.X             | v1.14.0           | release-1.14     |
| 1.13.X             | v1.13.0           | release-1.13     |
| 1.12.X             | v1.12.0           | release-1.12     |
| 1.11.X             | v1.11.1           | release-1.11     |
| 1.10.X             | v1.0.0-beta.2     | release-1.10     |
| 1.9.X              | v1.0.0-alpha.1    | release-1.9      |
| 1.8.X              | v0.2              | release-1.8      |
| 1.7.X              | v0.1              | release-1.7      |

It's recommended to use the same cri-tools and Kubernetes minor version, because
new features added to the Container Runtime Interface (CRI) may not be fully
supported if they diverge.

cri-tools follows the Kubernetes release cycles with respect to its minor
versions (`1.x.y`). Patch releases (`1.x.z`) for Kubernetes are not in sync with
those from cri-tools, because they are scheduled for each month, whereas
cri-tools provides them only if necessary. If a Kubernetes release goes [End of
Life](https://kubernetes.io/releases/patch-releases/), then the corresponding
cri-tools version can be considered in the same way.

All new minor versions of cri-tools are being created from the `master` branch,
whereas corresponding `release-1.x` branches will be created if a patch release
is planned.

See the [roadmap](docs/roadmap.md) for information about current and future
milestones.

## Install

### Install crictl

```sh
VERSION="v1.34.0"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-amd64.tar.gz
```

### Install critest

```sh
VERSION="v1.34.0"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/critest-$VERSION-linux-amd64.tar.gz
sudo tar zxvf critest-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f critest-$VERSION-linux-amd64.tar.gz
```

## Documentation

- **[CRI `crictl` CLI](docs/crictl.md)**
- [CRI validation test suite](docs/validation.md)
- [CRI performance benchmarking](docs/benchmark.md)

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- Slack: #sig-node
- Mailing List: <https://groups.google.com/a/kubernetes.io/g/sig-node>

## Contributing

Interested in contributing? Check out the [documentation](CONTRIBUTING.md).

## Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
