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

func TestMerge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}
	baseRev, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Make a change on a feature branch.
	if err := env.git.NewBranch(ctx, "feature", git.BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	feature, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Make a non-conflicting change on main.
	if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "bar.txt"); err != nil {
		t.Fatal(err)
	}
	upstream, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to start merge of feature branch into main branch.
	out, err := env.gg(ctx, env.root.String(), "merge", "feature")
	if len(out) > 0 {
		t.Logf("merge output:\n%s", out)
	}
	if err != nil {
		t.Error("merge:", err)
	}

	// Verify that HEAD is still the upstream commit. gg should not create a new commit.
	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[git.Hash]string{
		baseRev.Commit: "initial commit",
		upstream:       "main commit",
		feature:        "branch commit",
	}
	if curr.Commit != upstream {
		t.Errorf("after merge, HEAD = %s; want %s",
			prettyCommit(curr.Commit, names),
			prettyCommit(upstream, names))
	}

	// Verify that the to-be-merged commit is the feature branch.
	mergeHead, err := env.git.ParseRev(ctx, "MERGE_HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if mergeHead.Commit != feature {
		t.Errorf("after merge, MERGE_HEAD = %s; want %s",
			prettyCommit(curr.Commit, names),
			prettyCommit(feature, names))
	}

	// Verify that both of the changes are present in the working copy.
	if exists, err := env.root.Exists("foo.txt"); !exists || err != nil {
		t.Errorf("foo.txt does not exist. error = %v", err)
	}
	if exists, err := env.root.Exists("bar.txt"); !exists || err != nil {
		t.Errorf("bar.txt does not exist. error = %v", err)
	}
}

func TestMerge_Conflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Create the first commit adding foo.txt.
	if err := env.root.Apply(filesystem.Write("foo.txt", "In the beginning...\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	base, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Make a change on a feature branch.
	if err := env.git.NewBranch(ctx, "feature", git.BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "feature content\n")); err != nil {
		t.Fatal(err)
	}
	feature, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Make a conflicting change on main.
	if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "boring text\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	upstream, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to merge the feature branch into the main branch.
	out, err := env.gg(ctx, env.root.String(), "merge", "feature")
	if len(out) > 0 {
		t.Logf("merge output:\n%s", out)
	}
	if err == nil {
		t.Error("merge did not return error")
	} else if isUsage(err) {
		t.Errorf("merge returned usage error: %v", err)
	}

	// Verify that HEAD is still the upstream commit. gg should not create a new commit.
	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[git.Hash]string{
		base:     "initial commit",
		upstream: "main commit",
		feature:  "branch commit",
	}
	if curr.Commit != upstream {
		t.Errorf("after merge, HEAD = %s; want %s",
			prettyCommit(curr.Commit, names),
			prettyCommit(upstream, names))
	}

	// Verify that the to-be-merged commit is the feature branch.
	mergeHead, err := env.git.ParseRev(ctx, "MERGE_HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if mergeHead.Commit != feature {
		t.Errorf("after merge, MERGE_HEAD = %s; want %s",
			prettyCommit(curr.Commit, names),
			prettyCommit(feature, names))
	}
}
