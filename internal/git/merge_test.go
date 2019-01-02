// Copyright 2019 Google LLC
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
)

func TestIsMerging(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	t.Run("EmptyRepo", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		merging, err := env.g.IsMerging(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if merging {
			t.Error("IsMerging(ctx) = true; want false")
		}
	})
	t.Run("Merging", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with two commits, the second on a branch called "feature".
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "commit 1", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.NewBranch(ctx, "feature", BranchOptions{Checkout: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"bar.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "commit 2", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.CheckoutBranch(ctx, "master", CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}

		// Merge feature into master.
		// Use raw commands, as IsMerging is used as part of TestMerge.
		if _, err := env.g.Run(ctx, "merge", "--quiet", "--no-commit", "--no-ff", "feature"); err != nil {
			t.Fatal(err)
		}

		// Verify that IsMerging reports true.
		merging, err := env.g.IsMerging(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !merging {
			t.Error("IsMerging(ctx) = false; want true")
		}
	})
}

func TestMerge(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	t.Run("NoConflicts", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with the following commits:
		//
		// master -- a
		//       \
		//        -- b
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "commit 1", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.NewBranch(ctx, "a", BranchOptions{Checkout: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"bar.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "commit 2", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		a, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.NewBranch(ctx, "b", BranchOptions{StartPoint: "master", Checkout: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("baz.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"baz.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "commit 3", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		b, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Call Merge to merge b into a.
		if err := env.g.CheckoutBranch(ctx, "a", CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Merge(ctx, []string{"b"}); err != nil {
			t.Error("Merge:", err)
		}

		// Verify that HEAD is still pointing to a.
		head, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if head.Commit != a.Commit || head.Ref != "refs/heads/a" {
			t.Errorf("HEAD = %v (ref = %s); want %v (ref = refs/heads/a)", head.Commit, head.Ref, a.Commit)
		}
		// Verify that we are merging b.
		if mergeHead, err := env.g.ParseRev(ctx, "MERGE_HEAD"); err != nil {
			t.Error(err)
		} else if mergeHead.Commit != b.Commit {
			t.Errorf("MERGE_HEAD = %v; want %v (refs/heads/b)", mergeHead.Commit, b.Commit)
		}
	})
	t.Run("Conflicts", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with the following commits on foo.txt:
		//
		// master -- a
		//       \
		//        -- b
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", "content 1\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "commit 1", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.NewBranch(ctx, "a", BranchOptions{Checkout: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", "content 2\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.g.CommitAll(ctx, "commit 2", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		a, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.NewBranch(ctx, "b", BranchOptions{StartPoint: "master", Checkout: true}); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", "content 3\n")); err != nil {
			t.Fatal(err)
		}
		if err := env.g.CommitAll(ctx, "commit 3", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		b, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Call Merge to merge b into a.
		if err := env.g.CheckoutBranch(ctx, "a", CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Merge(ctx, []string{"b"}); err == nil {
			t.Error("Merge did not return an error")
		} else if got := err.Error(); !strings.Contains(got, "CONFLICT") {
			t.Errorf("Merge error: %s; want to contain \"CONFLICT\"", got)
		}

		// Verify that HEAD is still pointing to a.
		head, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if head.Commit != a.Commit || head.Ref != "refs/heads/a" {
			t.Errorf("HEAD = %v (ref = %s); want %v (ref = refs/heads/a)", head.Commit, head.Ref, a.Commit)
		}
		// Verify that we are merging b.
		if mergeHead, err := env.g.ParseRev(ctx, "MERGE_HEAD"); err != nil {
			t.Error(err)
		} else if mergeHead.Commit != b.Commit {
			t.Errorf("MERGE_HEAD = %v; want %v (refs/heads/b)", mergeHead.Commit, b.Commit)
		}
		// Verify that IsMerging returns true.
		if merging, err := env.g.IsMerging(ctx); err != nil {
			t.Error(err)
		} else if !merging {
			t.Error("IsMerging(ctx) = false; want true")
		}
	})
	t.Run("UnknownRev", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with a single commit.
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "initial commit", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		master, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Call Merge.
		if err := env.g.Merge(ctx, []string{"random"}); err == nil {
			t.Error("Merge did not return an error")
		}

		// Verify that HEAD is still pointing to master.
		head, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if head.Commit != master.Commit || head.Ref != "refs/heads/master" {
			t.Errorf("HEAD = %v (ref = %s); want %v (ref = refs/heads/master)", head.Commit, head.Ref, master.Commit)
		}
		// Verify that we are not merging.
		if merging, err := env.g.IsMerging(ctx); err != nil {
			t.Error(err)
		} else if merging {
			t.Error("Merge started merge")
		}
	})
	t.Run("ZeroRevisions", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with two commits, the first on a branch
		// called "old" tracking master.
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "commit 1", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.NewBranch(ctx, "old", BranchOptions{Track: true}); err != nil {
			t.Fatal(err)
		}
		old, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"bar.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Commit(ctx, "commit 2", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := env.g.CheckoutBranch(ctx, "old", CheckoutOptions{}); err != nil {
			t.Fatal(err)
		}

		// Call Merge with zero revisions. Naive implementations will merge with upstream.
		if err := env.g.Merge(ctx, nil); err == nil {
			t.Error("Merge(ctx, nil) did not return an error")
		}

		// Verify that HEAD is still pointing to old.
		head, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if head.Commit != old.Commit || head.Ref != "refs/heads/old" {
			t.Errorf("HEAD = %v (ref = %s); want %v (ref = refs/heads/old)", head.Commit, head.Ref, old.Commit)
		}
		// Verify that IsMerging reports false.
		merging, err := env.g.IsMerging(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if merging {
			t.Error("IsMerging(ctx) = true; want false")
		}
	})
}
