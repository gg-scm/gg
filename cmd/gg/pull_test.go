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

func TestPull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create repository A with some history and clone it to repository B.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	gitA := env.git.WithDir(env.root.FromSlash("repoA"))
	rev1, err := git.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit1 := rev1.Commit()
	if err := env.git.Run(ctx, "clone", "repoA", "repoB"); err != nil {
		t.Fatal(err)
	}

	// Make changes in repository A: add a tag and a new commit.
	if err := gitA.Run(ctx, "tag", "first"); err != nil {
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

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull"); err != nil {
		t.Fatal(err)
	}

	// Verify that HEAD has not moved from the first commit.
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "remote commit",
	}
	gitB := env.git.WithDir(repoBPath)
	if r, err := git.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != commit1 {
			t.Errorf("HEAD = %s; want %s",
				prettyCommit(r.Commit(), commitNames),
				prettyCommit(commit1, commitNames))
		}
		if r.Ref() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.Ref())
		}
	}

	// Verify that the remote tracking branch has moved to the new commit.
	if r, err := git.ParseRev(ctx, gitB, "origin/master"); err != nil {
		t.Error(err)
	} else if r.Commit() != commit2 {
		t.Errorf("origin/master = %s; want %s",
			prettyCommit(r.Commit(), commitNames),
			prettyCommit(commit2, commitNames))
	}

	// Verify that the tag was mirrored in repository B.
	if r, err := git.ParseRev(ctx, gitB, "first"); err != nil {
		t.Error(err)
	} else if r.Commit() != commit1 {
		t.Errorf("origin/master = %s; want %s",
			prettyCommit(r.Commit(), commitNames),
			prettyCommit(commit1, commitNames))
	}
}

func TestPullWithArgument(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create repository A with some history and clone it to repository B.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	rev1, err := git.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit1 := rev1.Commit()
	if err := env.git.Run(ctx, "clone", "repoA", "repoB"); err != nil {
		t.Fatal(err)
	}

	// Make changes in repository A: add a tag and a new commit.
	if err := gitA.Run(ctx, "tag", "first"); err != nil {
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

	// Move HEAD to a different commit in repository A.
	// We want to check that the corresponding branch is pulled independently of HEAD.
	if err := gitA.Run(ctx, "checkout", "--quiet", "--detach", "HEAD^"); err != nil {
		t.Fatal(err)
	}

	// Rename repository A to repository C so that if gg attempts to pull
	// from repository A, it will break (hopefully loudly).
	if err := env.root.Apply(filesystem.Rename("repoA", "repoC")); err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from repository C into repository B.
	repoBPath := env.root.FromSlash("repoB")
	repoCPath := env.root.FromSlash("repoC")
	if _, err := env.gg(ctx, repoBPath, "pull", repoCPath); err != nil {
		t.Fatal(err)
	}

	// Verify that HEAD has not moved from the first commit.
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "remote commit",
	}
	gitB := env.git.WithDir(repoBPath)
	if r, err := git.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != commit1 {
			t.Errorf("HEAD = %s; want %s",
				prettyCommit(r.Commit(), commitNames),
				prettyCommit(commit1, commitNames))
		}
		if r.Ref() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.Ref())
		}
	}

	// Verify that FETCH_HEAD has set to the second commit.
	if r, err := git.ParseRev(ctx, gitB, "FETCH_HEAD"); err != nil {
		t.Error(err)
	} else if r.Commit() != commit2 {
		t.Errorf("FETCH_HEAD = %s; want %s",
			prettyCommit(r.Commit(), commitNames),
			prettyCommit(commit2, commitNames))
	}

	// Verify that the tag was mirrored in repository B.
	if r, err := git.ParseRev(ctx, gitB, "first"); err != nil {
		t.Error(err)
	} else if r.Commit() != commit1 {
		t.Errorf("origin/master = %s; want %s",
			prettyCommit(r.Commit(), commitNames),
			prettyCommit(commit1, commitNames))
	}
}

func TestPullUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create repository A with some history and clone it to repository B.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	gitA := env.git.WithDir(env.root.FromSlash("repoA"))
	rev1, err := git.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit1 := rev1.Commit()
	if err := env.git.Run(ctx, "clone", "repoA", "repoB"); err != nil {
		t.Fatal(err)
	}

	// Make changes in repository A: add a tag and a new commit.
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

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull", "-u"); err != nil {
		t.Fatal(err)
	}

	// Verify that HEAD has moved to the second commit.
	commitNames := map[git.Hash]string{
		commit1: "shared commit",
		commit2: "remote commit",
	}
	gitB := env.git.WithDir(repoBPath)
	if r, err := git.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != commit2 {
			t.Errorf("HEAD = %s; want %s",
				prettyCommit(r.Commit(), commitNames),
				prettyCommit(commit1, commitNames))
		}
		if r.Ref() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.Ref())
		}
	}

	// Verify that the remote tracking branch has moved to the new commit.
	if r, err := git.ParseRev(ctx, gitB, "origin/master"); err != nil {
		t.Error(err)
	} else if r.Commit() != commit2 {
		t.Errorf("origin/master = %s; want %s",
			prettyCommit(r.Commit(), commitNames),
			prettyCommit(commit2, commitNames))
	}
}

func TestInferUpstream(t *testing.T) {
	t.Parallel()
	tests := []struct {
		localBranch string
		merge       git.Ref
		want        git.Ref
	}{
		{localBranch: "", want: "HEAD"},
		{localBranch: "master", want: "refs/heads/master"},
		{localBranch: "master", merge: "refs/heads/master", want: "refs/heads/master"},
		{localBranch: "foo", want: "refs/heads/foo"},
		{localBranch: "foo", merge: "refs/heads/bar", want: "refs/heads/bar"},
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		if test.merge != "" {
			if err := env.git.Run(ctx, "config", "--local", "branch."+test.localBranch+".merge", test.merge.String()); err != nil {
				t.Errorf("for localBranch = %q, merge = %q: %v", test.localBranch, test.merge, err)
				continue
			}
		}
		cfg, err := git.ReadConfig(ctx, env.git)
		if test.merge != "" {
			// Cleanup
			if err := env.git.Run(ctx, "config", "--local", "--unset", "branch."+test.localBranch+".merge"); err != nil {
				t.Errorf("cleaning up localBranch = %q, merge = %q: %v", test.localBranch, test.merge, err)
			}
		}
		if err != nil {
			t.Errorf("for localBranch = %q, merge = %q: %v", test.localBranch, test.merge, err)
			continue
		}
		got := inferUpstream(cfg, test.localBranch)
		if got != test.want {
			t.Errorf("inferUpstream(ctx, env.git, %q) (with branch.%s.merge = %q) = %q; want %q", test.localBranch, test.localBranch, test.merge, got, test.want)
		}
	}
}
