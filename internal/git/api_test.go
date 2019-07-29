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

package git

import (
	"context"
	"os/exec"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"golang.org/x/xerrors"
)

func TestCommandError(t *testing.T) {
	falsePath, err := exec.LookPath("false")
	if err != nil {
		t.Skip("could not find false:", err)
	}
	exitError := exec.Command(falsePath).Run()
	if _, ok := exitError.(*exec.ExitError); !ok {
		t.Fatalf("ran %s, got: %v; want exit error", falsePath, exitError)
	}

	tests := []struct {
		prefix   string
		runError error
		stderr   string

		want string
	}{
		{
			prefix:   "git commit",
			runError: xerrors.New("could not start because reasons"),
			want:     "git commit: could not start because reasons",
		},
		{
			prefix:   "git commit",
			runError: exitError,
			want:     "git commit: " + exitError.Error(),
		},
		{
			prefix:   "git commit",
			runError: xerrors.New("could not copy I/O"),
			stderr:   "fatal: everything failed\n",
			want:     "git commit: could not copy I/O\nfatal: everything failed",
		},
		{
			prefix:   "git commit",
			runError: xerrors.New("could not copy I/O"),
			stderr:   "fatal: everything failed", // no trailing newline
			want:     "git commit: could not copy I/O\nfatal: everything failed",
		},
		{
			prefix:   "git commit",
			runError: exitError,
			stderr:   "fatal: everything failed\n",
			want:     "git commit: fatal: everything failed",
		},
		{
			prefix:   "git commit",
			runError: xerrors.New("could not copy I/O"),
			stderr:   "fatal: everything failed\nThis is the work of Voldemort.\n",
			want:     "git commit: could not copy I/O\nfatal: everything failed\nThis is the work of Voldemort.",
		},
		{
			prefix:   "git commit",
			runError: xerrors.New("could not copy I/O"),
			stderr:   "fatal: everything failed\nThis is the work of Voldemort.", // no trailing newline
			want:     "git commit: could not copy I/O\nfatal: everything failed\nThis is the work of Voldemort.",
		},
		{
			prefix:   "git commit",
			runError: exitError,
			stderr:   "fatal: everything failed\nThis is the work of Voldemort.\n",
			want:     "git commit:\nfatal: everything failed\nThis is the work of Voldemort.",
		},
	}
	for _, test := range tests {
		e := commandError(test.prefix, test.runError, []byte(test.stderr))
		if got := e.Error(); got != test.want {
			t.Errorf("commandError(%q, %v, %q) = %q; want %q", test.prefix, test.runError, test.stderr, got, test.want)
		}
	}
}

func TestDirs(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()

	t.Run("SingleRepo", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Mkdir("foo")); err != nil {
			t.Fatal(err)
		}

		if got, err := env.g.GitDir(ctx); err != nil {
			t.Error("env.g.GitDir(ctx):", err)
		} else if want := env.root.FromSlash(".git"); got != want {
			t.Errorf("env.g.GitDir(ctx) = %q; want %q", got, want)
		}
		if got, err := env.g.CommonDir(ctx); err != nil {
			t.Error("env.g.CommonDir(ctx):", err)
		} else if want := env.root.FromSlash(".git"); got != want {
			t.Errorf("env.g.CommonDir(ctx) = %q; want %q", got, want)
		}
		t.Run("Subdir", func(t *testing.T) {
			// Regression test for https://github.com/zombiezen/gg/issues/105

			g := env.g.WithDir(env.root.FromSlash("foo"))
			if got, err := g.GitDir(ctx); err != nil {
				t.Error("env.g.WithDir(\"foo\").GitDir(ctx):", err)
			} else if want := env.root.FromSlash(".git"); got != want {
				t.Errorf("env.g.WithDir(\"foo\").GitDir(ctx) = %q; want %q", got, want)
			}
			if got, err := g.CommonDir(ctx); err != nil {
				t.Error("env.g.WithDir(\"foo\").CommonDir(ctx):", err)
			} else if want := env.root.FromSlash(".git"); got != want {
				t.Errorf("env.g.WithDir(\"foo\").CommonDir(ctx) = %q; want %q", got, want)
			}
		})
	})
	t.Run("WorkTree", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		// Create a repository with a single commit.
		sharedDir := env.root.FromSlash("shared")
		if err := env.g.Init(ctx, sharedDir); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("shared/file.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		sharedGit := env.g.WithDir(sharedDir)
		if err := sharedGit.Add(ctx, []Pathspec{"file.txt"}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
		if err := sharedGit.Commit(ctx, "first", CommitOptions{}); err != nil {
			t.Fatal(err)
		}
		// Create linked worktree "foo".
		linkedDir := env.root.FromSlash("foo")
		if err := env.g.WithDir(sharedDir).Run(ctx, "worktree", "add", linkedDir); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Mkdir("foo/bar")); err != nil {
			t.Fatal(err)
		}

		linkedGit := env.g.WithDir(env.root.FromSlash("foo"))
		if got, err := linkedGit.GitDir(ctx); err != nil {
			t.Error("env.g.WithDir(\"foo\").GitDir(ctx):", err)
		} else if want := env.root.FromSlash("shared/.git/worktrees/foo"); got != want {
			t.Errorf("env.g.WithDir(\"foo\").GitDir(ctx) = %q; want %q", got, want)
		}
		if got, err := linkedGit.CommonDir(ctx); err != nil {
			t.Error("env.g.WithDir(\"foo\").CommonDir(ctx):", err)
		} else if want := env.root.FromSlash("shared/.git"); got != want {
			t.Errorf("env.g.WithDir(\"foo\").CommonDir(ctx) = %q; want %q", got, want)
		}
		t.Run("Subdir", func(t *testing.T) {
			// Regression test for https://github.com/zombiezen/gg/issues/105

			subdirGit := env.g.WithDir(env.root.FromSlash("foo/bar"))
			if got, err := subdirGit.GitDir(ctx); err != nil {
				t.Error("env.g.WithDir(\"foo/bar\").GitDir(ctx):", err)
			} else if want := env.root.FromSlash("shared/.git/worktrees/foo"); got != want {
				t.Errorf("env.g.WithDir(\"foo/bar\").GitDir(ctx) = %q; want %q", got, want)
			}
			if got, err := subdirGit.CommonDir(ctx); err != nil {
				t.Error("env.g.WithDir(\"foo/bar\").CommonDir(ctx):", err)
			} else if want := env.root.FromSlash("shared/.git"); got != want {
				t.Errorf("env.g.WithDir(\"foo/bar\").CommonDir(ctx) = %q; want %q", got, want)
			}
		})
	})
}

func TestNullTreeHash(t *testing.T) {
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
	got, err := env.g.NullTreeHash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := Hash{0x4b, 0x82, 0x5d, 0xc6, 0x42, 0xcb, 0x6e, 0xb9, 0xa0, 0x60, 0xe5, 0x4b, 0xf8, 0xd6, 0x92, 0x88, 0xfb, 0xee, 0x49, 0x04}
	if got != want {
		t.Errorf("env.g.NullTreeHash(ctx) = %v; want %v", got, want)
	}
}
