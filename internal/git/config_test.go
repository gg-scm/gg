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

package git

import (
	"context"
	"strings"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseConfig(t *testing.T) {
	bigPrefix := strings.Repeat("spam\neggs\x00", 1024)
	tests := []struct {
		config string
		name   string
		want   string
	}{
		{"", "foo", ""},
		{"foo\x00bar\nbaz\x00", "foo", ""},
		{"foo\x00bar\nbaz\x00", "bar", "baz"},
		{"foo\nbar\x00", "foo", "bar"},
		{"foo\nbar\x00baz\nquux\x00", "foo", "bar"},
		{"foo\nbar\x00baz\nquux\x00", "baz", "quux"},
		{"foo\nbar\x00baz\nquux\x00", "salad", ""},
		{bigPrefix + "foo\nbar\x00baz\nquux\x00", "foo", "bar"},
		{bigPrefix + "foo\nbar\x00baz\nquux\x00", "baz", "quux"},
		{bigPrefix + "foo\nbar\x00baz\nquux\x00", "salad", ""},
	}
	for _, test := range tests {
		cfg, err := parseConfig([]byte(test.config))
		if err != nil {
			t.Errorf("parseConfig(%q): %v", test.config, err)
			continue
		}
		if got := cfg.Value(test.name); got != test.want {
			t.Errorf("parseConfig(%q).Value(%q) = %q; want %q", test.config, test.name, got, test.want)
		}
	}
}

func TestConfigValue(t *testing.T) {
	tests := []struct {
		config string
		name   string
	}{
		{"", "foo.bar"},
		{"[user]\n\temail = foo@example.com\n", "user.email"},
		{"[user]\n\temail = foo@example.com\n", "USER.EMAIL"},
		{"[user]\n\temail = foo@example.com\n", "foo.bar"},
		{"[user]\n\temail = foo@example.com\n\temail = bar@example.com\n", "user.email"},
		{"[foo]\n\tbar\n", "foo.bar"},
		{"[foo]\n\tbar =\n", "foo.bar"},
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
		if err := env.top.Apply(filesystem.Write(".gitconfig", test.config)); err != nil {
			t.Error(err)
			continue
		}
		cfg, err := env.g.ReadConfig(ctx)
		if err != nil {
			t.Errorf("For %q: %v", test.config, err)
			continue
		}
		want, err := env.g.Output(ctx, "config", "-z", test.name)
		if err == nil && strings.HasSuffix(want, "\x00") {
			want = want[:len(want)-1]
		} else {
			want = ""
		}
		got := cfg.Value(test.name)
		if got != want {
			t.Errorf("For %q, cfg.Value(%q) = %q; want %q", test.config, test.name, got, want)
		}
	}
}

func TestConfigBool(t *testing.T) {
	tests := []struct {
		config string
		name   string
	}{
		{"", "foo.bar"},
		{"[user]\n\temail = foo@example.com\n", "user.email"},
		{"[foo]\n\tbar\n", "foo.bar"},
		{"[foo]\n\tbar =\n", "foo.bar"},
		{"[foo]\n\tbar = true\n", "foo.bar"},
		{"[foo]\n\tbar = true\n", "FOO.BAR"},
		{"[foo]\n\tbar = TrUe\n", "foo.bar"},
		{"[foo]\n\tbar = yes\n", "foo.bar"},
		{"[foo]\n\tbar = on\n", "foo.bar"},
		{"[foo]\n\tbar = 1\n", "foo.bar"},
		{"[foo]\n\tbar = false\n", "foo.bar"},
		{"[foo]\n\tbar = FaLsE\n", "foo.bar"},
		{"[foo]\n\tbar = no\n", "foo.bar"},
		{"[foo]\n\tbar = off\n", "foo.bar"},
		{"[foo]\n\tbar = 0\n", "foo.bar"},
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
		if err := env.top.Apply(filesystem.Write(".gitconfig", test.config)); err != nil {
			t.Error(err)
			continue
		}
		cfg, err := env.g.ReadConfig(ctx)
		if err != nil {
			t.Errorf("For %q: %v", test.config, err)
			continue
		}
		got, gotErr := cfg.Bool(test.name)
		out, wantErr := env.g.Output(ctx, "config", "-z", "--bool", test.name)
		if wantErr != nil {
			if gotErr == nil {
				t.Errorf("For %q, cfg.Bool(%q) = _, <nil>; want error", test.config, test.name)
			}
			continue
		}
		if gotErr != nil {
			t.Errorf("For %q, cfg.Bool(%q): %v", test.config, test.name, gotErr)
			continue
		}
		var want bool
		switch out {
		case "true\x00":
			want = true
		case "false\x00":
			want = false
		default:
			t.Errorf("For %q, `git config --bool %s` printed unknown value %q", test.config, test.name, out)
			continue
		}
		if got != want {
			t.Errorf("For %q, cfg.Bool(%q) = %t; want %t", test.config, test.name, got, want)
		}
	}
}

func TestListRemotes(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{name: "Empty", config: ""},
		{
			name:   "OnlyHeader",
			config: "[remote \"origin\"]\n",
		},
		{
			name:   "PushDefault",
			config: "[remote]\npushDefault = myfork\n",
		},
		{
			name: "JustURL",
			config: "[remote \"origin\"]\n" +
				"url = https://example.com/foo.git\n",
		},
		{
			name: "URLAndFetch",
			config: "[remote \"origin\"]\n" +
				"url = https://example.com/foo.git\n" +
				"fetch = +refs/heads/*:refs/remotes/origin/*\n",
		},
		{
			name: "MultipleFetch",
			config: "[remote \"origin\"]\n" +
				"url = https://example.com/foo.git\n" +
				"fetch = +refs/heads/*:refs/remotes/origin/*\n" +
				"fetch = +refs/tags/*:refs/remotes/origin/tags/*\n",
		},
		{
			name: "FetchThenPushURL",
			config: "[remote \"origin\"]\n" +
				"url = https://example.com/foo.git\n" +
				"pushurl = https://example.com/bar.git\n",
		},
		{
			name: "PushThenFetchURL",
			config: "[remote \"origin\"]\n" +
				"pushurl = https://example.com/bar.git\n" +
				"url = https://example.com/foo.git\n",
		},
		{
			name: "ClearURL",
			config: "[remote \"origin\"]\n" +
				"url = https://example.com/foo.git\n" +
				"url =\n",
		},
		{
			name: "UnclearURL",
			config: "[remote \"origin\"]\n" +
				"url =\n" +
				"url = https://example.com/foo.git\n",
		},
		{
			name: "MultipleRemotes",
			config: "[remote \"origin\"]\n" +
				"url = https://example.com/foo.git\n" +
				"[remote \"myfork\"]\n" +
				"url = https://example.com/foo-fork.git\n",
		},
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
	// `git remote` requires to be in a repository.
	if err := env.g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	baseCfg, err := env.root.ReadFile(".git/config")
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := env.root.Apply(filesystem.Write(".git/config", baseCfg+test.config)); err != nil {
				t.Fatal(err)
			}
			cfg, err := env.g.ReadConfig(ctx)
			if err != nil {
				t.Fatal(err)
			}
			got := cfg.ListRemotes()
			out, err := env.g.Output(ctx, "remote")
			if err != nil {
				t.Fatal(err)
			}
			want := make(map[string]*Remote)
			for _, line := range strings.SplitAfter(out, "\n") {
				if line == "" {
					continue
				}
				name := strings.TrimSuffix(line, "\n")
				remote := &Remote{
					Name: name,
				}
				if url, err := env.g.Output(ctx, "remote", "get-url", name); err != nil {
					t.Fatal(err)
				} else {
					remote.FetchURL = strings.TrimSuffix(url, "\n")
				}
				if url, err := env.g.Output(ctx, "remote", "get-url", "--push", name); err != nil {
					t.Fatal(err)
				} else {
					remote.PushURL = strings.TrimSuffix(url, "\n")
				}
				if fetchOut, err := env.g.Output(ctx, "config", "--get-all", "remote."+name+".fetch"); err == nil {
					fetchSpecs := strings.Split(strings.TrimSuffix(fetchOut, "\n"), "\n")
					for _, spec := range fetchSpecs {
						remote.Fetch = append(remote.Fetch, FetchRefspec(spec))
					}
				}
				want[name] = remote
			}
			if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("For config:\n\n%s\n*** cfg.ListRemotes() (-want +got):\n%s", test.config, diff)
			}
		})
	}
}

func BenchmarkReadConfig(b *testing.B) {
	gitPath, err := findGit()
	if err != nil {
		b.Skip("git not found:", err)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx, gitPath)
	if err != nil {
		b.Fatal(err)
	}
	defer env.cleanup()
	const config = `[user]
	name = Anna Nonymous
	email = anna@example.com
[push]
	default = upstream
[alias]
	change = codereview change
	gofmt = codereview gofmt
	mail = codereview mail
	pending = codereview pending
	submit = codereview submit
	sync = codereview sync` + "\n"
	if err := env.top.Apply(filesystem.Write(".gitconfig", config)); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(config)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		env.g.ReadConfig(ctx)
	}
}

func BenchmarkOneConfigLine(b *testing.B) {
	gitPath, err := findGit()
	if err != nil {
		b.Skip("git not found:", err)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx, gitPath)
	if err != nil {
		b.Fatal(err)
	}
	defer env.cleanup()
	const config = `[user]
	name = Anna Nonymous
	email = anna@example.com
[push]
	default = upstream
[alias]
	change = codereview change
	gofmt = codereview gofmt
	mail = codereview mail
	pending = codereview pending
	submit = codereview submit
	sync = codereview sync` + "\n"
	if err := env.top.Apply(filesystem.Write(".gitconfig", config)); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(29) // first two lines
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		env.g.Run(ctx, "config", "user.email")
	}
}
