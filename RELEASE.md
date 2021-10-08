# Release Process

The Kubernetes cri-tools Project is released for every Kubernetes release. The
process is as follows:

1. An OWNER runs `git tag -m $VERSION -s $VERSION` and pushes the tag with `git push $VERSION`, where `$VERSION` follows the Kubernetes minor version.
1. An announcement email is sent to `kubernetes-dev@googlegroups.com` with the subject `[ANNOUNCE] cri-tools $VERSION is released`
