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
	"os"
	"path/filepath"
	"testing"

	"gg-scm.io/pkg/internal/gittool"
)

func TestRevert(t *testing.T) {
	tests := []struct {
		name         string
		dir          string
		stagedPath   string
		unstagedPath string
	}{
		{
			name:         "TopLevel",
			dir:          ".",
			stagedPath:   "staged.txt",
			unstagedPath: "unstaged.txt",
		},
		{
			name:         "FromSubdir",
			dir:          "foo",
			stagedPath:   filepath.Join("..", "staged.txt"),
			unstagedPath: filepath.Join("..", "unstaged.txt"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()

			if err := env.initEmptyRepo(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.mkdir("foo"); err != nil {
				t.Fatal(err)
			}
			if err := env.writeFile("staged.txt", "ohai"); err != nil {
				t.Fatal(err)
			}
			if err := env.writeFile("unstaged.txt", "kthxbai"); err != nil {
				t.Fatal(err)
			}
			if err := env.trackFiles(ctx, "staged.txt", "unstaged.txt"); err != nil {
				t.Fatal(err)
			}
			if err := env.newCommit(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.writeFile("staged.txt", "mumble mumble 1"); err != nil {
				t.Fatal(err)
			}
			if err := env.writeFile("unstaged.txt", "mumble mumble 2"); err != nil {
				t.Fatal(err)
			}
			if err := env.addFiles(ctx, "staged.txt"); err != nil {
				t.Fatal(err)
			}

			if _, err := env.gg(ctx, env.rel(test.dir), "revert", test.stagedPath); err != nil {
				t.Fatal(err)
			}
			if got, err := env.readFile("staged.txt"); err != nil {
				t.Error(err)
			} else if want := "ohai"; got != want {
				t.Errorf("staged.txt content = %q after revert; want %q", got, want)
			}
			if got, err := env.readFile("staged.txt.orig"); err != nil {
				t.Error(err)
			} else if want := "mumble mumble 1"; got != want {
				t.Errorf("staged.txt.orig content = %q after revert; want %q", got, want)
			}

			if _, err := env.gg(ctx, env.rel(test.dir), "revert", test.unstagedPath); err != nil {
				t.Fatal(err)
			}
			if got, err := env.readFile("unstaged.txt"); err != nil {
				t.Error(err)
			} else if want := "kthxbai"; got != want {
				t.Errorf("unstaged.txt content = %q after revert; want %q", got, want)
			}
			if got, err := env.readFile("unstaged.txt.orig"); err != nil {
				t.Error(err)
			} else if want := "mumble mumble 2"; got != want {
				t.Errorf("unstaged.txt.orig content = %q after revert; want %q", got, want)
			}
			if got, err := env.readFile("staged.txt"); err != nil {
				t.Error(err)
			} else if want := "ohai"; got != want {
				t.Error("unrelated file was reverted")
			}

			// Verify that working copy is clean (sans backup files).
			st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := st.Close(); err != nil {
					t.Error("st.Close():", err)
				}
			}()
			for st.Scan() {
				if name := st.Entry().Name(); name == "staged.txt.orig" || name == "unstaged.txt.orig" {
					continue
				}
				t.Errorf("Found status: %v; want clean working copy", st.Entry())
			}
			if err := st.Err(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestRevert_AddedFile(t *testing.T) {
	for _, backup := range []bool{true, false} {
		var name string
		if backup {
			name = "WithBackup"
		} else {
			name = "NoBackup"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()

			if err := env.initRepoWithHistory(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.writeFile("foo.txt", "hey there"); err != nil {
				t.Fatal(err)
			}
			if err := env.trackFiles(ctx, "foo.txt"); err != nil {
				t.Fatal(err)
			}

			revertArgs := []string{"revert"}
			if !backup {
				revertArgs = append(revertArgs, "--no-backup")
			}
			revertArgs = append(revertArgs, "foo.txt")
			if _, err := env.gg(ctx, env.root, revertArgs...); err != nil {
				t.Fatal(err)
			}
			if got, err := env.readFile("foo.txt"); err != nil {
				t.Error(err)
			} else if want := "hey there"; got != want {
				t.Errorf("content = %q after revert; want %q", got, want)
			}
			_, err = os.Stat(env.rel("foo.txt.orig"))
			if err == nil {
				t.Error("foo.txt.orig was created")
			} else if !os.IsNotExist(err) {
				t.Error(err)
			}

			// Verify that file is untracked.
			st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := st.Close(); err != nil {
					t.Error("st.Close():", err)
				}
			}()
			for st.Scan() {
				ent := st.Entry()
				switch ent.Name() {
				case "foo.txt":
					if got := ent.Code(); !got.IsUntracked() {
						t.Errorf("%s status code = '%v'; want '??'", ent.Name(), got)
					}
				case "foo.txt.orig":
					// Ignore, error already reported.
				default:
					t.Errorf("Found status: %v; want untracked foo.txt", st.Entry())
				}
			}
			if err := st.Err(); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestRevert_AddedFileBeforeFirstCommit(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("foo.txt", "ohai"); err != nil {
		t.Fatal(err)
	}
	if err := env.trackFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if got, err := env.readFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "ohai"; got != want {
		t.Errorf("content = %q after revert; want %q", got, want)
	}
	if _, err := os.Stat(env.rel("foo.txt.orig")); err == nil {
		t.Error("foo.txt.orig was created")
	} else if !os.IsNotExist(err) {
		t.Error(err)
	}

	// Verify that file is untracked.
	st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	for st.Scan() {
		ent := st.Entry()
		switch ent.Name() {
		case "foo.txt":
			if got := ent.Code(); !got.IsUntracked() {
				t.Errorf("foo.txt status code = '%v'; want '??'", got)
			}
		case "foo.txt.orig":
			// Ignore, error already reported.
		default:
			t.Errorf("Found status: %v; want untracked foo.txt", st.Entry())
		}
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRevert_All(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("staged.txt", "original stage"); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("unstaged.txt", "original audience"); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "staged.txt", "unstaged.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	// Make working tree changes.
	if err := env.writeFile("staged.txt", "randomness"); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("unstaged.txt", "randomness"); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "staged.txt"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", "--all"); err != nil {
		t.Fatal(err)
	}
	if got, err := env.readFile("staged.txt"); err != nil {
		t.Error(err)
	} else if want := "original stage"; got != want {
		t.Errorf("staged modified file content = %q after revert; want %q", got, want)
	}
	if got, err := env.readFile("unstaged.txt"); err != nil {
		t.Error(err)
	} else if want := "original audience"; got != want {
		t.Errorf("unstaged modified file content = %q after revert; want %q", got, want)
	}

	st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	for st.Scan() {
		if name := st.Entry().Name(); name == "staged.txt.orig" || name == "unstaged.txt.orig" {
			continue
		}
		t.Errorf("Found status: %v; want clean working copy", st.Entry())
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRevert_Rev(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("foo.txt", "original content"); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("foo.txt", "super-fresh content"); err != nil {
		t.Fatal(err)
	}
	if err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", "-r", "HEAD^", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if got, err := env.readFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "original content"; got != want {
		t.Errorf("foo.txt content = %q after revert; want %q", got, want)
	}
	_, err = os.Stat(env.rel("foo.txt.orig"))
	if err == nil {
		t.Error("foo.txt.orig was created")
	} else if !os.IsNotExist(err) {
		t.Error(err)
	}

	st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	found := false
	for st.Scan() {
		ent := st.Entry()
		switch ent.Name() {
		case "foo.txt":
			found = true
			if code := ent.Code(); !(code[0] == ' ' && code[1] == 'M') && !(code[0] == 'M' || code[1] == ' ') {
				t.Errorf("foo.txt status = '%v'; want ' M' or 'M '", code)
			}
		case "foo.txt.orig":
			// Error already reported.
		default:
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
	}
	if !found {
		t.Error("File foo.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRevert_Missing(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("foo.txt", "original content"); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(env.rel("foo.txt")); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if got, err := env.readFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "original content"; got != want {
		t.Errorf("file content = %q after revert; want %q", got, want)
	}

	st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	for st.Scan() {
		t.Errorf("Found status: %v; want clean working copy", st.Entry())
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRevert_NoBackup(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("foo.txt", "original content"); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.writeFile("foo.txt", "tears in rain"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", "--no-backup", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if got, err := env.readFile("foo.txt"); err != nil {
		t.Error(err)
	} else if want := "original content"; got != want {
		t.Errorf("file content = %q after revert; want %q", got, want)
	}
	if _, err := os.Stat(env.rel("foo.txt.orig")); err == nil {
		t.Error("foo.txt.orig was created")
	} else if !os.IsNotExist(err) {
		t.Error(err)
	}
}

func TestRevert_LocalRename(t *testing.T) {
	// The `git status` that gets reported here is a little weird on newer
	// versions of Git. This makes sure that revert doesn't do something
	// naive.

	tests := []struct {
		name          string
		revertFoo     bool
		revertRenamed bool
	}{
		{name: "RevertOriginal", revertFoo: true},
		{name: "RevertRenamed", revertRenamed: true},
		{name: "RevertBoth", revertFoo: true, revertRenamed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()
			if err := env.initEmptyRepo(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.writeFile("foo.txt", "original content"); err != nil {
				t.Fatal(err)
			}
			if err := env.addFiles(ctx, "foo.txt"); err != nil {
				t.Fatal(err)
			}
			if err := env.newCommit(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(env.rel("foo.txt"), env.rel("renamed.txt")); err != nil {
				t.Fatal(err)
			}
			if err := env.trackFiles(ctx, "renamed.txt"); err != nil {
				t.Fatal(err)
			}

			revertArgs := []string{"revert"}
			if test.revertFoo {
				revertArgs = append(revertArgs, "foo.txt")
			}
			if test.revertRenamed {
				revertArgs = append(revertArgs, "renamed.txt")
			}
			if _, err := env.gg(ctx, env.root, revertArgs...); err != nil {
				t.Fatal(err)
			}
			if test.revertFoo {
				if got, err := env.readFile("foo.txt"); err != nil {
					t.Error(err)
				} else if want := "original content"; got != want {
					t.Errorf("foo.txt content = %q after revert; want %q", got, want)
				}
			} else {
				if _, err := os.Stat(env.rel("foo.txt")); err == nil {
					t.Error("foo.txt was created")
				} else if !os.IsNotExist(err) {
					t.Error(err)
				}
			}
			// Don't create a backup for renamed.txt.
			_, err = os.Stat(env.rel("renamed.txt.orig"))
			if err == nil {
				t.Error("renamed.txt.orig was created")
			} else if !os.IsNotExist(err) {
				t.Error(err)
			}
			// Verify that renamed.txt is untracked.
			st, err := gittool.Status(ctx, env.git, gittool.StatusOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := st.Close(); err != nil {
					t.Error("st.Close():", err)
				}
			}()
			for st.Scan() {
				ent := st.Entry()
				switch ent.Name() {
				case "renamed.txt":
					if got := ent.Code(); test.revertRenamed && !got.IsUntracked() {
						t.Errorf("renamed.txt status code = '%v'; want '??'", got)
					} else if !test.revertRenamed && !got.IsAdded() {
						t.Errorf("renamed.txt status code = '%v'; want to contain 'A'", got)
					}
				case "foo.txt", "foo.txt.orig", "renamed.txt.orig":
					// Ignore, error already reported if needed.
				default:
					t.Errorf("Found status: %v; want untracked renamed.txt", st.Entry())
				}
			}
			if err := st.Err(); err != nil {
				t.Error(err)
			}
		})
	}
}
