<!-- TOC -->

- [v1.0.0-alpha.0](#v100-alpha0)
        - [CRI validation testing (critest)](#cri-validation-testing-critest)
        - [CRI CLI (crictl)](#cri-cli-crictl)
- [v0.2](#v02)
    - [CRI validation testing (critest)](#cri-validation-testing-critest-1)
    - [CRI CLI (crictl)](#cri-cli-crictl-1)
- [v0.1](#v01)
    - [Features](#features)
        - [CRI validation testing](#cri-validation-testing)
        - [crictl](#crictl)
        - [CRI performance benchmarking](#cri-performance-benchmarking)
    - [Documentation](#documentation)

<!-- /TOC -->

# v1.0.0-alpha.0

cri-tools v1.0.0-alpha.0 is mainly focus on UX improvements, including make crictl command more user friendly and add more subcommands. It also updates container runtime interface (CRI) to kubernetes v1.9 and fixes bugs in validation test suites.

### CRI validation testing (critest)

- [#164](https://github.com/kubernetes-incubator/cri-tools/pull/164) Fix security context test to not rely on `/etc/hosts`
- [#165](https://github.com/kubernetes-incubator/cri-tools/pull/165) Validate IPv4 only for port mapping tests
- [#196](https://github.com/kubernetes-incubator/cri-tools/pull/196) Fix privileged container validation by replacing `ip link` with `brctl addbr` command
-  [#197](https://github.com/kubernetes-incubator/cri-tools/pull/197) Fix hostIPC validation to support old ipcmk versions 
- [#199](https://github.com/kubernetes-incubator/cri-tools/pull/199) [#201](https://github.com/kubernetes-incubator/cri-tools/pull/201) Fix container logs validation
- [#200](https://github.com/kubernetes-incubator/cri-tools/pull/200) Add SELinux validation tests

### CRI CLI (crictl)

- [#156](https://github.com/kubernetes-incubator/cri-tools/pull/156) Fix empty RepoTags handling for `images` command
- [#163](https://github.com/kubernetes-incubator/cri-tools/pull/163) Add `--digest` option to `images` command
- [#167](https://github.com/kubernetes-incubator/cri-tools/pull/167) Add verbose for `status` command
- [#171](https://github.com/kubernetes-incubator/cri-tools/pull/171) Sort results by creation time for `ps`, `sandboxes` and `images` commands
- [#174](https://github.com/kubernetes-incubator/cri-tools/pull/174) Support select sandboxes by name for `sandboxes` and other commands 
- [#178](https://github.com/kubernetes-incubator/cri-tools/pull/178) [#190](https://github.com/kubernetes-incubator/cri-tools/pull/190) Replace golang json with `protobuf/jsonpb` library
- [#182](https://github.com/kubernetes-incubator/cri-tools/pull/182) Fix stdout and stderr for `attach` and `exec` command
- [#183](https://github.com/kubernetes-incubator/cri-tools/pull/183) Add created time to `sandboxes` command
- [#186](https://github.com/kubernetes-incubator/cri-tools/pull/186) Use kubelet's log library instead of a copied one 
- [#187](https://github.com/kubernetes-incubator/cri-tools/pull/187) Add image tag and attempt to `ps` command
- [#194](https://github.com/kubernetes-incubator/cri-tools/pull/194) Add `config` command
- [#217](https://github.com/kubernetes-incubator/cri-tools/pull/217) Add `--latest` and `--last` options to `ps` and `sandboxes` commands
- [#202](https://github.com/kubernetes-incubator/cri-tools/pull/202) [#203](https://github.com/kubernetes-incubator/cri-tools/pull/203) Add `--all`, `--latest`, `--last` and `--no-trunc` options to `ps` command
- [#205](https://github.com/kubernetes-incubator/cri-tools/pull/205) Improve logs command and add `--timestamps` and `--since` options
- [#206](https://github.com/kubernetes-incubator/cri-tools/pull/206) Add verbose debut output to `inspect` and `inspects` commands
- [#207](https://github.com/kubernetes-incubator/cri-tools/pull/207) Sort flags for all commands
- [#209](https://github.com/kubernetes-incubator/cri-tools/pull/209) Add `stats` command
- [#211](https://github.com/kubernetes-incubator/cri-tools/pull/211) Rewrite timestamps in container status and sandbox status to make them more user friendly
- [#213](https://github.com/kubernetes-incubator/cri-tools/pull/213) Add completion command
- [#216](https://github.com/kubernetes-incubator/cri-tools/pull/216) Add `--no-trunc` to `images` and `sandboxes` commands

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
