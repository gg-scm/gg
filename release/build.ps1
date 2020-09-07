# Copyright 2020 The gg Authors
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

param($out,$version='')

$buildTime = (Get-Date).ToUniversalTime() | Get-Date -UFormat '%Y-%m-%dT%TZ'
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o $out -ldflags="-X main.versionInfo=$version -X main.buildCommit=$env:GITHUB_SHA -X main.buildTime=$buildTime" gg-scm.io/tool/cmd/gg
