// Copyright 2020 The gg Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os/exec"
	"path/filepath"
)

func bashCommand(gitExe, line string) (*exec.Cmd, error) {
	// Git for Windows comes with an MSYS2 bash emulation that Git uses to invoke
	// shell commands. To preserve the semantics of the Git editor line, we use
	// the same shell.
	//
	// In the default install location, git.exe lives at C:\Program Files\Git\cmd\git.exe.
	// bash.exe lives at C:\Program Files\Git\bin\bash.exe and is not on the PATH.
	bash := filepath.Join(gitExe, "..", "..", "bin", "bash.exe")
	if _, err := exec.LookPath(bash); err != nil {
		return nil, err
	}
	return exec.Command(bash, "-c", line), nil
}
