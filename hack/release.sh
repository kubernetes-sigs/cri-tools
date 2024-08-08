#!/usr/bin/env bash

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

# Run critest with default containerd.

set -o errexit
set -o nounset
set -o pipefail

VERSION=$(git describe --abbrev=0 --tag)
CRI_CTL_PLATFORMS=(
    linux/amd64
    linux/386
    linux/arm64
    linux/ppc64le
    linux/s390x
    linux/mips64le
    linux/riscv64
    windows/amd64
    windows/386
    darwin/amd64
    darwin/arm64
)

CRI_TEST_PLATFORMS=(
    linux/amd64
    linux/386
    linux/arm64
    windows/amd64
    windows/386
    darwin/amd64
    darwin/arm64
)

# Create releases output directory.
PROJECT="sigs.k8s.io/cri-tools"
OUTPUT_DIR=_output
RELEASES_DIR=$OUTPUT_DIR/releases
mkdir -p "$RELEASES_DIR"

GO_LDFLAGS="-X ${PROJECT}/pkg/version.Version=${VERSION}"

# Build and package crictl releases.
for platform in "${CRI_CTL_PLATFORMS[@]}"; do
    os="${platform%/*}"
    arch=$(basename "${platform}")

    CRICTL_BIN="crictl"
    if [ "$os" == "windows" ]; then
        CRICTL_BIN="crictl.exe"
    fi

    output_bin=$OUTPUT_DIR/bin/$arch-$os/${CRICTL_BIN}
    GOARCH="$arch" GOOS="$os" CGO_ENABLED=0 go build \
        -o ${output_bin} \
        -ldflags "${GO_LDFLAGS}" \
        ${PROJECT}/cmd/crictl
    file ${output_bin}
    tar zcf "$RELEASES_DIR/crictl-$VERSION-$os-$arch.tar.gz" \
        -C $OUTPUT_DIR/bin/$arch-$os \
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

    output_bin=$OUTPUT_DIR/bin/$arch-$os/${CRITEST_BIN}
    GOARCH="$arch" GOOS="$os" CGO_ENABLED=0 go test -c \
        -o ${output_bin} \
        -ldflags "${GO_LDFLAGS}" \
        ${PROJECT}/cmd/critest
    file ${output_bin}
    tar zcf "$RELEASES_DIR/critest-$VERSION-$os-$arch.tar.gz" \
        -C $OUTPUT_DIR/bin/$arch-$os \
        ${CRITEST_BIN}
done

printf "\n## Downloads\n\n" | tee -a release-notes.md
echo "| file | sha256 | sha512" | tee -a release-notes.md
echo "| ---- | ------ | ------" | tee -a release-notes.md

# Show sha256/512 for release files
if [[ "${OSTYPE}" == "darwin"* ]]; then
    for file in "$RELEASES_DIR"/*.tar.gz; do
        SHA256=$(shasum -a 256 "$file" | sed -e "s,$file,," | awk '{print $1}' | tee "$file.sha256")
        SHA512=$(shasum -a 512 "$file" | sed -e "s,$file,," | awk '{print $1}' | tee "$file.sha512")
        BASE=$(basename "$file")
        echo "| $BASE | $SHA256 | $SHA512 |" | tee -a release-notes.md
    done
else
    for file in "$RELEASES_DIR"/*.tar.gz; do
        SHA256=$(sha256sum -b "$file" | sed -e "s,$file,," | awk '{print $1}' | tee "$file.sha256")
        SHA512=$(sha512sum -b "$file" | sed -e "s,$file,," | awk '{print $1}' | tee "$file.sha512")
        BASE=$(basename "$file")
        echo "| $BASE | $SHA256 | $SHA512 |" | tee -a release-notes.md
    done
fi
