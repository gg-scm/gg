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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestEvalSymlinksSloppy(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "gg_evaltest")
	if err != nil {
		t.Fatal(err)
	}
	origDir := dir
	t.Cleanup(func() { os.RemoveAll(origDir) })
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	type testCase struct {
		path string
		want string
	}
	tests := []testCase{
		{path: dir, want: dir},
		{path: `C:\`, want: `C:\`},
		{path: `C:\foo.txt`, want: `C:\foo.txt`},
	}
	for _, test := range tests {
		got, err := evalSymlinksSloppy(test.path)
		if got != test.want || err != nil {
			t.Errorf("evalSymlinksSloppy(%q) = %q, %v; want %q, <nil>", test.path, got, err, test.want)
		}
	}
}