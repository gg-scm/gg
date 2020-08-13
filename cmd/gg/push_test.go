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
	"net/url"
	"strings"
	"testing"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/filesystem"
)

func TestPush(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, env.root.FromSlash("repoB")); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to push from repo A to repo B.
	if _, err := env.gg(ctx, repoAPath, "push"); err != nil {
		t.Error(err)
	}

	// Verify that repo B has the new commit.
	gitB := env.git.WithDir(repoBPath)
	if r, err := gitB.ParseRev(ctx, "refs/heads/main"); err != nil {
		t.Error(err)
	} else if r.Commit != commit2 {
		names := map[git.Hash]string{
			rev1.Commit: "shared commit",
			commit2:     "local commit",
		}
		t.Errorf("refs/heads/main = %s; want %s",
			prettyCommit(r.Commit, names),
			prettyCommit(commit2, names))
	}
}

func TestPush_Arg(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, "repoB"); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Clone from repo B to repo C. We want to make sure that the origin remote is
	// not used at all.
	if err := env.git.Run(ctx, "clone", "--bare", "repoB", "repoC"); err != nil {
		t.Fatal(err)
	}

	// Call gg to push from repo A to repo C.
	repoCPath := env.root.FromSlash("repoC")
	if _, err := env.gg(ctx, repoAPath, "push", repoCPath); err != nil {
		t.Error(err)
	}

	// Ensure that repo C's main branch has moved to the new commit.
	commit1 := rev1.Commit
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "local commit",
	}
	gitC := env.git.WithDir(repoCPath)
	if r, err := gitC.ParseRev(ctx, "refs/heads/main"); err != nil {
		t.Error(err)
	} else if r.Commit != commit2 {
		t.Errorf("named remote refs/heads/main = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit2, commitNames))
	}

	// Verify that repo B's main branch has stayed the same.
	gitB := env.git.WithDir(repoBPath)
	if r, err := gitB.ParseRev(ctx, "refs/heads/main"); err != nil {
		t.Error(err)
	} else if r.Commit != commit1 {
		t.Errorf("origin refs/heads/main = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit1, commitNames))
	}
}

func TestPush_FailUnknownRef(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, "repoB"); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := gitA.NewBranch(ctx, "foo", git.BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to push from repo A's main branch to repo B's nonexistent foo branch.
	// This should return an error.
	if _, err := env.gg(ctx, repoAPath, "push", "-r", "foo"); err == nil {
		t.Error("push of new ref did not return error")
	} else if isUsage(err) {
		t.Errorf("push of new ref returned usage error: %v", err)
	}

	// Verify that repo B's main branch has not changed.
	commit1 := rev1.Commit
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "local commit",
	}
	gitB := env.git.WithDir(repoBPath)
	if r, err := gitB.ParseRev(ctx, "refs/heads/main"); err != nil {
		t.Error(err)
	} else if r.Commit != commit1 {
		t.Errorf("refs/heads/main = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit1, commitNames))
	}

	// Verify that repo B did not gain a foo branch.
	if r, err := gitB.ParseRev(ctx, "foo"); err == nil {
		if ref := r.Ref; ref != "" {
			t.Logf("foo resolved to %s", ref)
		}
		t.Errorf("on remote, foo = %s; want to not exist", prettyCommit(r.Commit, commitNames))
	}
}

func TestPush_CreateRef(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, "repoB"); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := gitA.NewBranch(ctx, "foo", git.BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to push the repo A main branch to repo B's foo branch.
	if _, err := env.gg(ctx, repoAPath, "push", "-r", "foo", "--new-branch"); err != nil {
		t.Error(err)
	}

	// Verify that repo B's foo branch has moved to the new commit.
	commit1 := rev1.Commit
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "local commit",
	}
	gitB := env.git.WithDir(repoBPath)
	if r, err := gitB.ParseRev(ctx, "refs/heads/foo"); err != nil {
		t.Error(err)
	} else if r.Commit != commit2 {
		t.Errorf("refs/heads/foo = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit2, commitNames))
	}

	// Verify that repo B's main branch has not changed.
	if r, err := gitB.ParseRev(ctx, "refs/heads/main"); err != nil {
		t.Error(err)
	} else if r.Commit != commit1 {
		t.Errorf("refs/heads/main = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit1, commitNames))
	}
}

func TestPush_RewindFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, "repoB"); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Push second commit to repo B.
	if err := gitA.Run(ctx, "push", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Move main in repo A back to the first commit.
	commit1 := rev1.Commit
	if err := gitA.Run(ctx, "reset", "--hard", commit1.String()); err != nil {
		t.Fatal(err)
	}

	// Call gg to push initial commit from repo A to repo B's main branch (a rewind).
	// This should return an error.
	if _, err := env.gg(ctx, repoAPath, "push", "-r", "main"); err == nil {
		t.Error("push of parent rev did not return error")
	} else if isUsage(err) {
		t.Errorf("push of parent rev returned usage error: %v", err)
	}

	// Verify that repo B's main branch has not changed.
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "local commit",
	}
	gitB := env.git.WithDir(repoBPath)
	if r, err := gitB.ParseRev(ctx, "refs/heads/main"); err != nil {
		t.Error(err)
	} else if r.Commit != commit2 {
		t.Errorf("refs/heads/main = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit2, commitNames))
	}
}

func TestPush_RewindForce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, "repoB"); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Push second commit to repo B.
	if err := gitA.Run(ctx, "push", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Move main in repo A back to the first commit.
	commit1 := rev1.Commit
	if err := gitA.Run(ctx, "reset", "--hard", commit1.String()); err != nil {
		t.Fatal(err)
	}

	// Call gg to push initial commit from repo A to repo B's main branch (a rewind) with
	// the -f flag.
	if _, err := env.gg(ctx, repoAPath, "push", "-f", "-r", "main"); err != nil {
		t.Error(err)
	}

	// Verify that repo B's main branch has moved to the new commit.
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "local commit",
	}
	gitB := env.git.WithDir(repoBPath)
	if r, err := gitB.ParseRev(ctx, "refs/heads/main"); err != nil {
		t.Error(err)
	} else if r.Commit != commit1 {
		t.Errorf("refs/heads/main = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit1, commitNames))
	}
}

func TestPush_DistinctPushURL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, "repoB"); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Set push URL of origin to a new repo C.
	if err := env.git.Run(ctx, "clone", "--bare", "repoB", "repoC"); err != nil {
		t.Fatal(err)
	}
	repoCPath := env.root.FromSlash("repoC")
	if err := gitA.Run(ctx, "remote", "set-url", "--push", "origin", repoCPath); err != nil {
		t.Fatal(err)
	}

	// Call gg to push from repo A to repo C.
	if _, err := env.gg(ctx, repoAPath, "push"); err != nil {
		t.Error(err)
	}

	// Verify that repo C's main branch has moved to the new commit.
	commit1 := rev1.Commit
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "local commit",
	}
	gitC := env.git.WithDir(repoCPath)
	if r, err := gitC.ParseRev(ctx, "main"); err != nil {
		t.Error("In push repo:", err)
	} else if r.Commit != commit2 {
		t.Errorf("main in push repo = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit2, commitNames))
	}

	// Verify that repo B's main branch has not changed.
	gitB := env.git.WithDir(repoBPath)
	if r, err := gitB.ParseRev(ctx, "main"); err != nil {
		t.Error("In fetch repo:", err)
	} else if r.Commit != commit1 {
		t.Errorf("main in fetch repo = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit1, commitNames))
	}
}

func TestPush_NoCreateFetchURLMissingBranch(t *testing.T) {
	// Ensure that -create=0 does not proceed if the branch is missing
	// from the fetch URL but is present in the push URL. See
	// https://github.com/zombiezen/gg/issues/75 for background.

	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, "repoB"); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Create a new branch in repo A called "newbranch".
	if err := gitA.NewBranch(ctx, "newbranch", git.BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}

	// Set repo A's origin push URL to a new repo C with branch "newbranch" present.
	if err := env.git.Run(ctx, "clone", "--bare", "repoB", "repoC"); err != nil {
		t.Fatal(err)
	}
	repoCPath := env.root.FromSlash("repoC")
	gitC := env.git.WithDir(repoCPath)
	if err := gitC.NewBranch(ctx, "newbranch", git.BranchOptions{StartPoint: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "remote", "set-url", "--push", "origin", repoCPath); err != nil {
		t.Fatal(err)
	}

	// Call gg to push from repo A to repo C.
	if _, err := env.gg(ctx, repoAPath, "push", "-r", "newbranch"); err == nil {
		t.Error("push of new ref did not return error")
	} else if isUsage(err) {
		t.Errorf("push of new ref returned usage error: %v", err)
	}

	// Verify that repo C's branch "newbranch" was not moved.
	commitNames := map[git.Hash]string{
		rev1.Commit: "shared commit",
		commit2:     "local commit",
	}
	if r, err := gitC.ParseRev(ctx, "newbranch"); err != nil {
		t.Error("In push repo:", err)
	} else if r.Commit != rev1.Commit {
		t.Errorf("newbranch in push repo = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(rev1.Commit, commitNames))
	}

	// Verify that repo B's branch "newbranch" was not created.
	gitB := env.git.WithDir(repoBPath)
	if r, err := gitB.ParseRev(ctx, "newbranch"); err == nil {
		t.Errorf("newbranch in fetch repo = %s; want to not exist", prettyCommit(r.Commit, commitNames))
	}
}

func TestPush_NoCreatePushURLMissingBranch(t *testing.T) {
	// Ensure that -create=0 succeeds if the branch is missing from the
	// push URL but is present in the fetch URL. See
	// https://github.com/zombiezen/gg/issues/75 for background.

	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	// Create repository with some junk history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := gitA.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Push from repo A to repo B.
	if err := env.git.InitBare(ctx, "repoB"); err != nil {
		t.Fatal(err)
	}
	repoBPath := env.root.FromSlash("repoB")
	if err := gitA.Run(ctx, "remote", "add", "origin", repoBPath); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "main"); err != nil {
		t.Fatal(err)
	}

	// Create a new commit in repo A.
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	commit2, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}

	// Create a new branch in repo A called "newbranch".
	if err := gitA.NewBranch(ctx, "newbranch", git.BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}

	// Set repo A's origin push URL to a new repo C with branch "newbranch" not present.
	// Create "newbranch" on repo B.
	if err := env.git.Run(ctx, "clone", "--bare", "repoB", "repoC"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(repoBPath)
	if err := gitB.NewBranch(ctx, "newbranch", git.BranchOptions{StartPoint: "main"}); err != nil {
		t.Fatal(err)
	}
	repoCPath := env.root.FromSlash("repoC")
	if err := gitA.Run(ctx, "remote", "set-url", "--push", "origin", repoCPath); err != nil {
		t.Fatal(err)
	}

	// Call gg to push from repo A to repo C.
	if _, err := env.gg(ctx, repoAPath, "push"); err != nil {
		t.Error(err)
	}

	// Verify that repo C's branch "newbranch" was moved to the new commit.
	commitNames := map[git.Hash]string{
		rev1.Commit: "shared commit",
		commit2:     "local commit",
	}
	gitC := env.git.WithDir(repoCPath)
	if r, err := gitC.ParseRev(ctx, "newbranch"); err != nil {
		t.Error("In push repo:", err)
	} else if r.Commit != commit2 {
		t.Errorf("newbranch in push repo = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(commit2, commitNames))
	}

	// Verify that repo B's branch "newbranch" was not moved.
	if r, err := gitB.ParseRev(ctx, "newbranch"); err != nil {
		t.Error("In fetch repo:", err)
	} else if r.Commit != rev1.Commit {
		t.Errorf("newbranch in fetch repo = %s; want %s",
			prettyCommit(r.Commit, commitNames),
			prettyCommit(rev1.Commit, commitNames))
	}
}

func TestGerritPushRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		branch string
		opts   *gerritOptions

		wantRef  git.Ref
		wantOpts map[string][]string
	}{
		{
			branch:   "main",
			wantRef:  "refs/for/main",
			wantOpts: map[string][]string{"no-publish-comments": nil},
		},
		{
			branch: "main",
			opts: &gerritOptions{
				publishComments: true,
			},
			wantRef:  "refs/for/main",
			wantOpts: map[string][]string{"publish-comments": nil},
		},
		{
			branch: "main",
			opts: &gerritOptions{
				message: "This is a rebase on main!",
			},
			wantRef: "refs/for/main",
			wantOpts: map[string][]string{
				"m":                   {"This is a rebase on main!"},
				"no-publish-comments": nil,
			},
		},
		{
			branch: "main",
			opts: &gerritOptions{
				reviewers: []string{"a@a.com", "c@r.com"},
				cc:        []string{"b@o.com", "d@zombo.com"},
			},
			wantRef: "refs/for/main",
			wantOpts: map[string][]string{
				"r":                   {"a@a.com", "c@r.com"},
				"cc":                  {"b@o.com", "d@zombo.com"},
				"no-publish-comments": nil,
			},
		},
		{
			branch: "main",
			opts: &gerritOptions{
				reviewers: []string{"a@a.com,c@r.com"},
				cc:        []string{"b@o.com,d@zombo.com"},
			},
			wantRef: "refs/for/main",
			wantOpts: map[string][]string{
				"r":                   {"a@a.com", "c@r.com"},
				"cc":                  {"b@o.com", "d@zombo.com"},
				"no-publish-comments": nil,
			},
		},
		{
			branch: "main",
			opts: &gerritOptions{
				notify:    "NONE",
				notifyTo:  []string{"a@a.com"},
				notifyCC:  []string{"b@b.com"},
				notifyBCC: []string{"c@c.com"},
			},
			wantRef: "refs/for/main",
			wantOpts: map[string][]string{
				"notify":              {"NONE"},
				"notify-to":           {"a@a.com"},
				"notify-cc":           {"b@b.com"},
				"notify-bcc":          {"c@c.com"},
				"no-publish-comments": nil,
			},
		},
	}
	for _, test := range tests {
		out := gerritPushRef(test.branch, test.opts)
		ref, opts, err := parseGerritRef(out)
		if err != nil {
			t.Errorf("gerritPushRef(%q, %+v) = %q; cannot parse: %v", test.branch, test.opts, out, err)
			continue
		}
		if ref != test.wantRef || !gerritOptionsEqual(opts, test.wantOpts) {
			t.Errorf("gerritPushRef(%q, %+v) = %q; want ref %q and options %q", test.branch, test.opts, out, test.wantRef, test.wantOpts)
		}
	}
}

func TestParseGerritRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ref  git.Ref
		base git.Ref
		opts map[string][]string
	}{
		{
			ref:  "refs/for/main",
			base: "refs/for/main",
		},
		{
			ref:  "refs/for/main%no-publish-comments",
			base: "refs/for/main",
			opts: map[string][]string{"no-publish-comments": nil},
		},
		{
			ref:  "refs/for/expiremental%topic=driver/i42",
			base: "refs/for/expiremental",
			opts: map[string][]string{"topic": {"driver/i42"}},
		},
		{
			ref:  "refs/for/main%notify=NONE,notify-to=a@a.com",
			base: "refs/for/main",
			opts: map[string][]string{"notify": {"NONE"}, "notify-to": {"a@a.com"}},
		},
		{
			ref:  "refs/for/main%m=This_is_a_rebase_on_main%21",
			base: "refs/for/main",
			opts: map[string][]string{"m": {"This is a rebase on main!"}},
		},
		{
			ref:  "refs/for/main%m=This+is+a+rebase+on+main%21",
			base: "refs/for/main",
			opts: map[string][]string{"m": {"This is a rebase on main!"}},
		},
		{
			ref:  "refs/for/main%l=Code-Review+1,l=Verified+1",
			base: "refs/for/main",
			opts: map[string][]string{"l": {"Code-Review+1", "Verified+1"}},
		},
		{
			ref:  "refs/for/main%r=a@a.com,cc=b@o.com",
			base: "refs/for/main",
			opts: map[string][]string{"r": {"a@a.com"}, "cc": {"b@o.com"}},
		},
		{
			ref:  "refs/for/main%r=a@a.com,cc=b@o.com,r=c@r.com",
			base: "refs/for/main",
			opts: map[string][]string{"r": {"a@a.com", "c@r.com"}, "cc": {"b@o.com"}},
		},
	}
	for _, test := range tests {
		base, opts, err := parseGerritRef(test.ref)
		if err != nil {
			t.Errorf("parseGerritRef(%q) = _, _, %v; want no error", test.ref, err)
			continue
		}
		if base != test.base || !gerritOptionsEqual(opts, test.opts) {
			t.Errorf("parseGerritRef(%q) = %q, %q, <nil>; want %q, %q, <nil>", test.ref, base, opts, test.base, test.opts)
		}
	}
}

func parseGerritRef(ref git.Ref) (git.Ref, map[string][]string, error) {
	start := strings.IndexByte(string(ref), '%')
	if start == -1 {
		return ref, nil, nil
	}
	opts := make(map[string][]string)
	q := string(ref[start+1:])
	for len(q) > 0 {
		sep := strings.IndexByte(q, ',')
		if sep == -1 {
			sep = len(q)
		}
		if eq := strings.IndexByte(q[:sep], '='); eq != -1 {
			k := q[:eq]
			v := q[eq+1 : sep]
			if k == "m" || k == "message" { // special-cased in Gerrit (see ReceiveCommits.java)
				var err error
				v, err = url.QueryUnescape(strings.Replace(q[eq+1:sep], "_", "+", -1))
				if err != nil {
					return "", nil, err
				}
			}
			opts[k] = append(opts[k], v)
		} else {
			k := q[:sep]
			if v := opts[k]; v != nil {
				opts[k] = append(v, "")
			} else {
				opts[k] = nil
			}
		}
		if sep >= len(q) {
			break
		}
		q = q[sep+1:]
	}
	return ref[:start], opts, nil
}

func gerritOptionsEqual(m1, m2 map[string][]string) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok || len(v1) != len(v2) || (v1 == nil) != (v2 == nil) {
			return false
		}
		for i := range v1 {
			if v1[i] != v2[i] {
				return false
			}
		}
	}
	return true
}
