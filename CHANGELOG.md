
- [v0.1](#v01)
    - [Features](#features)
        - [CRI validation testing](#cri-validation-testing)
        - [crictl](#crictl)
        - [CRI performance benchmarking](#cri-performance-benchmarking)
    - [Documentation](#documentation)


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
