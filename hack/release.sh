#!/bin/bash

# Copyright 2018 The Kubernetes Authors.
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

# Run critest with default dockershim.

set -o errexit
set -o nounset
set -o pipefail

UPLOAD_GCS=${UPLOAD_GCS:-false}
GCS_PATH=${GCS_PATH:-"kubernetes-release/crictl"}
VERSION=$(git describe --abbrev=0 --tag)
CRI_CTL_PLATFORMS=(
    linux/amd64
    linux/386
    linux/arm
    linux/arm64
    linux/ppc64le
    linux/s390x
    windows/amd64
    windows/386
    darwin/amd64
    darwin/386
)
CRI_TEST_PLATFORMS=(
    linux/amd64
    linux/386
    linux/arm
    linux/arm64
    windows/amd64
    windows/386
    darwin/amd64
    darwin/386
)

# Create releases output directory.
PROJECT="github.com/kubernetes-sigs/cri-tools"
CRI_TOOLS_ROOT="$GOPATH/src/$PROJECT"
mkdir -p ${CRI_TOOLS_ROOT}/_output/releases

GO_LDFLAGS="-X ${PROJECT}/pkg/version.Version=${VERSION}"

# Build and package crictl releases.
for platform in "${CRI_CTL_PLATFORMS[@]}"; do
    os="${platform%/*}"
    arch=$(basename "${platform}")

    CRICTL_BIN="crictl"
    if [ "$os" == "windows" ]; then
        CRICTL_BIN="crictl.exe"
    fi

    output_bin=${CRI_TOOLS_ROOT}/_output/bin/$arch-$os/${CRICTL_BIN}
    GOARCH="$arch" GOOS="$os" CGO_ENABLED=0 go build \
        -o ${output_bin} \
        -ldflags "${GO_LDFLAGS}" \
        ${PROJECT}/cmd/crictl
    if $UPLOAD_GCS; then
      gsutil cp ${output_bin} gs://${GCS_PATH}/crictl-$VERSION-$os-$arch
      sha1sum ${output_bin} | awk '{print $1}' | gsutil cp - gs://${GCS_PATH}/crictl-$VERSION-$os-$arch.sha1sum
    fi
    tar zcvf ${CRI_TOOLS_ROOT}/_output/releases/crictl-$VERSION-$os-$arch.tar.gz \
        -C ${CRI_TOOLS_ROOT}/_output/bin/$arch-$os \
        ${CRICTL_BIN}
done

# Build and package critest releases.
for platform in "${CRI_TEST_PLATFORMS[@]}"; do
    os="${platform%/*}"
    arch=$(basename "${platform}")

    CRITEST_BIN="critest"
    if [ "$os" == "windows" ]; then
        CRITEST_BIN="critest.exe"
    fi

    output_bin=${CRI_TOOLS_ROOT}/_output/bin/$arch-$os/${CRITEST_BIN}
    GOARCH="$arch" GOOS="$os" CGO_ENABLED=0 go test -c \
        -o ${output_bin} \
        -ldflags "${GO_LDFLAGS}" \
        ${PROJECT}/cmd/critest
    if $UPLOAD_GCS; then
      gsutil cp ${output_bin} gs://${GCS_PATH}/critest-$VERSION-$os-$arch
      sha1sum ${output_bin} | awk '{print $1}' | gsutil cp - gs://${GCS_PATH}/critest-$VERSION-$os-$arch.sha1sum
    fi
    tar zcvf ${CRI_TOOLS_ROOT}/_output/releases/critest-$VERSION-$os-$arch.tar.gz \
        -C ${CRI_TOOLS_ROOT}/_output/bin/$arch-$os \
        ${CRITEST_BIN}
done

# Show sha256 for release files
if [[ "${OSTYPE}" == "darwin"* ]]; then
  for file in $(ls ${CRI_TOOLS_ROOT}/_output/releases); do
    echo "$file $(shasum -a 256 ${CRI_TOOLS_ROOT}/_output/releases/$file | awk '{print $1}')"
  done
else
  for file in $(ls ${CRI_TOOLS_ROOT}/_output/releases); do
    echo "$file $(sha256sum -b ${CRI_TOOLS_ROOT}/_output/releases/$file | awk '{print $1}')"
  done
fi
