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
	"strings"
	"testing"

	"zombiezen.com/go/gg/internal/gitobj"
	"zombiezen.com/go/gg/internal/gittool"
)

func TestRebase(t *testing.T) {
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.git.Run(ctx, "init"); err != nil {
			t.Fatal(err)
		}
		base, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Initial import")
		if err != nil {
			t.Fatal(err)
		}
		c1, err := dummyRev(ctx, env.git, env.root, "topic", "bar.txt", "First feature change")
		if err != nil {
			t.Fatal(err)
		}
		c2, err := dummyRev(ctx, env.git, env.root, "topic", "baz.txt", "Second feature change")
		if err != nil {
			t.Fatal(err)
		}
		head, err := dummyRev(ctx, env.git, env.root, "master", "quux.txt", "Mainline change")
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
		ggArgs := []string{"rebase"}
		if arg := argFunc(head); arg != "" {
			ggArgs = append(ggArgs, "-base="+arg, "-dst="+arg)
		}
		_, err = env.gg(ctx, env.root, ggArgs...)
		if err != nil {
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
		if want := gitobj.Ref("refs/heads/topic"); curr.Ref() != want {
			t.Errorf("rebase changed ref to %s; want %s", curr.Ref(), want)
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
	})
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
	base, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Initial import")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRev(ctx, env.git, env.root, "topic", "bar.txt", "First feature change")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRev(ctx, env.git, env.root, "topic", "baz.txt", "Second feature change")
	if err != nil {
		t.Fatal(err)
	}
	head, err := dummyRev(ctx, env.git, env.root, "master", "quux.txt", "Mainline change")
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
	if want := gitobj.Ref("refs/heads/topic"); curr.Ref() != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.Ref(), want)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() != head {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(head, names))
	}
}

func TestRebase_SrcUnrelated(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Initial import")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRev(ctx, env.git, env.root, "topic", "bar.txt", "First feature change")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRev(ctx, env.git, env.root, "topic", "baz.txt", "Second feature change")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{
		base: "initial import",
		c1:   "change 1",
		c2:   "change 2",
	}

	if err := env.git.Run(ctx, "checkout", "--quiet", "master"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root, "rebase", "-src="+c2, "-dst=HEAD"); err != nil {
		t.Error(err)
	}

	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if curr.CommitHex() == base || curr.CommitHex() == c1 || curr.CommitHex() == c2 {
		t.Fatalf("HEAD = %s; want new commit", prettyCommit(curr.CommitHex(), names))
	}
	if err := objectExists(ctx, env.git, curr.CommitHex()+":baz.txt"); err != nil {
		t.Error("baz.txt not in rebased change:", err)
	}
	if want := gitobj.Ref("refs/heads/master"); curr.Ref() != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.Ref(), want)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() != base {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(base, names))
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
	base, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Initial import")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := dummyRev(ctx, env.git, env.root, "topic", "bar.txt", "First feature change")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := dummyRev(ctx, env.git, env.root, "topic", "baz.txt", "Second feature change")
	if err != nil {
		t.Fatal(err)
	}
	magic, err := dummyRev(ctx, env.git, env.root, "magic", "shazam.txt", "Something different")
	if err != nil {
		t.Fatal(err)
	}
	c3, err := dummyRev(ctx, env.git, env.root, "topic", "xyzzy.txt", "Third feature change")
	if err != nil {
		t.Fatal(err)
	}
	head, err := dummyRev(ctx, env.git, env.root, "master", "quux.txt", "Mainline change")
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
	if want := gitobj.Ref("refs/heads/topic"); curr.Ref() != want {
		t.Errorf("rebase changed ref to %s; want %s", curr.Ref(), want)
	}

	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
	if err != nil {
		t.Fatal(err)
	}
	if parent.CommitHex() != head {
		t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(head, names))
	}
}

func TestHistedit(t *testing.T) {
	runRebaseArgVariants(t, func(t *testing.T, argFunc rebaseArgFunc) {
		ctx := context.Background()
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.git.Run(ctx, "init"); err != nil {
			t.Fatal(err)
		}
		base, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Initial import")
		if err != nil {
			t.Fatal(err)
		}
		c, err := dummyRev(ctx, env.git, env.root, "foo", "bar.txt", "Divergence")
		if err != nil {
			t.Fatal(err)
		}
		head, err := dummyRev(ctx, env.git, env.root, "master", "baz.txt", "Upstream")
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]string{
			base: "initial import",
			c:    "branch change",
			head: "mainline change",
		}

		if err := env.git.Run(ctx, "checkout", "--quiet", "foo"); err != nil {
			t.Fatal(err)
		}
		rebaseEditor, err := env.editorCmd([]byte("reword " + c + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		const wantMessage = "New commit message for bar.txt"
		msgEditor, err := env.editorCmd([]byte(wantMessage + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		config := fmt.Sprintf("[sequence]\neditor = %s\n[core]\neditor = %s\n",
			configEscape(rebaseEditor), configEscape(msgEditor))
		if err := env.writeConfig([]byte(config)); err != nil {
			t.Fatal(err)
		}
		out, err := env.gg(ctx, env.root, appendNonEmpty([]string{"histedit"}, argFunc(head))...)
		if err != nil {
			t.Fatalf("failed: %v; output:\n%s", err, out)
		}

		curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if got := curr.CommitHex(); got == c || got == head || got == base {
			t.Fatalf("after histedit, commit = %s; want new commit", prettyCommit(got, names))
		}
		if err := objectExists(ctx, env.git, curr.CommitHex()+":bar.txt"); err != nil {
			t.Error("bar.txt not in rebased change:", err)
		}
		if want := gitobj.Ref("refs/heads/foo"); curr.Ref() != want {
			t.Errorf("rebase changed ref to %s; want %s", curr.Ref(), want)
		}
		if msg, err := readCommitMessage(ctx, env.git, curr.CommitHex()); err != nil {
			t.Error(err)
		} else if got := strings.TrimRight(string(msg), "\n"); got != wantMessage {
			t.Errorf("commit message = %q; want %q", got, wantMessage)
		}

		parent, err := gittool.ParseRev(ctx, env.git, "HEAD~1")
		if err != nil {
			t.Fatal(err)
		}
		if parent.CommitHex() != base {
			t.Errorf("HEAD~1 = %s; want %s", prettyCommit(parent.CommitHex(), names), prettyCommit(base, names))
		}
	})
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{``, `''`},
		{`abc`, `abc`},
		{`abc def`, `'abc def'`},
		{`abc/def`, `abc/def`},
		{`abc.def`, `abc.def`},
		{`"abc"`, `'"abc"'`},
		{`'abc'`, `''\''abc'\'''`},
		{`abc\`, `'abc\'`},
	}
	for _, test := range tests {
		if out := shellEscape(test.in); out != test.out {
			t.Errorf("shellEscape(%q) = %s; want %s", test.in, out, test.out)
		}
	}
}

type rebaseArgFunc = func(masterCommit string) string

func runRebaseArgVariants(t *testing.T, f func(*testing.T, rebaseArgFunc)) {
	t.Run("NoArg", func(t *testing.T) {
		f(t, func(_ string) string {
			return ""
		})
	})
	t.Run("BranchName", func(t *testing.T) {
		f(t, func(_ string) string {
			return "master"
		})
	})
	t.Run("CommitHex", func(t *testing.T) {
		f(t, func(masterCommit string) string {
			return masterCommit
		})
	})
}

func prettyCommit(hex string, names map[string]string) string {
	n := names[hex]
	if n == "" {
		return hex
	}
	return hex + " (" + n + ")"
}

func appendNonEmpty(args []string, s string) []string {
	if s == "" {
		return args
	}
	return append(args, s)
}
