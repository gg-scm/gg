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

package git

import (
	"context"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestCommit(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	t.Run("DefaultOptions", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}

		// Create the parent commit.
		const (
			addContent  = "And now...\n"
			modifiedOld = "The Larch\n"
			modifiedNew = "The Chestnut\n"
		)
		err = env.root.Apply(
			filesystem.Write("modified_unstaged.txt", modifiedOld),
			filesystem.Write("modified_staged.txt", modifiedOld),
			filesystem.Write("deleted.txt", dummyContent),
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"modified_unstaged.txt", "modified_staged.txt", "deleted.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		// (Use command-line directly, so as not to depend on system-under-test.)
		err = env.g.Run(ctx, "commit", "-m", "initial import")
		if err != nil {
			t.Fatal(err)
		}
		r1, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// Arrange working copy changes.
		err = env.root.Apply(
			filesystem.Write("modified_unstaged.txt", modifiedNew),
			filesystem.Write("modified_staged.txt", modifiedNew),
			filesystem.Write("added.txt", addContent),
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"added.txt", "modified_staged.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		// TODO(soon): Replace this with a call to env.g.Remove.
		if err := env.g.Run(ctx, "rm", "deleted.txt"); err != nil {
			t.Fatal(err)
		}

		// Call g.Commit.
		const wantMessage = "\n\ninternal/git made this commit\n\n"
		if err := env.g.Commit(ctx, wantMessage, CommitOptions{}); err != nil {
			t.Error("Commit error:", err)
		}

		// Verify that HEAD was moved to a new commit.
		r2, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if r2.Commit() == r1.Commit() {
			t.Error("new HEAD = initial import")
		}
		if r2.Ref() != r1.Ref() {
			t.Errorf("HEAD ref = %s; want %s", r2.Ref(), r1.Ref())
		}
		// Verify that the commit's parent is the initial commit.
		if parent, err := env.g.ParseRev(ctx, "HEAD~"); err != nil {
			t.Error(err)
		} else if parent.Commit() != r1.Commit() {
			t.Errorf("HEAD~ = %v; want %v", parent.Commit(), r1.Commit())
		}

		// TODO(soon): Verify contents of commit.
		// TODO(soon): Verify working copy state.
		// TODO(soon): Verify commit message.
	})
	t.Run("Empty", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}

		// Call g.Commit.
		const wantMessage = "\n\ninternal/git made this commit\n\n"
		if err := env.g.Commit(ctx, wantMessage, CommitOptions{AllowEmpty: true}); err != nil {
			t.Error("Commit error:", err)
		}

		// Verify that HEAD was moved to a new commit.
		r, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if r.Ref() != "refs/heads/master" {
			t.Errorf("HEAD ref = %s; want refs/heads/master", r.Ref())
		}

		// TODO(soon): Verify commit message.
	})
	// TODO(soon): Add test for Pathspecs.
	// TODO(soon): Add test for All.
}
