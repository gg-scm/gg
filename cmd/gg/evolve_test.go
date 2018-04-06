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
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"zombiezen.com/go/gg/internal/gittool"
)

func TestEvolve_NoOp(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyEvolveRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyEvolveRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyEvolveRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}

	if out, err := env.gg(ctx, env.root, "evolve", "-l"); err != nil {
		t.Error(err)
	} else if len(out) > 0 {
		t.Errorf("gg evolve -l = %q; want empty", out)
	}
	if _, err := env.gg(ctx, env.root, "evolve"); err != nil {
		t.Error(err)
	}

	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() == base {
		t.Errorf("HEAD = %s (base); want %s (change 2)", curr.CommitHex(), c2)
	} else if curr.CommitHex() == c1 {
		t.Errorf("HEAD = %s (change 1); want %s (change 2)", curr.CommitHex(), c2)
	} else if curr.CommitHex() != c2 {
		t.Errorf("HEAD = %s; want %s (change 2)", curr.CommitHex(), c2)
	}
}

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
	base, err := dummyEvolveRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyEvolveRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyEvolveRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	submit1, err := dummyEvolveRev(ctx, env, "master", "submitted.txt", "Submitted first feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
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
		t.Fatalf("HEAD after evolve -l = %q; want %q (change 2, HEAD before evolve -l)", curr.CommitHex(), c2)
	}

	if _, err := env.gg(ctx, env.root, "evolve"); err != nil {
		t.Fatal(err)
	}
	curr, err = gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() == base {
		t.Errorf("HEAD = %s (base); want new commit", base)
	} else if curr.CommitHex() == c1 {
		t.Errorf("HEAD = %s (change 1); want new commit", c1)
	} else if curr.CommitHex() == c2 {
		t.Errorf("HEAD = %s (change 2); want new commit", c2)
	} else if curr.CommitHex() == submit1 {
		t.Errorf("HEAD = %s (submitted change 1); want new commit", submit1)
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}
	parent, err := gittool.ParseRev(ctx, env.git, "HEAD^")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() == base {
		t.Errorf("HEAD^ = %s (base); want %s (submitted change 1)", base, submit1)
	} else if parent.CommitHex() == c1 {
		t.Errorf("HEAD^ = %s (change 1); want %s (submitted change 1)", c1, submit1)
	} else if parent.CommitHex() == c2 {
		t.Errorf("HEAD^ = %s (change 2); want %s (submitted change 1)", c2, submit1)
	} else if parent.CommitHex() != submit1 {
		t.Errorf("HEAD^ = %s; want %s (submitted change 1)", parent.CommitHex(), submit1)
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
	base, err := dummyEvolveRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyEvolveRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyEvolveRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	other, err := dummyEvolveRev(ctx, env, "master", "somestuff.txt", "Somebody else contributed!\n\nChange-Id: mnopqr")
	if err != nil {
		t.Fatal(err)
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
		t.Fatalf("HEAD after evolve -l = %q; want %q (change 2, HEAD before evolve -l)", curr.CommitHex(), c2)
	}

	if _, err := env.gg(ctx, env.root, "evolve"); err != nil {
		t.Fatal(err)
	}
	curr, err = gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() == base {
		t.Errorf("HEAD = %s (base); want %s (change 2)", base, c2)
	} else if curr.CommitHex() == c1 {
		t.Errorf("HEAD = %s (change 1); want %s (change 2)", c1, c2)
	} else if curr.CommitHex() == other {
		t.Errorf("HEAD = %s (upstream change); want %s (change 2)", other, c2)
	} else if curr.CommitHex() != c2 {
		t.Errorf("HEAD = %s; want %s (change 2)", curr.CommitHex(), c2)
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() == base {
		t.Errorf("HEAD~1 = %s (base); want %s (change 1)", base, c1)
	} else if parent.CommitHex() == c2 {
		t.Errorf("HEAD~1 = %s (change 2); want %s (change 1)", c2, c1)
	} else if parent.CommitHex() == other {
		t.Errorf("HEAD~1 = %s (upstream change); want %s (change 1)", other, c1)
	} else if parent.CommitHex() != c1 {
		t.Errorf("HEAD~1 = %s; want %s (change 1)", parent.CommitHex(), c1)
	}
	if err := objectExists(ctx, env.git, parent.CommitHex()+":bar.txt"); err != nil {
		t.Error("bar.txt not in rebased change:", err)
	}

	grandparent, err := gittool.ParseRev(ctx, env.git, "HEAD~2")
	if err != nil {
		t.Fatal(err)
	}
	if grandparent.CommitHex() == c1 {
		t.Errorf("HEAD~2 = %s (change 1); want %s (base)", c1, base)
	} else if grandparent.CommitHex() == c2 {
		t.Errorf("HEAD~2 = %s (change 2); want %s (base)", c2, base)
	} else if grandparent.CommitHex() == other {
		t.Errorf("HEAD~2 = %s (upstream change); want %s (base)", other, base)
	} else if grandparent.CommitHex() != base {
		t.Errorf("HEAD~2 = %s; want %s (base)", grandparent.CommitHex(), base)
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
	base, err := dummyEvolveRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyEvolveRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyEvolveRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	submit1, err := dummyEvolveRev(ctx, env, "master", "bar-submitted.txt", "Submitted first feature\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	other, err := dummyEvolveRev(ctx, env, "master", "somestuff.txt", "Somebody else contributed!\n\nChange-Id: mnopqr")
	if err != nil {
		t.Fatal(err)
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
		t.Fatalf("HEAD after evolve -l = %q; want %q (change 2, HEAD before evolve -l)", curr.CommitHex(), c2)
	}

	if _, err := env.gg(ctx, env.root, "evolve"); err != nil {
		t.Fatal(err)
	}
	curr, err = gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() == base {
		t.Errorf("HEAD = %s (base); want new commit", base)
	} else if curr.CommitHex() == c1 {
		t.Errorf("HEAD = %s (change 1); want new commit", c1)
	} else if curr.CommitHex() == c2 {
		t.Errorf("HEAD = %s (change 2); want new commit", c2)
	} else if curr.CommitHex() == submit1 {
		t.Errorf("HEAD = %s (submitted change 1); want new commit", submit1)
	} else if curr.CommitHex() == other {
		t.Errorf("HEAD = %s (upstream change); want new commit", other)
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() == base {
		t.Errorf("HEAD~1 = %s (base); want %s (submitted change 1)", base, submit1)
	} else if parent.CommitHex() == c1 {
		t.Errorf("HEAD~1 = %s (change 1); want %s (submitted change 1)", c1, submit1)
	} else if parent.CommitHex() == c2 {
		t.Errorf("HEAD~1 = %s (change 2); want %s (submitted change 1)", c2, submit1)
	} else if parent.CommitHex() == other {
		t.Errorf("HEAD~1 = %s (upstream change); want %s (submitted change 1)", other, submit1)
	} else if parent.CommitHex() != submit1 {
		t.Errorf("HEAD~1 = %s; want %s (submitted change 1)", parent.CommitHex(), submit1)
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
	base, err := dummyEvolveRev(ctx, env, "master", "foo.txt", "Initial import\n\nChange-Id: xyzzy")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyEvolveRev(ctx, env, "topic", "bar.txt", "First feature change\n\nChange-Id: abcdef")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyEvolveRev(ctx, env, "topic", "baz.txt", "Second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
	}
	submit2, err := dummyEvolveRev(ctx, env, "master", "submitted.txt", "Submitted second feature change\n\nChange-Id: ghijkl")
	if err != nil {
		t.Fatal(err)
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
		t.Fatalf("HEAD after evolve -l = %q; want %q (change 2, HEAD before evolve -l)", curr.CommitHex(), c2)
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
	if curr.CommitHex() == base {
		t.Errorf("HEAD = %s (base); want %s (change 2)", base, c2)
	} else if curr.CommitHex() == c1 {
		t.Errorf("HEAD = %s (change 1); want %s (change 2)", c1, c2)
	} else if curr.CommitHex() == submit2 {
		t.Errorf("HEAD = %s (submitted change 2); want %s (change 2)", submit2, c2)
	} else if curr.CommitHex() != c2 {
		t.Errorf("HEAD = %s; want %s (change 2)", curr.CommitHex(), c2)
	}
}

func dummyEvolveRev(ctx context.Context, env *testEnv, branch string, file string, msg string) (string, error) {
	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		// First commit
		if branch != "master" {
			return "", fmt.Errorf("make evolve rev: %v", err)
		}
	} else if curr.Branch() != branch {
		if _, err := gittool.ParseRev(ctx, env.git, "refs/heads/"+branch); err != nil {
			// Branch doesn't exist, create it.
			if err := env.git.Run(ctx, "branch", "--", branch); err != nil {
				return "", fmt.Errorf("make evolve rev: %v", err)
			}
			if err := env.git.Run(ctx, "branch", "--set-upstream-to="+curr.RefName(), "--", branch); err != nil {
				return "", fmt.Errorf("make evolve rev: %v", err)
			}
		}
		if err := env.git.Run(ctx, "checkout", "--quiet", branch); err != nil {
			return "", fmt.Errorf("make evolve rev: %v", err)
		}
	}
	err = ioutil.WriteFile(filepath.Join(env.root, file), []byte("dummy content"), 0666)
	if err != nil {
		return "", fmt.Errorf("make evolve rev: %v", err)
	}
	if err := env.git.Run(ctx, "add", file); err != nil {
		return "", fmt.Errorf("make evolve rev: %v", err)
	}
	if err := env.git.Run(ctx, "commit", "-m", msg); err != nil {
		return "", fmt.Errorf("make evolve rev: %v", err)
	}
	curr, err = gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		return "", fmt.Errorf("make evolve rev: %v", err)
	}
	return curr.CommitHex(), nil
}

func TestFindChangeID(t *testing.T) {
	tests := []struct {
		trailers string
		want     string
	}{
		{
			trailers: "",
			want:     "",
		},
		{
			trailers: "foo",
			want:     "",
		},
		{
			trailers: "Change-Id: xyzzy",
			want:     "xyzzy",
		},
		{
			trailers: "Change-Id: xyzzy\n",
			want:     "xyzzy",
		},
		{
			trailers: "Change-Id: xyzzy\nSigned-off-by: A. U. Thor <author@example.com>\n",
			want:     "xyzzy",
		},
		{
			trailers: "Change-Id: \n",
			want:     "",
		},
		{
			trailers: "Change-Id: xyzzy\n\nChange-Id: plugh",
			want:     "plugh",
		},
	}
	for _, test := range tests {
		got := findChangeID([]byte(test.trailers))
		if got != test.want {
			t.Errorf("findChangeID(%q) = %q; want %q", test.trailers, got, test.want)
		}
	}
}
