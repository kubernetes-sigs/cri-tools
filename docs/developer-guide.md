# Developer Guide

## Pre-requisites

Start with the following guide to setup pre-requisites:

[Setup-Guide-for-Kubernetes-Developers](https://developer.ibm.com/articles/setup-guide-for-kubernetes-developers/)

**Skip the `kubernetes` steps from the guide above if you don't plan on doing any development for `kubernetes` projects.**

`cri-tools` need the following dependencies installed for a development environment:

- [Go](https://golang.org/doc/install)
- [Build tools](https://github.com/containerd/cri#install-dependencies)

_**Important Note**: `Go` dependencies tend to follow the GitHub project URL path structure, i.e. when installing and trying to use `cri-tools` locally, it should be installed under a folder structure as follows `go/src/sigs.k8s.io/cri-tools`. Kubernetes is the only exception which does not follow this structure for legacy reasons._

Ensure the following after following the setup:

- There is a `go` directory within `$HOME` and the `go/bin` binary is a part of your `$PATH` environment variable

  ```bash
    $ ls $HOME
    go

    $ echo $PATH | grep go
    PATH=$PATH:/usr/local/go/bin:$HOME/go/bin # $PATH here refers to truncated version of additional `env` paths that are unrelated this guide / setup
  ```

## Build & install `containerd/cri`

The latest development version of `containerd` or CRI-O is required.

This guide will use `containerd/cri` as the container runtime. You may use CRI-O instead but you may need some additional steps to ensure it works. You can also follow the CRI-O [install guide](https://github.com/cri-o/cri-o/blob/master/install.md#install-packaged-versions-of-cri-o) if you prefer to use the packaged version.

Follow the [`containerd/cri` dev guide](https://github.com/containerd/cri#getting-started-for-developers) to `make` and install the `containerd/cri` runtime.

## Build & install `cri-tools`

You can build and install `cri-tools` as follows:

- Install dependencies:

  ```bash
    $ make install.tools
  ```

- Build `cri-tools` (`critest` and `crictl`) and install to common location:

  ```bash
    $ make && make install # prefix sudo if run into permissions issues
  ```

If all goes well at this point you should be able to check for `critest` and `crictl` binaries

```bash
  $ which crictl
  /usr/local/bin/crictl

  $ which critest
  /usr/local/bin/critest
```
