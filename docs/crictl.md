# Container Runtime Interface (CRI) CLI

`crictl` provides a CLI for CRI-compatible container runtimes. This allows the CRI runtime developers to debug their runtime without needing to set up Kubernetes components.

`crictl` has been GA since `v1.11.0` and is currently under active development. It is hosted at the [cri-tools](https://github.com/kubernetes-sigs/cri-tools) repository. We encourage the CRI developers to report bugs or help extend the coverage by adding more functionalities.

The tool expects JSON or YAML encoded files as input and passes them to the
corresponding container runtime using the [CRI API protocol](/vendor/k8s.io/cri-api/pkg/apis/runtime/v1/api.proto).

## Install crictl

> **NOTE:** The below steps are based on linux-amd64, however you can get downloads for all other platforms (Windows, ARM, etc) in the [releases page](https://github.com/kubernetes-sigs/cri-tools/releases).

`crictl` can be downloaded from cri-tools [release page](https://github.com/kubernetes-sigs/cri-tools/releases):

- using `wget`:

```sh
VERSION="v1.30.0" # check latest version in /releases page
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-amd64.tar.gz
```

- using `curl`:

```sh
VERSION="v1.30.0" # check latest version in /releases page
curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-${VERSION}-linux-amd64.tar.gz --output crictl-${VERSION}-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-amd64.tar.gz
```

## Usage

```sh
crictl [global options] command [command options] [arguments...]
```

COMMANDS:

- `attach`: Attach to a running container
- `create`: Create a new container
- `exec`: Run a command in a running container
- `version`: Display runtime version information
- `images, image, img`: List images
- `inspect`: Display the status of one or more containers
- `inspecti`: Return the status of one or more images
- `imagefsinfo`: Return image filesystem info
- `inspectp`: Display the status of one or more pods
- `logs`: Fetch the logs of a container
- `metricsp`: List pod metrics. Metrics are unstructured key/value pairs gathered by CRI meant to replace cAdvisor's /metrics/cadvisor endpoint.
- `metricdescs`: List metric descriptors. Returns information about the metrics available through the CRI.
- `port-forward`: Forward local port to a pod
- `ps`: List containers
- `pull`: Pull an image from a registry
- `run`: Run a new container inside a sandbox
- `runp`: Run a new pod
- `rm`: Remove one or more containers
- `rmi`: Remove one or more images
- `rmp`: Remove one or more pods
- `pods`: List pods
- `start`: Start one or more created containers
- `info`: Display information of the container runtime
- `stop`: Stop one or more running containers
- `stopp`: Stop one or more running pods
- `update`: Update one or more running containers
- `config`: Get, set and list crictl configuration options
- `stats`: List container(s) resource usage statistics
- `statsp`: List pod statistics. Stats represent a structured API that will fulfill the Kubelet's /stats/summary endpoint.
- `completion`: Output bash shell completion code
- `checkpoint`: Checkpoint one or more running containers
- `events, event`: Stream the events of containers
- `runtime-config`: Retrieve the container runtime configuration
- `update-runtime-config` Update the runtime configuration
- `help, h`: Shows a list of commands or help for one command

`crictl` by default connects on Unix to:

- `unix:///run/containerd/containerd.sock` or
- `unix:///run/crio/crio.sock` or
- `unix:///var/run/cri-dockerd.sock`

or on Windows to:

- `npipe:////./pipe/containerd-containerd` or
- `npipe:////./pipe/cri-dockerd`

For other runtimes, use:

- [frakti](https://github.com/kubernetes/frakti): `unix:///var/run/frakti.sock`

The endpoint can be set in three ways:

- By setting global option flags `--runtime-endpoint` (`-r`) and `--image-endpoint` (`-i`)
- By setting environment variables `CONTAINER_RUNTIME_ENDPOINT` and `IMAGE_SERVICE_ENDPOINT`
- By setting the endpoint in the config file `--config=/etc/crictl.yaml`

If the endpoint is not set then it works as follows:

- If the runtime endpoint is not set, `crictl` will by default try to connect using:
  - containerd
  - cri-o
  - cri-dockerd
- If the image endpoint is not set, `crictl` will by default use the runtime endpoint setting

> Note: The default endpoints are now deprecated and the runtime endpoint should always be set instead.
> The performance maybe affected as each default connection attempt takes n-seconds to complete before timing out and going to the next in sequence.

Unix:

```sh
$ cat /etc/crictl.yaml
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 2
debug: true
pull-image-on-create: false
```

Windows:

```cmd
C:\> type %USERPROFILE%\.crictl\crictl.yaml
runtime-endpoint: npipe:////./pipe/containerd-containerd
image-endpoint: npipe:////./pipe/containerd-containerd
timeout: 2
debug: true
pull-image-on-create: false
```

### Connection troubleshooting

Some runtimes might use [cmux](https://github.com/soheilhy/cmux) for connection
multiplexing, which can cause issues during the initial [gRPC](https://grpc.io)
connection setup. If it does not seem to be possible to connect to the runtime
`*.sock`, then exporting the environment variable
`GRPC_GO_REQUIRE_HANDSHAKE=off` might solve the issue. Please take into account
that the environment has to be preserved when running
via sudo (`sudo -E crictl ...`).

## Additional options

- `--timeout`, `-t`: Timeout of connecting to server in seconds (default: `2s`).
  0 or less is interpreted as unset and converted to the default. There is no
  option for no timeout value set and the smallest supported timeout is `1s`
- `--debug`, `-D`: Enable debug output
- `--help`, `-h`: show help
- `--version`, `-v`: print the version information of `crictl`
- `--config`, `-c`: Location of the client config file (default: `/etc/crictl.yaml`). Can be changed by setting `CRI_CONFIG_FILE` environment variable. If not specified and the default does not exist, the program's directory is searched as well
- `--enable-tracing`: Enable OpenTelemetry tracing (default: `false`)
- `--tracing-endpoint`: Address to which the gRPC tracing collector will send spans to (default: `127.0.0.1:4317`)
- `--tracing-sampling-rate-per-million`: Number of samples to collect per million OpenTelemetry spans. Set to 1000000 or -1 to always sample (default: `-1`)
- `--profile-cpu`: Write a pprof CPU profile to the provided path
- `--profile-mem`: Write a pprof memory profile to the provided path

## Client Configuration Options

Use the `crictl` config command to get, set and list the `crictl` client configuration
options.

USAGE:

```sh
crictl config [command options] [<crictl options>]
```

For example `crictl config --set debug=true` will enable debug mode when giving subsequent `crictl` commands.

COMMAND OPTIONS:

- `--get value`: Show the option value
- `--set value`: Set option (can specify multiple or separate values with commas: opt1=val1,opt2=val2)
- `--list`: Show all option values (default: `false`)
- `--help`, `-h`: Show help (default: `false`)

`crictl` OPTIONS:

- `runtime-endpoint`: Container runtime endpoint (no default value)
- `image-endpoint`: Image endpoint (no default value)
- `timeout`: Timeout of connecting to server (default: `2s`)
- `debug`: Enable debug output (default: `false`)
- `pull-image-on-create`: Enable pulling image on create requests (default: `false`)
- `disable-pull-on-run`: Disable pulling image on run requests (default: `false`)

> When enabled `pull-image-on-create` modifies the create container command to first pull the container's image.
> This feature is used as a helper to make creating containers easier and faster.
> Some users of `crictl` may desire to not pull the image necessary to create the container.
> For example, the image may have already been pulled or otherwise loaded into the container runtime, or the user may be running without a network. For this reason the default for `pull-image-on-create` is `false`.

> By default the run command first pulls the container image, and `disable-pull-on-run` is `false`.
> Some users of `crictl` may desire to set `disable-pull-on-run` to `true` to not pull the image by default when using the run command.

> To override these default pull configuration settings, `--no-pull` and `--with-pull` options are provided for the create and run commands.

## Examples

- [Run pod sandbox with config file](#run-pod-sandbox-with-config-file)
- [Run pod sandbox with runtime handler](#run-pod-sandbox-with-runtime-handler)
- [Pull a busybox image](#pull-a-busybox-image)
- [Filter images](#filter-images)

### Run pod sandbox with config file

```sh
$ cat pod-config.json
{
    "metadata": {
        "name": "nginx-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "log_directory": "/tmp",
    "linux": {
    }
}
```

If the runtime uses systemd as the cgroup driver, set the `"cgroup_parent"` field in the pod-config, similar to

```sh
$ cat pod-config.json
{
    "metadata": {
        "name": "nginx-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "log_directory": "/tmp",
    "linux": {
      "cgroup_parent": "/test.slice"
    }
}

$ crictl runp pod-config.json
f84dd361f8dc51518ed291fbadd6db537b0496536c1d2d6c05ff943ce8c9a54f
```

List pod sandboxes and check the sandbox is in Ready state:

```sh
$ crictl pods
POD ID              CREATED             STATE               NAME                NAMESPACE           ATTEMPT
f84dd361f8dc5       17 seconds ago      Ready               nginx-sandbox       default             1
```

### Run pod sandbox with runtime handler

Runtime handler requires runtime support. The following example shows running a pod sandbox with `runsc` handler on containerd runtime.

```sh
$ cat pod-config.json
{
    "metadata": {
        "name": "nginx-runsc-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "log_directory": "/tmp",
    "linux": {
    }
}

$ crictl runp --runtime=runsc pod-config.json
c112976cb6caa43a967293e2c62a2e0d9d8191d5109afef230f403411147548c

$ crictl inspectp c112976cb6caa43a967293e2c62a2e0d9d8191d5109afef230f403411147548c
...
    "runtime": {
      "runtimeType": "io.containerd.runtime.v1.linux",
      "runtimeEngine": "/usr/local/sbin/runsc",
      "runtimeRoot": "/run/containerd/runsc"
    },
...
```

### Pull a busybox image

```sh
$ crictl pull busybox
Image is up to date for busybox@sha256:141c253bc4c3fd0a201d32dc1f493bcf3fff003b6df416dea4f41046e0f37d47
```

List images and check the busybox image has been pulled:

```sh
$ crictl images
IMAGE               TAG                 IMAGE ID            SIZE
busybox             latest              8c811b4aec35f       1.15MB
k8s.gcr.io/pause    3.1                 da86e6ba6ca19       742kB
```

### Filter images

The following filters are available `--filter`, `-f`:

1. `before=<image-name>[:<tag>]|<image id>|<image@digest>`
1. `dangling=(true/false)`
1. `reference=/regex/`
1. `since=<image-name>[:<tag>]|<image id>|<image@digest>`

Filters can be combined and are applied in the order provided.

List all images:

```sh
$ crictl images --digests
IMAGE                                                      TAG                 DIGEST              IMAGE ID            SIZE
docker.io/library/busybox                                  latest              538721340ded1       3f57d9401f8d4       4.5MB
docker.io/library/nginx                                    latest              05aa73005987c       e4720093a3c13       191MB
gcr.io/k8s-staging-cri-tools/hostnet-nginx-amd64           latest              aa74ea387dbbe       1ee3f9825c42b       147MB
gcr.io/k8s-staging-cri-tools/test-image-predefined-group   latest              2b2fc189c502a       84410ab6e30d9       5.11MB
registry.k8s.io/e2e-test-images/busybox                    1.29-2              c318242786b13       84eebb9ca1734       1.37MB
registry.k8s.io/e2e-test-images/httpd                      2.4.39-4            3fe7acf013d12       444b9e2765dc9       132MB
registry.k8s.io/e2e-test-images/nginx                      1.14-2              13616070e3f29       02e45a31af51c       17.2MB
registry.k8s.io/e2e-test-images/nonewprivs                 1.3                 8ac1264691820       3e3d1785c0b6e       7.37MB
registry.k8s.io/pause                                      3.9                 7031c1b283388       e6f1816883972       750kB
```

List images by `reference`:

```sh
$ crictl images --filter 'reference=k8s'
IMAGE                                                      TAG                 IMAGE ID            SIZE
gcr.io/k8s-staging-cri-tools/hostnet-nginx-amd64           latest              1ee3f9825c42b       147MB
gcr.io/k8s-staging-cri-tools/test-image-predefined-group   latest              84410ab6e30d9       5.11MB
registry.k8s.io/e2e-test-images/busybox                    1.29-2              84eebb9ca1734       1.37MB
registry.k8s.io/e2e-test-images/httpd                      2.4.39-4            444b9e2765dc9       132MB
registry.k8s.io/e2e-test-images/nginx                      1.14-2              02e45a31af51c       17.2MB
registry.k8s.io/e2e-test-images/nonewprivs                 1.3                 3e3d1785c0b6e       7.37MB
registry.k8s.io/pause                                      3.9                 e6f1816883972       750kB
```

List images by `reference` using a regular expression:

```sh
$ crictl images --filter 'reference=nginx'
IMAGE                                              TAG                 IMAGE ID            SIZE
docker.io/library/nginx                            latest              e4720093a3c13       191MB
gcr.io/k8s-staging-cri-tools/hostnet-nginx-amd64   latest              1ee3f9825c42b       147MB
registry.k8s.io/e2e-test-images/nginx              1.14-2              02e45a31af51c       17.2MB
$ crictl images --filter 'reference=.*(nginx)$'
IMAGE                                   TAG                 IMAGE ID            SIZE
docker.io/library/nginx                 latest              e4720093a3c13       191MB
registry.k8s.io/e2e-test-images/nginx   1.14-2              02e45a31af51c       17.2MB
```

Combine multiple `--filter` arguments together:

```sh
$ crictl images --filter 'reference=nginx' --filter 'reference=\.k8s\.'
IMAGE                                   TAG                 IMAGE ID            SIZE
registry.k8s.io/e2e-test-images/nginx   1.14-2              02e45a31af51c       17.2MB
$ crictl images --filter 'since=registry.k8s.io/e2e-test-images/busybox@sha256:c318242786b139d18676b1c09a0ad7f15fc17f8f16a5b2e625cd0dc8c9703daf' --filter 'reference=nginx'
IMAGE                                              TAG                 IMAGE ID            SIZE
docker.io/library/nginx                            latest              e4720093a3c13       191MB
gcr.io/k8s-staging-cri-tools/hostnet-nginx-amd64   latest              1ee3f9825c42b       147MB
```

List images `before=<image-name>[:<tag>]`:

```sh
$ crictl images --filter 'before=gcr.io/k8s-staging-cri-tools/hostnet-nginx-amd64:latest'
IMAGE                                                      TAG                 IMAGE ID            SIZE
gcr.io/k8s-staging-cri-tools/test-image-predefined-group   latest              84410ab6e30d9       5.11MB
registry.k8s.io/e2e-test-images/busybox                    1.29-2              84eebb9ca1734       1.37MB
registry.k8s.io/e2e-test-images/httpd                      2.4.39-4            444b9e2765dc9       132MB
registry.k8s.io/e2e-test-images/nginx                      1.14-2              02e45a31af51c       17.2MB
registry.k8s.io/e2e-test-images/nonewprivs                 1.3                 3e3d1785c0b6e       7.37MB
registry.k8s.io/pause                                      3.9                 e6f1816883972       750kB
```

List images `since=<image-name>[:<tag>]`:

```sh
$ crictl images --filter 'since=gcr.io/k8s-staging-cri-tools/hostnet-nginx-amd64:latest'
IMAGE                       TAG                 IMAGE ID            SIZE
docker.io/library/busybox   latest              3f57d9401f8d4       4.5MB
docker.io/library/nginx     latest              e4720093a3c13       191MB
```

List images `since=<image@digest>`:

```sh
crictl images --filter 'since=registry.k8s.io/e2e-test-images/busybox@sha256:c318242786b139d18676b1c09a0ad7f15fc17f8f16a5b2e625cd0dc8c9703daf'
IMAGE                                                      TAG                 IMAGE ID            SIZE
docker.io/library/busybox                                  latest              3f57d9401f8d4       4.5MB
docker.io/library/nginx                                    latest              e4720093a3c13       191MB
gcr.io/k8s-staging-cri-tools/hostnet-nginx-amd64           latest              1ee3f9825c42b       147MB
gcr.io/k8s-staging-cri-tools/test-image-predefined-group   latest              84410ab6e30d9       5.11MB
```

### Create container in the pod sandbox with config file

```sh
$ cat pod-config.json
{
    "metadata": {
        "name": "nginx-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "log_directory": "/tmp",
    "linux": {
    }
}

$ cat container-config.json
{
  "metadata": {
      "name": "busybox"
  },
  "image":{
      "image": "busybox"
  },
  "command": [
      "top"
  ],
  "log_path":"busybox.0.log",
  "linux": {
  }
}

$ crictl create f84dd361f8dc51518ed291fbadd6db537b0496536c1d2d6c05ff943ce8c9a54f container-config.json pod-config.json
3e025dd50a72d956c4f14881fbb5b1080c9275674e95fb67f965f6478a957d60
```

List containers and check the container is in Created state:

```sh
$ crictl ps -a
CONTAINER ID        IMAGE               CREATED             STATE               NAME                ATTEMPT
3e025dd50a72d       busybox             32 seconds ago      Created             busybox             0
```

### Start container

```sh
$ crictl start 3e025dd50a72d956c4f14881fbb5b1080c9275674e95fb67f965f6478a957d60
3e025dd50a72d956c4f14881fbb5b1080c9275674e95fb67f965f6478a957d60

$ crictl ps
CONTAINER ID        IMAGE               CREATED              STATE               NAME                ATTEMPT
3e025dd50a72d       busybox             About a minute ago   Running             busybox             0
```

### Exec a command in container

```sh
crictl exec -i -t 3e025dd50a72d956c4f14881fbb5b1080c9275674e95fb67f965f6478a957d60 ls
bin   dev   etc   home  proc  root  sys   tmp   usr   var
```

### Create and start a container within one command

It is possible to start a container within a single command, whereas the image
will be pulled automatically, too:

```sh
$ cat pod-config.json
{
    "metadata": {
        "name": "nginx-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "log_directory": "/tmp",
    "linux": {
    }
}

$ cat container-config.json
{
  "metadata": {
      "name": "busybox"
  },
  "image":{
      "image": "busybox"
  },
  "command": [
      "top"
  ],
  "log_path":"busybox.0.log",
  "linux": {
  }
}

$ crictl run container-config.json pod-config.json
b25b4f26e342969eb40d05e98130eee0846557d667e93deac992471a3b8f1cf4
```

List containers and check the container is in Running state:

```sh
$ crictl ps
CONTAINER           IMAGE               CREATED             STATE               NAME                ATTEMPT             POD ID
b25b4f26e3429       busybox:latest      14 seconds ago      Running             busybox             0                   158d7a6665ff3
```

### Checkpoint a running container

```sh
$ crictl checkpoint --export=/path/to/checkpoint.tar 39fcdd7a4f1d4
39fcdd7a4f1d4
$ ls /path/to/checkpoint.tar
/path/to/checkpoint.tar
```

## More information

- See the [Kubernetes.io Debugging Kubernetes nodes with crictl doc](https://kubernetes.io/docs/tasks/debug-application-cluster/crictl/)
- Visit [kubernetes-sigs/cri-tools](https://github.com/kubernetes-sigs/cri-tools) for more information.
