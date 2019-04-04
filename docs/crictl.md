# Container Runtime Interface (CRI) CLI

crictl provides a CLI for CRI-compatible container runtimes. This allows the CRI runtime developers to debug their runtime without needing to set up Kubernetes components.

crictl is currently in Beta and still under quick iterations. It is hosted at the [cri-tools](https://github.com/kubernetes-sigs/cri-tools) repository. We encourage the CRI developers to report bugs or help extend the coverage by adding more functionalities.

## Install crictl

`crictl` can be downloaded from cri-tools [release page](https://github.com/kubernetes-sigs/cri-tools/releases):

- using `wget`:

```sh
VERSION="v1.14.0"
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-amd64.tar.gz
```

- using `curl`:

```sh
VERSION="v1.14.0"
curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-${VERSION}-linux-amd64.tar.gz --output crictl-${VERSION}-linux-amd64.tar.gz
sudo tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-amd64.tar.gz
```

## Usage

```sh
crictl SUBCOMMAND [FLAGS]
```

Subcommands includes:

- `attach`:       Attach to a running container
- `create`:       Create a new container
- `exec`:         Run a command in a running container
- `version`:      Display runtime version information
- `images`:       List images
- `inspect`:      Display the status of one or more containers
- `inspecti`:     Return the status of one ore more images
- `inspectp`:     Display the status of one or more pods
- `logs`:         Fetch the logs of a container
- `port-forward`: Forward local port to a pod
- `ps`:           List containers
- `pull`:         Pull an image from a registry
- `runp`:         Run a new pod
- `rm`:           Remove one or more containers
- `rmi`:          Remove one or more images
- `rmp`:          Remove one or more pods
- `pods`:         List pods
- `start`:        Start one or more created containers
- `info`:         Display information of the container runtime
- `stop`:         Stop one or more running containers
- `stopp`:        Stop one or more running pods
- `update`:       Update one or more running containers
- `config`:       Get and set crictl options
- `stats`:        List container(s) resource usage statistics
- `completion`:   Output bash shell completion code
- `help, h`:      Shows a list of commands or help for one command

crictl connects to Unix: `unix:///var/run/dockershim.sock` or Windows: `tcp://localhost:3735`  by default. For other runtimes, the endpoint can be set in three ways:

- By setting flags `--runtime-endpoint` and `--image-endpoint`
- By setting environment variables `CONTAINER_RUNTIME_ENDPOINT` and `IMAGE_SERVICE_ENDPOINT`
- By setting the endpoint in the config file `--config=/etc/crictl.yaml`

Unix:
```sh
$ cat /etc/crictl.yaml
runtime-endpoint: unix:///var/run/dockershim.sock
image-endpoint: unix:///var/run/dockershim.sock
timeout: 10
debug: true
```
Windows:
```cmd
C:\> type %USERPROFILE%\.crictl\crictl.yaml
runtime-endpoint: tcp://localhost:3735
image-endpoint: tcp://localhost:3735
timeout: 10
debug: true
```

## Additional options

- `--runtime-endpoint`, `-r`: CRI server runtime endpoint (default: Unix: `unix:///var/run/dockershim.sock` or Windows: `tcp://localhost:3735`). The default server is dockershim. If we want to debug other CRI server such as frakti, we can add flag `--runtime-endpoint=/var/run/frakti.sock`
- `--image-endpoint`, `-i`: CRI server image endpoint, default same as runtime endpoint.
- `--timeout`, `-t`: Timeout of connecting to server (default: 10s)
- `--debug`, `-D`: Enable debug output
- `--help`, `-h`: show help
- `--version`, `-v`: print the version information of crictl
- `--config`, `-c`: Config file in yaml format. Overrided by flags or environment variables.

## Examples

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
    "logDirectory": "/tmp",
    "linux": {
    }
}

$ crictl runp pod-config.json
f84dd361f8dc51518ed291fbadd6db537b0496536c1d2d6c05ff943ce8c9a54f
```

List pod sandboxes and check the sandbox is in Ready state:

```sh
$ crictl pods
POD ID              CREATED             STATE               NAME                NAMESPACE           ATTEMPT
f84dd361f8dc5       17 seconds ago      Ready               busybox-sandbox     default             1
```

#### Run pod sandbox with runtime handler

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
    "logDirectory": "/tmp",
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
  "log_path":"busybox/0.log",
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

## More information

Visit [kubernetes-sigs/cri-tools](https://github.com/kubernetes-sigs/cri-tools) for more information.
