# Contributing guidelines

## How to become a contributor and submit your own code

### Contributor License Agreements

We'd love to accept your patches! Before we can take them, we have to jump a couple of legal hurdles.

Please fill out either the individual or corporate Contributor License Agreement (CLA).

- If you are an individual writing original source code and you're sure you own the intellectual property, then you'll need to sign an [individual CLA](https://identity.linuxfoundation.org/node/285/node/285/individual-signup).
- If you work for a company that wants to allow you to contribute your work, then you'll need to sign a [corporate CLA](https://identity.linuxfoundation.org/node/285/organization-signup).

Follow either of the two links above to access the appropriate CLA and instructions for how to sign and return it. Once we receive it, we'll be able to accept your pull requests.

### Contributing A Patch

1. Submit an issue describing your proposed change to the repo in question.
1. The [repo owners](OWNERS) will respond to your issue promptly.
1. If your proposed change is accepted, and you haven't already done so, sign a Contributor License Agreement (see details above).
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.

### Adding dependencies

If your patch depends on new packages, add that package to the `go.mod` file,
run `make vendor` and commit the changes.

## Becoming a reviewer and approver

The [SIG Node Contributor Ladder][ladder] outlines the base criteria to become a
project reviewer and approver. Beside them, the following recurring working
areas can be used to achieve that goal:

[ladder]: https://github.com/kubernetes/community/blob/5e4c312/sig-node/sig-node-contributor-ladder.md#sig-node-contributor-ladder

- Triaging all [issues](https://github.com/kubernetes-sigs/cri-tools/issues) (weekly)
  - Answering questions, for example regarding CRI vs docker APIs
  - Accepting new features
  - Addressing bugs
- Reviewing PRs (weekly)
  - Accepting dependency updates if the CI passes
  - Providing feedback for all other PRs
- Cutting [releases](https://github.com/kubernetes-sigs/cri-tools/releases) (for each Kubernetes release or critical bugfix)
  - The automation allows to push a tag and the rest will be done by the CI
  - Built artifacts have to be updated in multiple other locations like the k/release GCS bucket and within the official k/kubernetes repository
  - Updating the docs (like README.md) to mention the latest available releases
- Maintaining the CI (if required)
  - Updating golang versions
  - Updating dependencies like linter, used CRI-O versions (containerd versions are exceptional to us but can be bumped as well in case the CI fails)
  - Monitoring the main branch and PRs for CI failures not caused by code changes and fixing them.
