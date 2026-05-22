# Agent Development Guide

This document provides a guide for agents developing in the `cri-tools` repository.

## Project Overview

`cri-tools` is a set of command-line tools for interacting with CRI (Container Runtime Interface) compatible container runtimes. The project is written in Go and uses the Ginkgo testing framework for its extensive test suite.

## Development Environment

To contribute to this project, you will need a Go development environment.

To test code you need to have a functioning container runtime installed. The two popular container runtimes are Containerd and CRI-O.

## Building and Testing

The project uses `make` for building and running tests.

### Validation

Before completing any task, agents **must** verify their changes by running `make verify` and ensuring relevant tests pass (see the [Testing](#testing) section).

Failure to run these checks may result in CI failures.

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

To build the `crictl` e2e tests binary, run:

```bash
make crictl-e2e
```

### Linter rules

To install the recommended linter (`golangci-lint`), run:

```bash
make install.lint
```

The right way to run linter in this repository is using makefile:

```bash
make verify-lint
```

If you encounter linting issues, first attempt to automatically fix them by `lint-fix` target:

```bash
make lint-fix
```

Removing linter rules or adding linter ignore directives must be the last resort. The detailed explanation must be added to each ignore directive.

### Prettier

Prettier is used to maintain consistent formatting, primarily for Markdown files. To use these targets, you must have the Node.js toolchain (`npm` and `npx`) installed on your system.

To install Prettier locally, run:

```bash
make install.prettier
```

To verify the formatting (highly recommended when modifying `.md` files), run:

```bash
make verify-prettier
```

To automatically fix formatting issues, run:

```bash
make prettier-fix
```

### Testing

The test suite is built on top of the Ginkgo testing framework. To run the tests, you will need to first build the `critest` binary and then execute it.

#### Running `critest` tests in Parallel

The tests can be run in parallel to speed up execution. The following command runs the tests in parallel with 8 processes (replace `<GOOS>` and `<GOARCH>` with your operating system and architecture, e.g., `linux` and `amd64`):

```bash
sudo PATH="$PATH:$(pwd)/build/bin/<GOOS>/<GOARCH>" ./build/bin/<GOOS>/<GOARCH>/critest --ginkgo.vv --parallel=8
```

This is how tests will run on CI/CD. In order to run a single test while debugging, run the following command:

```bash
sudo PATH="$PATH:$(pwd)/build/bin/<GOOS>/<GOARCH>" ./build/bin/<GOOS>/<GOARCH>/critest --ginkgo.vv --ginkgo.focus="<regex_to_match_test_description>"
```

#### Running Tests in a Container

If you don't have a local container runtime installed, you can run the tests in a container using Docker. This will build a local image with `containerd` and run the tests inside it. This approach doesn't require `sudo` if your user has access to Docker.

**Note:** AppArmor tests must be skipped as the containerized environment does not support them. The `make` targets below already include the skip flag.

##### Running `critest` with containerd

To run all `critest` validation tests:

```bash
make test-critest-containerd
```

##### Running `crictl` e2e tests with containerd

To run `crictl` e2e tests:

```bash
make test-crictl-e2e-containerd
```

##### Running a subset of tests

You can pass `TESTFLAGS` to focus on a specific test.

```bash
make test-critest-containerd TESTFLAGS='--ginkgo.focus="public image"'
```

Or for `crictl` e2e:

```bash
make test-crictl-e2e-containerd TESTFLAGS='--ginkgo.focus="pull"'
```

##### Selecting the containerd version

By default the image builds containerd from source at the `main` git ref,
matching CI's primary matrix entry (NRI enabled, `--parallel=8`). Pass
`CONTAINERD_VERSION` to test another ref, and `RUNC_FLAVOR` / `RUNTIME` to
change the runtime:

```bash
make test-critest-containerd CONTAINERD_VERSION=release/1.7
make test-critest-containerd CONTAINERD_VERSION=main RUNC_FLAVOR=crun
```

critest runs with `--parallel=8` by default (mirroring CI); override with
`make test-critest-containerd PARALLEL=4`.

The built image is tagged per version (`containerd-local-test:<version>`, with
`-<flavor>` appended for a non-default `RUNC_FLAVOR`, e.g.
`containerd-local-test:main-crun`) and each version gets its own data volume,
so images are cached and reused across runs — only a changed
`CONTAINERD_VERSION`/`RUNC_FLAVOR` triggers a rebuild. Set `FORCE_REBUILD=1` to
force a fresh build that bypasses the Docker layer cache (`--no-cache --pull`),
which is needed to refetch a moving ref such as `main`. To delete the cached
images and volumes so they are regenerated:

```bash
make clean-containerd-test-images
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

## Skills

- [After Release](.skills/after-release.md) — Update the master branch after a release is created
