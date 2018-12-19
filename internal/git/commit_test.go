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
	"io"
	"strings"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestCommit(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	t.Run("Message", func(t *testing.T) {
		tests := []struct {
			name    string
			msg     string
			wantErr bool
		}{
			{name: "Empty", msg: "", wantErr: true},
			{name: "OneLineNoEOL", msg: "hello world"},
			{name: "OneLine", msg: "hello world\n"},
			{name: "OneLineUntrimmed", msg: " \t hello world\t \t\n"},
			{name: "TwoLinesNoEOL", msg: "hello\nworld"},
			{name: "TwoLines", msg: "hello\nworld\n"},
			{name: "ThreeLinesNoEOL", msg: "hello\nworld\n!"},
			{name: "ThreeLines", msg: "hello\nworld\n!\n"},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				env, err := newTestEnv(ctx, gitPath)
				if err != nil {
					t.Fatal(err)
				}
				defer env.cleanup()
				if err := env.g.Init(ctx, "."); err != nil {
					t.Fatal(err)
				}
				if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
					t.Fatal(err)
				}
				if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
					t.Fatal(err)
				}
				err = env.g.Commit(ctx, test.msg)
				if err != nil {
					if !test.wantErr {
						t.Error("Commit error:", err)
					}
					return
				}
				if test.wantErr {
					t.Fatal("Commit did not return error; want error")
				}
				info, err := env.g.CommitInfo(ctx, "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				if info.Message != test.msg {
					t.Errorf("message = %q; want %q", info.Message, test.msg)
				}
			})
		}
	})
	t.Run("LocalChanges", func(t *testing.T) {
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
		const wantMessage = "\n\ninternal/git made this commit"
		if err := env.g.Commit(ctx, wantMessage); err != nil {
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

		// Verify contents of commit.
		if tree, err := env.g.ListTree(ctx, "HEAD"); err != nil {
			t.Error(err)
		} else {
			if _, exists := tree["added.txt"]; !exists {
				t.Error("added.txt not present in commit")
			}
			if _, exists := tree["deleted.txt"]; exists {
				t.Error("deleted.txt present in commit")
			}
		}
		if got, err := catFile(ctx, env.g, "HEAD", "modified_staged.txt"); err != nil {
			t.Error(err)
		} else if got != modifiedNew {
			t.Errorf("modified_staged.txt @ HEAD = %q; want %q", got, modifiedNew)
		}
		if got, err := catFile(ctx, env.g, "HEAD", "modified_unstaged.txt"); err != nil {
			t.Error(err)
		} else if got != modifiedOld {
			t.Errorf("modified_unstaged.txt @ HEAD = %q; want %q", got, modifiedOld)
		}

		// Verify commit message.
		if info, err := env.g.CommitInfo(ctx, "HEAD"); err != nil {
			t.Error(err)
		} else if info.Message != wantMessage {
			t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
		}
	})
}

func TestCommitFiles(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	t.Run("Empty", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if version, err := env.g.getVersion(ctx); err == nil {
			// Versions of Git < 2.11.1 fail at creating empty commits.
			// Skip the test.
			skipPrefixes := []string{
				"git version 2.7",
				"git version 2.8",
				"git version 2.9",
				"git version 2.10",
				"git version 2.11",
			}
			for _, p := range skipPrefixes {
				if strings.HasPrefix(version, p) && (len(version) == len(p) || version[len(p)] == '.') {
					t.Skipf("Version = %q (<2.11.1); skipping", version)
				}
			}
			if strings.HasPrefix(version, "git version 2.11.0") {
				t.Skipf("Version = %q (<2.11.1); skipping", version)
			}
		}
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}

		// Call g.CommitFiles.
		const wantMessage = "\n\ninternal/git made this commit\n\n"
		if err := env.g.CommitFiles(ctx, wantMessage, nil); err != nil {
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

		// Verify commit message.
		info, err := env.g.CommitInfo(ctx, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if info.Message != wantMessage {
			t.Errorf("message = %q; want %q", info.Message, wantMessage)
		}
	})
	t.Run("Unstaged", func(t *testing.T) {
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
			oldContent = "The Larch\n"
			newContent = "The Chestnut\n"
		)
		err = env.root.Apply(
			filesystem.Write("unstaged.txt", oldContent),
			filesystem.Write("staged.txt", oldContent),
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"unstaged.txt", "staged.txt"}, AddOptions{}); err != nil {
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
			filesystem.Write("unstaged.txt", newContent),
			filesystem.Write("staged.txt", newContent),
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.g.Add(ctx, []Pathspec{"staged.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}

		// Call g.CommitFiles.
		const wantMessage = "\n\ninternal/git made this commit"
		if err := env.g.CommitFiles(ctx, wantMessage, []Pathspec{"unstaged.txt"}); err != nil {
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

		// Verify contents of commit.
		if got, err := catFile(ctx, env.g, "HEAD", "staged.txt"); err != nil {
			t.Error(err)
		} else if got != oldContent {
			t.Errorf("staged.txt @ HEAD = %q; want %q", got, oldContent)
		}
		if got, err := catFile(ctx, env.g, "HEAD", "unstaged.txt"); err != nil {
			t.Error(err)
		} else if got != newContent {
			t.Errorf("unstaged.txt @ HEAD = %q; want %q", got, newContent)
		}

		// Verify commit message.
		if info, err := env.g.CommitInfo(ctx, "HEAD"); err != nil {
			t.Error(err)
		} else if info.Message != wantMessage {
			t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
		}
	})
}

func TestCommitAll(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
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
		oldContent = "The Larch\n"
		newContent = "The Chestnut\n"
	)
	err = env.root.Apply(
		filesystem.Write("unstaged.txt", oldContent),
		filesystem.Write("staged.txt", oldContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.g.Add(ctx, []Pathspec{"unstaged.txt", "staged.txt"}, AddOptions{}); err != nil {
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
		filesystem.Write("unstaged.txt", newContent),
		filesystem.Write("staged.txt", newContent),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.g.Add(ctx, []Pathspec{"staged.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}

	// Call g.CommitAll.
	const wantMessage = "\n\ninternal/git made this commit"
	if err := env.g.CommitAll(ctx, wantMessage); err != nil {
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

	// Verify contents of commit.
	if got, err := catFile(ctx, env.g, "HEAD", "staged.txt"); err != nil {
		t.Error(err)
	} else if got != newContent {
		t.Errorf("staged.txt @ HEAD = %q; want %q", got, newContent)
	}
	if got, err := catFile(ctx, env.g, "HEAD", "unstaged.txt"); err != nil {
		t.Error(err)
	} else if got != newContent {
		t.Errorf("unstaged.txt @ HEAD = %q; want %q", got, newContent)
	}

	// Verify commit message.
	if info, err := env.g.CommitInfo(ctx, "HEAD"); err != nil {
		t.Error(err)
	} else if info.Message != wantMessage {
		t.Errorf("commit message = %q; want %q", info.Message, wantMessage)
	}
}

func catFile(ctx context.Context, g *Git, rev string, path TopPath) (string, error) {
	rc, err := g.Cat(ctx, rev, path)
	if err != nil {
		return "", err
	}
	sb := new(strings.Builder)
	if _, err := io.Copy(sb, rc); err != nil {
		rc.Close()
		return sb.String(), err
	}
	if err := rc.Close(); err != nil {
		return sb.String(), err
	}
	return sb.String(), nil
}
