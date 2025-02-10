#!/usr/bin/env bash

# Copyright 2023 The Kubernetes Authors.
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

HACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
GO_MD2MAN=go-md2man

if ! command -v ${GO_MD2MAN} >/dev/null; then
    echo "Installing go-md2man"
    GOBIN=${HACK_DIR} go install github.com/cpuguy83/${GO_MD2MAN}/v2@latest
    GO_MD2MAN=${HACK_DIR}/${GO_MD2MAN}
fi

${GO_MD2MAN} -in docs/crictl.md -out docs/crictl.1

STATUS=$(git status --porcelain)
if [[ -z $STATUS ]]; then
    echo "tree is clean"
else
    echo "tree is dirty, please commit all changes"
    echo ""
    echo "$STATUS"
    exit 1
fi
