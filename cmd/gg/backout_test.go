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
	"testing"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/filesystem"
)

func TestBackout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Hello, World!\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	c1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Hello, World!\nI had a thought...\n")); err != nil {
		t.Fatal(err)
	}
	c2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "backout", "--edit=0", "HEAD"); err != nil {
		t.Error(err)
	}
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "Hello, World!\n"; got != want {
		t.Errorf("After backout, content = %q; want %q", got, want)
	}
	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[git.Hash]string{
		c1: "commit 1",
		c2: "commit 2",
	}
	if got := curr.Commit; got == c1 || got == c2 {
		t.Errorf("After backout, HEAD = %s; want new commit", prettyCommit(got, names))
	}

	parent, err := env.git.ParseRev(ctx, "HEAD~")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := parent.Commit, c2; got != want {
		t.Errorf("After backout, HEAD~ = %s; want %s", prettyCommit(got, names), prettyCommit(want, names))
	}
}

func TestBackout_NoCommit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Hello, World!\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	c1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Hello, World!\nI had a thought...\n")); err != nil {
		t.Fatal(err)
	}
	c2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "backout", "--no-commit", "--edit=0", "HEAD"); err != nil {
		t.Error(err)
	}
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "Hello, World!\n"; got != want {
		t.Errorf("After backout, content = %q; want %q", got, want)
	}
	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[git.Hash]string{
		c1: "commit 1",
		c2: "commit 2",
	}
	if got, want := curr.Commit, c2; got != want {
		t.Errorf("After backout, HEAD = %s; want %s", prettyCommit(got, names), prettyCommit(want, names))
	}
}
