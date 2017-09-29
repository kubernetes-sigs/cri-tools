<!-- TOC -->

- [v0.2](#v02)
    - [CRI validation testing (critest)](#cri-validation-testing-critest)
    - [CRI CLI (crictl)](#cri-cli-crictl)
- [v0.1](#v01)
    - [Features](#features)
        - [CRI validation testing](#cri-validation-testing)
        - [crictl](#crictl)
        - [CRI performance benchmarking](#cri-performance-benchmarking)
    - [Documentation](#documentation)

<!-- /TOC -->

# v0.2

cri-tools v0.2 enhances validation testings, improves crictl UX and also fixes several bugs.  It has also updates container runtime interface (CRI) to kubernetes v1.8.

## CRI validation testing (critest)

- [#127](https://github.com/kubernetes-incubator/cri-tools/pull/127) Adds validation tests for supplemental groups
- [#135](https://github.com/kubernetes-incubator/cri-tools/pull/135) [#137](https://github.com/kubernetes-incubator/cri-tools/pull/137) and [#144](https://github.com/kubernetes-incubator/cri-tools/pull/144) Adds validation tests for seccomp 
- [#139](https://github.com/kubernetes-incubator/cri-tools/pull/139) Adds validation tests for sysctls
- [#140](https://github.com/kubernetes-incubator/cri-tools/pull/140) Adds validation tests for AppArmor
- [#141](https://github.com/kubernetes-incubator/cri-tools/pull/141) Adds validation tests for NoNewPrivs
- [#142](https://github.com/kubernetes-incubator/cri-tools/pull/142) Adds validation tests for mount propagation
- [#115](https://github.com/kubernetes-incubator/cri-tools/pull/115) Fixes image validation tests
- [#116](https://github.com/kubernetes-incubator/cri-tools/pull/116) Fixes validation message
- [#126](https://github.com/kubernetes-incubator/cri-tools/pull/126) Fixes sandbox leak in port forward validation tests

## CRI CLI (crictl)

- [#122](https://github.com/kubernetes-incubator/cri-tools/pull/122) Adds support for authenticated image pull
- [#123](https://github.com/kubernetes-incubator/cri-tools/pull/123) Improves crictl UX
- [#124](https://github.com/kubernetes-incubator/cri-tools/pull/124) Adds support for creating sandboxes and containers from yaml
- [#133](https://github.com/kubernetes-incubator/cri-tools/pull/133) Adds timeout support for container stop

# v0.1

cri-tools provides a set of tools for Kubelet Container Runtime Interface (CRI):

- **CRI validation testing**
  - provides a test framework and a suite of tests to validate that the Container Runtime Interface (CRI) server implementation meets all the requirements.
  - allows the CRI runtime developers to verify that their runtime conforms to CRI, without needing to set up Kubernetes components or run Kubernetes end-to-end tests.
- **crictl**
  - provides a CLI for CRI-compatible container runtimes.
  - allows the CRI runtime developers to debug of their runtime without needing to set up Kubernetes components.
- **CRI performance benchmarking**
  - provides a benchmarking framework for CRI-compatible container runtimes.
  - allows the CRI runtime developers to benchmark the performance of their runtime without needing to set up Kubernetes components or run Kubernetes benchmark tests.

## Features

### CRI validation testing

  - basic sandbox and container operations
  - basic image operations
  - networking, e.g. DNS config, port mapping
  - streaming, e.g. exec, attach, portforward
  - security context, e.g.
    - hostPID, hostIPC, hostNetwork
    - runAsUser, readOnlyRootfs, privileged
  - execSync,version,status

### crictl

  - get version and status
  - sandbox run, stop, status, list, and remove
  - container create, start, stop, status, list and remove
  - image pull, list, status and remove
  - streaming attach, exec and portforward

### CRI performance benchmarking

  - parallel sandbox run, stop, status, list and remove
  - parallel container create, start, stop, status, list and remove
  - parallel image pull, list and remove

## Documentation

See [cri-tools](https://github.com/kubernetes-incubator/cri-tools/#documentation).
