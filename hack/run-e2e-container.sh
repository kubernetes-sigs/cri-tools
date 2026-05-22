#!/usr/bin/env bash

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

set -euo pipefail

# Root of the repository
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

# Build the binaries locally if they don't exist
# We assume the user has Go installed and can build for linux.
# If they are on another OS, they need to set GOOS=linux.
export GOOS=linux
make binaries crictl-e2e install.ginkgo

# Determine architecture for mounting the right binary directory
ARCH=$(go env GOARCH)
BINARY_DIR="${REPO_ROOT}/build/bin/linux/${ARCH}"

# Containerd version (git ref) to build into the image, the runc flavor, and the
# runtime handler. Defaults mirror CI's primary matrix entry (main + runc).
CONTAINERD_VERSION="${CONTAINERD_VERSION:-main}"
RUNC_FLAVOR="${RUNC_FLAVOR:-runc}"
RUNTIME="${RUNTIME:-io.containerd.runc.v2}"

# Sanitize the version for use in a Docker tag: tags may not contain "/", so
# "release/1.7" becomes "release-1.7". Include the runc flavor in the tag when
# it is not the default so crun images do not collide with runc images.
SANITIZED_VERSION="${CONTAINERD_VERSION//\//-}"
IMAGE_TAG="${SANITIZED_VERSION}"
if [ "${RUNC_FLAVOR}" != "runc" ]; then
    IMAGE_TAG="${IMAGE_TAG}-${RUNC_FLAVOR}"
fi
IMAGE_NAME="containerd-local-test:${IMAGE_TAG}"

# Namespace the data volume per containerd version: different containerd
# versions can use incompatible content/metadata stores, so sharing one volume
# across versions risks corruption. The clean target removes all of them.
DATA_VOLUME="containerd-local-test-data-${IMAGE_TAG}"

# Build the runner image, tagged per containerd version so it is cached and
# reused on subsequent runs. Skip the build when the tagged image already
# exists locally unless FORCE_REBUILD is set. A changed CONTAINERD_VERSION /
# RUNC_FLAVOR produces a new tag, so it builds a fresh image rather than
# reusing the cached one.
if [ -z "${FORCE_REBUILD:-}" ] && docker image inspect "${IMAGE_NAME}" >/dev/null 2>&1; then
    echo "Reusing cached image ${IMAGE_NAME} (set FORCE_REBUILD=1 to rebuild)..."
else
    # The Dockerfile clones containerd and checks out CONTAINERD_VERSION in
    # cached layers, so for a moving ref (e.g. main) a plain rebuild would reuse
    # the previously fetched commit. FORCE_REBUILD must therefore bypass the
    # layer cache (--no-cache) and refresh the base images (--pull) so the image
    # is rebuilt against the current ref, matching CI which always checks out
    # fresh.
    BUILD_FLAGS=()
    if [ -n "${FORCE_REBUILD:-}" ]; then
        BUILD_FLAGS+=(--no-cache --pull)
    fi
    # Extract Go version from go.mod, mirroring CI's go-version-file approach.
    GO_VERSION="$(grep '^go ' go.mod | awk '{print $2}')"

    echo "Building the containerd local test image ${IMAGE_NAME}..."
    docker build \
        "${BUILD_FLAGS[@]+"${BUILD_FLAGS[@]}"}" \
        --build-arg "GO_VERSION=${GO_VERSION}" \
        --build-arg "CONTAINERD_VERSION=${CONTAINERD_VERSION}" \
        --build-arg "RUNC_FLAVOR=${RUNC_FLAVOR}" \
        -t "${IMAGE_NAME}" \
        -f images/containerd-local-test/Dockerfile .
fi

# If no command is provided, default to critest
if [ $# -eq 0 ]; then
    set -- /usr/local/bin/critest-tools/critest --runtime-endpoint=unix:///run/containerd/containerd.sock
fi

# Run the e2e tests in the container
# We mount the local build directory to /usr/local/bin/critest-tools
# so that the container has access to the locally built binaries.
# We also use a volume for containerd's data directory so that
# pulled images are persisted between runs.
echo "Running in the container..."

# Build optional volume mounts that may not exist on all platforms (e.g., macOS)
OPTIONAL_MOUNTS=()
if [ -d "/lib/modules" ]; then
    OPTIONAL_MOUNTS+=(-v "/lib/modules:/lib/modules:ro")
fi
if [ -d "/etc/apparmor.d" ]; then
    OPTIONAL_MOUNTS+=(-v "/etc/apparmor.d:/etc/apparmor.d:ro")
fi

docker run --rm --privileged \
    -e "RUNTIME=${RUNTIME}" \
    -e "CONTAINERD_VERSION=${CONTAINERD_VERSION}" \
    ${ENABLE_NRI:+-e "ENABLE_NRI=${ENABLE_NRI}"} \
    -v "${BINARY_DIR}:/usr/local/bin/critest-tools:ro" \
    -v "${DATA_VOLUME}:/var/lib/containerd" \
    ${OPTIONAL_MOUNTS[@]+"${OPTIONAL_MOUNTS[@]}"} \
    "${IMAGE_NAME}" \
    "$@"
