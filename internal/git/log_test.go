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
	"strings"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestCommitInfo(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()

	t.Run("EmptyMaster", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		_, err = env.g.CommitInfo(ctx, "master")
		if err == nil {
			t.Error("CommitInfo did not return error", err)
		}
	})
	t.Run("FirstCommit", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with a single commit to foo.txt.
		// Uses raw commands, as CommitInfo is used to verify the state of other APIs.
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
			t.Fatal(err)
		}
		// Message does not have trailing newline to verify verbatim processing.
		const wantMsg = "\t foobarbaz  \r\n\n  initial import  "
		{
			c := env.g.Command(ctx, "commit", "--cleanup=verbatim", "--file=-")
			c.Stdin = strings.NewReader(wantMsg)
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}
		info, err := env.g.CommitInfo(ctx, "HEAD")
		if err != nil {
			t.Fatal("CommitInfo:", err)
		}
		if info.Hash == (Hash{}) {
			t.Errorf("info.Hash = %v; want non-zero", info.Hash)
		}
		if len(info.Parents) > 0 {
			t.Errorf("info.Parents = %v; want []", info.Parents)
		}
		if info.Message != wantMsg {
			t.Errorf("info.Message = %q; want %q", info.Message, wantMsg)
		}
	})
	t.Run("SecondCommit", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with two commits.
		// Uses raw commands, as CommitInfo is used to verify the state of other APIs.
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "commit", "-m", "initial import"); err != nil {
			t.Fatal(err)
		}
		rev1, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.Remove(ctx, []Pathspec{"foo.txt"}, RemoveOptions{}); err != nil {
			t.Fatal(err)
		}
		// Message does not have trailing newline to verify verbatim processing.
		const wantMsg = "\t foobarbaz  \r\n\n  the second commit  "
		{
			c := env.g.Command(ctx, "commit", "--quiet", "--cleanup=verbatim", "--file=-")
			c.Stdin = strings.NewReader(wantMsg)
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}

		info, err := env.g.CommitInfo(ctx, "HEAD")
		if err != nil {
			t.Fatal("CommitInfo:", err)
		}
		if info.Hash == (Hash{}) || info.Hash == rev1.Commit() {
			t.Errorf("info.Hash = %v; want non-zero and != %v", info.Hash, rev1.Commit())
		}
		if len(info.Parents) != 1 || info.Parents[0] != rev1.Commit() {
			t.Errorf("info.Parents = %v; want %v", info.Parents, []Hash{rev1.Commit()})
		}
		if info.Message != wantMsg {
			t.Errorf("info.Message = %q; want %q", info.Message, wantMsg)
		}
	})
	t.Run("MergeCommit", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with a merge commit.
		// Uses raw commands, as CommitInfo is used to verify the state of other APIs.
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "commit", "-m", "initial import"); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "bar.txt"); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "commit", "-m", "first parent"); err != nil {
			t.Fatal(err)
		}
		rev1, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "checkout", "--quiet", "-b", "diverge", "HEAD~"); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("baz.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "baz.txt"); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "commit", "-m", "second parent"); err != nil {
			t.Fatal(err)
		}
		rev2, err := env.g.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "checkout", "--quiet", "master"); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "merge", "diverge"); err != nil {
			t.Fatal(err)
		}

		info, err := env.g.CommitInfo(ctx, "HEAD")
		if err != nil {
			t.Fatal("CommitInfo:", err)
		}
		if len(info.Parents) != 2 || info.Parents[0] != rev1.Commit() || info.Parents[1] != rev2.Commit() {
			t.Errorf("info.Parents = %v; want %v", info.Parents, []Hash{rev1.Commit(), rev2.Commit()})
		}
	})
}
