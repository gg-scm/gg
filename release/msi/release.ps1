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

param($version='')

If ( ! ( Test-Path Env:wix ) ) {
  Write-Error 'WiX not installed; cannot find %wix%.'
  exit 1
}

$filename="gg"
If ( $version -ne '' ) {
  $filename+="_${version}"
}
$filename+=".msi"

$wixVersion="0.0.0"
$wixVersionMatch=[regex]::Match($version, '^v([0-9]+\.[0-9]+\.[0-9]+)')
If ( $wixVersionMatch.success ) {
  $wixVersion=$wixVersionMatch.captures.groups[1].value
} Elseif ( $version -ne '' ) {
  Write-Error "Invalid version $version"
  exit 1
}

& "${env:wix}bin\\candle.exe" `
  -nologo `
  -arch x64 `
  "-dGgVersion=$version" `
  "-dWixGgVersion=$wixVersion" `
  gg.wxs
If ( $LastExitCode -ne 0 ) {
  exit $LastExitCode
}
& "${env:wix}bin\\light.exe" `
  -nologo `
  -dcl:high `
  -ext WixUIExtension `
  -ext WixUtilExtension `
  gg.wixobj `
  -o $filename
If ( $LastExitCode -ne 0 ) {
  exit $LastExitCode
}
Write-Output "::set-output name=file::${filename}"
