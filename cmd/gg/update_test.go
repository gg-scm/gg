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

package main

import (
	"context"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"gg-scm.io/pkg/internal/git"
)

func TestUpdate_NoArgsFastForward(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with two commits, with master behind the "upstream" branch.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Apple\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	h1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "--quiet", "-b", "upstream"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Banana\n")); err != nil {
		t.Fatal(err)
	}
	h2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "--quiet", "master"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "branch", "--quiet", "--set-upstream-to=upstream"); err != nil {
		t.Fatal(err)
	}

	// Call gg to update.
	_, err = env.gg(ctx, env.root.String(), "update")
	if err != nil {
		t.Error(err)
	}

	// Verify that HEAD moved to the second commit.
	if r, err := env.git.Head(ctx); err != nil {
		t.Fatal(err)
	} else if r.Commit() != h2 {
		names := map[git.Hash]string{
			h1: "first commit",
			h2: "second commit",
		}
		t.Errorf("after update, HEAD = %s; want %s",
			prettyCommit(r.Commit(), names),
			prettyCommit(h2, names))
	}

	// Verify that foo.txt has the second commit's content.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "Banana\n"; got != want {
		t.Errorf("foo.txt = %q; want %q", got, want)
	}
}

func TestUpdate_SwitchBranch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Start a repository with an arbitrary master branch.
	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}
	initRev, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Create a commit on another branch.
	if err := env.git.Run(ctx, "checkout", "--quiet", "-b", "foo"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	h2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Check out master branch.
	if err := env.git.Run(ctx, "checkout", "--quiet", "master"); err != nil {
		t.Fatal(err)
	}

	// Call gg to switch to foo branch.
	_, err = env.gg(ctx, env.root.String(), "update", "foo")
	if err != nil {
		t.Error(err)
	}

	// Verify that HEAD was moved to foo branch.
	if r, err := env.git.Head(ctx); err != nil {
		t.Fatal(err)
	} else {
		if r.Commit() != h2 {
			names := map[git.Hash]string{
				initRev.Commit(): "first commit",
				h2:               "second commit",
			}
			t.Errorf("after update foo, HEAD = %s; want %s",
				prettyCommit(r.Commit(), names),
				prettyCommit(h2, names))
		}
		if got, want := r.Ref(), git.BranchRef("foo"); got != want {
			t.Errorf("after update foo, HEAD ref = %s; want %s", got, want)
		}
	}

	// Verify that foo.txt has the branch commit's content.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := dummyContent; got != want {
		t.Errorf("foo.txt = %q; want %q", got, want)
	}
}

func TestUpdate_ToCommit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with two commits.
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Apple\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	h1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Banana\n")); err != nil {
		t.Fatal(err)
	}
	h2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to update to the first commit.
	_, err = env.gg(ctx, env.root.String(), "update", h1.String())
	if err != nil {
		t.Error(err)
	}

	// Verify that HEAD is the first commit.
	if r, err := env.git.Head(ctx); err != nil {
		t.Fatal(err)
	} else {
		if r.Commit() != h1 {
			names := map[git.Hash]string{
				h1: "first commit",
				h2: "second commit",
			}
			t.Errorf("after update master, HEAD = %s; want %s",
				prettyCommit(r.Commit(), names),
				prettyCommit(h1, names))
		}
		if got := r.Ref(); got != git.Head {
			t.Errorf("after update master, HEAD ref = %s; want %s", got, git.Head)
		}
	}

	// Verify that foo.txt has the first commit's content.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "Apple\n"; got != want {
		t.Errorf("foo.txt = %q; want %q", got, want)
	}
}
