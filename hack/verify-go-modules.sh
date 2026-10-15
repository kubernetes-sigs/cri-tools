#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
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

export GO111MODULE=on

# Root of the repository
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if ! go mod tidy -diff; then
    echo "go.mod or go.sum is not tidy, run 'make vendor' and commit the result" >&2
    exit 1
fi

go mod vendor -o "$TMP_DIR"
if ! diff -rq vendor "$TMP_DIR"; then
    echo "vendor is out of date, run 'make vendor' and commit the result" >&2
    exit 1
fi

ignored="$(cd "$TMP_DIR" && find . -type f | sed 's|^\./|vendor/|' | git -C "$REPO_ROOT" check-ignore --no-index --stdin || true)"
if [[ -n $ignored ]]; then
    echo "vendored files are gitignored and would not be committed:" >&2
    echo "$ignored" >&2
    exit 1
fi

go mod verify

# Test if we can resolve all go modules
go list -mod=readonly -m all
