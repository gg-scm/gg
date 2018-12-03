// Copyright 2018 Google LLC
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

//+build darwin dragonfly freebsd linux netbsd openbsd plan9 solaris

package git

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestConfigColor(t *testing.T) {
	tests := []struct {
		color string
		want  string
		err   bool
	}{
		// Outputs checked against git 2.17.0
		//
		// We don't test directly against git config --get-color because
		// older versions don't support all the attributes.
		//
		// To get golden output:
		//    git -c foo.bar=INPUT config --get-color foo.bar

		{color: "", want: ""},
		{color: "normal", want: ""},
		{color: "black", want: "\x1b[30m"},
		{color: "red", want: "\x1b[31m"},
		{color: "green", want: "\x1b[32m"},
		{color: "yellow", want: "\x1b[33m"},
		{color: "blue", want: "\x1b[34m"},
		{color: "magenta", want: "\x1b[35m"},
		{color: "cyan", want: "\x1b[36m"},
		{color: "white", want: "\x1b[37m"},
		{color: "normal red", want: "\x1b[41m"},
		{color: "normal blue", want: "\x1b[44m"},
		{color: "normal normal", want: ""},
		{color: "normal black", want: "\x1b[40m"},
		{color: "normal red", want: "\x1b[41m"},
		{color: "normal green", want: "\x1b[42m"},
		{color: "normal yellow", want: "\x1b[43m"},
		{color: "normal blue", want: "\x1b[44m"},
		{color: "normal magenta", want: "\x1b[45m"},
		{color: "normal cyan", want: "\x1b[46m"},
		{color: "normal white", want: "\x1b[47m"},
		{color: "0", want: "\x1b[30m"},
		{color: "7", want: "\x1b[37m"},
		{color: "8", want: "\x1b[38;5;8m"},
		{color: "15", want: "\x1b[38;5;15m"},
		{color: "16", want: "\x1b[38;5;16m"},
		{color: "128", want: "\x1b[38;5;128m"},
		{color: "231", want: "\x1b[38;5;231m"},
		{color: "232", want: "\x1b[38;5;232m"},
		{color: "255", want: "\x1b[38;5;255m"},
		{color: "#ff0ab3", want: "\x1b[38;2;255;10;179m"},
		{color: "bold", want: "\x1b[1m"},
		{color: "dim", want: "\x1b[2m"},
		{color: "ul", want: "\x1b[4m"},
		{color: "blink", want: "\x1b[5m"},
		{color: "reverse", want: "\x1b[7m"},
		{color: "italic", want: "\x1b[3m"},
		{color: "strike", want: "\x1b[9m"},
		{color: "nobold", want: "\x1b[22m"},
		{color: "nodim", want: "\x1b[22m"},
		{color: "noul", want: "\x1b[24m"},
		{color: "noblink", want: "\x1b[25m"},
		{color: "noreverse", want: "\x1b[27m"},
		{color: "noitalic", want: "\x1b[23m"},
		{color: "nostrike", want: "\x1b[29m"},
		{color: "no-bold", want: "\x1b[22m"},
		{color: "no-dim", want: "\x1b[22m"},
		{color: "no-ul", want: "\x1b[24m"},
		{color: "no-blink", want: "\x1b[25m"},
		{color: "no-reverse", want: "\x1b[27m"},
		{color: "no-italic", want: "\x1b[23m"},
		{color: "no-strike", want: "\x1b[29m"},
		{color: "red blue bold", want: "\x1b[1;31;44m"},
		{color: "red green blue", err: true},
		{color: "bold red blue", want: "\x1b[1;31;44m"},
		{color: "red bold blue", want: "\x1b[1;31;44m"},
	}
	for _, test := range tests {
		got, err := parseColorDesc(test.color)
		if err != nil {
			if !test.err {
				t.Errorf("parseColorDesc(%q): %v", test.color, err)
			}
			continue
		}
		if test.err {
			t.Errorf("parseColorDesc(%q) = %q, <nil>; want error", test.color, got)
			continue
		}
		if !bytes.Equal(got, []byte(test.want)) {
			t.Errorf("parseColorDesc(%q) = %q; want %q", test.color, got, test.want)
		}
	}
}

func TestConfigColorBool(t *testing.T) {
	tests := []struct {
		config string
		name   string
	}{
		// Empty config
		{"", "color.diff"},
		{"", "color.xyzzy"},
		{"", "color.ui"},

		// Direct checks
		{"[color]\nxyzzy\n", "color.xyzzy"},
		{"[color]\nxyzzy = false\n", "color.xyzzy"},
		{"[color]\nxyzzy = true\n", "color.xyzzy"},
		{"[color]\nxyzzy = auto\n", "color.xyzzy"},
		{"[color]\nxyzzy = always\n", "color.xyzzy"},
		{"[color]\nxyzzy = never\n", "color.xyzzy"},
		{"[color]\nxyzzy = borkbork\n", "color.xyzzy"},

		// Direct value not set, but ui is.
		{"[color]\nui = false\n", "color.xyzzy"},
		{"[color]\nui = true\n", "color.xyzzy"},
		{"[color]\nui = auto\n", "color.xyzzy"},
		{"[color]\nui = always\n", "color.xyzzy"},
		{"[color]\nui = never\n", "color.xyzzy"},

		// Check color.ui
		{"[color]\nui = borkbork\n", "color.ui"},
		{"[color]\nui = false\n", "color.ui"},
		{"[color]\nui = true\n", "color.ui"},
		{"[color]\nui = auto\n", "color.ui"},
		{"[color]\nui = always\n", "color.ui"},
		{"[color]\nui = never\n", "color.ui"},

		// Both color.ui and direct value set.
		{"[color]\nui = true\nxyzzy = false\n", "color.xyzzy"},
		{"[color]\nui = true\nxyzzy = true\n", "color.xyzzy"},
		{"[color]\nui = true\nxyzzy = auto\n", "color.xyzzy"},
		{"[color]\nui = true\nxyzzy = always\n", "color.xyzzy"},
		{"[color]\nui = true\nxyzzy = never\n", "color.xyzzy"},
		{"[color]\nui = false\nxyzzy = false\n", "color.xyzzy"},
		{"[color]\nui = false\nxyzzy = true\n", "color.xyzzy"},
		{"[color]\nui = false\nxyzzy = auto\n", "color.xyzzy"},
		{"[color]\nui = false\nxyzzy = always\n", "color.xyzzy"},
		{"[color]\nui = false\nxyzzy = never\n", "color.xyzzy"},
		{"[color]\nui = auto\nxyzzy = false\n", "color.xyzzy"},
		{"[color]\nui = auto\nxyzzy = true\n", "color.xyzzy"},
		{"[color]\nui = auto\nxyzzy = auto\n", "color.xyzzy"},
		{"[color]\nui = auto\nxyzzy = always\n", "color.xyzzy"},
		{"[color]\nui = auto\nxyzzy = never\n", "color.xyzzy"},
		{"[color]\nui = always\nxyzzy = false\n", "color.xyzzy"},
		{"[color]\nui = always\nxyzzy = true\n", "color.xyzzy"},
		{"[color]\nui = always\nxyzzy = auto\n", "color.xyzzy"},
		{"[color]\nui = always\nxyzzy = always\n", "color.xyzzy"},
		{"[color]\nui = always\nxyzzy = never\n", "color.xyzzy"},
		{"[color]\nui = never\nxyzzy = false\n", "color.xyzzy"},
		{"[color]\nui = never\nxyzzy = true\n", "color.xyzzy"},
		{"[color]\nui = never\nxyzzy = auto\n", "color.xyzzy"},
		{"[color]\nui = never\nxyzzy = always\n", "color.xyzzy"},
		{"[color]\nui = never\nxyzzy = never\n", "color.xyzzy"},

		// Aliasing for color.diff and diff.color.
		{"[color]\ndiff = false\n", "diff.color"},
		{"[color]\ndiff = true\n", "diff.color"},
		{"[color]\ndiff = auto\n", "diff.color"},
		{"[color]\ndiff = always\n", "diff.color"},
		{"[color]\ndiff = never\n", "diff.color"},
		{"[diff]\ncolor = false\n", "color.diff"},
		{"[diff]\ncolor = true\n", "color.diff"},
		{"[diff]\ncolor = auto\n", "color.diff"},
		{"[diff]\ncolor = always\n", "color.diff"},
		{"[diff]\ncolor = never\n", "color.diff"},
	}
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx, gitPath)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	for _, test := range tests {
		err := ioutil.WriteFile(
			filepath.Join(env.root, ".gitconfig"),
			[]byte(test.config),
			0666)
		if err != nil {
			t.Error(err)
			continue
		}
		cfg, err := ReadConfig(ctx, env.g)
		if err != nil {
			t.Errorf("For %q: %v", test.config, err)
			continue
		}
		for _, isTerm := range []bool{false, true} {
			got, gotErr := cfg.ColorBool(test.name, isTerm)
			out, wantErr := env.g.RunOneLiner(ctx, '\n', "config", "--get-colorbool", test.name, fmt.Sprint(isTerm))
			if wantErr != nil {
				if gotErr == nil {
					t.Errorf("For %q, cfg.ColorBool(%q, %t) = _, <nil>; want error", test.config, test.name, isTerm)
				}
				continue
			}
			if gotErr != nil {
				t.Errorf("For %q, cfg.Bool(%q): %v", test.config, test.name, gotErr)
				continue
			}
			switch string(out) {
			case "true":
				if !got {
					t.Errorf("For %q, cfg.ColorBool(%q, %t) = false; want true", test.config, test.name, isTerm)
				}
			case "false":
				if got {
					t.Errorf("For %q, cfg.ColorBool(%q, %t) = true; want false", test.config, test.name, isTerm)
				}
			default:
				t.Errorf("For %q, `git config --get-colorbool %s %t` printed unknown value %q", test.config, test.name, isTerm, out)
			}
		}
	}
}
