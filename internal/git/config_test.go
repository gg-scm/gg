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
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
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
		cfg, err := parseConfig(strings.NewReader(test.config))
		if err != nil {
			t.Errorf("parseConfig(%q): %v", test.config, err)
			continue
		}
		if got := cfg.Value(test.name); got != test.want {
			t.Errorf("parseConfig(%q).Value(%q) = %q; want %q", test.config, test.name, got, test.want)
		}
	}
	t.Run("DataErr", func(t *testing.T) {
		for _, test := range tests {
			cfg, err := parseConfig(iotest.DataErrReader(strings.NewReader(test.config)))
			if err != nil {
				t.Errorf("parseConfig(%q): %v", test.config, err)
				continue
			}
			if got := cfg.Value(test.name); got != test.want {
				t.Errorf("parseConfig(%q).Value(%q) = %q; want %q", test.config, test.name, got, test.want)
			}
		}
	})
	t.Run("Half", func(t *testing.T) {
		for _, test := range tests {
			cfg, err := parseConfig(iotest.HalfReader(strings.NewReader(test.config)))
			if err != nil {
				t.Errorf("parseConfig(%q): %v", test.config, err)
				continue
			}
			if got := cfg.Value(test.name); got != test.want {
				t.Errorf("parseConfig(%q).Value(%q) = %q; want %q", test.config, test.name, got, test.want)
			}
		}
	})
	t.Run("OneByte", func(t *testing.T) {
		for _, test := range tests {
			cfg, err := parseConfig(iotest.OneByteReader(strings.NewReader(test.config)))
			if err != nil {
				t.Errorf("parseConfig(%q): %v", test.config, err)
				continue
			}
			if got := cfg.Value(test.name); got != test.want {
				t.Errorf("parseConfig(%q).Value(%q) = %q; want %q", test.config, test.name, got, test.want)
			}
		}
	})
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
		cfg, err := env.g.ReadConfig(ctx)
		if err != nil {
			t.Errorf("For %q: %v", test.config, err)
			continue
		}
		want, err := env.g.RunOneLiner(ctx, 0, "config", "-z", test.name)
		if err != nil {
			want = nil
		}
		got := cfg.Value(test.name)
		if got != string(want) {
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
		cfg, err := env.g.ReadConfig(ctx)
		if err != nil {
			t.Errorf("For %q: %v", test.config, err)
			continue
		}
		got, gotErr := cfg.Bool(test.name)
		out, wantErr := env.g.RunOneLiner(ctx, 0, "config", "-z", "--bool", test.name)
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
		switch string(out) {
		case "true":
			want = true
		case "false":
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

func BenchmarkReadConfig(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping due to -short")
	}
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
	err = ioutil.WriteFile(filepath.Join(env.root, ".gitconfig"), []byte(config), 0666)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(config)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		env.g.ReadConfig(ctx)
	}
}

func BenchmarkOneConfigLine(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping due to -short")
	}
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
	err = ioutil.WriteFile(filepath.Join(env.root, ".gitconfig"), []byte(config), 0666)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(29) // first two lines
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		env.g.RunOneLiner(ctx, '\n', "config", "user.email")
	}
}
