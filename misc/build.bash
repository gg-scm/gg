#!/bin/bash

# Copyright 2018 The gg Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

commitinfo() {
  git rev-parse HEAD | tr -d '\n' || return 1
  if [[ "$(git status --porcelain | wc -l)" -gt 0 ]]; then
    echo "+"
  else
    echo
  fi
}

if [[ $# -ne 1 && $# -ne 2 ]]; then
  echo "usage: misc/build.bash OUT [VERSION]" 1>&2
  exit 64
fi
buildtime="$(date -u '+%Y-%m-%dT%TZ')"
cd "$(dirname "$(dirname "${BASH_SOURCE[0]}")")"
commit="${GITHUB_SHA:-$(commitinfo)}"
version="${2:-}"
go build -o "$1" -ldflags="-X main.versionInfo=${version} -X main.buildCommit=${commit} -X main.buildTime=${buildtime}" gg-scm.io/tool/cmd/gg
