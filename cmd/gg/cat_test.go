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

package main

import (
	"context"
	"path/filepath"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestCat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	err = env.root.Apply(
		filesystem.Write("foo.txt", "foo 1\n"),
		filesystem.Write("bar.txt", "bar 1\n"),
		filesystem.Mkdir("baz"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	err = env.root.Apply(
		filesystem.Write("foo.txt", "foo 2\n"),
		filesystem.Write("bar.txt", "bar 2\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	err = env.root.Apply(
		filesystem.Write("foo.txt", "dirty foo\n"),
		filesystem.Write("bar.txt", "dirty bar\n"),
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		dir  string
		args []string
		out  string
	}{
		{
			name: "NoRevFlag",
			args: []string{"foo.txt"},
			out:  "foo 2\n",
		},
		{
			name: "RevFlag",
			args: []string{"-r", "HEAD~", "foo.txt"},
			out:  "foo 1\n",
		},
		{
			name: "MultipleFiles",
			args: []string{"foo.txt", "bar.txt"},
			out:  "foo 2\nbar 2\n",
		},
		{
			name: "MultipleFilesRevFlag",
			args: []string{"-r", "HEAD~", "foo.txt", "bar.txt"},
			out:  "foo 1\nbar 1\n",
		},
		{
			name: "InSubdir",
			dir:  "baz",
			args: []string{filepath.Join("..", "foo.txt")},
			out:  "foo 2\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"cat"}, test.args...)
			out, err := env.gg(ctx, filepath.Join(env.root.String(), test.dir), args...)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.out {
				t.Errorf("output = %q; want %q", out, test.out)
			}
		})
	}
}
