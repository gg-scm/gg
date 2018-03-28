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
	"strings"
	"testing"

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
	if err := stageCommitTest(ctx, env); err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	const wantMessage = "gg made this commit"
	if err := env.gg(ctx, env.root, "commit", "-m", wantMessage); err != nil {
		t.Fatal(err)
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.CommitHex() == r2.CommitHex() {
		t.Fatal("commit did not create a new commit in the working copy")
	}
	if ref := r2.RefName(); ref != "refs/heads/master" {
		t.Fatalf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if data, err := catBlob(ctx, env.git, r2.CommitHex(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitAddedFileContent {
		t.Errorf("added.txt = %q; want %q", data, commitAddedFileContent)
	}
	if data, err := catBlob(ctx, env.git, r2.CommitHex(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.CommitHex()+":deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.CommitHex()); err != nil {
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
	if err := stageCommitTest(ctx, env); err != nil {
		t.Fatal(err)
	}
	r1, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	const wantMessage = "gg made this commit"
	if err := env.gg(ctx, env.root, "commit", "-m", wantMessage, "modified.txt"); err != nil {
		t.Fatal(err)
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.CommitHex() == r2.CommitHex() {
		t.Fatal("commit did not create a new commit in the working copy")
	}
	if ref := r2.RefName(); ref != "refs/heads/master" {
		t.Fatalf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if data, err := catBlob(ctx, env.git, r2.CommitHex(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.CommitHex()+":added.txt"); err == nil {
		t.Error("added.txt was added but not put in arguments")
	}
	if err := objectExists(ctx, env.git, r2.CommitHex()+":deleted.txt"); err != nil {
		t.Error("deleted.txt was removed but not put in arguments:", err)
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.CommitHex()); err != nil {
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
	if err := stageCommitTest(ctx, env); err != nil {
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
	if err := env.gg(ctx, env.root, "commit", "--amend", "-m", wantMessage); err != nil {
		t.Fatal(err)
	}
	if newParent, err := gittool.ParseRev(ctx, env.git, "HEAD~"); err != nil {
		t.Error(err)
	} else if newParent.CommitHex() == r1.CommitHex() {
		t.Error("commit --amend created new commit descending from HEAD")
	} else if newParent.CommitHex() != parent.CommitHex() {
		t.Errorf("HEAD~ = %s; want %s", newParent.CommitHex(), parent.CommitHex())
	}
	r2, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if r1.CommitHex() == r2.CommitHex() {
		t.Fatal("commit --amend did not create a new commit in the working copy")
	}
	if ref := r2.RefName(); ref != "refs/heads/master" {
		t.Fatalf("HEAD ref = %q; want refs/heads/master", ref)
	}
	if data, err := catBlob(ctx, env.git, r2.CommitHex(), "added.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitAddedFileContent {
		t.Errorf("added.txt = %q; want %q", data, commitAddedFileContent)
	}
	if data, err := catBlob(ctx, env.git, r2.CommitHex(), "modified.txt"); err != nil {
		t.Error(err)
	} else if string(data) != commitModifiedFileContent {
		t.Errorf("modified.txt = %q; want %q", data, commitModifiedFileContent)
	}
	if err := objectExists(ctx, env.git, r2.CommitHex()+":deleted.txt"); err == nil {
		t.Error("deleted.txt exists")
	}
	if msg, err := readCommitMessage(ctx, env.git, r2.CommitHex()); err != nil {
		t.Error(err)
	} else if got := strings.TrimRight(string(msg), "\n"); got != wantMessage {
		t.Errorf("commit message = %q; want %q", got, wantMessage)
	}
}

func stageCommitTest(ctx context.Context, env *testEnv) error {
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

	return nil
}

func catBlob(ctx context.Context, git *gittool.Tool, rev, path string) ([]byte, error) {
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
	if err := git.Run(ctx, "cat-file", "-e", obj); err != nil {
		return fmt.Errorf("object %s does not exist: %v", obj, err)
	}
	return nil
}
