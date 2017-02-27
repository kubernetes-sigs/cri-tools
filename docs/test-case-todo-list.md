#Test case todo list

## 1.Functionality Test

**Expected operations**

- Basic operations on pod. (run/stop/list/remove/status)
- Basic operations on container. (create/start/stop/list/remove/status/exec)
- Basic operations on image. (pull/remove/list/status)

**Unexpected operations**

- Remove/stop/list  pod/container/image which dose not exist.
- Start a running container. Stop a stopped container/pod.
- Create pod/container with a name which has already been used.

## 2.Performance Test
Based on [Docker micro benchmark](https://github.com/kubernetes/contrib/tree/master/docker-micro-benchmark)

- Performance of `list` container/pod.
- Performance of `status` container/pod.
- Performance of `create`/`start` container/pod.
- Performance of `stop`/`remove` container/pod.
- Performance of `list` container/pod when we are running `create`/`start` container/pod.

Use `ginkgo --focus` to separate performance test from functionality test.

## 3.Proposal of design

- Run functionality test by default.
- Add `-p` flag to run performance test only.
- Add `-a` flag to run both.