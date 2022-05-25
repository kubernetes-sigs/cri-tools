# Container Runtime Interface (CRI) Performance Benchmarking

CRI performance benchmarking provides a benchmarking framework for CRI-compatible container runtimes.  This allows the CRI runtime developers to benchmark the performance of their runtime without needing to set up Kubernetes components or run Kubernetes benchmark tests.

## Install

The benchmarking tests binary `critest` can be downloaded from [Releasing page](https://github.com/kubernetes-sigs/cri-tools/releases):

```sh
VERSION="v1.17.0"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/critest-$VERSION-linux-amd64.tar.gz
sudo tar zxvf critest-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f critest-$VERSION-linux-amd64.tar.gz
```

For v1.0.0-alpha.0 and previous versions, Go and cri-tools source code are also required to run `critest`. The source code could get by running

```sh
# Replace branch name from version matrix in README.md
git clone https://github.com/kubernetes-sigs/cri-tools -b release-1.9 $GOPATH/src/github.com/kubernetes-sigs/cri-tools
```

## Running tests

### Prerequisite

Before running the test, you need to _ensure that the CRI server under test is running and listening on a Unix socket_ or a Windows tcp socket. Because the benchmark tests are designed to request changes (e.g., create/delete) to the containers and verify that correct status is reported, it expects to be the only user of the CRI server. Please make sure that 1) there are no existing CRI-managed containers running on the node, and 2) no other processes (e.g., Kubelet) will interfere with the tests.

### Defining benchmarking parameters

You can optionally specify some parameters detailing how benchmarks should be run.

```yaml
# The number of container lifecycle benchmarks to run:
containersNumber: 100

# The number of container lifecycle benchmarks to run in parallel.
# The total number of samples will be floor(containersNumber / containersNumberParallel)
containersNumberParallel: 2


# The number of pod lifecycle benchmarks to run:
podsNumber: 1000
# The number of pod lifecycle benchmarks to run in parallel.
# The total number of samples will be floor(podsNumber/ podsNumberParallel)
podsNumberParallel: 1
```

### Run

```sh
critest -benchmark
    [--benchmarking-params-file /path/to/params.yml]
    [--benchmarking-output-dir /path/to/outdir/]
```

This will

- Connect to the shim of CRI container runtime
- Run the benchmark tests using `ginkgo`
- Output the test results to STDOUT

critest connects to Unix: `unix:///run/containerd/containerd.sock` or Windows: `tcp://localhost:3735` by default. For other runtimes, the endpoint can be set by flags `--runtime-endpoint` and `--image-endpoint`.

## Additional options

- `-ginkgo.focus`: Only run the tests that match the regular expression.
- `-image-endpoint`: Set the endpoint of image service. Same with runtime-endpoint if not specified.
- `-runtime-endpoint`: Set the endpoint of runtime service. If not set, attempt
  to search a list of known endpoints.

   **Note:** The default endpoints are deprecated. Always set the
   `--runtime-endpoint`. Performance might be affected as each default
   connection attempt takes n-seconds to complete before timing out and going
   to the next in sequence.

- `-benchmarking-params-file`: optional path to a YAML file containing parameters describing which
benchmarks should be run.
- `-benchmarking-output-dir`: optional path to a pre-existing directory in which to write JSON
  files detailing the results of the benchmarks.
- `-ginkgo.skip`: Skip the tests that match the regular expression.
- `-h`: Show help and all supported options.
