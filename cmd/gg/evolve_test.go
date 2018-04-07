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

	"zombiezen.com/go/gg/internal/gittool"
)

func TestEvolve_FirstChangeSubmitted(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRebaseRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRebaseRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRebaseRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	submit1, err := dummyRebaseRev(ctx, env, "master", "submitted.txt", "Submitted first feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{
		base:    "base",
		c1:      "change 1",
		c2:      "change 2",
		submit1: "submitted change 1",
	}

	if err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
		t.Fatal(err)
	}
	if out, err := env.gg(ctx, env.root, "evolve", "-l"); err != nil {
		t.Error(err)
	} else {
		want1 := "< " + c1 + "\n"
		want2 := "> " + submit1 + "\n"
		if !bytes.Contains(out, []byte(want1)) || !bytes.Contains(out, []byte(want2)) {
			t.Errorf("gg evolve -l = %q; want to contain %q and %q", out, want1, want2)
		}
	}
	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() != c2 {
		t.Fatalf("HEAD after evolve -l = %s; want %s", prettyCommit(curr.CommitHex(), names), prettyCommit(c2, names))
	}

	if _, err := env.gg(ctx, env.root, "evolve"); err != nil {
		t.Fatal(err)
	}
	curr, err = gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if names[curr.CommitHex()] != "" {
		t.Errorf("HEAD = %s; want new commit", prettyCommit(curr.CommitHex(), names))
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}
	parent, err := gittool.ParseRev(ctx, env.git, "HEAD^")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() != submit1 {
		t.Errorf("HEAD^ = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(submit1, names))
	}
}

func TestEvolve_Unrelated(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRebaseRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRebaseRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRebaseRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	other, err := dummyRebaseRev(ctx, env, "master", "somestuff.txt", "Somebody else contributed!\n\nChange-Id: mnopqr")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{
		base:  "base",
		c1:    "change 1",
		c2:    "change 2",
		other: "upstream",
	}

	if err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
		t.Fatal(err)
	}
	if out, err := env.gg(ctx, env.root, "evolve", "-l"); err != nil {
		t.Error(err)
	} else if len(out) > 0 {
		t.Errorf("gg evolve -l = %q; want empty", out)
	}
	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() != c2 {
		t.Fatalf("HEAD after evolve -l = %s; want %s", prettyCommit(curr.CommitHex(), names), prettyCommit(c2, names))
	}

	if _, err := env.gg(ctx, env.root, "evolve"); err != nil {
		t.Fatal(err)
	}
	curr, err = gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() != c2 {
		t.Errorf("HEAD = %s; want %s", prettyCommit(curr.CommitHex(), names), prettyCommit(c2, names))
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() != c1 {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(c1, names))
	}
	if err := objectExists(ctx, env.git, parent.CommitHex()+":bar.txt"); err != nil {
		t.Error("bar.txt not in rebased change:", err)
	}

	grandparent, err := gittool.ParseRev(ctx, env.git, "HEAD~2")
	if err != nil {
		t.Fatal(err)
	}
	if grandparent.CommitHex() != base {
		t.Errorf("HEAD~2 = %s; want %s", prettyCommit(grandparent.CommitHex(), names), prettyCommit(base, names))
	}
}

func TestEvolve_UnrelatedOnTopOfSubmitted(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRebaseRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRebaseRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRebaseRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	submit1, err := dummyRebaseRev(ctx, env, "master", "bar-submitted.txt", "Submitted first feature\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	other, err := dummyRebaseRev(ctx, env, "master", "somestuff.txt", "Somebody else contributed!\n\nChange-Id: mnopqr")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{
		base:    "base",
		c1:      "change 1",
		c2:      "change 2",
		submit1: "submitted change 1",
		other:   "upstream",
	}

	if err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
		t.Fatal(err)
	}
	if out, err := env.gg(ctx, env.root, "evolve", "-l"); err != nil {
		t.Error(err)
	} else {
		want1 := "< " + c1 + "\n"
		want2 := "> " + submit1 + "\n"
		if !bytes.Contains(out, []byte(want1)) || !bytes.Contains(out, []byte(want2)) {
			t.Errorf("gg evolve -l = %q; want to contain %q and %q", out, want1, want2)
		}
	}
	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() != c2 {
		t.Fatalf("HEAD after evolve -l = %s; want %s", prettyCommit(curr.CommitHex(), names), prettyCommit(c2, names))
	}

	if _, err := env.gg(ctx, env.root, "evolve"); err != nil {
		t.Fatal(err)
	}
	curr, err = gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if names[curr.CommitHex()] != "" {
		t.Errorf("HEAD = %s; want new commit", prettyCommit(base, names))
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() != submit1 {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(submit1, names))
	}
}

func TestEvolve_AbortIfReordersLocal(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRebaseRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRebaseRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRebaseRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	submit2, err := dummyRebaseRev(ctx, env, "master", "submitted.txt", "Submitted second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{
		base:    "base",
		c1:      "change 1",
		c2:      "change 2",
		submit2: "submitted change 2",
	}

	if err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
		t.Fatal(err)
	}
	if out, err := env.gg(ctx, env.root, "evolve", "-l"); err != nil {
		t.Error(err)
	} else {
		want1 := "< " + c2 + "\n"
		want2 := "> " + submit2 + "\n"
		if !bytes.Contains(out, []byte(want1)) || !bytes.Contains(out, []byte(want2)) {
			t.Errorf("gg evolve -l = %q; want to contain %q and %q", out, want1, want2)
		}
	}
	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() != c2 {
		t.Fatalf("HEAD after evolve -l = %s; want %s", prettyCommit(curr.CommitHex(), names), prettyCommit(c2, names))
	}

	if _, err := env.gg(ctx, env.root, "evolve"); err == nil {
		t.Error("gg evolve did not return error")
	} else if _, isUsage := err.(*usageError); isUsage {
		t.Error("gg evolve returned usage error:", err)
	}
	curr, err = gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() != c2 {
		t.Errorf("HEAD = %s; want %s", prettyCommit(curr.CommitHex(), names), prettyCommit(c2, names))
	}
}

func TestFindChangeID(t *testing.T) {
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
		got := findChangeID([]byte(test.commitMsg))
		if got != test.want {
			t.Errorf("findChangeID(%q) = %q; want %q", test.commitMsg, got, test.want)
		}
	}
}
