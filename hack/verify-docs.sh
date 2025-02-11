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

if ! command -v go-md2man >/dev/null; then
    echo "Installing go-md2man"
    HACK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
    MD2MAN=github.com/cpuguy83/go-md2man/v2
    go install -C "${HACK_DIR}/tools" "$MD2MAN@$(go list -m -f \{\{.Version\}\} $MD2MAN)"
fi

go-md2man -in docs/crictl.md -out docs/crictl.1

STATUS=$(git status --porcelain)
if [[ -z $STATUS ]]; then
    echo "tree is clean"
else
    echo "tree is dirty, please commit all changes"
    echo ""
    echo "$STATUS"
    exit 1
fi
