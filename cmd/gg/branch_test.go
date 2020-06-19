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
)

func TestBranch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}
	first, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "branch", "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	if r, err := env.git.Head(ctx); err != nil {
		t.Error(err)
	} else {
		if r.Commit != first.Commit {
			t.Errorf("HEAD = %s; want %s", r.Commit, first.Commit)
		}
		if r.Ref != "refs/heads/foo" {
			t.Errorf("HEAD refname = %q; want refs/heads/foo", r.Ref)
		}
	}
	if r, err := env.git.ParseRev(ctx, "bar"); err != nil {
		t.Error(err)
	} else {
		if r.Commit != first.Commit {
			t.Errorf("bar = %s; want %s", r.Commit, first.Commit)
		}
		if r.Ref != "refs/heads/bar" {
			t.Errorf("bar refname = %q; want refs/heads/bar", r.Ref)
		}
	}
}

func TestBranch_Upstream(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initRepoWithHistory(ctx, "repo1"); err != nil {
		t.Fatal(err)
	}
	git1 := env.git.WithDir(env.root.FromSlash("repo1"))
	first, err := git1.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "clone", "repo1", "repo2"); err != nil {
		t.Fatal(err)
	}

	repoPath2 := env.root.FromSlash("repo2")
	if _, err := env.gg(ctx, repoPath2, "branch", "foo"); err != nil {
		t.Fatal(err)
	}
	git2 := env.git.WithDir(repoPath2)
	if r, err := git2.Head(ctx); err != nil {
		t.Error(err)
	} else {
		if r.Commit != first.Commit {
			t.Errorf("HEAD = %s; want %s", r.Commit, first.Commit)
		}
		if r.Ref != "refs/heads/foo" {
			t.Errorf("HEAD refname = %q; want refs/heads/foo", r.Ref)
		}
	}
	cfg, err := git2.ReadConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if remote := cfg.Value("branch.foo.remote"); remote != "origin" {
		t.Errorf("branch.foo.remote = %q; want \"origin\"", remote)
	}
	if mergeBranch := cfg.Value("branch.foo.merge"); mergeBranch != "refs/heads/main" {
		t.Errorf("branch.foo.remote = %q; want \"refs/heads/main\"", mergeBranch)
	}
}

func TestBranch_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Merged", func(t *testing.T) {
		t.Parallel()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.git.NewBranch(ctx, "foo", git.BranchOptions{}); err != nil {
			t.Fatal(err)
		}

		if _, err := env.gg(ctx, env.root.String(), "branch", "--delete", "foo"); err != nil {
			t.Error(err)
		}

		// Verify that "refs/heads/foo" is no longer valid.
		if r, err := env.git.ParseRev(ctx, "refs/heads/foo"); err == nil {
			t.Errorf("refs/heads/foo = %v; should not exist", r.Commit)
		}
	})
	t.Run("Unmerged", func(t *testing.T) {
		t.Parallel()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.git.NewBranch(ctx, "foo", git.BranchOptions{Checkout: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "foo.txt"); err != nil {
			t.Fatal(err)
		}
		want, err := env.newCommit(ctx, ".")
		if err != nil {
			t.Fatal(err)
		}
		if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}

		if _, err := env.gg(ctx, env.root.String(), "branch", "--delete", "foo"); err == nil {
			t.Error("gg branch --delete did not return an error")
		} else if isUsage(err) {
			t.Error(err)
		}

		// Verify that "refs/heads/foo" is still valid.
		if r, err := env.git.ParseRev(ctx, "refs/heads/foo"); err != nil {
			t.Error(err)
		} else if r.Commit != want {
			t.Errorf("refs/heads/foo = %v; want %v", r.Commit, want)
		}
	})
	t.Run("UnmergedForce", func(t *testing.T) {
		t.Parallel()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		if err := env.initRepoWithHistory(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.git.NewBranch(ctx, "foo", git.BranchOptions{Checkout: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.addFiles(ctx, "foo.txt"); err != nil {
			t.Fatal(err)
		}
		if _, err := env.newCommit(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}

		if _, err := env.gg(ctx, env.root.String(), "branch", "--force", "--delete", "foo"); err != nil {
			t.Error(err)
		}

		// Verify that "refs/heads/foo" is no longer valid.
		if r, err := env.git.ParseRev(ctx, "refs/heads/foo"); err == nil {
			t.Errorf("refs/heads/foo = %v; should not exist", r.Commit)
		}
	})
}

func TestBranch_ListNewRepo(t *testing.T) {
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

	out, err := env.gg(ctx, env.root.String(), "branch")
	if err != nil {
		t.Error(err)
	}
	if len(out) > 0 {
		t.Errorf("stdout = %q; want \"\"", out)
	}
}
