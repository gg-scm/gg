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

set -o pipefail
mkdir -p "$1" || exit 1
cd "$1" || exit 1
curl -fsSL https://www.kernel.org/pub/software/scm/git/git-"$2".tar.xz | xz -d | tar xf - --strip-components=1 || exit 1
NO_GETTEXT=1 make -j2 all || exit 1
