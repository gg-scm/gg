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

package gittool

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestConfigColor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
	tests := []string{
		"",
		"normal",
		"black",
		"red",
		"green",
		"yellow",
		"blue",
		"magenta",
		"cyan",
		"white",
		"normal red",
		"normal blue",
		"normal normal",
		"normal black",
		"normal red",
		"normal green",
		"normal yellow",
		"normal blue",
		"normal magenta",
		"normal cyan",
		"normal white",
		"0",
		"7",
		"8",
		"15",
		"16",
		"128",
		"231",
		"232",
		"255",
		"\"#ff0ab3\"",
		"bold",
		"dim",
		"ul",
		"blink",
		"reverse",
		"italic",
		"strike",
		"nobold",
		"nodim",
		"noul",
		"noblink",
		"noreverse",
		"noitalic",
		"nostrike",
		"no-bold",
		"no-dim",
		"no-ul",
		"no-blink",
		"no-reverse",
		"no-italic",
		"no-strike",
		"red blue bold",
		"red green blue",
		"bold red blue",
		"red bold blue",
	}
	if gitPathError != nil {
		t.Skip("git not found:", gitPathError)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	for _, test := range tests {
		err := ioutil.WriteFile(
			filepath.Join(env.root, ".gitconfig"),
			[]byte("[color \"foo\"]\n\tbar = "+test+"\n"),
			0666)
		if err != nil {
			t.Error(err)
			continue
		}
		cfg, err := ReadConfig(ctx, env.git)
		if err != nil {
			t.Errorf("For %q: %v", test, err)
			continue
		}
		got, gotErr := cfg.Color("color.foo.bar", "")
		p, err := env.git.Start(ctx, "config", "--get-color", "color.foo.bar")
		if err != nil {
			t.Errorf("For %q: %v", test, err)
			continue
		}
		want, _ := ioutil.ReadAll(p)
		waitErr := p.Wait()
		if waitErr != nil {
			if gotErr == nil {
				t.Errorf("For %q, cfg.Color(...) = _, <nil>; want error", test)
			}
			continue
		}
		if gotErr != nil {
			t.Errorf("For %q, cfg.Color(...) error: %v; want %q", test, gotErr, want)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("For %q, cfg.Color(...) = %q; want %q", test, got, want)
		}
	}
}

func TestConfigColorBool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
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
	if gitPathError != nil {
		t.Skip("git not found:", gitPathError)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx)
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
		cfg, err := ReadConfig(ctx, env.git)
		if err != nil {
			t.Errorf("For %q: %v", test.config, err)
			continue
		}
		for _, isTerm := range []bool{false, true} {
			got, gotErr := cfg.ColorBool(test.name, isTerm)
			out, wantErr := env.git.RunOneLiner(ctx, '\n', "config", "--get-colorbool", test.name, fmt.Sprint(isTerm))
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
