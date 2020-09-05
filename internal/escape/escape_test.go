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

package escape

import "testing"

func TestShellUnix(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{``, `''`},
		{`abc`, `abc`},
		{`abc def`, `'abc def'`},
		{`abc/def`, `abc/def`},
		{`abc.def`, `abc.def`},
		{`"abc"`, `'"abc"'`},
		{`'abc'`, `''\''abc'\'''`},
		{`abc\`, `'abc\'`},
	}
	for _, test := range tests {
		if out := shellUnix(test.in); out != test.out {
			t.Errorf("shellUnix(%q) = %s; want %s", test.in, out, test.out)
		}
	}
}

func TestShellWindows(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{``, `""`},
		{`abc`, `abc`},
		{`abc def`, `"abc def"`},
		{`abc/def`, `abc/def`},
		{`abc.def`, `abc.def`},
		{`"abc"`, `"""abc"""`},
		{`'abc'`, `"'abc'"`},
		{`abc\`, `abc\`},
	}
	for _, test := range tests {
		if out := shellWindows(test.in); out != test.out {
			t.Errorf("shellWindows(%q) = %s; want %s", test.in, out, test.out)
		}
	}
}

func TestGitConfig(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{``, `""`},
		{`abc`, `"abc"`},
		{"abc\ndef", `"abc\ndef"`},
		{`abc\def`, `"abc\\def"`},
		{`abc"def`, `"abc\"def"`},
	}
	for _, test := range tests {
		if out := GitConfig(test.in); out != test.out {
			t.Errorf("GitConfig(%q) = %s; want %s", test.in, out, test.out)
		}
	}
}
