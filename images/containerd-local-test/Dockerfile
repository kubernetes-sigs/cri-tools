# Copyright The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Build containerd (and runc/crun + CNI) from source so the local test image
# matches what CI exercises. CI builds containerd from a checkout of
# containerd/containerd at a branch ref (main, release/1.7) and installs the
# runtime engine and CNI plugins via containerd's own setup scripts. See
# .github/workflows/containerd.yml.

# Go toolchain version, extracted from go.mod by hack/run-e2e-container.sh
# (mirroring CI's go-version-file approach). The default is a fallback only.
ARG GO_VERSION=1.26.3

FROM golang:${GO_VERSION} AS containerd-builder

# CONTAINERD_VERSION is the containerd git ref to build (matches the CI matrix:
# main, release/1.7). RUNC_FLAVOR selects the OCI runtime engine (runc or crun).
ARG CONTAINERD_VERSION=main
ARG RUNC_FLAVOR=runc

# Same build dependencies CI installs for building containerd
# (.github/workflows/containerd.yml). curl is needed by install-runc when
# RUNC_FLAVOR=crun (it downloads a released crun binary).
RUN apt-get update && apt-get install -y \
    btrfs-progs \
    build-essential \
    curl \
    git \
    libbtrfs-dev \
    libseccomp-dev \
    libseccomp2 \
    socat \
    && rm -rf /var/lib/apt/lists/*

# Build containerd from source at the requested ref, mirroring CI's
# "make && make install" (.github/workflows/containerd.yml).
RUN git clone https://github.com/containerd/containerd \
    /go/src/github.com/containerd/containerd
WORKDIR /go/src/github.com/containerd/containerd
RUN git checkout "${CONTAINERD_VERSION}"
RUN make && make install

# Install the runc/crun runtime engine via containerd's own script, mirroring
# CI's install-runc step (.github/workflows/containerd.yml).
RUN RUNC_FLAVOR="${RUNC_FLAVOR}" script/setup/install-runc

# Install CNI plugins via containerd's own script, mirroring CI's install-cni
# steps (.github/workflows/containerd.yml). release/1.7 must be invoked with no
# argument; other refs pass the containernetworking/plugins version pinned in
# containerd's go.mod so CNI matches the containerd ref under test.
RUN if [ "${CONTAINERD_VERSION}" = "release/1.7" ]; then \
        script/setup/install-cni; \
    else \
        script/setup/install-cni \
            "$(grep 'github.com/containernetworking/plugins' go.mod | awk '{print $2}')"; \
    fi

# Runtime stage: ubuntu matching the CI runner (ubuntu-24.04), with only the
# packages needed to run containerd and critest. The build toolchain stays in
# the builder stage and never reaches the final image.
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    apparmor-utils \
    ca-certificates \
    iptables \
    libseccomp2 \
    socat \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# containerd, ctr and the containerd shims (installed under /usr/local/bin).
COPY --from=containerd-builder /usr/local/bin/ /usr/local/bin/
# runc / crun (installed under /usr/local/sbin).
COPY --from=containerd-builder /usr/local/sbin/ /usr/local/sbin/
# CNI plugins.
COPY --from=containerd-builder /opt/cni/bin/ /opt/cni/bin/

# Copy setup scripts
COPY hack/setup-containerd.sh /usr/local/bin/setup-containerd.sh
COPY hack/wait-for-containerd.sh /usr/local/bin/wait-for-containerd.sh
RUN chmod +x /usr/local/bin/setup-containerd.sh /usr/local/bin/wait-for-containerd.sh

# Copy entrypoint
COPY images/containerd-local-test/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV PATH="/usr/local/bin/critest-tools:${PATH}"

ENTRYPOINT ["/entrypoint.sh"]
