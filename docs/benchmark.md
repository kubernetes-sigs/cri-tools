# Container Runtime Interface (CRI) Performance Benchmarking

CRI performance benchmarking provides a benchmarking framework for CRI-compatible container runtimes.  This allows the CRI runtime developers to benchmark the performance of their runtime without needing to set up Kubernetes components or run Kubernetes benchmark tests.

## Install

The benchmarking tests can be installed easily via `go get` command:

```sh
go get github.com/kubernetes-incubator/cri-tools/cmd/critest
```

Then `critest` binary can be found in `$GOPATH/bin`.

*Note: ensure GO is installed and GOPATH is set before installing critest.*

## Running tests

### Prerequisite

Before running the test, you need to _ensure that the CRI server under test is running and listening on a Unix socket_. Because the benchmark tests are designed to request changes (e.g., create/delete) to the containers and verify that correct status is reported, it expects to be the only user of the CRI server. Please make sure that 1) there are no existing CRI-managed containers running on the node, and 2) no other processes (e.g., Kubelet) will interfere with the tests.

### Run

```sh
critest benchmark
```

This will

- Connect to the shim of CRI container runtime
- Run the benchmark tests using `ginkgo`
- Output the test results to STDOUT

critest connects to `/var/run/dockershim.sock` by default. For other runtimes, the endpoint can be set in two ways:

- By setting flags `--runtime-endpoint` and `--image-endpoint`
- By setting environment variables `CRI_RUNTIME_ENDPOINT` and `CRI_IMAGE_ENDPOINT`

## Additional options

- `--focus`, `-f`: Only run the tests that match the regular expression.
- -`-ginkgo-flags`, `-g`: Space-separated list of arguments to pass to Ginkgo test runner.
- `--image-endpoint`, `-i`: Set the endpoint of image service. Same with runtime-endpoint if not specified.
- `--runtime-endpoint`, `-r`: Set the endpoint of runtime service. Default to `/var/run/dockershim.sock`.
- `--skip`, `-s`: Skip the tests that match the regular expression.
