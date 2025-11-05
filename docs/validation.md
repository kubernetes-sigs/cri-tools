# Container Runtime Interface (CRI) Validation Testing

CRI validation testing provides a test framework and a suite of tests to validate that the Container Runtime Interface (CRI) server implementation meets all the requirements. This allows the CRI runtime developers to verify that their runtime conforms to CRI, without needing to set up Kubernetes components or run Kubernetes end-to-end tests.

CRI validation testing is GA since v1.11.0. We encourage the CRI developers to report bugs or help extend the test coverage by adding more tests.

## Install

The benchmarking tests binary `critest` can be downloaded from [Releasing page](https://github.com/kubernetes-sigs/cri-tools/releases):

```sh
VERSION="v1.34.0"
ARCH="amd64"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/critest-$VERSION-linux-$ARCH.tar.gz
sudo tar zxvf critest-$VERSION-linux-$ARCH.tar.gz -C /usr/local/bin
rm -f critest-$VERSION-linux-$ARCH.tar.gz
```

critest uses [ginkgo v2](https://github.com/onsi/ginkgo) as its test framework. For parallel test execution, ginkgo can be installed by

```sh
go install github.com/onsi/ginkgo/v2/ginkgo@latest
```

For v1.0.0-alpha.0 and previous versions, Go and cri-tools source code are also required to run `critest`. The source code could get by running

```sh
# Replace branch name from version matrix in README.md
git clone https://github.com/kubernetes-sigs/cri-tools -b release-1.9 $GOPATH/src/github.com/kubernetes-sigs/cri-tools
```

## Running tests

### Prerequisite

Before running the test, you need to _ensure that the CRI server under test is running and listening on a Unix socket_. Because the validation tests are designed to request changes (e.g., create/delete) to the containers and verify that correct status is reported, it expects to be the only user of the CRI server. Please make sure that 1) there are no existing CRI-managed containers running on the node, and 2) no other processes (e.g., Kubelet) will interfere with the tests.

### Run

```sh
critest
```

This will

- Connect to the shim of CRI container runtime
- Run the tests using `ginkgo`
- Output the test results to STDOUT

critest connects to Unix: `unix:///run/containerd/containerd.sock` or Windows: `npipe:////./pipe/containerd-containerd` by default. For other runtimes, the endpoint can be set by flags `--runtime-endpoint` and `--image-endpoint`.

## Additional options

### Runtime and Image Service Configuration

- `-runtime-endpoint`: Set the endpoint of runtime service. Default to `unix:///run/containerd/containerd.sock` or Windows: `npipe:////./pipe/containerd-containerd`.
- `-image-endpoint`: Set the endpoint of image service. Same with runtime-endpoint if not specified.
- `-runtime-service-timeout`: Timeout when trying to connect to a runtime service (default: 300s).
- `-image-service-timeout`: Timeout when trying to connect to image service (default: 300s).
- `-runtime-handler`: Runtime handler to use in the test.
- `-config`: Location of the client config file. If not specified and the default does not exist, the program's directory is searched as well.

### Test Execution and Filtering

- `-ginkgo.focus`: Only run the tests that match the regular expression.
- `-ginkgo.skip`: Skip the tests that match the regular expression.
- `-ginkgo.label-filter`: Filter specs by label expressions (e.g., `(cat || dog) && !fruit`).
- `-parallel`: The number of parallel test nodes to run (default 1). [ginkgo](https://github.com/onsi/ginkgo) must be installed to run parallel tests.
- `-ginkgo.fail-fast`: Stop running tests after the first failure.
- `-ginkgo.flake-attempts`: Make up to N attempts to run each spec. If any attempt succeeds, the spec passes.
- `-ginkgo.timeout`: Test suite fails if it does not complete within the specified timeout (default: 1h).

### Test Images and Registry

- `-test-images-file`: Optional path to a YAML file containing references to custom container images to be used in tests.
- `-registry-prefix`: A possible registry prefix added to all images, like 'localhost:5000'.

### Benchmarking

- `-benchmark`: Run benchmarks instead of validation tests.
- `-benchmarking-params-file`: Optional path to a YAML file specifying benchmarking configuration options.
- `-benchmarking-output-dir`: Optional path to a directory in which benchmarking data should be placed.

### Reporting

- `-report-dir`: Path to the directory where the JUnit XML reports should be saved. Default is empty, which doesn't generate these reports.
- `-report-prefix`: Optional prefix for JUnit XML reports. Default is empty, which doesn't prepend anything to the default name.
- `-ginkgo.json-report`: Generate a JSON-formatted test report at the specified location.
- `-ginkgo.junit-report`: Generate a conformant junit test report in the specified file.

### Streaming Options

- `-websocket-exec`: Use websocket connections over SPDY for exec streaming tests.
- `-websocket-attach`: Use websocket connections over SPDY for attach streaming tests.
- `-websocket-portforward`: Use websocket connections over SPDY for portforward streaming tests.

### Output and Verbosity

- `-ginkgo.v`: Emit more output including GinkgoWriter contents.
- `-ginkgo.vv`: Emit with maximal verbosity - includes skipped and pending tests.
- `-ginkgo.no-color`: Suppress color output in default reporter.
- `-ginkgo.trace`: Print out the full stack trace when a failure occurs.

### Other Options

- `-version`: Display version of critest.
- `-h`: Show help and all supported options.
