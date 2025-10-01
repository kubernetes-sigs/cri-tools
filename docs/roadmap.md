# ROADMAP

CRI tools is a set of tools for Kubelet Container Runtime Interface (CRI). It is

- designing for all CRI-compatible container runtimes.
- aiming to help runtime maintainers for validation and debugging
- portable and easy to use

## Components

CRI tools is composed of two components:

- critest: validation and performance benchmark test suites
- crictl: CLI for Kubelet CRI

## Roadmap

### Kubernetes v1.7

- CRI validation test suites
  - Basic sandbox/container operations
  - Basic image operations
  - networking
    - DNS config
    - port mapping
  - streaming
    - exec, attach, portforward
  - security context
    - hostPID, hostIPC, hostNetwork
    - runAsUser, readOnlyRootfs, privileged
  - execSync
  - version
  - status

### Kubernetes v1.8

- Supporting TLS for streaming operations
- Validation test suites for experimental features, e.g. SELinux, Apparmor, Seccomp, Devices, Resource Limits
- Benchmark test suites, e.g.
  - Latency
  - resource usage (CPU, memory, disk IO, network IO)
- UX improving, e.g.
  - support more output formats: -o json|yaml
- Debugging
