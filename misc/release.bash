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

parse_tag_ref() {
  python -c 'import re, sys; x = sys.stdin.readline().strip(); x = x[x.rindex("/")+1:] if x.rfind("/") != -1 else x; print(x.lstrip("v") if re.match(r"v[0-9]", x) else "")'
}

if [[ $# -gt 1 ]]; then
  echo "usage: misc/release.bash [VERSION]" 1>&2
  exit 64
fi
srcroot="$(dirname "$(dirname "${BASH_SOURCE[0]}")")"
release_version="${1:-$(echo "${GITHUB_REF:-}" | parse_tag_ref)}"
if [[ -z "$release_version" ]]; then
  echo "misc/release.bash: cannot infer version, please pass explicitly" 1>&2
  exit 1
fi
release_os="$(go env GOOS)"
release_arch="$(go env GOARCH)"
if [[ "$release_version" == "dev" ]]; then
  if [[ -z "${GITHUB_SHA:-}" ]]; then
    echo "misc/release.bash: must set GITHUB_SHA for dev" 1>&2
    exit 1
  fi
  release_name="gg_${GITHUB_SHA:-}_${release_os}_${release_arch}"
else
  release_name="gg_${release_version}_${release_os}_${release_arch}"
fi

echo "Creating ${release_name}.tar.gz..." 1>&2
stagedir="$(mktemp -d 2>/dev/null || mktemp -d -t gg_release)"
trap 'rm -rf $stagedir' EXIT
distroot="$stagedir/$release_name"
mkdir "$distroot"
cp "$srcroot/README.md" "$srcroot/CHANGELOG.md" "$srcroot/LICENSE" "$distroot/"
mkdir "$distroot/misc"
cp "$srcroot/misc/completion.zsh" "$srcroot/misc/_gg_complete.bash" "$distroot/misc/"
if [[ "$release_version" == "dev" ]]; then
  "$srcroot/misc/build.bash" "$distroot/gg"
else
  "$srcroot/misc/build.bash" "$distroot/gg" "$release_version"
fi
tar -zcf - -C "$stagedir" "$release_name" > "${release_name}.tar.gz"
echo "::set-output name=file::${release_name}.tar.gz"
