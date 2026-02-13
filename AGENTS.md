# Agent Development Guide

This document provides a guide for agents developing in the `cri-tools` repository.

## Project Overview

`cri-tools` is a set of command-line tools for interacting with CRI (Container Runtime Interface) compatible container runtimes. The project is written in Go and uses the Ginkgo testing framework for its extensive test suite.

## Development Environment

To contribute to this project, you will need a Go development environment.

To test code you need to have a functioning container runtime installed. The two popular container runtimes are Containerd and CRI-O.

## Building and Testing

The project uses `make` for building and running tests.

### Adding dependencies

If your patch depends on new packages, add that package to the `go.mod` file,
run `make vendor` and commit the changes.

### Building

To build the `critest` binary, run the following command:

```bash
make critest
```

To build `crictl` run:

```bash
make crictl
```

### Linter rules

To install the recommended linter (`golangci-lint`), run:

```bash
./build/bin/linux/amd64/golangci-lint
make install.lint
```

Note, you may need to re-install linter if it fails as linter binary is cached and may become stale.

The right way to run linter in this repository is using makefile:

```bash
make verify-lint
```

If you encounter linting issues, first attempt to automatically fix them by running the linter with the `--fix` argument:

```bash
./build/bin/linux/amd64/golangci-lint run --fix
```

Removing linter rules or adding linter ignore directives must be the last resort. The detailed explanation must be added to each ignore directive.

### Testing

The test suite is built on top of the Ginkgo testing framework. To run the tests, you will need to first build the `critest` binary and then execute it.

#### Running `critest` tests in Parallel

The tests can be run in parallel to speed up execution. The following command runs the tests in parallel with 8 processes:

```bash
sudo PATH="$PATH:$(pwd)/build/bin/linux/amd64" ./build/bin/linux/amd64/critest --ginkgo.vv --parallel=8
```

This is how tests will run on CI/CD. In order to run a single test while debugging, run the following command:

```bash
sudo PATH="$PATH:$(pwd)/build/bin/linux/amd64" ./build/bin/linux/amd64/critest --ginkgo.vv --ginkgo.focus="<regex_to_match_test_description>"
```

#### Running Tests Serially

Some tests are sensitive to parallel execution and may fail due to race conditions. To avoid these issues, you can run the tests serially by adding the `Serial` decorator to the test case in the source code.

For example, to run a test serially, modify the test case as follows:

```go
It("should not fail on simultaneous RemoveImage calls [Conformance]", Serial, func() {
    // ...
})
```

## CI/CD

The project's CI/CD pipeline is defined in the `.github/workflows` directory. The `containerd.yml` file defines the workflow for running tests against `containerd`.

### Containerd Workflow

The `containerd.yml` workflow runs tests against different versions of `containerd` on both Linux and Windows. It also tests against different runtimes and CNI configurations.
