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

func TestUpdate_NoArgs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("UpstreamSameName", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository A and clone it to repository B.
		if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
			t.Fatal(err)
		}
		rev1, err := env.git.WithDir("repoA").Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.Run(ctx, "clone", "repoA", "repoB"); err != nil {
			t.Fatal(err)
		}

		// Create a new commit in repository A.
		if err := env.root.Apply(filesystem.Write("repoA/foo.txt", "Apple\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
			t.Fatal(err)
		}
		h2, err := env.newCommit(ctx, "repoA")
		if err != nil {
			t.Fatal(err)
		}

		// Run `git fetch origin` in repository B to update remote
		// tracking branches.
		repoBPath := env.root.FromSlash("repoB")
		gitB := env.git.WithDir(repoBPath)
		if err := gitB.Run(ctx, "fetch", "origin"); err != nil {
			t.Fatal(err)
		}

		// Call gg to update master in repository B.
		_, err = env.gg(ctx, repoBPath, "update")
		if err != nil {
			t.Error(err)
		}

		// Verify that HEAD moved to the second commit.
		if r, err := gitB.Head(ctx); err != nil {
			t.Fatal(err)
		} else if r.Commit != h2 {
			names := map[git.Hash]string{
				rev1.Commit: "first commit",
				h2:          "second commit",
			}
			t.Errorf("after update, HEAD = %s; want %s",
				prettyCommit(r.Commit, names),
				prettyCommit(h2, names))
		}

		// Verify that foo.txt has the second commit's content.
		if got, err := env.root.ReadFile("repoB/foo.txt"); err != nil {
			t.Error(err)
		} else if want := "Apple\n"; got != want {
			t.Errorf("foo.txt = %q; want %q", got, want)
		}
	})
	t.Run("UpstreamAndPushDiverge", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository A and clone it to repository B.
		if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
			t.Fatal(err)
		}
		base, err := env.git.WithDir("repoA").Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.Run(ctx, "clone", "repoA", "repoB"); err != nil {
			t.Fatal(err)
		}

		// Create a new commit in repository A on master.
		if err := env.root.Apply(filesystem.Write("repoA/foo.txt", "Apple\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
			t.Fatal(err)
		}
		master2, err := env.newCommit(ctx, "repoA")
		if err != nil {
			t.Fatal(err)
		}

		// Create a new commit in repository A on a new branch called "feature".
		if err := env.git.WithDir("repoA").NewBranch(ctx, "feature", git.BranchOptions{Checkout: true, StartPoint: "HEAD~"}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("repoA/foo.txt", "Banana\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
			t.Fatal(err)
		}
		feature2, err := env.newCommit(ctx, "repoA")
		if err != nil {
			t.Fatal(err)
		}

		// Create a new branch in repository B called "feature"
		// that tracks "refs/remotes/origin/master".
		repoBPath := env.root.FromSlash("repoB")
		gitB := env.git.WithDir(repoBPath)
		err = gitB.NewBranch(ctx, "feature", git.BranchOptions{
			StartPoint: "refs/remotes/origin/master",
			Checkout:   true,
			Track:      true,
		})
		if err != nil {
			t.Fatal(err)
		}

		// Run `git fetch origin` in repository B to update remote
		// tracking branches.
		if err := gitB.Run(ctx, "fetch", "origin"); err != nil {
			t.Fatal(err)
		}

		// Call gg to update "feature" in repository B.
		_, err = env.gg(ctx, repoBPath, "update")
		if err != nil {
			t.Error(err)
		}

		// Verify that HEAD moved to the second commit.
		if r, err := gitB.Head(ctx); err != nil {
			t.Fatal(err)
		} else if r.Commit != feature2 {
			names := map[git.Hash]string{
				base.Commit: "first commit",
				master2:     "upstream commit",
				feature2:    "push branch commit",
			}
			t.Errorf("after update, HEAD = %s; want %s",
				prettyCommit(r.Commit, names),
				prettyCommit(feature2, names))
		}

		// Verify that foo.txt has the second commit's content.
		if got, err := env.root.ReadFile("repoB/foo.txt"); err != nil {
			t.Error(err)
		} else if want := "Banana\n"; got != want {
			t.Errorf("foo.txt = %q; want %q", got, want)
		}
	})
	t.Run("UpstreamNoPush", func(t *testing.T) {
		// Until gg 0.7, this was the default test.
		// Behavior changed as part of https://github.com/zombiezen/gg/issues/80.

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
		if err := env.git.NewBranch(ctx, "upstream", git.BranchOptions{Checkout: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", "Banana\n")); err != nil {
			t.Fatal(err)
		}
		h2, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.CheckoutBranch(ctx, "master", git.CheckoutOptions{}); err != nil {
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

		// Verify that HEAD did not move.
		if r, err := env.git.Head(ctx); err != nil {
			t.Fatal(err)
		} else if r.Commit != h1 {
			names := map[git.Hash]string{
				h1: "first commit",
				h2: "second commit",
			}
			t.Errorf("after update, HEAD = %s; want %s",
				prettyCommit(r.Commit, names),
				prettyCommit(h1, names))
		}

		// Verify that foo.txt has the first commit's content.
		if got, err := env.root.ReadFile("foo.txt"); err != nil {
			t.Error(err)
		} else if want := "Apple\n"; got != want {
			t.Errorf("foo.txt = %q; want %q", got, want)
		}
	})
	t.Run("Merge", func(t *testing.T) {
		// Regression test for https://github.com/zombiezen/gg/issues/76.

		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository A with a commit and clone it to repository B.
		if err := env.initEmptyRepo(ctx, "repoA"); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("repoA/foo.txt", "Apple\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
			t.Fatal(err)
		}
		h1, err := env.newCommit(ctx, "repoA")
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.Run(ctx, "clone", "repoA", "repoB"); err != nil {
			t.Fatal(err)
		}

		// Create a second commit on master in repository A.
		if err := env.root.Apply(filesystem.Write("repoA/foo.txt", "Apple\nBanana\n")); err != nil {
			t.Fatal(err)
		}
		h2, err := env.newCommit(ctx, "repoA")
		if err != nil {
			t.Fatal(err)
		}

		// Make local changes to foo.txt in repository B.
		if err := env.root.Apply(filesystem.Write("repoB/foo.txt", "Coconut\nApple\n")); err != nil {
			t.Fatal(err)
		}

		// Run `git fetch origin` in repository B to update remote
		// tracking branches.
		repoBPath := env.root.FromSlash("repoB")
		gitB := env.git.WithDir(repoBPath)
		if err := gitB.Run(ctx, "fetch", "origin"); err != nil {
			t.Fatal(err)
		}

		// Call gg to update in repository B and merge local changes.
		_, err = env.gg(ctx, repoBPath, "update", "-merge")
		if err != nil {
			t.Error(err)
		}

		// Verify that HEAD moved to the second commit.
		if r, err := gitB.Head(ctx); err != nil {
			t.Fatal(err)
		} else if r.Commit != h2 {
			names := map[git.Hash]string{
				h1: "first commit",
				h2: "second commit",
			}
			t.Errorf("after update, HEAD = %s; want %s",
				prettyCommit(r.Commit, names),
				prettyCommit(h2, names))
		}

		// Verify that foo.txt has the merged content.
		if got, err := env.root.ReadFile("repoB/foo.txt"); err != nil {
			t.Error(err)
		} else if want := "Coconut\nApple\nBanana\n"; got != want {
			t.Errorf("foo.txt = %q; want %q", got, want)
		}
	})
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
	if err := env.git.NewBranch(ctx, "foo", git.BranchOptions{Checkout: true}); err != nil {
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
	if err := env.git.CheckoutBranch(ctx, "master", git.CheckoutOptions{}); err != nil {
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
		if r.Commit != h2 {
			names := map[git.Hash]string{
				initRev.Commit: "first commit",
				h2:             "second commit",
			}
			t.Errorf("after update foo, HEAD = %s; want %s",
				prettyCommit(r.Commit, names),
				prettyCommit(h2, names))
		}
		if got, want := r.Ref, git.BranchRef("foo"); got != want {
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
		if r.Commit != h1 {
			names := map[git.Hash]string{
				h1: "first commit",
				h2: "second commit",
			}
			t.Errorf("after update master, HEAD = %s; want %s",
				prettyCommit(r.Commit, names),
				prettyCommit(h1, names))
		}
		if got := r.Ref; got != git.Head {
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
