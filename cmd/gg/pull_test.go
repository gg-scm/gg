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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type pullTestCommits struct {
	originalMain   git.Hash
	newMain        git.Hash
	localCommit    git.Hash
	divergeCommitA git.Hash
	divergeCommitB git.Hash
}

func (commits pullTestCommits) Names() map[git.Hash]string {
	return map[git.Hash]string{
		commits.originalMain:   "original main",
		commits.newMain:        "new main",
		commits.localCommit:    "commit to local branch",
		commits.divergeCommitA: "diverge commit in repoA",
		commits.divergeCommitB: "diverge commit in repoB",
	}
}

// setupPullTest arranges two repositories in the test environment, repoA and
// repoB, with repoB as a clone of repoA. repoA and repoB are then modified to
// test a bunch of salient conditions:
//
//   - repoB will have a branch "main" that is one commit behind repoA.
//     This will be the checked out branch.
//   - repoB will have a branch "local" that is one commit ahead repoA.
//   - repoB will have a branch "diverge" that is one commit ahead and one
//     commit behind repoA.
//   - repoA will have a branch "newbranch" that isn't present in repoB.
//   - repoB will have a branch "delbranch" that was originally in repoA, but
//     was deleted after the initial clone.
//   - repoB will have a branch "delbranch-local" that was originally
//     in repoA, but was deleted after the initial clone. It will contain
//     a commit that wasn't in repoA.
//   - repoB will have a tag "zero" for the original main commit.
//   - repoA and repoB will have a tag "first" on the original main commit.
//   - repoA will have a tag "second" for the new main branch.
func setupPullTest(ctx context.Context, env *testEnv) (pullTestCommits, error) {
	var commits pullTestCommits

	// Make shared history.
	if err := env.initRepoWithHistory(ctx, "repoA"); err != nil {
		return pullTestCommits{}, err
	}
	gitA := env.git.WithDir(env.root.FromSlash("repoA"))
	for _, name := range []string{"delbranch", "delbranch-local", "diverge", "local"} {
		if err := gitA.NewBranch(ctx, name, git.BranchOptions{}); err != nil {
			return pullTestCommits{}, err
		}
	}
	rev1, err := gitA.Head(ctx)
	if err != nil {
		return pullTestCommits{}, err
	}
	commits.originalMain = rev1.Commit
	if err := gitA.Run(ctx, "tag", "first"); err != nil {
		return pullTestCommits{}, err
	}
	if _, err := env.gg(ctx, env.root.String(), "clone", "repoA", "repoB"); err != nil {
		return pullTestCommits{}, err
	}

	// Make changes to repoA.
	if err := gitA.NewBranch(ctx, "newbranch", git.BranchOptions{}); err != nil {
		return pullTestCommits{}, err
	}
	err = gitA.MutateRefs(ctx, map[git.Ref]git.RefMutation{
		"refs/heads/delbranch":       git.DeleteRef(),
		"refs/heads/delbranch-local": git.DeleteRef(),
	})
	if err != nil {
		return pullTestCommits{}, err
	}
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", dummyContent)); err != nil {
		return pullTestCommits{}, err
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		return pullTestCommits{}, err
	}
	commits.newMain, err = env.newCommit(ctx, "repoA")
	if err != nil {
		return pullTestCommits{}, err
	}
	if err := gitA.Run(ctx, "tag", "-a", "-m", "my tag", "second"); err != nil {
		return pullTestCommits{}, err
	}
	if err := gitA.CheckoutBranch(ctx, "diverge", git.CheckoutOptions{}); err != nil {
		return pullTestCommits{}, err
	}
	if err := env.root.Apply(filesystem.Write("repoA/bar.txt", dummyContent)); err != nil {
		return pullTestCommits{}, err
	}
	if err := env.addFiles(ctx, "repoA/bar.txt"); err != nil {
		return pullTestCommits{}, err
	}
	commits.divergeCommitA, err = env.newCommit(ctx, "repoA")
	if err != nil {
		return pullTestCommits{}, err
	}
	// Ensure repoA's HEAD points to main.
	if err := gitA.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
		return pullTestCommits{}, err
	}

	// Make changes to repoB.
	gitB := env.git.WithDir(env.root.FromSlash("repoB"))
	if err := gitB.Run(ctx, "tag", "zero", commits.originalMain.String()); err != nil {
		return pullTestCommits{}, err
	}
	if err := gitB.CheckoutBranch(ctx, "diverge", git.CheckoutOptions{}); err != nil {
		return pullTestCommits{}, err
	}
	if err := env.root.Apply(filesystem.Write("repoB/baz.txt", dummyContent)); err != nil {
		return pullTestCommits{}, err
	}
	if err := env.addFiles(ctx, "repoB/baz.txt"); err != nil {
		return pullTestCommits{}, err
	}
	commits.divergeCommitB, err = env.newCommit(ctx, "repoB")
	if err != nil {
		return pullTestCommits{}, err
	}
	if err := gitB.CheckoutBranch(ctx, "local", git.CheckoutOptions{}); err != nil {
		return pullTestCommits{}, err
	}
	if err := env.root.Apply(filesystem.Write("repoB/local.txt", dummyContent)); err != nil {
		return pullTestCommits{}, err
	}
	if err := env.addFiles(ctx, "repoB/local.txt"); err != nil {
		return pullTestCommits{}, err
	}
	commits.localCommit, err = env.newCommit(ctx, "repoB")
	if err != nil {
		return pullTestCommits{}, err
	}
	err = gitB.MutateRefs(ctx, map[git.Ref]git.RefMutation{
		"refs/heads/delbranch-local": git.SetRef(commits.localCommit.String()),
	})
	if err != nil {
		return pullTestCommits{}, err
	}
	// Ensure repoB's HEAD points to main.
	if err := gitB.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
		return pullTestCommits{}, err
	}

	return commits, nil
}

func TestPull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull"); err != nil {
		t.Error(err)
	}

	gitB := env.git.WithDir(repoBPath)
	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := gitB.ReadConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref        git.Ref
		want       git.Hash
		wantGone   bool
		wantRemote string
		wantMerge  string
	}{
		{ref: git.Head, want: commits.originalMain},
		{ref: "refs/remotes/origin/main", want: commits.newMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.divergeCommitA},
		{ref: "refs/remotes/origin/newbranch", want: commits.originalMain},
		{ref: "refs/remotes/origin/delbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch-local", wantGone: true},
		{ref: "refs/ggpull/main", wantGone: true},
		{ref: "refs/ggpull/local", wantGone: true},
		{ref: "refs/ggpull/diverge", wantGone: true},
		{ref: "refs/ggpull/newbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/gg-old/delbranch", want: commits.originalMain},
		{ref: "refs/heads/main", want: commits.originalMain, wantRemote: "origin", wantMerge: "refs/heads/main"},
		{ref: "refs/heads/local", want: commits.localCommit, wantRemote: "origin", wantMerge: "refs/heads/local"},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB, wantRemote: "origin", wantMerge: "refs/heads/diverge"},
		{ref: "refs/heads/newbranch", want: commits.originalMain, wantRemote: "origin", wantMerge: "refs/heads/newbranch"},
		{ref: "refs/heads/delbranch", wantGone: true},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit, wantRemote: "origin", wantMerge: "refs/heads/delbranch-local"},
		{ref: "refs/tags/zero", want: commits.originalMain},
		{ref: "refs/tags/first", want: commits.originalMain},
		{ref: "refs/tags/second", want: commits.newMain},
	}
	for _, check := range refChecks {
		if branch := check.ref.Branch(); branch != "" {
			gotRemote := cfg.Value("branch." + branch + ".remote")
			gotMerge := cfg.Value("branch." + branch + ".merge")
			if gotRemote != check.wantRemote || gotMerge != check.wantMerge {
				t.Errorf("branch %q remote = %q, merge = %q; want remote = %q, merge = %q", branch, gotRemote, gotMerge, check.wantRemote, check.wantMerge)
			}
		}
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestPullWithArgument(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
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
		t.Error(err)
	}

	gitB := env.git.WithDir(repoBPath)
	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := gitB.ReadConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref        git.Ref
		want       git.Hash
		wantGone   bool
		wantRemote string
		wantMerge  string
	}{
		{ref: git.Head, want: commits.originalMain},
		{ref: "refs/remotes/origin/main", want: commits.originalMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.originalMain},
		{ref: "refs/remotes/origin/newbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch", want: commits.originalMain},
		{ref: "refs/remotes/origin/delbranch-local", want: commits.originalMain},
		{ref: "refs/ggpull/main", want: commits.newMain},
		{ref: "refs/ggpull/local", want: commits.originalMain},
		{ref: "refs/ggpull/diverge", want: commits.divergeCommitA},
		{ref: "refs/ggpull/newbranch", want: commits.originalMain},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/gg-old/delbranch", wantGone: true},
		{ref: "refs/gg-old/delbranch-local", wantGone: true},
		{ref: "refs/heads/main", want: commits.originalMain, wantRemote: "origin", wantMerge: "refs/heads/main"},
		{ref: "refs/heads/local", want: commits.localCommit, wantRemote: "origin", wantMerge: "refs/heads/local"},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB, wantRemote: "origin", wantMerge: "refs/heads/diverge"},
		{ref: "refs/heads/newbranch", want: commits.originalMain},
		{ref: "refs/heads/delbranch", want: commits.originalMain, wantRemote: "origin", wantMerge: "refs/heads/delbranch"},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit, wantRemote: "origin", wantMerge: "refs/heads/delbranch-local"},
		{ref: "refs/tags/zero", want: commits.originalMain},
		{ref: "refs/tags/first", want: commits.originalMain},
		{ref: "refs/tags/second", want: commits.newMain},
	}
	for _, check := range refChecks {
		if branch := check.ref.Branch(); branch != "" {
			gotRemote := cfg.Value("branch." + branch + ".remote")
			gotMerge := cfg.Value("branch." + branch + ".merge")
			if gotRemote != check.wantRemote || gotMerge != check.wantMerge {
				t.Errorf("branch %q remote = %q, merge = %q; want remote = %q, merge = %q", branch, gotRemote, gotMerge, check.wantRemote, check.wantMerge)
			}
		}
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestPullUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull", "-u"); err != nil {
		t.Error(err)
	}

	gitB := env.git.WithDir(repoBPath)
	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref      git.Ref
		want     git.Hash
		wantGone bool
	}{
		{ref: git.Head, want: commits.newMain},
		{ref: "refs/remotes/origin/main", want: commits.newMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.divergeCommitA},
		{ref: "refs/remotes/origin/newbranch", want: commits.originalMain},
		{ref: "refs/remotes/origin/delbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch-local", wantGone: true},
		{ref: "refs/ggpull/main", wantGone: true},
		{ref: "refs/ggpull/local", wantGone: true},
		{ref: "refs/ggpull/diverge", wantGone: true},
		{ref: "refs/ggpull/newbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/gg-old/delbranch", want: commits.originalMain},
		{ref: "refs/heads/main", want: commits.newMain},
		{ref: "refs/heads/local", want: commits.localCommit},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB},
		{ref: "refs/heads/newbranch", want: commits.originalMain},
		{ref: "refs/heads/delbranch", wantGone: true},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit},
		{ref: "refs/tags/zero", want: commits.originalMain},
		{ref: "refs/tags/first", want: commits.originalMain},
		{ref: "refs/tags/second", want: commits.newMain},
	}
	for _, check := range refChecks {
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestPullRev(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull", "-r", "diverge"); err != nil {
		t.Error(err)
	}

	gitB := env.git.WithDir(repoBPath)
	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref      git.Ref
		want     git.Hash
		wantGone bool
	}{
		{ref: git.Head, want: commits.originalMain},
		{ref: "refs/remotes/origin/main", want: commits.originalMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.divergeCommitA},
		{ref: "refs/remotes/origin/newbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch", want: commits.originalMain},
		{ref: "refs/remotes/origin/delbranch-local", want: commits.originalMain},
		{ref: "refs/ggpull/main", wantGone: true},
		{ref: "refs/ggpull/local", wantGone: true},
		{ref: "refs/ggpull/diverge", wantGone: true},
		{ref: "refs/ggpull/newbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/heads/main", want: commits.originalMain},
		{ref: "refs/heads/local", want: commits.localCommit},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB},
		{ref: "refs/heads/delbranch", want: commits.originalMain},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit},
	}
	for _, check := range refChecks {
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestPullRevTag(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull", "-r", "second"); err != nil {
		t.Error(err)
	}

	gitB := env.git.WithDir(repoBPath)
	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref      git.Ref
		want     git.Hash
		wantGone bool
	}{
		{ref: git.Head, want: commits.originalMain},
		{ref: "refs/remotes/origin/main", want: commits.originalMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.originalMain},
		{ref: "refs/remotes/origin/newbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch", want: commits.originalMain},
		{ref: "refs/remotes/origin/delbranch-local", want: commits.originalMain},
		{ref: "refs/ggpull/main", wantGone: true},
		{ref: "refs/ggpull/local", wantGone: true},
		{ref: "refs/ggpull/diverge", wantGone: true},
		{ref: "refs/ggpull/newbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/gg-old/delbranch", wantGone: true},
		{ref: "refs/gg-old/delbranch-local", wantGone: true},
		{ref: "refs/heads/main", want: commits.originalMain},
		{ref: "refs/heads/local", want: commits.localCommit},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB},
		{ref: "refs/heads/delbranch", want: commits.originalMain},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit},
		{ref: "refs/tags/zero", want: commits.originalMain},
		{ref: "refs/tags/first", want: commits.originalMain},
		{ref: "refs/tags/second", want: commits.newMain},
	}
	for _, check := range refChecks {
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestPullDeletedRev(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull", "-r", "delbranch"); err != nil {
		t.Error(err)
	}

	gitB := env.git.WithDir(repoBPath)
	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref      git.Ref
		want     git.Hash
		wantGone bool
	}{
		{ref: git.Head, want: commits.originalMain},
		{ref: "refs/remotes/origin/main", want: commits.originalMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.originalMain},
		{ref: "refs/remotes/origin/newbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch-local", want: commits.originalMain},
		{ref: "refs/ggpull/main", wantGone: true},
		{ref: "refs/ggpull/local", wantGone: true},
		{ref: "refs/ggpull/diverge", wantGone: true},
		{ref: "refs/ggpull/newbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/gg-old/delbranch", want: commits.originalMain},
		{ref: "refs/gg-old/delbranch-local", wantGone: true},
		{ref: "refs/heads/main", want: commits.originalMain},
		{ref: "refs/heads/local", want: commits.localCommit},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB},
		{ref: "refs/heads/delbranch", wantGone: true},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit},
	}
	for _, check := range refChecks {
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestPullDeletedRefCheckedOut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	gitB := env.git.WithDir(repoBPath)
	if err := gitB.CheckoutBranch(ctx, "delbranch", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, repoBPath, "pull"); err != nil {
		t.Error(err)
	}

	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := gitB.ReadConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref        git.Ref
		want       git.Hash
		wantGone   bool
		wantRemote string
		wantMerge  string
	}{
		{ref: git.Head, want: commits.originalMain},
		{ref: "refs/remotes/origin/main", want: commits.newMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.divergeCommitA},
		{ref: "refs/remotes/origin/newbranch", want: commits.originalMain},
		{ref: "refs/remotes/origin/delbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch-local", wantGone: true},
		{ref: "refs/ggpull/main", wantGone: true},
		{ref: "refs/ggpull/local", wantGone: true},
		{ref: "refs/ggpull/diverge", wantGone: true},
		{ref: "refs/ggpull/newbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/gg-old/delbranch", want: commits.originalMain},
		{ref: "refs/heads/main", want: commits.newMain, wantRemote: "origin", wantMerge: "refs/heads/main"},
		{ref: "refs/heads/local", want: commits.localCommit, wantRemote: "origin", wantMerge: "refs/heads/local"},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB, wantRemote: "origin", wantMerge: "refs/heads/diverge"},
		{ref: "refs/heads/newbranch", want: commits.originalMain, wantRemote: "origin", wantMerge: "refs/heads/newbranch"},
		{ref: "refs/heads/delbranch", want: commits.originalMain},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit, wantRemote: "origin", wantMerge: "refs/heads/delbranch-local"},
		{ref: "refs/tags/zero", want: commits.originalMain},
		{ref: "refs/tags/first", want: commits.originalMain},
		{ref: "refs/tags/second", want: commits.newMain},
	}
	for _, check := range refChecks {
		if branch := check.ref.Branch(); branch != "" {
			gotRemote := cfg.Value("branch." + branch + ".remote")
			gotMerge := cfg.Value("branch." + branch + ".merge")
			if gotRemote != check.wantRemote || gotMerge != check.wantMerge {
				t.Errorf("branch %q remote = %q, merge = %q; want remote = %q, merge = %q", branch, gotRemote, gotMerge, check.wantRemote, check.wantMerge)
			}
		}
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestPullChangedTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	if err := gitA.Run(ctx, "tag", "--force", "first", commits.newMain.String()); err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull"); err == nil {
		t.Error("pull did not return an error")
	} else {
		t.Log("pull error:", err)
	}

	gitB := env.git.WithDir(repoBPath)
	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref      git.Ref
		want     git.Hash
		wantGone bool
	}{
		{ref: git.Head, want: commits.originalMain},
		{ref: "refs/remotes/origin/main", want: commits.originalMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.originalMain},
		{ref: "refs/remotes/origin/newbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch", want: commits.originalMain},
		{ref: "refs/remotes/origin/delbranch-local", want: commits.originalMain},
		{ref: "refs/ggpull/main", wantGone: true},
		{ref: "refs/ggpull/local", wantGone: true},
		{ref: "refs/ggpull/diverge", wantGone: true},
		{ref: "refs/ggpull/newbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/gg-old/delbranch", wantGone: true},
		{ref: "refs/heads/main", want: commits.originalMain},
		{ref: "refs/heads/local", want: commits.localCommit},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB},
		{ref: "refs/heads/newbranch", wantGone: true},
		{ref: "refs/heads/delbranch", want: commits.originalMain},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit},
		{ref: "refs/tags/zero", want: commits.originalMain},
		{ref: "refs/tags/first", want: commits.originalMain},
		{ref: "refs/tags/second", wantGone: true},
	}
	for _, check := range refChecks {
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestPullForceTags(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	commits, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	repoAPath := env.root.FromSlash("repoA")
	gitA := env.git.WithDir(repoAPath)
	if err := gitA.Run(ctx, "tag", "--force", "first", commits.newMain.String()); err != nil {
		t.Fatal(err)
	}

	// Call gg to pull from A into B.
	repoBPath := env.root.FromSlash("repoB")
	if _, err := env.gg(ctx, repoBPath, "pull", "--force-tags"); err != nil {
		t.Error(err)
	}

	gitB := env.git.WithDir(repoBPath)
	refs, err := gitB.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	refChecks := []struct {
		ref      git.Ref
		want     git.Hash
		wantGone bool
	}{
		{ref: git.Head, want: commits.originalMain},
		{ref: "refs/remotes/origin/main", want: commits.newMain},
		{ref: "refs/remotes/origin/local", want: commits.originalMain},
		{ref: "refs/remotes/origin/diverge", want: commits.divergeCommitA},
		{ref: "refs/remotes/origin/newbranch", want: commits.originalMain},
		{ref: "refs/remotes/origin/delbranch", wantGone: true},
		{ref: "refs/remotes/origin/delbranch-local", wantGone: true},
		{ref: "refs/ggpull/main", wantGone: true},
		{ref: "refs/ggpull/local", wantGone: true},
		{ref: "refs/ggpull/diverge", wantGone: true},
		{ref: "refs/ggpull/newbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch", wantGone: true},
		{ref: "refs/ggpull/delbranch-local", wantGone: true},
		{ref: "refs/gg-old/delbranch", want: commits.originalMain},
		{ref: "refs/heads/main", want: commits.originalMain},
		{ref: "refs/heads/local", want: commits.localCommit},
		{ref: "refs/heads/diverge", want: commits.divergeCommitB},
		{ref: "refs/heads/newbranch", want: commits.originalMain},
		{ref: "refs/heads/delbranch", wantGone: true},
		{ref: "refs/heads/delbranch-local", want: commits.localCommit},
		{ref: "refs/tags/zero", want: commits.originalMain},
		{ref: "refs/tags/first", want: commits.newMain},
		{ref: "refs/tags/second", want: commits.newMain},
	}
	for _, check := range refChecks {
		got, exists := refs[check.ref]
		if !exists {
			if !check.wantGone {
				t.Errorf("%v is missing; want %s", check.ref, prettyCommit(check.want, commits.Names()))
			}
			continue
		}
		if check.wantGone {
			t.Errorf("%v = %s; should not exist", check.ref, prettyCommit(got, commits.Names()))
			continue
		}
		if got != check.want {
			names := commits.Names()
			t.Errorf("%v = %s; want %s", check.ref, prettyCommit(got, names), prettyCommit(check.want, names))
		}
	}
}

func TestIsRefOrphaned(t *testing.T) {
	remotes := map[string]*git.Remote{
		"origin": {
			Name:  "origin",
			Fetch: []git.FetchRefspec{"+refs/heads/*:refs/remotes/origin/*"},
		},
		"myfork": {
			Name:  "myfork",
			Fetch: []git.FetchRefspec{"+refs/heads/*:refs/remotes/myfork/*"},
		},
	}
	localRefs := map[git.Ref]git.Hash{
		"refs/heads/main":          hashLiteral("9473ef855db1cbac6ea012b8aee4929978e751f9"),
		"refs/remotes/origin/main": hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),

		"refs/heads/debian":          hashLiteral("bbcece7808eabb8e2df284b7111af2c75f6f4f93"),
		"refs/remotes/origin/debian": hashLiteral("bbcece7808eabb8e2df284b7111af2c75f6f4f93"),

		"refs/heads/fix":          hashLiteral("4a241fd101add0050f367872eaac27f4b721d692"),
		"refs/remotes/origin/fix": hashLiteral("4a241fd101add0050f367872eaac27f4b721d692"),
		"refs/remotes/myfork/fix": hashLiteral("4a241fd101add0050f367872eaac27f4b721d692"),
	}
	tests := []struct {
		name           string
		currRemoteName string
		ref            git.Ref
		want           bool
	}{
		{
			name:           "UnchangedSingleRemote",
			currRemoteName: "origin",
			ref:            "refs/heads/debian",
			want:           true,
		},
		{
			name:           "ChangedSingleRemote",
			currRemoteName: "origin",
			ref:            "refs/heads/main",
			want:           false,
		},
		{
			name:           "UnchangedMultipleRemotes",
			currRemoteName: "origin",
			ref:            "refs/heads/fix",
			want:           false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := isRefOrphaned(remotes, localRefs, test.currRemoteName, test.ref)
			if got != test.want {
				t.Errorf("isRefOrphaned(..., %q, %q) = %t; want %t", test.currRemoteName, test.ref, got, test.want)
			}
		})
	}
}

func TestReverseFetchMap(t *testing.T) {
	tests := []struct {
		name      string
		fetch     []git.FetchRefspec
		localRefs map[git.Ref]git.Hash
		want      map[git.Ref]git.Hash
	}{
		{
			name: "Empty",
		},
		{
			name:  "Basic",
			fetch: []git.FetchRefspec{"+refs/heads/*:refs/remotes/origin/*"},
			localRefs: map[git.Ref]git.Hash{
				"refs/heads/main":            hashLiteral("9473ef855db1cbac6ea012b8aee4929978e751f9"),
				"refs/remotes/origin/HEAD":   hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/remotes/origin/main":   hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/remotes/origin/debian": hashLiteral("bbcece7808eabb8e2df284b7111af2c75f6f4f93"),
			},
			want: map[git.Ref]git.Hash{
				"refs/heads/main":   hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/heads/debian": hashLiteral("bbcece7808eabb8e2df284b7111af2c75f6f4f93"),
			},
		},
		{
			name:  "Fixed",
			fetch: []git.FetchRefspec{"+FOO:refs/remotes/origin/FOO"},
			localRefs: map[git.Ref]git.Hash{
				"refs/heads/main":            hashLiteral("9473ef855db1cbac6ea012b8aee4929978e751f9"),
				"refs/remotes/origin/FOO":    hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/remotes/origin/HEAD":   hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/remotes/origin/main":   hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/remotes/origin/debian": hashLiteral("bbcece7808eabb8e2df284b7111af2c75f6f4f93"),
			},
			want: map[git.Ref]git.Hash{
				"FOO": hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
			},
		},
		{
			name:  "FixedAmbiguous",
			fetch: []git.FetchRefspec{"+refs/heads/*:refs/remotes/origin/*", "+FOO:refs/remotes/origin/FOO"},
			localRefs: map[git.Ref]git.Hash{
				"refs/heads/main":            hashLiteral("9473ef855db1cbac6ea012b8aee4929978e751f9"),
				"refs/remotes/origin/FOO":    hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/remotes/origin/HEAD":   hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/remotes/origin/main":   hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/remotes/origin/debian": hashLiteral("bbcece7808eabb8e2df284b7111af2c75f6f4f93"),
			},
			want: map[git.Ref]git.Hash{
				"FOO":               hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/heads/main":   hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/heads/FOO":    hashLiteral("561a0c1274efd215f8565478539832f85605ac99"),
				"refs/heads/debian": hashLiteral("bbcece7808eabb8e2df284b7111af2c75f6f4f93"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := reverseFetchMap(test.fetch, test.localRefs)
			if diff := cmp.Diff(test.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("refs (-want +got):\n%s", diff)
			}
		})
	}
}

func hashLiteral(s string) git.Hash {
	var h git.Hash
	if err := h.UnmarshalText([]byte(s)); err != nil {
		panic(err)
	}
	return h
}
