// Copyright 2018 The gg Authors
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

package git

import (
	"os/exec"
	"testing"

	"golang.org/x/xerrors"
)

func TestCommandError(t *testing.T) {
	falsePath, err := exec.LookPath("false")
	if err != nil {
		t.Skip("could not find false:", err)
	}
	exitError := exec.Command(falsePath).Run()
	if _, ok := exitError.(*exec.ExitError); !ok {
		t.Fatalf("ran %s, got: %v; want exit error", falsePath, exitError)
	}

	tests := []struct {
		prefix   string
		runError error
		stderr   string

		want string
	}{
		{
			prefix:   "git commit",
			runError: xerrors.New("could not start because reasons"),
			want:     "git commit: could not start because reasons",
		},
		{
			prefix:   "git commit",
			runError: exitError,
			want:     "git commit: " + exitError.Error(),
		},
		{
			prefix:   "git commit",
			runError: xerrors.New("could not copy I/O"),
			stderr:   "fatal: everything failed\n",
			want:     "git commit: could not copy I/O\nfatal: everything failed",
		},
		{
			prefix:   "git commit",
			runError: xerrors.New("could not copy I/O"),
			stderr:   "fatal: everything failed", // no trailing newline
			want:     "git commit: could not copy I/O\nfatal: everything failed",
		},
		{
			prefix:   "git commit",
			runError: exitError,
			stderr:   "fatal: everything failed\n",
			want:     "git commit: fatal: everything failed",
		},
		{
			prefix:   "git commit",
			runError: xerrors.New("could not copy I/O"),
			stderr:   "fatal: everything failed\nThis is the work of Voldemort.\n",
			want:     "git commit: could not copy I/O\nfatal: everything failed\nThis is the work of Voldemort.",
		},
		{
			prefix:   "git commit",
			runError: xerrors.New("could not copy I/O"),
			stderr:   "fatal: everything failed\nThis is the work of Voldemort.", // no trailing newline
			want:     "git commit: could not copy I/O\nfatal: everything failed\nThis is the work of Voldemort.",
		},
		{
			prefix:   "git commit",
			runError: exitError,
			stderr:   "fatal: everything failed\nThis is the work of Voldemort.\n",
			want:     "git commit:\nfatal: everything failed\nThis is the work of Voldemort.",
		},
	}
	for _, test := range tests {
		e := commandError(test.prefix, test.runError, []byte(test.stderr))
		if got := e.Error(); got != test.want {
			t.Errorf("commandError(%q, %v, %q) = %q; want %q", test.prefix, test.runError, test.stderr, got, test.want)
		}
	}
}
