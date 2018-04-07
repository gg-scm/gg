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
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"zombiezen.com/go/gg/internal/gittool"
)

func TestRebase_NoArgs(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRebaseRev(ctx, env, "master", "foo.txt", "Initial import")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRebaseRev(ctx, env, "topic", "bar.txt", "First feature change")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRebaseRev(ctx, env, "topic", "baz.txt", "Second feature change")
	if err != nil {
		t.Fatal(err)
	}
	head, err := dummyRebaseRev(ctx, env, "master", "quux.txt", "Mainline change")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{
		base: "initial import",
		c1:   "change 1",
		c2:   "change 2",
		head: "mainline change",
	}

	if err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root, "rebase"); err != nil {
		t.Error(err)
	}

	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() == c2 {
		t.Fatal("rebase did not change commit; want new commit")
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}
	if want := "refs/heads/topic"; curr.RefName() != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.RefName(), want)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() == c1 {
		t.Fatal("rebase did not change parent commit; want new commit")
	}
	if err := objectExists(ctx, env.git, parent.CommitHex()+":bar.txt"); err != nil {
		t.Error("bar.txt not in rebased change:", err)
	}

	grandparent, err := gittool.ParseRev(ctx, env.git, "HEAD~2")
	if err != nil {
		t.Fatal(err)
	}
	if grandparent.CommitHex() != head {
		t.Errorf("HEAD~2 = %s; want %s", prettyCommit(grandparent.CommitHex(), names), prettyCommit(head, names))
	}
}

func TestRebase_Src(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRebaseRev(ctx, env, "master", "foo.txt", "Initial import")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRebaseRev(ctx, env, "topic", "bar.txt", "First feature change")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRebaseRev(ctx, env, "topic", "baz.txt", "Second feature change")
	if err != nil {
		t.Fatal(err)
	}
	head, err := dummyRebaseRev(ctx, env, "master", "quux.txt", "Mainline change")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{
		base: "initial import",
		c1:   "change 1",
		c2:   "change 2",
		head: "mainline change",
	}

	if err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root, "rebase", "-src="+c2); err != nil {
		t.Error(err)
	}

	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() == c2 {
		t.Fatal("rebase did not change commit; want new commit", c2)
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}
	if want := "refs/heads/topic"; curr.RefName() != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.RefName(), want)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() != head {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(head, names))
	}
}

func TestRebase_Base(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRebaseRev(ctx, env, "master", "foo.txt", "Initial import")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRebaseRev(ctx, env, "topic", "bar.txt", "First feature change")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRebaseRev(ctx, env, "topic", "baz.txt", "Second feature change")
	if err != nil {
		t.Fatal(err)
	}
	magic, err := dummyRebaseRev(ctx, env, "magic", "shazam.txt", "Something different")
	if err != nil {
		t.Fatal(err)
	}
	c3, err := dummyRebaseRev(ctx, env, "topic", "xyzzy.txt", "Third feature change")
	if err != nil {
		t.Fatal(err)
	}
	head, err := dummyRebaseRev(ctx, env, "master", "quux.txt", "Mainline change")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{
		base:  "initial import",
		c1:    "change 1",
		c2:    "change 2",
		c3:    "change 3",
		magic: "magic",
		head:  "mainline change",
	}

	if err := env.git.Run(ctx, "checkout", "--quiet", "topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root, "rebase", "-base="+magic); err != nil {
		t.Error(err)
	}

	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() == c3 {
		t.Fatal("rebase did not change commit; want new commit", c3)
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":xyzzy.txt"); err != nil {
		t.Error("xyzzy.txt not in rebased change:", err)
	}
	if want := "refs/heads/topic"; curr.RefName() != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.RefName(), want)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() != head {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(head, names))
	}
}

func prettyCommit(hex string, names map[string]string) string {
	n := names[hex]
	if n == "" {
		return hex
	}
	return hex + " (" + n + ")"
}

func dummyRebaseRev(ctx context.Context, env *testEnv, branch string, file string, msg string) (string, error) {
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
