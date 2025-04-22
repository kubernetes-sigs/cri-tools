#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
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

BUILD_DIR=build
BINARY=$BUILD_DIR/release-notes
VERSION=v0.18.0

mkdir -p $BUILD_DIR
curl -sSfL --retry 5 --retry-delay 10 -o $BINARY \
    https://github.com/kubernetes/release/releases/download/$VERSION/release-notes-amd64-linux
chmod +x $BINARY

PREVIOUS_TAG=$(git tag | tail -2 | head -1)
LATEST_TAG=$(git tag | tail -1)

$BINARY \
    --org kubernetes-sigs \
    --repo cri-tools \
    --required-author "" \
    --branch master \
    --start-rev "$PREVIOUS_TAG" \
    --end-rev "$LATEST_TAG" \
    --output release-notes.md
