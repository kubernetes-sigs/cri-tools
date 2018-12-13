# Container Runtime Interface (CRI) Validation Testing

CRI validation testing provides a test framework and a suite of tests to validate that the Container Runtime Interface (CRI) server implementation meets all the requirements. This allows the CRI runtime developers to verify that their runtime conforms to CRI, without needing to set up Kubernetes components or run Kubernetes end-to-end tests.

CRI validation testing is currently Alpha and still under quick iterations. We encourage the CRI developers to report bugs or help extend the test coverage by adding more tests.

## Install

The benchmarking tests binary `critest` can be downloaded from [Releasing page](https://github.com/kubernetes-sigs/cri-tools/releases):

```sh
VERSION="v1.13.0"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/critest-$VERSION-linux-amd64.tar.gz
sudo tar zxvf critest-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f critest-$VERSION-linux-amd64.tar.gz
```

critest requires [ginkgo](https://github.com/onsi/ginkgo) to run parallel tests. It could be installed by

```sh
go get -u github.com/onsi/ginkgo/ginkgo
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

critest connects to Unix: `unix:///var/run/dockershim.sock` or Windows: `tcp://localhost:3735` by default. For other runtimes, the endpoint can be set by flags `--runtime-endpoint` and `--image-endpoint`.

## Additional options

- `-ginkgo.focus`: Only run the tests that match the regular expression.
- `-image-endpoint`: Set the endpoint of image service. Same with runtime-endpoint if not specified.
- `-runtime-endpoint`: Set the endpoint of runtime service. Default to `unix:///var/run/dockershim.sock` or Windows: `tcp://localhost:3735`.
- `-ginkgo.skip`: Skip the tests that match the regular expression.
- `-parallel`: The number of parallel test nodes to run (default 1). [ginkgo](https://github.com/onsi/ginkgo) must be installed to run parallel tests.
- `-h`: Should help and all supported options.
