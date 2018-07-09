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

if [[ $# -ne 2 ]]; then
  echo "usage: add-hugo-analytics.bash FILE ID" 1>&2
  exit 64
fi
temp="${1}.new"
echo "googleAnalytics = \"$2\"" > "$temp" || exit 1
sed -e '/^googleAnalytics\>/d' "$1" >> "$temp" || exit 1
mv "${1}.new" "$1" || exit 1
