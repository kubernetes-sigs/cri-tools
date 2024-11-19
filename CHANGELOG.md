<!-- TOC -->

- [v1.17.0](#v1170)
  - [CRI CLI (crictl)](#cri-cli-crictl)
  - [Downloads](#downloads)
- [v1.16.0](#v1160)
  - [CRI CLI (crictl)](#cri-cli-crictl-1)
  - [CRI validation testing (critest)](#cri-validation-testing-critest)
  - [Downloads](#downloads-1)
- [v1.15.0](#v1150)
  - [CRI CLI (crictl)](#cri-cli-crictl-2)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-1)
  - [Downloads](#downloads-2)
- [v1.14.0](#v1140)
  - [CRI CLI (crictl)](#cri-cli-crictl-3)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-2)
  - [Downloads](#downloads-3)
- [v1.13.0](#v1130)
  - [CRI CLI (crictl)](#cri-cli-crictl-4)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-3)
  - [Downloads](#downloads-4)
- [v1.12.0](#v1120)
  - [CRI CLI (crictl)](#cri-cli-crictl-5)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-4)
  - [Downloads](#downloads-5)
- [v1.11.1](#v1111)
  - [CRI CLI (crictl)](#cri-cli-crictl-6)
- [v1.11.0](#v1110)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-5)
  - [CRI CLI (crictl)](#cri-cli-crictl-7)
- [v1.0.0-beta.1](#v100-beta1)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-6)
  - [CRI CLI (crictl)](#cri-cli-crictl-8)
- [v1.0.0-beta.0](#v100-beta0)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-7)
  - [CRI CLI (crictl)](#cri-cli-crictl-9)
- [v1.0.0-alpha.0](#v100-alpha0)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-8)
  - [CRI CLI (crictl)](#cri-cli-crictl-10)
- [v0.2](#v02)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-9)
  - [CRI CLI (crictl)](#cri-cli-crictl-11)
- [v0.1](#v01)
  - [Features](#features)
    - [CRI validation testing](#cri-validation-testing)
    - [crictl](#crictl)
    - [CRI performance benchmarking](#cri-performance-benchmarking)
  - [Documentation](#documentation)

<!-- /TOC -->

# v1.17.0

cri-tools v1.17.0 mainly focuses on bug fixes and stability improvements.

## CRI CLI (crictl)

- [#562](https://github.com/kubernetes-sigs/cri-tools/pull/562) Bump cri-tools version in markdown docs
- [#561](https://github.com/kubernetes-sigs/cri-tools/pull/561) Vendor kubernetes v1.17.0
- [#557](https://github.com/kubernetes-sigs/cri-tools/pull/557) Update golang dependencies
- [#556](https://github.com/kubernetes-sigs/cri-tools/pull/556) Print the previous log of the container(just like kubectl)
- [#555](https://github.com/kubernetes-sigs/cri-tools/pull/555) Add rmi --prune command
- [#553](https://github.com/kubernetes-sigs/cri-tools/pull/553) Update README.md to contain latest version hint
- [#552](https://github.com/kubernetes-sigs/cri-tools/pull/552) Download golangci-lint via wget
- [#550](https://github.com/kubernetes-sigs/cri-tools/pull/550) Fix pod and container name filter for JSON/YAML output
- [#545](https://github.com/kubernetes-sigs/cri-tools/pull/545) Fix e2e tests by pinning CRI-O and conmon
- [#544](https://github.com/kubernetes-sigs/cri-tools/pull/544) Fixed Tim St. Clair's username
- [#537](https://github.com/kubernetes-sigs/cri-tools/pull/537) Add fish shell completion support

## Downloads

| file                                                                                                                                                | sha256                                                           |
| --------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| [crictl-v1.17.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-linux-386.tar.gz)           | cffa443cf76ab4b760a68d4db555d1854cb692e8b20b3360cf23221815ca151e |
| [crictl-v1.17.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-linux-amd64.tar.gz)       | 7b72073797f638f099ed19550d52e9b9067672523fc51b746e65d7aa0bafa414 |
| [crictl-v1.17.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-linux-arm.tar.gz)           | 9700957218e8e7bdc02cbc8fda4c189f5b6223a93ba89d876bdfd77b6117e9b7 |
| [crictl-v1.17.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-linux-arm64.tar.gz)       | d89afd89c2852509fafeaff6534d456272360fcee732a8d0cb89476377387e12 |
| [crictl-v1.17.0-linux-ppc64le.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-linux-ppc64le.tar.gz)   | a61c52b9ac5bffe94ae4c09763083c60f3eccd30eb351017b310f32d1cafb855 |
| [crictl-v1.17.0-linux-s390x.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-linux-s390x.tar.gz)       | 0db445f0b74ecb51708b710480a462b728174155c5f2709a39d1cc2dc975e350 |
| [crictl-v1.17.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-windows-386.tar.gz)       | 2e285250d36b5cb3e8c047b191c0c0af606fed7c0034bb140ba95cc1498f4996 |
| [crictl-v1.17.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/crictl-v1.17.0-windows-amd64.tar.gz)   | e18150d5546d3ddf6b165bd9aec0f65c18aacf75b94fb28bb26bfc0238f07b28 |
| [critest-v1.17.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/critest-v1.17.0-linux-386.tar.gz)         | 60745268547a4979e668289bf9c19b1308d4bdcc2d1c6246901ac2300defcf8f |
| [critest-v1.17.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/critest-v1.17.0-linux-amd64.tar.gz)     | 17088d99f70f3e03d8215de3594cbec367749b0a187f147cc3b2b975e497b433 |
| [critest-v1.17.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/critest-v1.17.0-linux-arm.tar.gz)         | c5d4299d52000e43f39ebe2d9122eeae6a5867801f93c76ed97dde6b6253f646 |
| [critest-v1.17.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/critest-v1.17.0-linux-arm64.tar.gz)     | c125f4fb8b944909c7ff6e3019d9a0c2997df9074a0ace3bf79b4ee353760366 |
| [critest-v1.17.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/critest-v1.17.0-windows-386.tar.gz)     | 9748549bc3b7c87b9168fd92e369ed6975f82af4ab619ffecf60668816b3ab9a |
| [critest-v1.17.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.17.0/critest-v1.17.0-windows-amd64.tar.gz) | a178771cc1a369e0132cd8ade95e7c28225a67d8d44db462cc3614dcfffea41b |

# v1.16.0

cri-tools v1.16.0 has upgraded to kubernetes v1.16. It mainly focus on bug fixes and stability improvements.

## CRI CLI (crictl)

- [#490](https://github.com/kubernetes-sigs/cri-tools/pull/490) Switch to go modules.
- [#492](https://github.com/kubernetes-sigs/cri-tools/pull/492) Add container 'run' subcommand.
- [#496](https://github.com/kubernetes-sigs/cri-tools/pull/496) Pull image on container creation.
- [#502](https://github.com/kubernetes-sigs/cri-tools/pull/502) Make runtime and image client non-global variables.
- [#505](https://github.com/kubernetes-sigs/cri-tools/pull/505) Bump google.golang.org/grpc from 1.22.1 to 1.23.0.
- [#508](https://github.com/kubernetes-sigs/cri-tools/pull/508) Update urfave/cli to v1.21.0.
- [#512](https://github.com/kubernetes-sigs/cri-tools/pull/512) Treat first `crictl stats` argument as filter ID.
- [#513](https://github.com/kubernetes-sigs/cri-tools/pull/513) Add -a (remove all) option to `rmp`.
- [#516](https://github.com/kubernetes-sigs/cri-tools/pull/516) Create directory for config file.
- [#521](https://github.com/kubernetes-sigs/cri-tools/pull/521) Add container `rm` `--force, -f` and `--all, -a` flags.
- [#522](https://github.com/kubernetes-sigs/cri-tools/pull/522) Add `images` aliases `image`, `img`.
- [#523](https://github.com/kubernetes-sigs/cri-tools/pull/523) Add `--all, -a` flag to image removal (`rmi`).
- [#532](https://github.com/kubernetes-sigs/cri-tools/pull/532) Update Kubernetes vendors to 1.16.0 and add addiontion IPs for Pods.

## CRI validation testing (critest)

- [#477](https://github.com/kubernetes-sigs/cri-tools/pull/477) Add registry prefix option.
- [#487](https://github.com/kubernetes-sigs/cri-tools/pull/487) Add test for MaskedPaths and ReadonlyPaths.
- [#488](https://github.com/kubernetes-sigs/cri-tools/pull/488) Switch to multi-arch nonewprivs image.
- [#489](https://github.com/kubernetes-sigs/cri-tools/pull/489) Use arch specific hostnet-nginx image.
- [#494](https://github.com/kubernetes-sigs/cri-tools/pull/494) Move from gometalinter to golangci-lint.
- [#503](https://github.com/kubernetes-sigs/cri-tools/pull/503) Use a dedicated image for all reference test.
- [#506](https://github.com/kubernetes-sigs/cri-tools/pull/506) Fix image build script for image test image.
- [#509](https://github.com/kubernetes-sigs/cri-tools/pull/509) Use IsAppArmorEnabled of Kubernetes instead of local one.
- [#517](https://github.com/kubernetes-sigs/cri-tools/pull/517) Fix exec test timeout issues.
- [#518](https://github.com/kubernetes-sigs/cri-tools/pull/518) Fix DNS test on Windows.
- [#519](https://github.com/kubernetes-sigs/cri-tools/pull/519) Fix exec test on Windows.
- [#520](https://github.com/kubernetes-sigs/cri-tools/pull/520) Fix CRI test on Windows.
- [#529](https://github.com/kubernetes-sigs/cri-tools/pull/529) [#534](https://github.com/kubernetes-sigs/cri-tools/pull/534) Add crictl e2e tests.
- [#533](https://github.com/kubernetes-sigs/cri-tools/pull/533) Fix attach test on Windows.

## Downloads

| file                                                                                                                                               | sha256                                                           |
| -------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| [crictl-v1.16.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-386.tar.gz)          | b6311cb75f37930721bf3e9e415dc503216c1020ca2bcde4d9b175f30cb270c6 |
| [crictl-v1.16.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-amd64.tar.gz)      | a3eefa10a483c643ad85aee3d7832a720976ef7e80dde46b212eaaacd7d09512 |
| [crictl-v1.16.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-arm64.tar.gz)      | aa118c31d6f6fd2d24bb2de4a33598a14a5952e1d01f93d5c3267c2b5334743b |
| [crictl-v1.16.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-arm.tar.gz)          | 331c49bd9196009b8230f7a36ec272924a7bcf4c1614ecddf0eb9598c787da0e |
| [crictl-v1.16.0-linux-ppc64le.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-ppc64le.tar.gz)  | 3545a8872abee65bcacdcbf80c7ac1beb3c115c53f71d10c4786f1306125f405 |
| [crictl-v1.16.0-linux-s390x.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-s390x.tar.gz)      | 5af5fa1879cc487f41784c0d0fc2dc179553aa588d795f6607d150d3ffa83615 |
| [crictl-v1.16.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-windows-386.tar.gz)      | 781e4d76feddc1709716aecebeb46502d6d9485d4c6f4e083201b9798f508c58 |
| [crictl-v1.16.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-windows-amd64.tar.gz)  | df15b2d0ad4fcde7580672f950022cbc6f273d3214bc12d5765ae9daa4baf7b5 |
| [critest-v1.16.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-386.tar.gz)         | c90d49eceae58b0ffb9384cd9b53a8d8bb2329ce5125c2302d7941298939529c |
| [critest-v1.16.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-amd64.tar.gz)     | 909ba3e1404c86b82a957d4c69ae9a40e418a09e104fc017e708b877f78eaa73 |
| [critest-v1.16.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-arm64.tar.gz)     | f28f7965b688b1b2f6756a265f460517a4633c85e00df15575be79d4668b0c06 |
| [critest-v1.16.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-linux-arm.tar.gz)         | c0a85d2216f7f45ce9306cf33de786f99eedce1d62b0d983fe4301b51c761598 |
| [critest-v1.16.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-windows-386.tar.gz)     | 56796161d8437eea9d82636857bb899263601637ab2e911cff9f6fc7deaebc3f |
| [critest-v1.16.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.16.0/crictl-v1.16.0-windows-amd64.tar.gz) | f69ce1a962e53a72f034dedfe38a20d5db85a5694bda5ad703c63806e56c2947 |

# v1.15.0

cri-tools v1.15.0 has upgraded to kubernetes v1.15. It mainly focus on bug fixes and stability improvements.

> Since the CRI hasn't changed in this release, this version is also compatible with Kubernetes v1.14.

## CRI CLI (crictl)

- [#453](https://github.com/kubernetes-sigs/cri-tools/pull/453) Added examples to download crictl with `curl`.
- [#459](https://github.com/kubernetes-sigs/cri-tools/pull/459) Improves error message for failed to connect.
- [#462](https://github.com/kubernetes-sigs/cri-tools/pull/462) Fixes log directory typo in pod config examples.
- [#463](https://github.com/kubernetes-sigs/cri-tools/pull/463) Searches program directory for config as fallback.
- [#465](https://github.com/kubernetes-sigs/cri-tools/pull/465) Adds new options `--force, -f` to `crictl rmp` command.
- [#466](https://github.com/kubernetes-sigs/cri-tools/pull/466) Fixes issues that can't set debug to false.
- [#467](https://github.com/kubernetes-sigs/cri-tools/pull/467) Supports live monitoring by new option `--watch, -w` for `crictl stats`.
- [#471](https://github.com/kubernetes-sigs/cri-tools/pull/471) Fallback to repo digest name if repoTag is `<none>:<none>`.

## CRI validation testing (critest)

- [#458](https://github.com/kubernetes-sigs/cri-tools/pull/458) Sets `SelinuxRelabel` to true for bind mounts.
- [#468](https://github.com/kubernetes-sigs/cri-tools/pull/468) Adds image status tests for various references.
- [#476](https://github.com/kubernetes-sigs/cri-tools/pull/476) Fixes typo in parallel tests error message.

## Downloads

| file                                                                                                                                                | sha256                                                           |
| --------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| [crictl-v1.15.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/crictl-v1.15.0-linux-386.tar.gz)           | 51bbab1fad26f8d012344707fdefecc9ffa6b99e536c14b62a9b0ee3f81a519c |
| [crictl-v1.15.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/crictl-v1.15.0-linux-amd64.tar.gz)       | c3b71be1f363e16078b51334967348aab4f72f46ef64a61fe7754e029779d45a |
| [crictl-v1.15.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/crictl-v1.15.0-linux-arm64.tar.gz)       | 785c3da7e058f6fd00b0a48de24b9199eb6bae940d13f509c44ea6dd7ad9ffcd |
| [crictl-v1.15.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/crictl-v1.15.0-linux-arm.tar.gz)           | f31f8c3b4791608a48d030d1aa1a694a73849ae057b23a90ce4ef17e5afde9e8 |
| [crictl-v1.15.0-linux-ppc64le.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/crictl-v1.15.0-linux-ppc64le.tar.gz)   | df6559d7d5e6d83e60e654ae88114d3a008111a752c886791985690e3e9070ce |
| [crictl-v1.15.0-linux-s390x.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/crictl-v1.15.0-linux-s390x.tar.gz)       | 3fdbfe9b4fe51faf3cad9b235fefa728dc2742df9604bacef03ae5bff99caf25 |
| [crictl-v1.15.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/crictl-v1.15.0-windows-386.tar.gz)       | bd5b22c21b18f415afe37c93c676a32a28ea75f11872c39acff62d452e8fbe91 |
| [crictl-v1.15.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/crictl-v1.15.0-windows-amd64.tar.gz)   | 13d5050d45f67b008f68c6b65c55d3bb2f69517a5467ea9017e445f05aa01781 |
| [critest-v1.15.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/critest-v1.15.0-linux-386.tar.gz)         | 24995a20f54252a667c05d797867c4d051cab842da132db04c950ad28d9909ad |
| [critest-v1.15.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/critest-v1.15.0-linux-amd64.tar.gz)     | 099ac7f169b2c1b080448788b21969118659a24cd345e16836a4c959b95493e5 |
| [critest-v1.15.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/critest-v1.15.0-linux-arm64.tar.gz)     | 06df914f5b9b81708450eb9b50514b7db6b762e99d625534046dfc42bf60eca9 |
| [critest-v1.15.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/critest-v1.15.0-linux-arm.tar.gz)         | 64e2e858b88144ee6c1af712e280a42f6d90c6ceb9b0cb23fc388649f6afb62e |
| [critest-v1.15.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/critest-v1.15.0-windows-386.tar.gz)     | 694956163c41bfb84137fd6be15290a8acb4769ce823ad8cc8018fe6a2e6ce49 |
| [critest-v1.15.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.15.0/critest-v1.15.0-windows-amd64.tar.gz) | 297d813272048d61e6b5e5cc2f8a18e766200404add87f601e4704b7e6858cbe |

# v1.14.0

cri-tools v1.14.0 has upgraded to kubernetes v1.14, and Go version to 1.12. It mainly focus on bug fixes and stability improvements. It also adds full critest on Windows.

## CRI CLI (crictl)

- [#417](https://github.com/kubernetes-sigs/cri-tools/pull/417) Adds imagefsinfo subcommand for CRI ImageFSInfo() method.
- [#424](https://github.com/kubernetes-sigs/cri-tools/pull/424) Adds support to filter containers by image.
- [#425](https://github.com/kubernetes-sigs/cri-tools/pull/425) Fixes a bug when removing multiple containers.
- [#428](https://github.com/kubernetes-sigs/cri-tools/pull/428) Reduces the default connection timeout value.
- [#431](https://github.com/kubernetes-sigs/cri-tools/pull/431) Fixes the exit code for crictl exec.
- [#434](https://github.com/kubernetes-sigs/cri-tools/pull/434) Updated the instructions fro godep.
- [#437](https://github.com/kubernetes-sigs/cri-tools/pull/437) Adds support of -q for crictl info.
- [#442](https://github.com/kubernetes-sigs/cri-tools/pull/442) Adds support of zsh completion.
- [#451](https://github.com/kubernetes-sigs/cri-tools/pull/451) Upgrades kubernetes version to 1.14.

## CRI validation testing (critest)

- [#419](https://github.com/kubernetes-sigs/cri-tools/pull/419) Adds a benchmark testcase for measuring the time of creating pod and a container.
- [#429](https://github.com/kubernetes-sigs/cri-tools/pull/429) Changes streaming tests to omit newlines on echo
- [#430](https://github.com/kubernetes-sigs/cri-tools/pull/430) Adds support of critest for Windows container runtime.
- [#445](https://github.com/kubernetes-sigs/cri-tools/pull/445) Updates test environment to xenial and fixes docker installation.
- [#450](https://github.com/kubernetes-sigs/cri-tools/pull/450) Updates Go version to 1.12.

## Downloads

| file                                                                                                                                                | sha256                                                           |
| --------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| [crictl-v1.14.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/crictl-v1.14.0-linux-386.tar.gz)           | 665d5f1d31979039b85ff3756db66e377be69618dcefae077ddd42422a09b79f |
| [crictl-v1.14.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/crictl-v1.14.0-linux-amd64.tar.gz)       | 483c90a9fe679590df4332ba807991c49232e8cd326c307c575ecef7fe22327b |
| [crictl-v1.14.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/crictl-v1.14.0-linux-arm64.tar.gz)       | f76b3d00a272c8d210e9a45f77d07d3770bee310d99c4fd9a72d6f55278882e5 |
| [crictl-v1.14.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/crictl-v1.14.0-linux-arm.tar.gz)           | 9910cecfd6558239ba015323066c7233d8371af359b9ddd0b2a35d5223bcf945 |
| [crictl-v1.14.0-linux-ppc64le.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/crictl-v1.14.0-linux-ppc64le.tar.gz)   | 1e2cd11a1e025ed9755521cf13bb1bda986afa0052597a9bb44d31e62583413b |
| [crictl-v1.14.0-linux-s390x.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/crictl-v1.14.0-linux-s390x.tar.gz)       | 8b7b5749cba88ef337997ae90aa04380e3cab2c040b44b505b2fcd691c4935e4 |
| [crictl-v1.14.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/crictl-v1.14.0-windows-386.tar.gz)       | 02b1ba64c6fa7bbc7319e7c637ed89b4f898acd1b6f25f2012ac4a538de5be89 |
| [crictl-v1.14.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/crictl-v1.14.0-windows-amd64.tar.gz)   | 1f2d2e6612e930ff77640a10308a2cd40393a449a35b2dfbfeb06ae8d31e072d |
| [critest-v1.14.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/critest-v1.14.0-linux-386.tar.gz)         | 1eeaf7fdd386e14940326c83ed2a47fb28f6512c8d93b08d3c2a2f9e3d4270bb |
| [critest-v1.14.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/critest-v1.14.0-linux-amd64.tar.gz)     | 4c640219e297c877b56e7b13dcd0f10ab2a76092dd7bdb888d83c64372f0a3f9 |
| [critest-v1.14.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/critest-v1.14.0-linux-arm64.tar.gz)     | e4d5e9cc9b6eb22a96a37477e392d17a5e091c40aad1ae96ec06da3708fe605a |
| [critest-v1.14.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/critest-v1.14.0-linux-arm.tar.gz)         | bb29da5ff3405dae0442a5e2672524bf46837d6dea5c3b1ac4a99f0ad8bf78c4 |
| [critest-v1.14.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/critest-v1.14.0-windows-386.tar.gz)     | 8003a50ae0e0398237e42bb26da7cbfd8675deb4258ec812cb28ed989a03d7df |
| [critest-v1.14.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.14.0/critest-v1.14.0-windows-amd64.tar.gz) | 07dd737b207b12d22f68425dbe5347483059a9b48f47e95c37044de960023147 |

# v1.13.0

cri-tools v1.13.0 has upgraded to kubernetes v1.13. It mainly focus on bug fixes and stability improvements.

## CRI CLI (crictl)

- [#390](https://github.com/kubernetes-sigs/cri-tools/pull/390) Adds `--auth` options for pull command.
- [#392](https://github.com/kubernetes-sigs/cri-tools/pull/392) Fixes URL parsing for exec and attach.
- [#393](https://github.com/kubernetes-sigs/cri-tools/pull/393) Upgrades Go version to 1.11.1.
- [#394](https://github.com/kubernetes-sigs/cri-tools/pull/394) Enables Windows CI on travis.
- [#398](https://github.com/kubernetes-sigs/cri-tools/pull/398) Switches Windows default endpoints to npipe.
- [#402](https://github.com/kubernetes-sigs/cri-tools/pull/402) Updates version matrix information for the project.
- [#404](https://github.com/kubernetes-sigs/cri-tools/pull/404) Adds container name filter to ps command.
- [#406](https://github.com/kubernetes-sigs/cri-tools/pull/406) Adds metadata to filters.
- [#407](https://github.com/kubernetes-sigs/cri-tools/pull/407) Prints annotations and labels for inspect command.

## CRI validation testing (critest)

- [#391](https://github.com/kubernetes-sigs/cri-tools/pull/391) Add tests for multiple containers in a pod.
- [#405](https://github.com/kubernetes-sigs/cri-tools/pull/405) Adds runtime handler support for critest.
- [#410](https://github.com/kubernetes-sigs/cri-tools/pull/410) Adds exec sync timeout test cases.
- [#411](https://github.com/kubernetes-sigs/cri-tools/pull/411) Fixes readonly filesystem test cases.

## Downloads

| file                                                                                                                                              | sha256                                                           |
| ------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| [crictl-v1.13.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-386.tar.gz)         | 8a289d86b97f678fd5ddbd973503f772cfab9c29ef5e391930130c6214feecc9 |
| [crictl-v1.13.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-amd64.tar.gz)     | 9bdbea7a2b382494aff2ff014da328a042c5aba9096a7772e57fdf487e5a1d51 |
| [crictl-v1.13.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-arm64.tar.gz)     | 68949c0cb5a37e7604c145d189cf1e109c08c93d9c710ba663db026b9c6f2746 |
| [crictl-v1.13.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-arm.tar.gz)         | 2e478ebed85f9d70d49fd8f1d1089c8fba6e37d3461aeef91813f1ab0f0df586 |
| [crictl-v1.13.0-linux-ppc64le.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-ppc64le.tar.gz) | e85c3f95afd9752c65ec5d94a374a33e80576548ce95c2771a0973d7e3d9e6fa |
| [crictl-v1.13.0-linux-s390x.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-s390x.tar.gz)     | fe623c98ddff7e4b8679169bc9bb222d1c5dc81867234f95e9966dcd410e7b6b |
| [crictl-v1.13.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-windows-386.tar.gz)     | 641db1383708735d00a82fa947cc43850eb1a80de7129120967af59b24c2cf13 |
| [crictl-v1.13.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-windows-amd64.tar.gz) | 1a8468d4b67f8f73b05d38e7df146160033561b25fe7e2cee7d3aa374842e72c |
| [critest-v1.13.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/critest-v1.13.0-linux-386.tar.gz)       | 020f3dea6a6360655b85c2180a8958aab9ae458d33cb50d12ac1faa329704aac |
| [critest-v1.13.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/critest-v1.13.0-linux-amd64.tar.gz)   | 0161bbaf1a891fc87a852659da103165fa788aa773a32fa2a1ed584b5dd04d99 |
| [critest-v1.13.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/critest-v1.13.0-linux-arm64.tar.gz)   | 76ad6796aa1bcff6412d18b45ee4015f32b9cd96589704af414930ddeb7dff91 |
| [critest-v1.13.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/critest-v1.13.0-linux-arm.tar.gz)       | fb8ff0a90cd59f18878cb81b40dd2b4223973d068d9a5c484de4f8f3224d249e |

# v1.12.0

cri-tools v1.12.0 has upgraded to kubernetes v1.12. It mainly focus on bug fixes and new features introduced in kubernetes v1.12. It has also moved to <https://github.com/kubernetes-sigs/cri-tools>.

## CRI CLI (crictl)

- [#345](https://github.com/kubernetes-sigs/cri-tools/pull/345) Fixes missing Windows library
- [#354](https://github.com/kubernetes-sigs/cri-tools/pull/354) Properly returns errors when the output format is not supported
- [#357](https://github.com/kubernetes-sigs/cri-tools/pull/357) Fixes version information and install guides
- [#361](https://github.com/kubernetes-sigs/cri-tools/pull/361) Show concise image info for crictl ps
- [#363](https://github.com/kubernetes-sigs/cri-tools/pull/363) Fixes crictl ps and crictl pods
- [#367](https://github.com/kubernetes-sigs/cri-tools/pull/367) Fixes version information for release scripts
- [#369](https://github.com/kubernetes-sigs/cri-tools/pull/369) Adds podID in output of `crictl ps`
- [#370](https://github.com/kubernetes-sigs/cri-tools/pull/370) Fixes non JSON keys support in info map
- [#374](https://github.com/kubernetes-sigs/cri-tools/pull/374) Adds support for Windows npipe `\.\pipe\dockershim`
- [#375](https://github.com/kubernetes-sigs/cri-tools/pull/375) Adds sandbox config to `image pull`
- [#378](https://github.com/kubernetes-sigs/cri-tools/pull/378) Fixes unmarshal issues in `crictl inspecti`
- [#383](https://github.com/kubernetes-sigs/cri-tools/pull/383) Adds support for runtime handler
- [#384](https://github.com/kubernetes-sigs/cri-tools/pull/384) Fixes timeout for grpc dialer

## CRI validation testing (critest)

- [#377](https://github.com/kubernetes-sigs/cri-tools/pull/377) Adds new test to critest for privileged container

## Downloads

| file                                                                                                                                              | sha256                                                           |
| ------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| [crictl-v1.12.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-386.tar.gz)         | 028ccea08422e011fcf11db4ebed772b1c434b44c4dd717cecd80bd0d1e57417 |
| [crictl-v1.12.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-amd64.tar.gz)     | e7d913bcce40bf54e37ab1d4b75013c823d0551e6bc088b217bc1893207b4844 |
| [crictl-v1.12.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-arm64.tar.gz)     | 8466f08b59bf36d2eebcb9428c3d4e6e224c3065d800ead09ad730ce374da6fe |
| [crictl-v1.12.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-arm.tar.gz)         | ca6b4ac80278d32d9cc8b8b19de140fd1cc35640f088969f7068fea2df625490 |
| [crictl-v1.12.0-linux-ppc64le.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-ppc64le.tar.gz) | ec6254f1f6ffa064ba41825aab5612b7b005c8171fbcdac2ca3927d4e393000f |
| [crictl-v1.12.0-linux-s390x.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-s390x.tar.gz)     | 814aa9cd496be416612c2653097a1c9eb5784e38aa4889034b44ebf888709057 |
| [crictl-v1.12.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-windows-386.tar.gz)     | 4520520b106b232a8a6e99ecece19a83bf58b94d48e28b4c0483a4a0f59fe161 |
| [crictl-v1.12.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-windows-amd64.tar.gz) | e401db715a9f843acaae40846a4c18f6938df95c34d06af08aac2fc3e591b2a7 |
| [critest-v1.12.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/critest-v1.12.0-linux-386.tar.gz)       | ae9da4a95147e1486575d649b4384e91ba701a0aecadbc91c70ea3a963ba1b6b |
| [critest-v1.12.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/critest-v1.12.0-linux-amd64.tar.gz)   | 681055657a19b8ce2ecb2571e71cc7b069f33847f2f5ae72e220f55292a5e976 |
| [critest-v1.12.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/critest-v1.12.0-linux-arm64.tar.gz)   | b3eb282ab6d845e8c640c51aa266dc9d373d991a824cf550fbc12c36f98dcc5d |
| [critest-v1.12.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/critest-v1.12.0-linux-arm.tar.gz)       | 4593d86afffa373ab2ec5ae3b66fc0ca5413db3dd8268603e13a4820e0f8633d |

# v1.11.1

cri-tools v1.11.1 mainly focused on UX improvement and bug fix.

## CRI CLI (crictl)

- [#338](https://github.com/kubernetes-sigs/cri-tools/pull/338) Allow filtering the pods with prefix matching of name and namespace
- [#342](https://github.com/kubernetes-sigs/cri-tools/pull/342) Clarify flag description in `crictl ps` and `crictl pods`.
- [#343](https://github.com/kubernetes-sigs/cri-tools/pull/343) Better terminal support in `crictl exec` and `crictl attach`, which also fixes issue [#288](https://github.com/kubernetes-sigs/cri-tools/issues/288) and [#181](https://github.com/kubernetes-sigs/cri-tools/issues/181).

# v1.11.0

cri-tools v1.11.0 mainly focused on stability improvements and multi-arch support. Container runtime interface (CRI) has been updated to v1alpha2 in order to be compatible with kubernetes v1.11.

## CRI validation testing (critest)

- [#300](https://github.com/kubernetes-sigs/cri-tools/pull/300) Make image-user test images multi-arch.
- [#311](https://github.com/kubernetes-sigs/cri-tools/pull/311) Adds push-manifest into all target in the image-user Makefile.
- [#313](https://github.com/kubernetes-sigs/cri-tools/pull/313) Make hostnet-nginx test images multi-arch.
- [#315](https://github.com/kubernetes-sigs/cri-tools/pull/315) Makes image-test test images multi-arch.
- [#320](https://github.com/kubernetes-sigs/cri-tools/pull/320) Adds container host path validation tests.

## CRI CLI (crictl)

- [#306](https://github.com/kubernetes-sigs/cri-tools/pull/306) Fixes argument parsing for crictl exec.
- [#312](https://github.com/kubernetes-sigs/cri-tools/pull/312) Fixes a typo in inspecti usage.
- [#316](https://github.com/kubernetes-sigs/cri-tools/pull/316) Cleanups container and sandbox state.
- [#321](https://github.com/kubernetes-sigs/cri-tools/pull/321) Improves documentation and examples of crictl.
- [#325](https://github.com/kubernetes-sigs/cri-tools/pull/325) Upgrades kubernetes vendor to v1.11 branch.

# v1.0.0-beta.1

cri-tools v1.0.0-beta.1 mainly focused on critest coverage improvement, and bug fixes.

## CRI validation testing (critest)

- [#282](https://github.com/kubernetes-sigs/cri-tools/pull/282) Add RunAsGroup test. The test `runtime should return error if RunAsGroup is set without RunAsUser` only works with Kubernetes 1.11+.
- [#289](https://github.com/kubernetes-sigs/cri-tools/pull/289) Add host network pod portforward test.
- [#290](https://github.com/kubernetes-sigs/cri-tools/pull/290) Use busybox:1.28 instead of busybox:1.26 in the test to better support multi-arch.
- [#296](https://github.com/kubernetes-sigs/cri-tools/pull/296) Make `critest` binary statically linked.

## CRI CLI (crictl)

- [#278](https://github.com/kubernetes-sigs/cri-tools/pull/278) Remove "sandbox" from `crictl` command description.
- [#279](https://github.com/kubernetes-sigs/cri-tools/pull/279) Remove `oom-score-adj` flag from `crictl update` because it is not supported by `runc`.
- [#291](https://github.com/kubernetes-sigs/cri-tools/pull/291) Fix a bug that `crictl` generates a log file in `/tmp` directory each run. This can potentially fill `/tmp` directory.
- [#296](https://github.com/kubernetes-sigs/cri-tools/pull/296) Make `crictl` binary statically linked.

# v1.0.0-beta.0

cri-tools v1.0.0-beta.0 is mainly focus on UX improvements, including make crictl command more user friendly and add initial Windows support. Container runtime interface (CRI) has been updated to v1alpha2 in order to be compatible with kubernetes v1.10. Version matrix and branches for different kubernetes versions are also added.

## CRI validation testing (critest)

- [#227](https://github.com/kubernetes-sigs/cri-tools/pull/227) Set StdinOnce to true for attach test
- [#232](https://github.com/kubernetes-sigs/cri-tools/pull/232) Improves CRI log parser
- [#242](https://github.com/kubernetes-sigs/cri-tools/pull/242) Add validation of reopening container logs
- [#250](https://github.com/kubernetes-sigs/cri-tools/pull/250) Add validation of username not empty in ImageStatus
- [#252](https://github.com/kubernetes-sigs/cri-tools/pull/252) Improve image test and make test run in parallel
- [#257](https://github.com/kubernetes-sigs/cri-tools/pull/257) Add golang 1.10 and fix a race condition
- [#261](https://github.com/kubernetes-sigs/cri-tools/pull/261) [#273](https://github.com/kubernetes-sigs/cri-tools/pull/273) Remove dependency of source code
- [#267](https://github.com/kubernetes-sigs/cri-tools/pull/267) Add test for pid namespace
- [#269](https://github.com/kubernetes-sigs/cri-tools/pull/269) Add validation of tty settings for exec

## CRI CLI (crictl)

- [#222](https://github.com/kubernetes-sigs/cri-tools/pull/222) Rename `sandboxes` subcommand to `pods` and rename`sandbox` to `podsandbox` in all subcommands
- [#225](https://github.com/kubernetes-sigs/cri-tools/pull/225) Add support of windows
- [#238](https://github.com/kubernetes-sigs/cri-tools/pull/238) Update CRI to v1alpha2
- [#255](https://github.com/kubernetes-sigs/cri-tools/pull/255) Add support of multiple Ids to subcommands
- [#256](https://github.com/kubernetes-sigs/cri-tools/pull/256) Add `crictl ps -q`
- [#258](https://github.com/kubernetes-sigs/cri-tools/pull/258) Rename CRI endpoints environment variable to `CONTAINER_RUNTIME_ENDPOINT` and `IMAGE_SERVICE_ENDPOINT`
- [#268](https://github.com/kubernetes-sigs/cri-tools/pull/268) Avoid panic when runtimes are using truncated IDs
- [#274](https://github.com/kubernetes-sigs/cri-tools/pull/274) Add support of insecure TLS without auth

# v1.0.0-alpha.0

cri-tools v1.0.0-alpha.0 is mainly focus on UX improvements, including make crictl command more user friendly and add more subcommands. It also updates container runtime interface (CRI) to kubernetes v1.9 and fixes bugs in validation test suites.

## CRI validation testing (critest)

- [#164](https://github.com/kubernetes-sigs/cri-tools/pull/164) Fix security context test to not rely on `/etc/hosts`
- [#165](https://github.com/kubernetes-sigs/cri-tools/pull/165) Validate IPv4 only for port mapping tests
- [#196](https://github.com/kubernetes-sigs/cri-tools/pull/196) Fix privileged container validation by replacing `ip link` with `brctl addbr` command
- [#197](https://github.com/kubernetes-sigs/cri-tools/pull/197) Fix hostIPC validation to support old ipcmk versions
- [#199](https://github.com/kubernetes-sigs/cri-tools/pull/199) [#201](https://github.com/kubernetes-sigs/cri-tools/pull/201) Fix container logs validation
- [#200](https://github.com/kubernetes-sigs/cri-tools/pull/200) Add SELinux validation tests

## CRI CLI (crictl)

- [#156](https://github.com/kubernetes-sigs/cri-tools/pull/156) Fix empty RepoTags handling for `images` command
- [#163](https://github.com/kubernetes-sigs/cri-tools/pull/163) Add `--digest` option to `images` command
- [#167](https://github.com/kubernetes-sigs/cri-tools/pull/167) Add verbose for `status` command
- [#171](https://github.com/kubernetes-sigs/cri-tools/pull/171) Sort results by creation time for `ps`, `sandboxes` and `images` commands
- [#174](https://github.com/kubernetes-sigs/cri-tools/pull/174) Support select sandboxes by name for `sandboxes` and other commands
- [#178](https://github.com/kubernetes-sigs/cri-tools/pull/178) [#190](https://github.com/kubernetes-sigs/cri-tools/pull/190) Replace golang json with `protobuf/jsonpb` library
- [#182](https://github.com/kubernetes-sigs/cri-tools/pull/182) Fix stdout and stderr for `attach` and `exec` command
- [#183](https://github.com/kubernetes-sigs/cri-tools/pull/183) Add created time to `sandboxes` command
- [#186](https://github.com/kubernetes-sigs/cri-tools/pull/186) Use kubelet's log library instead of a copied one
- [#187](https://github.com/kubernetes-sigs/cri-tools/pull/187) Add image tag and attempt to `ps` command
- [#194](https://github.com/kubernetes-sigs/cri-tools/pull/194) Add `config` command
- [#217](https://github.com/kubernetes-sigs/cri-tools/pull/217) Add `--latest` and `--last` options to `ps` and `sandboxes` commands
- [#202](https://github.com/kubernetes-sigs/cri-tools/pull/202) [#203](https://github.com/kubernetes-sigs/cri-tools/pull/203) Add `--all`, `--latest`, `--last` and `--no-trunc` options to `ps` command
- [#205](https://github.com/kubernetes-sigs/cri-tools/pull/205) Improve logs command and add `--timestamps` and `--since` options
- [#206](https://github.com/kubernetes-sigs/cri-tools/pull/206) Add verbose debut output to `inspect` and `inspects` commands
- [#207](https://github.com/kubernetes-sigs/cri-tools/pull/207) Sort flags for all commands
- [#209](https://github.com/kubernetes-sigs/cri-tools/pull/209) Add `stats` command
- [#211](https://github.com/kubernetes-sigs/cri-tools/pull/211) Rewrite timestamps in container status and sandbox status to make them more user friendly
- [#213](https://github.com/kubernetes-sigs/cri-tools/pull/213) Add completion command
- [#216](https://github.com/kubernetes-sigs/cri-tools/pull/216) Add `--no-trunc` to `images` and `sandboxes` commands

# v0.2

cri-tools v0.2 enhances validation testings, improves crictl UX and also fixes several bugs. It has also updates container runtime interface (CRI) to kubernetes v1.8.

## CRI validation testing (critest)

- [#127](https://github.com/kubernetes-sigs/cri-tools/pull/127) Adds validation tests for supplemental groups
- [#135](https://github.com/kubernetes-sigs/cri-tools/pull/135) [#137](https://github.com/kubernetes-sigs/cri-tools/pull/137) and [#144](https://github.com/kubernetes-sigs/cri-tools/pull/144) Adds validation tests for seccomp
- [#139](https://github.com/kubernetes-sigs/cri-tools/pull/139) Adds validation tests for sysctls
- [#140](https://github.com/kubernetes-sigs/cri-tools/pull/140) Adds validation tests for AppArmor
- [#141](https://github.com/kubernetes-sigs/cri-tools/pull/141) Adds validation tests for NoNewPrivs
- [#142](https://github.com/kubernetes-sigs/cri-tools/pull/142) Adds validation tests for mount propagation
- [#115](https://github.com/kubernetes-sigs/cri-tools/pull/115) Fixes image validation tests
- [#116](https://github.com/kubernetes-sigs/cri-tools/pull/116) Fixes validation message
- [#126](https://github.com/kubernetes-sigs/cri-tools/pull/126) Fixes sandbox leak in port forward validation tests

## CRI CLI (crictl)

- [#122](https://github.com/kubernetes-sigs/cri-tools/pull/122) Adds support for authenticated image pull
- [#123](https://github.com/kubernetes-sigs/cri-tools/pull/123) Improves crictl UX
- [#124](https://github.com/kubernetes-sigs/cri-tools/pull/124) Adds support for creating sandboxes and containers from yaml
- [#133](https://github.com/kubernetes-sigs/cri-tools/pull/133) Adds timeout support for container stop

# v0.1

cri-tools provides a set of tools for Kubelet Container Runtime Interface (CRI):

- **CRI validation testing**
  - provides a test framework and a suite of tests to validate that the Container Runtime Interface (CRI) server implementation meets all the requirements.
  - allows the CRI runtime developers to verify that their runtime conforms to CRI, without needing to set up Kubernetes components or run Kubernetes end-to-end tests.
- **crictl**
  - provides a CLI for CRI-compatible container runtimes.
  - allows the CRI runtime developers to debug of their runtime without needing to set up Kubernetes components.
- **CRI performance benchmarking**
  - provides a benchmarking framework for CRI-compatible container runtimes.
  - allows the CRI runtime developers to benchmark the performance of their runtime without needing to set up Kubernetes components or run Kubernetes benchmark tests.

## Features

### CRI validation testing

- basic sandbox and container operations
- basic image operations
- networking, e.g. DNS config, port mapping
- streaming, e.g. exec, attach, portforward
- security context, e.g.
  - hostPID, hostIPC, hostNetwork
  - runAsUser, readOnlyRootfs, privileged
- execSync,version,status

### crictl

- get version and status
- sandbox run, stop, status, list, and remove
- container create, start, stop, status, list and remove
- image pull, list, status and remove
- streaming attach, exec and portforward

### CRI performance benchmarking

- parallel sandbox run, stop, status, list and remove
- parallel container create, start, stop, status, list and remove
- parallel image pull, list and remove

## Documentation

See [cri-tools](https://github.com/kubernetes-sigs/cri-tools/#documentation).
