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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zombiezen.com/go/gg/internal/gitobj"
	"zombiezen.com/go/gg/internal/gittool"
)

const (
	commitAddedFileContent       = "A brave new file\n"
	commitModifiedFileContent    = "What has changed?\n"
	commitModifiedFileOldContent = "The history and lore\n"
)

func TestCommit_NoArgs(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := stageCommitTest(ctx, env, true); err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	const wantMessage = "gg made this commit"
	if _, err := env.gg(ctx, env.root, "commit", "-m", wantMessage); err != nil {
		t.Fatal(err)
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Commit() == r2.Commit() {
		t.Fatal("commit did not create a new commit in the working copy")
	}
	if ref := r2.Ref(); ref != "refs/heads/master" {
		t.Errorf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitAddedFileContent {
		t.Errorf("added.txt = %q; want %q", data, commitAddedFileContent)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.Commit().String()+":deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.Commit().String()); err != nil {
		t.Error(err)
	} else if got := strings.TrimRight(string(msg), "\n"); got != wantMessage {
		t.Errorf("commit message = %q; want %q", got, wantMessage)
	}
}

func TestCommit_Selective(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := stageCommitTest(ctx, env, true); err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	const wantMessage = "gg made this commit"
	if _, err := env.gg(ctx, env.root, "commit", "-m", wantMessage, "modified.txt"); err != nil {
		t.Fatal(err)
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Commit() == r2.Commit() {
		t.Fatal("commit did not create a new commit in the working copy")
	}
	if ref := r2.Ref(); ref != "refs/heads/master" {
		t.Errorf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.Commit().String()+":added.txt"); err == nil {
		t.Error("added.txt was added but not put in arguments")
	}
	if err := objectExists(ctx, env.git, r2.Commit().String()+":deleted.txt"); err != nil {
		t.Error("deleted.txt was removed but not put in arguments:", err)
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.Commit().String()); err != nil {
		t.Error(err)
	} else if got := strings.TrimRight(string(msg), "\n"); got != wantMessage {
		t.Errorf("commit message = %q; want %q", got, wantMessage)
	}
}

func TestCommit_Amend(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := stageCommitTest(ctx, env, true); err != nil {
		t.Fatal(err)
	}
	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~")
	if err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	const wantMessage = "gg amended this commit"
	changes := map[gitobj.Hash]string{
		parent.Commit(): "parent commit",
		r1.Commit():     "tip",
	}
	if _, err := env.gg(ctx, env.root, "commit", "--amend", "-m", wantMessage); err != nil {
		t.Fatal(err)
	}
	if newParent, err := gittool.ParseRev(ctx, env.git, "HEAD~"); err != nil {
		t.Error(err)
	} else if newParent.Commit() != parent.Commit() {
		t.Errorf("HEAD~ after amend = %s; want %s",
			prettyCommit(newParent.Commit(), changes),
			prettyCommit(parent.Commit(), changes))
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Commit() == r2.Commit() {
		t.Fatal("commit --amend did not create a new commit in the working copy")
	}
	if ref := r2.Ref(); ref != "refs/heads/master" {
		t.Errorf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitAddedFileContent {
		t.Errorf("added.txt = %q; want %q", data, commitAddedFileContent)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.Commit().String()+":deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.Commit().String()); err != nil {
		t.Error(err)
	} else if got := strings.TrimRight(string(msg), "\n"); got != wantMessage {
		t.Errorf("commit message = %q; want %q", got, wantMessage)
	}
}

func TestCommit_NoChanges(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := stageCommitTest(ctx, env, false); err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root, "commit", "-m", "nothing to see here"); err == nil {
		t.Error("commit with no changes did not return error")
	} else if isUsage(err) {
		t.Errorf("commit with no changes returned usage error: %v", err)
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Commit() != r2.Commit() {
		t.Errorf("commit created new commit %s; wanted to stay on %s", r2.Commit(), r1.Commit())
	}
	if ref := r2.Ref(); ref != "refs/heads/master" {
		t.Errorf("HEAD ref = %q; want refs/heads/master", ref)
	}
}

func TestCommit_AmendJustMessage(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := stageCommitTest(ctx, env, false); err != nil {
		t.Fatal(err)
	}
	parent, err := gittool.ParseRev(ctx, env.git, "HEAD~")
	if err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	const wantMessage = "gg amended this commit"
	if _, err := env.gg(ctx, env.root, "commit", "--amend", "-m", wantMessage); err != nil {
		t.Fatal(err)
	}
	changes := map[gitobj.Hash]string{
		parent.Commit(): "parent commit",
		r1.Commit():     "tip",
	}
	if newParent, err := gittool.ParseRev(ctx, env.git, "HEAD~"); err != nil {
		t.Error(err)
	} else if newParent.Commit() != parent.Commit() {
		t.Errorf("HEAD~ after amend = %s; want %s",
			prettyCommit(newParent.Commit(), changes),
			prettyCommit(parent.Commit(), changes))
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Commit() == r2.Commit() {
		t.Fatal("commit --amend did not create a new commit in the working copy")
	}
	if ref := r2.Ref(); ref != "refs/heads/master" {
		t.Errorf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.Commit().String()); err != nil {
		t.Error(err)
	} else if got := strings.TrimRight(string(msg), "\n"); got != wantMessage {
		t.Errorf("commit message = %q; want %q", got, wantMessage)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileOldContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.Commit().String()+":deleted.txt"); err != nil {
		t.Error("deleted.txt was removed:", err)
	}
}

func TestCommit_NoArgs_InSubdir(t *testing.T) {
	// Regression test for https://github.com/zombiezen/gg/issues/10

	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := stageCommitTest(ctx, env, true); err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(env.root, "foo")
	if err := os.Mkdir(subdir, 0777); err != nil {
		t.Fatal(err)
	}
	const wantMessage = "gg made this commit"
	if _, err := env.gg(ctx, subdir, "commit", "-m", wantMessage); err != nil {
		t.Fatal(err)
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Commit() == r2.Commit() {
		t.Fatal("commit did not create a new commit in the working copy")
	}
	if ref := r2.Ref(); ref != "refs/heads/master" {
		t.Errorf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitAddedFileContent {
		t.Errorf("added.txt = %q; want %q", data, commitAddedFileContent)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.Commit().String()+":deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.Commit().String()); err != nil {
		t.Error(err)
	} else if got := strings.TrimRight(string(msg), "\n"); got != wantMessage {
		t.Errorf("commit message = %q; want %q", got, wantMessage)
	}
}

func TestCommit_Named_InSubdir(t *testing.T) {
	// Regression test for https://github.com/zombiezen/gg/issues/10

	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := stageCommitTest(ctx, env, true); err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(env.root, "foo")
	if err := os.Mkdir(subdir, 0777); err != nil {
		t.Fatal(err)
	}
	const wantMessage = "gg made this commit"
	if _, err := env.gg(ctx, subdir, "commit", "-m", wantMessage, "../added.txt", "../deleted.txt", "../modified.txt"); err != nil {
		t.Fatal(err)
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Commit() == r2.Commit() {
		t.Fatal("commit did not create a new commit in the working copy")
	}
	if ref := r2.Ref(); ref != "refs/heads/master" {
		t.Errorf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitAddedFileContent {
		t.Errorf("added.txt = %q; want %q", data, commitAddedFileContent)
	}
	if data, err := catBlob(ctx, env.git, r2.Commit().String(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.Commit().String()+":deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.Commit().String()); err != nil {
		t.Error(err)
	} else if got := strings.TrimRight(string(msg), "\n"); got != wantMessage {
		t.Errorf("commit message = %q; want %q", got, wantMessage)
	}
}

func TestCommit_Merge(t *testing.T) {
	// Regression test for https://github.com/zombiezen/gg/issues/38

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
	if err := env.git.Run(ctx, "checkout", "--quiet", "-b", "feature"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(env.root, "foo.txt"), []byte("feature content\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "Made a change myself"); err != nil {
		t.Fatal(err)
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "--quiet", "master"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(env.root, "foo.txt"), []byte("boring text\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "Upstream change"); err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// Run the merge, resolve the conflict.
	// Using gg, since this is a multi-interaction integration test.
	out, err := env.gg(ctx, env.root, "merge", "feature")
	if len(out) > 0 {
		t.Logf("merge output:\n%s", out)
	}
	if err == nil {
		t.Errorf("Wanted merge to return error (conflict). Output:\n%s", out)
	} else if isUsage(err) {
		t.Fatalf("merge returned usage error: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(env.root, "foo.txt"), []byte("merged content!\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, env.root, "add", "foo.txt"); err != nil {
		t.Error("add:", err)
	}

	// Commit merge.
	out, err = env.gg(ctx, env.root, "commit", "-m", "Merged feature into master")
	if len(out) > 0 {
		t.Logf("commit output:\n%s", out)
	}
	if err != nil {
		t.Error("commit:", err)
	}
	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	names := map[gitobj.Hash]string{
		base:        "initial commit",
		r1.Commit(): "master commit",
		r2.Commit(): "branch commit",
	}
	if curr.Commit() == base || curr.Commit() == r1.Commit() || curr.Commit() == r2.Commit() {
		t.Errorf("after merge commit, HEAD = %s; want new commit",
			prettyCommit(curr.Commit(), names))
	}
	parent1, err := gittool.ParseRev(ctx, env.git, "HEAD^1")
	if err != nil {
		t.Fatal(err)
	}
	if parent1.Commit() != r1.Commit() {
		t.Errorf("after merge commit, HEAD^1 = %s; want %s",
			prettyCommit(parent1.Commit(), names),
			prettyCommit(r1.Commit(), names))
	}
	parent2, err := gittool.ParseRev(ctx, env.git, "HEAD^2")
	if err != nil {
		t.Fatal(err)
	}
	if parent2.Commit() != r2.Commit() {
		t.Errorf("after merge commit, HEAD^2 = %s; want %s",
			prettyCommit(parent2.Commit(), names),
			prettyCommit(r2.Commit(), names))
	}
}

func TestToPathspecs(t *testing.T) {
	tests := []struct {
		top  string
		wd   string
		file string

		wantTop bool
		want    string
	}{
		{
			top:     "foo",
			wd:      "foo",
			file:    "bar.txt",
			wantTop: true,
			want:    "bar.txt",
		},
		{
			top:     "foo",
			wd:      "foo/bar",
			file:    "baz.txt",
			wantTop: true,
			want:    "bar/baz.txt",
		},
		{
			top:     "foo",
			wd:      "foo/bar",
			file:    "../baz.txt",
			wantTop: true,
			want:    "baz.txt",
		},
		{
			top:     "foo",
			wd:      "foo",
			file:    "../baz.txt",
			wantTop: false,
			want:    "baz.txt",
		},
		{
			top:     "foo",
			wd:      "bar",
			file:    "../baz.txt",
			wantTop: false,
			want:    "baz.txt",
		},
		{
			top:     "foo",
			wd:      "bar",
			file:    "../foo/baz.txt",
			wantTop: true,
			want:    "baz.txt",
		},
	}
	root, err := ioutil.TempDir("", "gg_pathspec_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		wd := filepath.Join(root, filepath.FromSlash(test.wd))
		if err := os.MkdirAll(wd, 0777); err != nil {
			t.Error(err)
			continue
		}
		top := filepath.Join(root, filepath.FromSlash(test.top))
		if err := os.MkdirAll(top, 0777); err != nil {
			t.Error(err)
			continue
		}
		files := []string{test.file}
		if err := toPathspecs(wd, top, files); err != nil {
			t.Errorf("toPathspecs(%q, %q, [%q]): %v", test.wd, test.top, test.file, err)
			continue
		}
		magic, got := parsePathspec(files[0])
		isLiteral, isTop := false, false
		for _, word := range magic {
			switch word {
			case "literal":
				isLiteral = true
			case "top":
				isTop = true
			default:
				t.Errorf("toPathspecs(%q, %q, [%q]) has magic word %q (full spec: %v)", test.wd, test.top, test.file, word, files[0])
			}
		}
		if !isLiteral {
			t.Errorf("toPathspecs(%q, %q, [%q]) does not have expected magic word \"literal\" (full spec: %v)", test.wd, test.top, test.file, files[0])
		}
		if isTop {
			if !test.wantTop {
				t.Errorf("toPathspecs(%q, %q, [%q]) has magic word \"top\" (full spec: %v)", test.wd, test.top, test.file, files[0])
			}
			if want := filepath.FromSlash(test.want); got != want {
				t.Errorf("toPathspecs(%q, %q, [%q]) = %q; want %q", test.wd, test.top, test.file, files[0], ":(top,literal)"+want)
			}
		} else {
			if test.wantTop {
				t.Errorf("toPathspecs(%q, %q, [%q]) does not have expected magic word \"top\" (full spec: %v)", test.wd, test.top, test.file, files[0])
			}
			if want := filepath.Join(root, filepath.FromSlash(test.want)); got != want {
				t.Errorf("toPathspecs(%q, %q, [%q]) = %q; want %q", test.wd, test.top, test.file, files[0], ":(literal)"+want)
			}
		}
	}
}

func parsePathspec(spec string) ([]string, string) {
	switch {
	case strings.HasPrefix(spec, ":("):
		i := strings.IndexByte(spec, ')')
		if i == -1 {
			return nil, spec
		}
		return strings.Split(spec[2:i], ","), spec[i+1:]
	case strings.HasPrefix(spec, ":"):
		i := strings.IndexByte(spec[1:], ':')
		if i == -1 {
			return nil, spec
		}
		// Test only cares about long-form magic.
		return nil, spec[i+2:]
	default:
		return nil, spec
	}
}

func stageCommitTest(ctx context.Context, env *testEnv, changeWC bool) error {
	if err := env.git.Run(ctx, "init"); err != nil {
		return err
	}
	err := ioutil.WriteFile(filepath.Join(env.root, "modified.txt"), []byte("Hello, World!\n"), 0666)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(env.root, "deleted.txt"), []byte("To be removed...\n"), 0666)
	if err != nil {
		return err
	}
	if err := env.git.Run(ctx, "add", "modified.txt", "deleted.txt"); err != nil {
		return err
	}
	if err := env.git.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath.Join(env.root, "modified.txt"), []byte(commitModifiedFileOldContent), 0666)
	if err != nil {
		return err
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "second commit (so amend will have a parent)"); err != nil {
		return err
	}

	if changeWC {
		err = ioutil.WriteFile(filepath.Join(env.root, "added.txt"), []byte(commitAddedFileContent), 0666)
		if err != nil {
			return err
		}
		if err := env.git.Run(ctx, "add", "-N", "added.txt"); err != nil {
			return err
		}
		err = ioutil.WriteFile(filepath.Join(env.root, "modified.txt"), []byte(commitModifiedFileContent), 0666)
		if err != nil {
			return err
		}
		if err := env.git.Run(ctx, "rm", "deleted.txt"); err != nil {
			return err
		}
	}

	return nil
}

func catBlob(ctx context.Context, git *gittool.Tool, rev string, path string) ([]byte, error) {
	p, err := git.Start(ctx, "cat-file", "blob", rev+":"+path)
	if err != nil {
		return nil, fmt.Errorf("cat %s @ %s: %v", path, rev, err)
	}
	data, err := ioutil.ReadAll(p)
	waitErr := p.Wait()
	if err != nil {
		return nil, fmt.Errorf("cat %s @ %s: %v", path, rev, err)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("cat %s @ %s: %v", path, rev, waitErr)
	}
	return data, nil
}

func readCommitMessage(ctx context.Context, git *gittool.Tool, rev string) ([]byte, error) {
	p, err := git.Start(ctx, "show", "-s", "--format=%B", rev)
	if err != nil {
		return nil, fmt.Errorf("log %s: %v", rev, err)
	}
	data, err := ioutil.ReadAll(p)
	waitErr := p.Wait()
	if err != nil {
		return nil, fmt.Errorf("log %s: %v", rev, err)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("log %s: %v", rev, waitErr)
	}
	return data, nil
}

func objectExists(ctx context.Context, git *gittool.Tool, obj string) error {
	exists, err := git.Query(ctx, "cat-file", "-e", obj)
	if err != nil {
		return fmt.Errorf("object %s does not exist: %v", obj, err)
	}
	if !exists {
		return fmt.Errorf("object %s does not exist", obj)
	}
	return nil
}
