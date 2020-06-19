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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"gg-scm.io/pkg/internal/git"
	"github.com/google/go-cmp/cmp"
)

func TestCommit_NoArgs(t *testing.T) {
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

	// Create the parent commit.
	const (
		addContent  = "And now...\n"
		modifiedOld = "The Larch\n"
		modifiedNew = "The Chestnut\n"
	)
	err = env.root.Apply(
		filesystem.Write("modified.txt", modifiedOld),
		filesystem.Write("deleted.txt", dummyContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "modified.txt", "deleted.txt"); err != nil {
		t.Fatal(err)
	}
	r1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Arrange working copy changes.
	err = env.root.Apply(
		filesystem.Write("modified.txt", modifiedNew),
		filesystem.Write("added.txt", addContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.trackFiles(ctx, "added.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Remove(ctx, []git.Pathspec{"deleted.txt"}, git.RemoveOptions{}); err != nil {
		t.Fatal(err)
	}

	// Call gg to make a commit.
	const wantMessage = "gg made this commit\n"
	if _, err := env.gg(ctx, env.root.String(), "commit", "-m", "gg made this commit"); err != nil {
		t.Fatal(err)
	}

	// Verify that a new commit was created and is parented to the first commit.
	r2, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Commit == r1 {
		t.Fatal("commit did not create a new commit in the working copy")
	}
	if ref := r2.Ref; ref != "refs/heads/main" {
		t.Errorf("HEAD ref = %q; want refs/heads/main", ref)
	}
	if parent, err := env.git.ParseRev(ctx, "HEAD~"); err != nil {
		t.Error(err)
	} else if parent.Commit != r1 {
		t.Errorf("HEAD~ = %v; want %v", parent.Commit, r1)
	}

	// Verify that the commit incorporated all changes from the working copy.
	if data, err := catBlob(ctx, env.git, r2.Commit.String(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != addContent {
		t.Errorf("added.txt = %q; want %q", data, addContent)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit.String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != modifiedNew {
		t.Errorf("modified.txt = %q; want %q", data, modifiedNew)
	}
	if err := objectExists(ctx, env.git, r2.Commit.String(), "deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}

	// Verify that the commit message matches the given message.
	if info, err := env.git.CommitInfo(ctx, r2.Commit.String()); err != nil {
		t.Error(err)
	} else if info.Message != wantMessage {
		t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
	}
}

func TestCommit_Selective(t *testing.T) {
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

	// Create the parent commit.
	const (
		modifiedOld = "The Larch\n"
		modifiedNew = "The Chestnut\n"
	)
	err = env.root.Apply(
		filesystem.Write("modified.txt", modifiedOld),
		filesystem.Write("deleted.txt", dummyContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "modified.txt", "deleted.txt"); err != nil {
		t.Fatal(err)
	}
	r1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Arrange working copy changes.
	err = env.root.Apply(
		filesystem.Write("modified.txt", modifiedNew),
		filesystem.Write("added.txt", dummyContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.trackFiles(ctx, "added.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Remove(ctx, []git.Pathspec{"deleted.txt"}, git.RemoveOptions{}); err != nil {
		t.Fatal(err)
	}

	// Call gg to make a commit.
	const wantMessage = "gg made this commit\n"
	if _, err := env.gg(ctx, env.root.String(), "commit", "-m", "gg made this commit", "modified.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that a new commit was created and is parented to the first commit.
	r2, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Commit == r1 {
		t.Fatal("commit did not create a new commit in the working copy")
	}
	if ref := r2.Ref; ref != "refs/heads/main" {
		t.Errorf("HEAD ref = %q; want refs/heads/main", ref)
	}
	if parent, err := env.git.ParseRev(ctx, "HEAD~"); err != nil {
		t.Error(err)
	} else if parent.Commit != r1 {
		t.Errorf("HEAD~ = %v; want %v", parent.Commit, r1)
	}

	// Verify that the commit only changed modified.txt.
	if data, err := catBlob(ctx, env.git, r2.Commit.String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != modifiedNew {
		t.Errorf("modified.txt = %q; want %q", data, modifiedNew)
	}
	if err := objectExists(ctx, env.git, r2.Commit.String(), "added.txt"); err == nil {
		t.Error("added.txt was added but not put in arguments")
	}
	if err := objectExists(ctx, env.git, r2.Commit.String(), "deleted.txt"); err != nil {
		t.Error("deleted.txt was removed but not put in arguments:", err)
	}

	// Verify that the commit message matches the given message.
	if info, err := env.git.CommitInfo(ctx, r2.Commit.String()); err != nil {
		t.Error(err)
	} else if info.Message != wantMessage {
		t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
	}
}

func TestCommit_SelectiveWrongFile(t *testing.T) {
	// Regression test for https://github.com/zombiezen/gg/issues/63

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
	r, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "commit", "-m", "bad", "bar.txt"); err == nil {
		t.Error("gg did not return error")
	} else if isUsage(err) {
		t.Fatal(err)
	}
	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if curr.Commit != r.Commit {
		t.Error("Created a new commit; wanted no-op")
	}
}

func TestCommit_PartialWrongFile(t *testing.T) {
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
	r, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "commit", "-m", "foo.txt", "bad", "bar.txt"); err == nil {
		t.Error("gg did not return error")
	} else if isUsage(err) {
		t.Fatal(err)
	}
	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if curr.Commit != r.Commit {
		t.Error("Created a new commit; wanted no-op")
	}
}

func TestCommit_Amend(t *testing.T) {
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

	// Create the first commit with modified.txt and deleted.txt.
	const (
		addContent   = "It's...\n"
		modifiedInit = "And now...\n"
		modifiedOld  = "The Larch\n"
		modifiedNew  = "The Chestnut\n"
	)
	err = env.root.Apply(
		filesystem.Write("modified.txt", modifiedInit),
		filesystem.Write("deleted.txt", dummyContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "modified.txt", "deleted.txt"); err != nil {
		t.Fatal(err)
	}
	parent, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Create a second commit with a small change to modified.txt.
	// This is the commit that will be amended.
	if err := env.root.Apply(filesystem.Write("modified.txt", modifiedOld)); err != nil {
		t.Fatal(err)
	}
	r1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Arrange working copy changes.
	err = env.root.Apply(
		filesystem.Write("modified.txt", modifiedNew),
		filesystem.Write("added.txt", addContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.trackFiles(ctx, "added.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Remove(ctx, []git.Pathspec{"deleted.txt"}, git.RemoveOptions{}); err != nil {
		t.Fatal(err)
	}

	// Call gg to make a commit.
	const wantMessage = "gg amended this commit\n"
	if _, err := env.gg(ctx, env.root.String(), "commit", "--amend", "-m", "gg amended this commit"); err != nil {
		t.Fatal(err)
	}

	// Verify that a new commit was created and has a parent of HEAD~.
	changes := map[git.Hash]string{
		parent: "parent commit",
		r1:     "tip",
	}
	r2, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Commit == r1 {
		t.Fatal("commit --amend did not create a new commit in the working copy")
	}
	if ref := r2.Ref; ref != "refs/heads/main" {
		t.Errorf("HEAD ref = %q; want refs/heads/main", ref)
	}
	if newParent, err := env.git.ParseRev(ctx, "HEAD~"); err != nil {
		t.Error(err)
	} else if newParent.Commit != parent {
		t.Errorf("HEAD~ after amend = %s; want %s",
			prettyCommit(newParent.Commit, changes),
			prettyCommit(parent, changes))
	}

	// Verify that the commit incorporated all the changes from the working copy.
	if data, err := catBlob(ctx, env.git, r2.Commit.String(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != addContent {
		t.Errorf("added.txt = %q; want %q", data, addContent)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit.String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != modifiedNew {
		t.Errorf("modified.txt = %q; want %q", data, modifiedNew)
	}
	if err := objectExists(ctx, env.git, r2.Commit.String(), "deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}

	// Verify that the commit message matches the given message.
	if info, err := env.git.CommitInfo(ctx, r2.Commit.String()); err != nil {
		t.Error(err)
	} else if info.Message != wantMessage {
		t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
	}
}

func TestCommit_AmendRootCommit(t *testing.T) {
	// Regression test for https://github.com/zombiezen/gg/issues/106

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

	// Create the first commit with modified.txt and deleted.txt.
	const (
		addContent  = "It's...\n"
		modifiedOld = "The Larch\n"
		modifiedNew = "The Chestnut\n"
	)
	err = env.root.Apply(
		filesystem.Write("modified.txt", modifiedOld),
		filesystem.Write("deleted.txt", dummyContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "modified.txt", "deleted.txt"); err != nil {
		t.Fatal(err)
	}
	r1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Arrange working copy changes.
	err = env.root.Apply(
		filesystem.Write("modified.txt", modifiedNew),
		filesystem.Write("added.txt", addContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.trackFiles(ctx, "added.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Remove(ctx, []git.Pathspec{"deleted.txt"}, git.RemoveOptions{}); err != nil {
		t.Fatal(err)
	}

	// Call gg to make a commit.
	const wantMessage = "gg amended this commit\n"
	if _, err := env.gg(ctx, env.root.String(), "commit", "--amend", "-m", "gg amended this commit"); err != nil {
		t.Error(err)
	}

	// Verify that a new commit was created and has a parent of HEAD~.
	r2, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changes := map[git.Hash]string{
		r1:        "first commit",
		r2.Commit: "amended commit",
	}
	if r2.Commit == r1 {
		t.Fatal("commit --amend did not create a new commit in the working copy")
	}
	if ref := r2.Ref; ref != "refs/heads/main" {
		t.Errorf("HEAD ref = %q; want refs/heads/main", ref)
	}
	if newParent, err := env.git.ParseRev(ctx, "HEAD~"); err == nil {
		t.Errorf("HEAD~ = %s; want error", prettyCommit(newParent.Commit, changes))
	}

	// Verify that the commit incorporated all the changes from the working copy.
	if data, err := catBlob(ctx, env.git, r2.Commit.String(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != addContent {
		t.Errorf("added.txt = %q; want %q", data, addContent)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit.String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != modifiedNew {
		t.Errorf("modified.txt = %q; want %q", data, modifiedNew)
	}
	if err := objectExists(ctx, env.git, r2.Commit.String(), "deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}

	// Verify that the commit message matches the given message.
	if info, err := env.git.CommitInfo(ctx, r2.Commit.String()); err != nil {
		t.Error(err)
	} else if info.Message != wantMessage {
		t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
	}
}

func TestCommit_NoChanges(t *testing.T) {
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
	r1, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "commit", "-m", "nothing to see here"); err == nil {
		t.Error("commit with no changes did not return error")
	} else if isUsage(err) {
		t.Errorf("commit with no changes returned usage error: %v", err)
	}
	r2, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Commit != r1.Commit {
		t.Errorf("commit created new commit %s; wanted to stay on %s", r2.Commit, r1.Commit)
	}
	if ref := r2.Ref; ref != "refs/heads/main" {
		t.Errorf("HEAD ref = %q; want refs/heads/main", ref)
	}
}

func TestCommit_AmendJustMessage(t *testing.T) {
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

	// Create the first commit with a file foo.txt.
	const (
		oldContent = "The Larch\n"
		newContent = "The Chestnut\n"
	)
	if err := env.root.Apply(filesystem.Write("foo.txt", oldContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	parent, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Create a second commit that changes foo.txt.
	if err := env.root.Apply(filesystem.Write("foo.txt", newContent)); err != nil {
		t.Fatal(err)
	}
	r1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Call gg to amend the commit.
	const wantMessage = "gg amended this commit\n"
	if _, err := env.gg(ctx, env.root.String(), "commit", "--amend", "-m", "gg amended this commit"); err != nil {
		t.Fatal(err)
	}

	// Verify that a new commit was created with the parent set to the parent of
	// the working copy's commit.
	changes := map[git.Hash]string{
		parent: "parent commit",
		r1:     "tip",
	}
	r2, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Commit == r1 {
		t.Fatal("commit --amend did not create a new commit in the working copy")
	}
	if ref := r2.Ref; ref != "refs/heads/main" {
		t.Errorf("HEAD ref = %q; want refs/heads/main", ref)
	}
	if newParent, err := env.git.ParseRev(ctx, "HEAD~"); err != nil {
		t.Error(err)
	} else if newParent.Commit != parent {
		t.Errorf("HEAD~ after amend = %s; want %s",
			prettyCommit(newParent.Commit, changes),
			prettyCommit(parent, changes))
	}

	// Verify that the commit message matches the one given.
	if info, err := env.git.CommitInfo(ctx, r2.Commit.String()); err != nil {
		t.Error(err)
	} else if info.Message != wantMessage {
		t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
	}

	if data, err := catBlob(ctx, env.git, r2.Commit.String(), "foo.txt"); err != nil {
		t.Error(err)
	} else if string(data) != newContent {
		t.Errorf("foo.txt = %q; want %q", data, newContent)
	}
}

func TestCommit_InSubdir(t *testing.T) {
	// Regression test for https://github.com/zombiezen/gg/issues/10

	t.Parallel()
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "NoArgs",
			args: nil,
		},
		{
			name: "Named",
			args: []string{
				filepath.Join("..", "added.txt"),
				filepath.Join("..", "deleted.txt"),
				filepath.Join("..", "modified.txt"),
			},
		},
	}
	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()
			if err := env.initEmptyRepo(ctx, "."); err != nil {
				t.Fatal(err)
			}

			// Create the first commit.
			const (
				addContent  = "And now...\n"
				modifiedOld = "The Larch\n"
				modifiedNew = "The Chestnut\n"
			)
			err = env.root.Apply(
				filesystem.Write("modified.txt", modifiedOld),
				filesystem.Write("deleted.txt", dummyContent),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := env.addFiles(ctx, "modified.txt", "deleted.txt"); err != nil {
				t.Fatal(err)
			}
			r1, err := env.newCommit(ctx, ".")
			if err != nil {
				t.Fatal(err)
			}

			// Arrange the changes to the working copy, including creating the foo directory.
			err = env.root.Apply(
				filesystem.Mkdir("foo"),
				filesystem.Write("modified.txt", modifiedNew),
				filesystem.Write("added.txt", addContent),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := env.trackFiles(ctx, "added.txt"); err != nil {
				t.Fatal(err)
			}
			if err := env.git.Remove(ctx, []git.Pathspec{"deleted.txt"}, git.RemoveOptions{}); err != nil {
				t.Fatal(err)
			}

			// Call gg to create the commit, appending the test case's arguments.
			const wantMessage = "gg made this commit\n"
			args := append([]string{"commit", "-m", "gg made this commit"}, test.args...)
			if _, err := env.gg(ctx, env.root.FromSlash("foo"), args...); err != nil {
				t.Fatal(err)
			}

			// Verify that a new commit was created with the parent of the working copy's commit.
			r2, err := env.git.Head(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if r2.Commit == r1 {
				t.Fatal("commit did not create a new commit in the working copy")
			}
			if ref := r2.Ref; ref != "refs/heads/main" {
				t.Errorf("HEAD ref = %q; want refs/heads/main", ref)
			}
			if parent, err := env.git.ParseRev(ctx, "HEAD~"); err != nil {
				t.Error(err)
			} else if parent.Commit != r1 {
				t.Errorf("HEAD~ = %v; want %v", parent.Commit, r1)
			}

			// Verify that the commit incorporates all the changes from the working copy.
			if data, err := catBlob(ctx, env.git, r2.Commit.String(), "added.txt"); err != nil {
				t.Error(err)
			} else if string(data) != addContent {
				t.Errorf("added.txt = %q; want %q", data, addContent)
			}
			if data, err := catBlob(ctx, env.git, r2.Commit.String(), "modified.txt"); err != nil {
				t.Error(err)
			} else if string(data) != modifiedNew {
				t.Errorf("modified.txt = %q; want %q", data, modifiedNew)
			}
			if err := objectExists(ctx, env.git, r2.Commit.String(), "deleted.txt"); err == nil {
				t.Error("deleted.txt exists")
			}

			// Verify that the commit message matches the one given.
			if info, err := env.git.CommitInfo(ctx, r2.Commit.String()); err != nil {
				t.Error(err)
			} else if info.Message != wantMessage {
				t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
			}
		})
	}
}

func TestCommit_Merge(t *testing.T) {
	// Regression test for https://github.com/zombiezen/gg/issues/38

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

	// Create the base commit.
	if err := env.root.Apply(filesystem.Write("foo.txt", "Base content\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	base, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Create a diverging commit on a feature branch.
	if err := env.git.NewBranch(ctx, "feature", git.BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "feature content\n")); err != nil {
		t.Fatal(err)
	}
	r2, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Create another commit on main.
	if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "boring text\n")); err != nil {
		t.Fatal(err)
	}
	r1, err := env.newCommit(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}

	// Run the merge using gg, since this is a multi-interaction integration test.
	out, err := env.gg(ctx, env.root.String(), "merge", "feature")
	if len(out) > 0 {
		t.Logf("merge output:\n%s", out)
	}
	if err == nil {
		t.Errorf("Wanted merge to return error (conflict). Output:\n%s", out)
	} else if isUsage(err) {
		t.Fatalf("merge returned usage error: %v", err)
	}

	// Resolve the conflict.
	if err := env.root.Apply(filesystem.Write("foo.txt", "merged content!\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root.String(), "add", "foo.txt"); err != nil {
		t.Error("add:", err)
	}

	// Commit the merge.
	out, err = env.gg(ctx, env.root.String(), "commit", "-m", "Merged feature into main")
	if len(out) > 0 {
		t.Logf("commit output:\n%s", out)
	}
	if err != nil {
		t.Error("commit:", err)
	}

	// Verify that a new commit was created with the main commit as the first
	// parent and the feature commit as the second parent.
	curr, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[git.Hash]string{
		base: "initial commit",
		r1:   "main commit",
		r2:   "branch commit",
	}
	if curr.Commit == base || curr.Commit == r1 || curr.Commit == r2 {
		t.Errorf("after merge commit, HEAD = %s; want new commit",
			prettyCommit(curr.Commit, names))
	}
	parent1, err := env.git.ParseRev(ctx, "HEAD^1")
	if err != nil {
		t.Fatal(err)
	}
	if parent1.Commit != r1 {
		t.Errorf("after merge commit, HEAD^1 = %s; want %s",
			prettyCommit(parent1.Commit, names),
			prettyCommit(r1, names))
	}
	parent2, err := env.git.ParseRev(ctx, "HEAD^2")
	if err != nil {
		t.Fatal(err)
	}
	if parent2.Commit != r2 {
		t.Errorf("after merge commit, HEAD^2 = %s; want %s",
			prettyCommit(parent2.Commit, names),
			prettyCommit(r2, names))
	}
}

// Regression test for https://github.com/zombiezen/gg/issues/74
func TestCommit_DirectoryWithUntracked(t *testing.T) {
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

	// Add bar.txt and baz.txt in foo directory. Only track bar.txt.
	err = env.root.Apply(
		filesystem.Write("foo/bar.txt", dummyContent),
		filesystem.Write("foo/baz.txt", dummyContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo/bar.txt"); err != nil {
		t.Fatal(err)
	}

	// Call gg to make a commit.
	if _, err := env.gg(ctx, env.root.String(), "commit", "-m", "first", "foo"); err != nil {
		t.Error(err)
	}

	// Verify that a new commit contains just foo/bar.txt.
	got, err := env.git.ListTree(ctx, "HEAD", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[git.TopPath]struct{}{
		"foo/bar.txt": {},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("HEAD tree (-want +got)\n%s", diff)
	}
}

func TestCommitMessageTemplate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	tests := []struct {
		name string

		status        []git.DiffStatusEntry
		amend         bool
		commentChar   string
		branchName    string
		headCommitMsg string
		mergeMsg      string

		want string
	}{
		{
			name: "ModifiedFile",
			status: []git.DiffStatusEntry{
				{Name: "foo/bar.txt", Code: git.DiffStatusModified},
			},
			commentChar: "#",
			branchName:  "main",
			want: "\n" + `
# Please enter a commit message.
# Lines starting with '#' will be ignored.
#
# branch main
# modified foo/bar.txt` + "\n",
		},
		{
			name: "MultipleFiles",
			status: []git.DiffStatusEntry{
				{Name: "foo/bar.txt", Code: git.DiffStatusModified},
				{Name: "uvw/xyz.txt", Code: git.DiffStatusDeleted},
				{Name: "abc/def.txt", Code: git.DiffStatusAdded},
			},
			commentChar: "#",
			branchName:  "main",
			want: "\n" + `
# Please enter a commit message.
# Lines starting with '#' will be ignored.
#
# branch main
# added abc/def.txt
# modified foo/bar.txt
# removed uvw/xyz.txt` + "\n",
		},
		{
			name: "DetachedHEAD",
			status: []git.DiffStatusEntry{
				{Name: "foo/bar.txt", Code: git.DiffStatusModified},
			},
			commentChar: "#",
			branchName:  "",
			want: "\n" + `
# Please enter a commit message.
# Lines starting with '#' will be ignored.
#
# detached HEAD
# modified foo/bar.txt` + "\n",
		},
		{
			name: "Amend",
			status: []git.DiffStatusEntry{
				{Name: "foo/bar.txt", Code: git.DiffStatusModified},
			},
			amend:         true,
			commentChar:   "#",
			branchName:    "main",
			headCommitMsg: "Original content\n",
			want: "Original content\n" + `
# Please enter a commit message.
# Lines starting with '#' will be ignored.
#
# branch main
# modified foo/bar.txt` + "\n",
		},
		{
			name: "Merge",
			status: []git.DiffStatusEntry{
				{Name: "foo/bar.txt", Code: git.DiffStatusModified},
			},
			commentChar: "#",
			branchName:  "main",
			mergeMsg:    "Merged remote-tracking branch 'refs/remotes/origin/main' into 'main'\n",
			want: "Merged remote-tracking branch 'refs/remotes/origin/main' into 'main'\n" + `
# Please enter a commit message.
# Lines starting with '#' will be ignored.
#
# branch main
# modified foo/bar.txt` + "\n",
		},
	}

	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			headCommitMsg := test.headCommitMsg
			if headCommitMsg == "" {
				headCommitMsg = "fixed commit message for reproducibility\n"
			}
			err := env.git.AmendFiles(ctx, nil, git.AmendOptions{
				Message: headCommitMsg,
			})
			if err != nil {
				t.Fatal(err)
			}
			if test.branchName == "" {
				// Detached HEAD.
				if err := env.git.CheckoutRev(ctx, "HEAD", git.CheckoutOptions{}); err != nil {
					t.Error(err)
				}
				defer func() {
					if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
						t.Error(err)
					}
				}()
			} else if test.branchName != "main" {
				err := env.git.NewBranch(ctx, test.branchName, git.BranchOptions{
					Checkout:  true,
					Overwrite: true,
				})
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
						t.Error(err)
					}
				}()
			}
			if test.mergeMsg != "" {
				err := env.root.Apply(filesystem.Write(".git/MERGE_MSG", test.mergeMsg))
				defer func() {
					if err := env.root.Apply(filesystem.Remove(".git/MERGE_MSG")); err != nil {
						t.Error(err)
					}
				}()
				if err != nil {
					t.Fatal(err)
				}
			}

			buf := new(bytes.Buffer)
			if test.amend {
				buf.WriteString(headCommitMsg)
			} else {
				buf.Write(maybeMergeMessage(ctx, env.git))
			}
			err = commitMessageTemplate(ctx, env.git, test.status, buf, test.commentChar)
			if err != nil {
				t.Fatal("commitMessageTemplate:", err)
			}
			if got := buf.String(); got != test.want {
				t.Errorf("commitMessageTemplate(...) =\n%s*** want: ***\n%s", got, test.want)
			}
		})
	}
}

func TestCleanupMessage(t *testing.T) {
	tests := []struct {
		in          string
		commentChar string
		want        string
	}{
		{"", "#", ""},
		{"\n\n", "#", ""},
		{"\r\n\r\n", "#", ""},
		{"\n\n# This is a commit message.\n", "#", ""},
		{"Hello, World!", "#", "Hello, World!\n"},
		{"Hello, World!\n", "#", "Hello, World!\n"},
		{"Hello, World!\r\n", "#", "Hello, World!\n"},
		{"Hello, World! \t \r\n", "#", "Hello, World!\n"},
		{"Hello, World!\n\n", "#", "Hello, World!\n"},
		{"Hello, World!\nNext", "#", "Hello, World!\nNext\n"},
		{"Hello, World!\nNext\n", "#", "Hello, World!\nNext\n"},
		{"Hello, World!\n\nNext\n", "#", "Hello, World!\n\nNext\n"},
		{"Hello, World!\n   \nNext\n", "#", "Hello, World!\n\nNext\n"},
		{"  Indent", "#", "  Indent\n"},
		{"# This is a comment.", "#", ""},
		{"# This is a comment.\n", "#", ""},
		{"# This is a comment.\n\n", "#", ""},
		{"! This is a comment.", "!", ""},
		{"! This is a comment.\n", "!", ""},
		{"! This is a comment.\n\n", "!", ""},
		{"# This is not a comment.", "!", "# This is not a comment.\n"},
		{"# This is not a comment.\n", "!", "# This is not a comment.\n"},
		{"# This is not a comment.\n\n", "!", "# This is not a comment.\n"},
		{"# This is not a comment.", "", "# This is not a comment.\n"},
		{"# This is not a comment.\n", "", "# This is not a comment.\n"},
		{"# This is not a comment.\n\n", "", "# This is not a comment.\n"},
		{" # Not a comment\n", "#", " # Not a comment\n"},
		{"Foo\n\n# This is a commit message.\n", "#", "Foo\n"},
	}
	for _, test := range tests {
		if got := cleanupMessage(test.in, test.commentChar); got != test.want {
			t.Errorf("cleanupMessage(%q, %q) = %q; want %q", test.in, test.commentChar, got, test.want)
		}
	}
}

func catBlob(ctx context.Context, g *git.Git, rev string, path git.TopPath) ([]byte, error) {
	r, err := g.Cat(ctx, rev, path)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(r)
	closeErr := r.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return data, nil
}

func objectExists(ctx context.Context, g *git.Git, rev string, path git.TopPath) error {
	tree, err := g.ListTree(ctx, rev, nil)
	if err != nil {
		return err
	}
	if _, exists := tree[path]; !exists {
		return fmt.Errorf("object %s:%s does not exist", rev, path)
	}
	return nil
}
