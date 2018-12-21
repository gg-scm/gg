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
	"bytes"
	"context"
	"testing"

	"gg-scm.io/pkg/internal/git"
)

func TestEvolve_FirstChangeSubmitted(t *testing.T) {
	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initEmptyRepo(ctx, "."); err != nil {
			t.Fatal(err)
		}
		base, err := dummyRev(ctx, env.git, env.root.String(), "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
		if err != nil {
			t.Fatal(err)
		}
		c1, err := dummyRev(ctx, env.git, env.root.String(), "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
		if err != nil {
			t.Fatal(err)
		}
		c2, err := dummyRev(ctx, env.git, env.root.String(), "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
		if err != nil {
			t.Fatal(err)
		}
		submit1, err := dummyRev(ctx, env.git, env.root.String(), "master", "submitted.txt", "Submitted first feature change\n\nChange-Id: abcdef")
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			base:    "base",
			c1:      "change 1",
			c2:      "change 2",
			submit1: "submitted change 1",
		}

		if _, err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
			t.Fatal(err)
		}
		out, err := env.gg(ctx, env.root.String(), appendNonEmpty([]string{"evolve", "-l"}, argFunc(submit1))...)
		if err != nil {
			t.Error(err)
		} else {
			want1 := "< " + c1.String() + "\n"
			want2 := "> " + submit1.String() + "\n"
			if !bytes.Contains(out, []byte(want1)) || !bytes.Contains(out, []byte(want2)) {
				t.Errorf("gg evolve -l = %q; want to contain %q and %q", out, want1, want2)
			}
		}
		curr, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if curr.Commit != c2 {
			t.Fatalf("HEAD after evolve -l = %s; want %s", prettyCommit(curr.Commit, names), prettyCommit(c2, names))
		}

		_, err = env.gg(ctx, env.root.String(), appendNonEmpty([]string{"evolve"}, argFunc(submit1))...)
		if err != nil {
			t.Fatal(err)
		}
		curr, err = env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if names[curr.Commit] != "" {
			t.Errorf("HEAD = %s; want new commit", prettyCommit(curr.Commit, names))
		}
		if err := objectExists(ctx, env.git, curr.Commit.String(), "baz.txt"); err != nil {
			t.Error("baz.txt not in rebased change:", err)
		}
		parent, err := env.git.ParseRev(ctx, "HEAD^")
		if err != nil {
			t.Fatal(err)
		}
		if parent.Commit != submit1 {
			t.Errorf("HEAD^ = %s; want %s", prettyCommit(parent.Commit, names), prettyCommit(submit1, names))
		}
	})
}

func TestEvolve_Unrelated(t *testing.T) {
	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initEmptyRepo(ctx, "."); err != nil {
			t.Fatal(err)
		}
		base, err := dummyRev(ctx, env.git, env.root.String(), "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
		if err != nil {
			t.Fatal(err)
		}
		c1, err := dummyRev(ctx, env.git, env.root.String(), "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
		if err != nil {
			t.Fatal(err)
		}
		c2, err := dummyRev(ctx, env.git, env.root.String(), "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
		if err != nil {
			t.Fatal(err)
		}
		other, err := dummyRev(ctx, env.git, env.root.String(), "master", "somestuff.txt", "Somebody else contributed!\n\nChange-Id: mnopqr")
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			base:  "base",
			c1:    "change 1",
			c2:    "change 2",
			other: "upstream",
		}

		if _, err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
			t.Fatal(err)
		}
		out, err := env.gg(ctx, env.root.String(), appendNonEmpty([]string{"evolve", "-l"}, argFunc(other))...)
		if err != nil {
			t.Error(err)
		} else if len(out) > 0 {
			t.Errorf("gg evolve -l = %q; want empty", out)
		}
		curr, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if curr.Commit != c2 {
			t.Fatalf("HEAD after evolve -l = %s; want %s", prettyCommit(curr.Commit, names), prettyCommit(c2, names))
		}

		_, err = env.gg(ctx, env.root.String(), appendNonEmpty([]string{"evolve"}, argFunc(other))...)
		if err != nil {
			t.Fatal(err)
		}
		curr, err = env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if curr.Commit != c2 {
			t.Errorf("HEAD = %s; want %s", prettyCommit(curr.Commit, names), prettyCommit(c2, names))
		}
		if err := objectExists(ctx, env.git, curr.Commit.String(), "baz.txt"); err != nil {
			t.Error("baz.txt not in rebased change:", err)
		}

		parent, err := env.git.ParseRev(ctx, "HEAD~1")
		if err != nil {
			t.Fatal(err)
		}
		if parent.Commit != c1 {
			t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.Commit, names), prettyCommit(c1, names))
		}
		if err := objectExists(ctx, env.git, parent.Commit.String(), "bar.txt"); err != nil {
			t.Error("bar.txt not in rebased change:", err)
		}

		grandparent, err := env.git.ParseRev(ctx, "HEAD~2")
		if err != nil {
			t.Fatal(err)
		}
		if grandparent.Commit != base {
			t.Errorf("HEAD~2 = %s; want %s", prettyCommit(grandparent.Commit, names), prettyCommit(base, names))
		}
	})
}

func TestEvolve_UnrelatedOnTopOfSubmitted(t *testing.T) {
	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initEmptyRepo(ctx, "."); err != nil {
			t.Fatal(err)
		}
		base, err := dummyRev(ctx, env.git, env.root.String(), "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
		if err != nil {
			t.Fatal(err)
		}
		c1, err := dummyRev(ctx, env.git, env.root.String(), "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
		if err != nil {
			t.Fatal(err)
		}
		c2, err := dummyRev(ctx, env.git, env.root.String(), "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
		if err != nil {
			t.Fatal(err)
		}
		submit1, err := dummyRev(ctx, env.git, env.root.String(), "master", "bar-submitted.txt", "Submitted first feature\n\nChange-Id: abcdef")
		if err != nil {
			t.Fatal(err)
		}
		other, err := dummyRev(ctx, env.git, env.root.String(), "master", "somestuff.txt", "Somebody else contributed!\n\nChange-Id: mnopqr")
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			base:    "base",
			c1:      "change 1",
			c2:      "change 2",
			submit1: "submitted change 1",
			other:   "upstream",
		}

		if _, err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
			t.Fatal(err)
		}
		out, err := env.gg(ctx, env.root.String(), appendNonEmpty([]string{"evolve", "-l"}, argFunc(other))...)
		if err != nil {
			t.Error(err)
		} else {
			want1 := "< " + c1.String() + "\n"
			want2 := "> " + submit1.String() + "\n"
			if !bytes.Contains(out, []byte(want1)) || !bytes.Contains(out, []byte(want2)) {
				t.Errorf("gg evolve -l = %q; want to contain %q and %q", out, want1, want2)
			}
		}
		curr, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if curr.Commit != c2 {
			t.Fatalf("HEAD after evolve -l = %s; want %s", prettyCommit(curr.Commit, names), prettyCommit(c2, names))
		}

		_, err = env.gg(ctx, env.root.String(), appendNonEmpty([]string{"evolve"}, argFunc(other))...)
		if err != nil {
			t.Fatal(err)
		}
		curr, err = env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if names[curr.Commit] != "" {
			t.Errorf("HEAD = %s; want new commit", prettyCommit(base, names))
		}
		if err := objectExists(ctx, env.git, curr.Commit.String(), "baz.txt"); err != nil {
			t.Error("baz.txt not in rebased change:", err)
		}

		parent, err := env.git.ParseRev(ctx, "HEAD~1")
		if err != nil {
			t.Fatal(err)
		}
		if parent.Commit != submit1 {
			t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.Commit, names), prettyCommit(submit1, names))
		}
	})
}

func TestEvolve_AbortIfReordersLocal(t *testing.T) {
	t.Parallel()
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.initEmptyRepo(ctx, "."); err != nil {
			t.Fatal(err)
		}
		base, err := dummyRev(ctx, env.git, env.root.String(), "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
		if err != nil {
			t.Fatal(err)
		}
		c1, err := dummyRev(ctx, env.git, env.root.String(), "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
		if err != nil {
			t.Fatal(err)
		}
		c2, err := dummyRev(ctx, env.git, env.root.String(), "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
		if err != nil {
			t.Fatal(err)
		}
		submit2, err := dummyRev(ctx, env.git, env.root.String(), "master", "submitted.txt", "Submitted second feature change\n\nChange-Id: ghijkl")
		if err != nil {
			t.Fatal(err)
		}
		names := map[git.Hash]string{
			base:    "base",
			c1:      "change 1",
			c2:      "change 2",
			submit2: "submitted change 2",
		}

		if _, err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
			t.Fatal(err)
		}
		out, err := env.gg(ctx, env.root.String(), appendNonEmpty([]string{"evolve", "-l"}, argFunc(submit2))...)
		if err != nil {
			t.Error(err)
		} else {
			want1 := "< " + c2.String() + "\n"
			want2 := "> " + submit2.String() + "\n"
			if !bytes.Contains(out, []byte(want1)) || !bytes.Contains(out, []byte(want2)) {
				t.Errorf("gg evolve -l = %q; want to contain %q and %q", out, want1, want2)
			}
		}
		curr, err := env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if curr.Commit != c2 {
			t.Fatalf("HEAD after evolve -l = %s; want %s", prettyCommit(curr.Commit, names), prettyCommit(c2, names))
		}

		_, err = env.gg(ctx, env.root.String(), appendNonEmpty([]string{"evolve"}, argFunc(submit2))...)
		if err == nil {
			t.Error("gg evolve did not return error")
		} else if isUsage(err) {
			t.Error("gg evolve returned usage error:", err)
		}
		curr, err = env.git.Head(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if curr.Commit != c2 {
			t.Errorf("HEAD = %s; want %s", prettyCommit(curr.Commit, names), prettyCommit(c2, names))
		}
	})
}

func TestFindChangeID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		commitMsg string
		want      string
	}{
		{
			commitMsg: "",
			want:      "",
		},
		{
			commitMsg: "foo",
			want:      "",
		},
		{
			commitMsg: "Change-Id: foo",
			want:      "",
		},
		{
			commitMsg: "Change-Id: foo\n",
			want:      "",
		},
		{
			commitMsg: "\n\nChange-Id: foo",
			want:      "",
		},
		{
			commitMsg: "\n\nChange-Id: foo\n",
			want:      "",
		},
		{
			commitMsg: "foo\n\nChange-Id: xyzzy",
			want:      "xyzzy",
		},
		{
			commitMsg: "foo\n\nChange-Id: xyzzy\n",
			want:      "xyzzy",
		},
		{
			commitMsg: "foo\n\nChange-Id: xyzzy\nSigned-off-by: A. U. Thor <author@example.com>\n",
			want:      "xyzzy",
		},
		{
			commitMsg: "foo\n\nChange-Id: xyzzy\n\nSigned-off-by: A. U. Thor <author@example.com>\n",
			want:      "",
		},
		{
			commitMsg: "foo\n\nChange-Id: \n",
			want:      "",
		},
		{
			commitMsg: "foo\n\nChange-Id: xyzzy\n\nChange-Id: plugh",
			want:      "plugh",
		},
	}
	for _, test := range tests {
		got := findChangeID(test.commitMsg)
		if got != test.want {
			t.Errorf("findChangeID(%q) = %q; want %q", test.commitMsg, got, test.want)
		}
	}
}
