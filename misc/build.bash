#!/bin/bash

# Copyright 2018 Google LLC
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

set -o pipefail

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
buildtime="$(date -u '+%Y-%m-%dT%TZ')" || exit 1
cd "$(dirname "$(dirname "${BASH_SOURCE[0]}")")" || exit 1
commit="${TRAVIS_COMMIT:-$(commitinfo)}" || exit 1
version="${2:-$(echo "$TRAVIS_TAG" | sed -n -e 's/v\([0-9].*\)/\1/p')}" || exit 1
vgo build -o "$1" -ldflags="-X main.versionInfo=${version} -X main.buildCommit=${commit} -X main.buildTime=${buildtime}" zombiezen.com/go/gg/cmd/gg
