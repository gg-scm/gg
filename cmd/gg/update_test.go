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

	"gg-scm.io/pkg/internal/filesystem"
	"gg-scm.io/pkg/internal/git"
	"github.com/google/go-cmp/cmp"
)

func TestUpdate_NoArgs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("UpstreamSameName", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

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

		// Call gg to update main in repository B.
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

		// Create a new commit in repository A on main.
		if err := env.root.Apply(filesystem.Write("repoA/foo.txt", "Apple\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
			t.Fatal(err)
		}
		main2, err := env.newCommit(ctx, "repoA")
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
		// that tracks "refs/remotes/origin/main".
		repoBPath := env.root.FromSlash("repoB")
		gitB := env.git.WithDir(repoBPath)
		err = gitB.NewBranch(ctx, "feature", git.BranchOptions{
			StartPoint: "refs/remotes/origin/main",
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
				main2:       "upstream commit",
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

		// Create a repository with two commits, with main behind the "upstream" branch.
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
		if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
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

		// Create a second commit on main in repository A.
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
		_, err = env.gg(ctx, repoBPath, "update")
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

	t.Run("UpToDate", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

		// Start a repository with an arbitrary main branch.
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

		// Check out main branch.
		if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
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
	})
	t.Run("OutOfDateFastForwards", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

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
		const wantContent = "Apple\n"
		if err := env.root.Apply(filesystem.Write("repoA/foo.txt", wantContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
			t.Fatal(err)
		}
		h2, err := env.newCommit(ctx, "repoA")
		if err != nil {
			t.Fatal(err)
		}

		// Switch to a new branch in repository B.
		repoBPath := env.root.FromSlash("repoB")
		gitB := env.git.WithDir(repoBPath)
		if err := gitB.NewBranch(ctx, "foo", git.BranchOptions{Checkout: true}); err != nil {
			t.Fatal(err)
		}

		// Run `git fetch origin` in repository B to update remote
		// tracking branches.
		if err := gitB.Run(ctx, "fetch", "origin"); err != nil {
			t.Fatal(err)
		}

		// Call gg to switch to main branch.
		_, err = env.gg(ctx, repoBPath, "update", "main")
		if err != nil {
			t.Error(err)
		}

		// Verify that HEAD was moved to main branch.
		if r, err := gitB.Head(ctx); err != nil {
			t.Fatal(err)
		} else {
			if r.Commit != h2 {
				names := map[git.Hash]string{
					rev1.Commit: "first commit",
					h2:          "second commit",
				}
				t.Errorf("after update main, HEAD = %s; want %s",
					prettyCommit(r.Commit, names),
					prettyCommit(h2, names))
			}
			if got, want := r.Ref, git.BranchRef("main"); got != want {
				t.Errorf("after update main, HEAD ref = %s; want %s", got, want)
			}
		}

		// Verify that foo.txt has the branch commit's content.
		if got, err := env.root.ReadFile("repoB/foo.txt"); err != nil {
			t.Error(err)
		} else if got != wantContent {
			t.Errorf("foo.txt = %q; want %q", got, wantContent)
		}
	})
	t.Run("ToAheadBranch", func(t *testing.T) {
		// Regression test for https://github.com/zombiezen/gg/issues/103

		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

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

		// Create a new commit in repository B.
		const wantContent = "Apple\n"
		if err := env.root.Apply(filesystem.Write("repoB/foo.txt", wantContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "repoB/foo.txt"); err != nil {
			t.Fatal(err)
		}
		h2, err := env.newCommit(ctx, "repoB")
		if err != nil {
			t.Fatal(err)
		}

		// Switch to a new branch in repository B.
		repoBPath := env.root.FromSlash("repoB")
		gitB := env.git.WithDir(repoBPath)
		err = gitB.NewBranch(ctx, "foo", git.BranchOptions{
			Checkout:   true,
			StartPoint: rev1.Commit.String(),
		})
		if err != nil {
			t.Fatal(err)
		}

		// Call gg to switch back to main branch.
		_, err = env.gg(ctx, repoBPath, "update", "main")
		if err != nil {
			t.Error(err)
		}

		// Verify that HEAD was moved to main branch.
		if r, err := gitB.Head(ctx); err != nil {
			t.Fatal(err)
		} else {
			if r.Commit != h2 {
				names := map[git.Hash]string{
					rev1.Commit: "first commit",
					h2:          "second commit",
				}
				t.Errorf("after update main, HEAD = %s; want %s",
					prettyCommit(r.Commit, names),
					prettyCommit(h2, names))
			}
			if got, want := r.Ref, git.BranchRef("main"); got != want {
				t.Errorf("after update main, HEAD ref = %s; want %s", got, want)
			}
		}

		// Verify that foo.txt has the branch commit's content.
		if got, err := env.root.ReadFile("repoB/foo.txt"); err != nil {
			t.Error(err)
		} else if got != wantContent {
			t.Errorf("foo.txt = %q; want %q", got, wantContent)
		}
	})
	t.Run("NotPresentInRemote", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}

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

		// Create a new branch in repository B.
		repoBPath := env.root.FromSlash("repoB")
		gitB := env.git.WithDir(repoBPath)
		if err := gitB.NewBranch(ctx, "foo", git.BranchOptions{}); err != nil {
			t.Fatal(err)
		}

		// Call gg to switch to foo branch.
		_, err = env.gg(ctx, repoBPath, "update", "foo")
		if err != nil {
			t.Error(err)
		}

		// Verify that HEAD was moved to foo branch.
		if r, err := gitB.Head(ctx); err != nil {
			t.Fatal(err)
		} else {
			if r.Commit != rev1.Commit {
				names := map[git.Hash]string{
					rev1.Commit: "first commit",
				}
				t.Errorf("after update foo, HEAD = %s; want %s",
					prettyCommit(r.Commit, names),
					prettyCommit(rev1.Commit, names))
			}
			if got, want := r.Ref, git.BranchRef("foo"); got != want {
				t.Errorf("after update foo, HEAD ref = %s; want %s", got, want)
			}
		}
	})
}

func TestUpdate_ToCommit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

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
			t.Errorf("after update main, HEAD = %s; want %s",
				prettyCommit(r.Commit, names),
				prettyCommit(h1, names))
		}
		if got := r.Ref; got != git.Head {
			t.Errorf("after update main, HEAD ref = %s; want %s", got, git.Head)
		}
	}

	// Verify that foo.txt has the first commit's content.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "Apple\n"; got != want {
		t.Errorf("foo.txt = %q; want %q", got, want)
	}
}

func TestUpdate_Unclean(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

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

	// Introduce local changes.
	if err := env.root.Apply(filesystem.Write("foo.txt", "Coconut\n")); err != nil {
		t.Fatal(err)
	}

	// Call gg to update to the first commit. It should cause a merge
	// conflict, but not fail.
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
			t.Errorf("after update %v, HEAD = %s; want %s",
				h1,
				prettyCommit(r.Commit, names),
				prettyCommit(h1, names))
		}
		if got, want := r.Ref, git.Head; got != want {
			t.Errorf("after update %v, HEAD ref = %s; want %s", h1, got, want)
		}
	}

	// Verify that foo.txt has merge conflicts.
	st, err := env.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []git.StatusEntry{
		{Code: git.StatusCode{'U', 'U'}, Name: "foo.txt"},
	}
	if diff := cmp.Diff(want, st); diff != "" {
		t.Errorf("status (-want +got):\n%s", diff)
	}
}

func TestUpdate_Clean(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

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

	// Introduce local changes.
	if err := env.root.Apply(filesystem.Write("foo.txt", "Coconut\n")); err != nil {
		t.Fatal(err)
	}

	// Call gg to update cleanly to the first commit.
	_, err = env.gg(ctx, env.root.String(), "update", "--clean", h1.String())
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
			t.Errorf("after update %v, HEAD = %s; want %s",
				h1,
				prettyCommit(r.Commit, names),
				prettyCommit(h1, names))
		}
		if got, want := r.Ref, git.Head; got != want {
			t.Errorf("after update %v, HEAD ref = %s; want %s", h1, got, want)
		}
	}

	// Verify that foo.txt has the first commit's content.
	if got, err := env.root.ReadFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "Apple\n"; got != want {
		t.Errorf("foo.txt = %q; want %q", got, want)
	}
}
