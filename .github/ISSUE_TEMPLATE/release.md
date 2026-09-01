---
name: Release
about: Track a new cri-tools release
labels: sig/release, sig/node
---

## Release cri-tools v1.Y.Z

### Pre-release

- [ ] Vendor the final Kubernetes release candidate so that cri-tools can be released before Kubernetes
- [ ] Create a new signed tag (`git tag -s -m v1.Y.Z v1.Y.Z`) and push it. Wait for the [release GitHub Actions CI](https://github.com/kubernetes-sigs/cri-tools/actions/workflows/release.yml) to finish. The release actions job adds the notes to the release and attaches the artifacts.

### Post-release

- [ ] Update cri-tools version references in the repo ([example](https://github.com/kubernetes-sigs/cri-tools/pull/2172))
- [ ] Ask a [Kubernetes Release Manager](https://k8s.io/releases/release-managers/) to update the official OBS packages using [`krel`](https://github.com/kubernetes/release/releases/latest):
  ```
  krel obs stage \
      --packages cri-tools \
      --version 1.Y.Z \
      --project isv:kubernetes:core:stable:v1.Y:build \
      --stream \
      --nomock
  ```
  ```
  krel obs release \
      --packages cri-tools \
      --stream \
      --project isv:kubernetes:core:stable:v1.Y:build \
      --nomock
  ```
- [ ] Update cri-tools in k/kubernetes ([example](https://github.com/kubernetes/kubernetes/pull/122271))
- [ ] Update cri-tools in [cri-o/packaging](https://github.com/cri-o/packaging)
