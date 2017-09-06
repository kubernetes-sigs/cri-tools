# Container Runtime Interface (CRI) CLI

crictl provides a CLI for CRI-compatible container runtimes. This allows the CRI runtime developers to debug of their runtime without needing to set up Kubernetes components.

crictl is currently Alpha and still under quick iterations. We encourage the CRI developers to report bugs or help extend the coverage by adding more functionalities.

## Install

The CRI CLI can be installed easily via `go get` command:

```sh
go get github.com/kubernetes-incubator/cri-tools/cmd/crictl
```

Then `crictl` binary can be found in `$GOPATH/bin`.

*Note: ensure GO is installed and GOPATH is set before installing crictl.*

## Usage

```sh
crictl SUBCOMMAND [FLAGS]
```

Subcommands includes:

- `info`:          Display runtime version information
- `runs`:          Run a new sandbox
- `stops`:         Stop a running sandbox
- `rms`:           Remove a sandbox
- `inspects`:      Display the status of a sandbox
- `sandboxes`:     List sandboxes
- `create`:        Create a new container
- `start`:         Start a created container
- `stop`:          Stop a running container
- `rm`:            Remove a container
- `inspect`:       Display the status of a container
- `ps`:            List containers
- `status`:        Display status of the container runtime
- `attach`:        Attach to a running container
- `pull`:          Pull an image from a registry
- `images`:        List images
- `inspecti`:      Return the status of an image
- `rmi`:           Remove an image
- `exec`:          Run a command in a running container
- `port-forward`:  Forward local port to a sandbox
- `logs`:          Fetch the logs of a container
- `help`:          Shows a list of commands or help for one command

crictl connects to `/var/run/dockershim.sock` by default. For other runtimes, the endpoint can be set in three ways:

- By setting flags `--runtime-endpoint` and `--image-endpoint`
- By setting environment variables `CRI_RUNTIME_ENDPOINT` and `CRI_IMAGE_ENDPOINT`
- By setting the endpoint in the config file `--config=/etc/crictl.yaml`

```
# cat /etc/crictl.yaml
runtime-endpoint: /var/run/dockershim.sock
image-endpoint: /var/run/dockershim.sock
timeout: 10
debug: true
```

## Additional options

- `--runtime-endpoint`, `-r`: CRI server runtime endpoint (default: "/var/run/dockershim.sock").The default server is dockershim. If we want to debug other CRI server such as frakti, we can add flag `--runtime-endpoint=/var/run/frakti.sock`
- `--image-endpoint`, `-i`: CRI server image endpoint, default same as runtime endpoint.
- `--timeout`, `-t`: Timeout of connecting to server (default: 10s)
- `--debug`, `-D`: Enable debug output
- `--help`, `-h`: show help
- `--version`, `-v`: print the version information of crictl
- `--config`, `-c`: Config file in yaml format. Overrided by flags or environment variables.

## Examples

### Run sandbox with config file

```
# cat sandbox-config.json
{
    "metadata": {
        "name": "nginx-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "linux": {
    }
}

# crictl runs sandbox-config.json
e1c83b0b8d481d4af8ba98d5f7812577fc175a37b10dc824335951f52addbb4e
# crictl sandboxes
SANDBOX ID                                                         NAME                STATE
e1c83b0b8d481d4af8ba98d5f7812577fc175a37b10dc824335951f52addbb4e   nginx-sandbox       SANDBOX_READY
```

### Pull a busybox image

```
# crictl pull busybox
Image is update to date for busybox@sha256:b82b5740006c1ab823596d2c07f081084ecdb32fd258072707b99f52a3cb8692
# crictl images
IMAGE                                  TAG                 IMAGE ID            SIZE
busybox                                latest              d20ae45477cbc       1.13MB
gcr.io/google_containers/pause-amd64   3.0                 99e59f495ffaa       747kB
```

### Create container in a sandbox with config file

```
# cat sandbox-config.json
{
    "metadata": {
        "name": "nginx-sandbox",
        "namespace": "default",
        "attempt": 1,
        "uid": "hdishd83djaidwnduwk28bcsb"
    },
    "linux": {
    }
}

# cat container-config.json
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
  "linux": {
  }
}

# crictl create e1c83b0b8d481d4af8ba98d5f7812577fc175a37b10dc824335951f52addbb4e container-config.json sandbox-config.json
0a2c761303163f2acaaeaee07d2ba143ee4cea7e3bde3d32190e2a36525c8a05
# crictl ps
CONTAINER ID                                                       CREATED             STATE               NAME
0a2c761303163f2acaaeaee07d2ba143ee4cea7e3bde3d32190e2a36525c8a05   1 minutes ago       CONTAINER_CREATED   busybox
```

### Start container

```
# crictl start 0a2c761303163f2acaaeaee07d2ba143ee4cea7e3bde3d32190e2a36525c8a05
0a2c761303163f2acaaeaee07d2ba143ee4cea7e3bde3d32190e2a36525c8a05
# crictl ps
CONTAINER ID                                                       CREATED             STATE               NAME
0a2c761303163f2acaaeaee07d2ba143ee4cea7e3bde3d32190e2a36525c8a05   2 minutes ago       CONTAINER_RUNNING   busybox
```

### Exec a command in container

```
# crictl exec -i -t 0a2c761303163f2acaaeaee07d2ba143ee4cea7e3bde3d32190e2a36525c8a05 ls
bin   dev   etc   home  proc  root  sys   tmp   usr   var
```
