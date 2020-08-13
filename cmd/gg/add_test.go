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

package main

import (
	"context"
	"testing"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/filesystem"
	"github.com/google/go-cmp/cmp"
)

func TestAdd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	st, err := env.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 1 || st[0].Name != "foo.txt" || (st[0].Code[0] != 'A' && st[0].Code[1] != 'A') {
		t.Errorf("status = %v; want foo.txt with 'A'", st)
	}
}

func TestAdd_DoesNotStageModified(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Something different\n")); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	st, err := env.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []git.StatusEntry{
		{Code: git.StatusCode{' ', 'M'}, Name: "foo.txt"},
	}
	if diff := cmp.Diff(want, st); diff != "" {
		t.Errorf("status (-want +got):\n%s", diff)
	}
}

func TestAdd_WholeRepo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "add", "."); err != nil {
		t.Error(err)
	}
	st, err := env.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 1 || st[0].Name != "foo.txt" || (st[0].Code[0] != 'A' && st[0].Code[1] != 'A') {
		t.Errorf("status = %v; want foo.txt with 'A'", st)
	}
}

func TestAdd_ResolveUnmerged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Change A\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.git.NewBranch(ctx, "feature", git.BranchOptions{Checkout: true, StartPoint: "HEAD~"}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "Change B\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Merge(ctx, []string{"feature"}); err == nil {
		t.Fatal("Merge did not exit; want conflict")
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", "I resolved it!\n")); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	st, err := env.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []git.StatusEntry{
		{Code: git.StatusCode{'M', ' '}, Name: "foo.txt"},
	}
	if diff := cmp.Diff(want, st); diff != "" {
		t.Errorf("status (-want +got):\n%s", diff)
	}
}

func TestAdd_Directory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo/bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo/bar.txt", "Change A\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.git.NewBranch(ctx, "feature", git.BranchOptions{Checkout: true, StartPoint: "HEAD~"}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo/bar.txt", "Change B\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.git.CheckoutBranch(ctx, "main", git.CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Merge(ctx, []string{"feature"}); err == nil {
		t.Fatal("Merge did not exit; want conflict")
	}
	err = env.root.Apply(
		filesystem.Write("foo/bar.txt", "I resolved it!\n"),
		filesystem.Write("foo/newfile.txt", "Another file!\n"),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "add", "foo"); err != nil {
		t.Error("gg:", err)
	}
	st, err := env.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	foundBar, foundNewFile := false, false
	for _, ent := range st {
		switch ent.Name {
		case "foo/bar.txt":
			foundBar = true
			if ent.Code[0] != 'M' || ent.Code[1] != ' ' {
				t.Errorf("foo/bar.txt status = '%v'; want 'M '", ent.Code)
			}
		case "foo/newfile.txt":
			foundNewFile = true
			if ent.Code[0] != 'A' && ent.Code[1] != 'A' {
				t.Errorf("foo/newfile.txt status = '%v'; want to contain 'A'", ent.Code)
			}
		default:
			t.Errorf("Unknown line in status: %v", ent)
		}
	}
	if !foundBar {
		t.Error("File foo/bar.txt not in git status")
	}
	if !foundNewFile {
		t.Error("File foo/newfile.txt not in git status")
	}
}

func TestAdd_IgnoredFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	err = env.root.Apply(
		filesystem.Write(".gitignore", "/foo.txt\n"),
		filesystem.Write("foo.txt", dummyContent),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	st, err := env.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ent := range st {
		switch ent.Name {
		case "foo.txt":
			found = true
			if ent.Code[0] != 'A' && ent.Code[1] != 'A' {
				t.Errorf("foo.txt status = '%v'; want to contain 'A'", ent.Code)
			}
		case ".gitignore":
			if !ent.Code.IsUntracked() {
				t.Errorf(".gitignore status = '%v'; want untracked", ent.Code)
			}
		default:
			t.Errorf("Unknown line in status: %v", ent)
		}
	}
	if !found {
		t.Error("File foo.txt not in git status")
	}
}

func TestAdd_IgnoredFileInDirectory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	err = env.root.Apply(
		filesystem.Write(".gitignore", "/foo/bar.txt\n"),
		filesystem.Write("foo/bar.txt", dummyContent),
		filesystem.Write("foo/baz.txt", dummyContent),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "add", "foo"); err != nil {
		t.Error("gg:", err)
	}
	st, err := env.git.Status(ctx, git.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	foundBaz := false
	for _, ent := range st {
		switch ent.Name {
		case "foo/bar.txt":
			if ent.Code[0] != '!' || ent.Code[1] != '!' {
				t.Errorf("foo/bar.txt status = '%v'; want '!!'", ent.Code)
			}
		case "foo/baz.txt":
			foundBaz = true
			if ent.Code[0] != 'A' && ent.Code[1] != 'A' {
				t.Errorf("foo/baz.txt status = '%v'; want to contain 'A'", ent.Code)
			}
		case ".gitignore":
			if !ent.Code.IsUntracked() {
				t.Errorf(".gitignore status = '%v'; want untracked", ent.Code)
			}
		default:
			t.Errorf("Unknown line in status: %v", ent)
		}
	}
	if !foundBaz {
		t.Error("File foo/baz.txt not in git status")
	}
}
