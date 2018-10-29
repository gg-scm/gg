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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gg-scm.io/pkg/internal/gittool"
)

const removeTestFileName = "foo.txt"

func TestRemove(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := stageRemoveTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "rm", removeTestFileName); err != nil {
		t.Fatal(err)
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
		if ent.Name() != removeTestFileName {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("%s status = '%v'; want 'D '", removeTestFileName, code)
		}
	}
	if !found {
		t.Errorf("File %s unmodified", removeTestFileName)
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_AddedFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, removeTestFileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", removeTestFileName); err != nil {
		t.Fatal(err)
	}

	if _, err = env.gg(ctx, env.root, "rm", removeTestFileName); err == nil {
		t.Error("`gg rm` returned success on added file")
	} else if isUsage(err) {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
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
		if ent.Name() != removeTestFileName {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'A' || code[1] != ' ' {
			t.Errorf("%s status = '%v'; want 'A '", removeTestFileName, code)
		}
	}
	if !found {
		t.Errorf("File %s removed", removeTestFileName)
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_AddedForce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, removeTestFileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", removeTestFileName); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "rm", "-f", removeTestFileName); err != nil {
		t.Fatal(err)
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

func TestRemove_ModifiedFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := stageRemoveTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, removeTestFileName),
		[]byte("The world has changed...\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = env.gg(ctx, env.root, "rm", removeTestFileName); err == nil {
		t.Error("`gg rm` returned success on modified file")
	} else if isUsage(err) {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
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
		if ent.Name() != removeTestFileName {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != ' ' || code[1] != 'M' {
			t.Errorf("%s status = '%v'; want ' M'", removeTestFileName, code)
		}
	}
	if !found {
		t.Errorf("File %s reverted", removeTestFileName)
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_ModifiedForce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := stageRemoveTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, removeTestFileName),
		[]byte("The world has changed...\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "rm", "-f", removeTestFileName); err != nil {
		t.Fatal(err)
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
		if ent.Name() != removeTestFileName {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("%s status = '%v'; want 'D '", removeTestFileName, code)
		}
	}
	if !found {
		t.Errorf("File %s unmodified", removeTestFileName)
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_MissingFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := stageRemoveTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(env.root, removeTestFileName)); err != nil {
		t.Fatal(err)
	}

	if _, err = env.gg(ctx, env.root, "rm", removeTestFileName); err == nil {
		t.Error("`gg rm` returned success on missing file")
	} else if isUsage(err) {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
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
		if ent.Name() != removeTestFileName {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != ' ' || code[1] != 'D' {
			t.Errorf("%s status = '%v'; want ' D'", removeTestFileName, code)
		}
	}
	if !found {
		t.Errorf("File %s reverted", removeTestFileName)
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_MissingAfter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := stageRemoveTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(env.root, removeTestFileName)); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "rm", "-after", removeTestFileName); err != nil {
		t.Fatal(err)
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
		if ent.Name() != removeTestFileName {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("%s status = '%v'; want 'D '", removeTestFileName, code)
		}
	}
	if !found {
		t.Errorf("File %s reverted", removeTestFileName)
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_Recursive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := os.Mkdir(filepath.Join(env.root, "foo"), 0777); err != nil {
		t.Fatal(err)
	}
	relpath := filepath.Join("foo", "bar.txt")
	err = ioutil.WriteFile(
		filepath.Join(env.root, relpath),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "init", env.root); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", relpath); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "committed"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "rm", "-r", "foo"); err != nil {
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
		if ent.Name() != "foo/bar.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("foo/bar.txt status = '%v'; want 'D '", code)
		}
	}
	if !found {
		t.Errorf("File foo/bar.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_RecursiveMissingFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := os.Mkdir(filepath.Join(env.root, "foo"), 0777); err != nil {
		t.Fatal(err)
	}
	relpath := filepath.Join("foo", "bar.txt")
	err = ioutil.WriteFile(
		filepath.Join(env.root, relpath),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "init", env.root); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", relpath); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "committed"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(env.root, "foo")); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "rm", "-r", "foo"); err == nil {
		t.Error("`gg rm -r` returned success on missing directory")
	} else if isUsage(err) {
		t.Errorf("`gg rm -r` error: %v; want failure, not usage", err)
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
		if ent.Name() != "foo/bar.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != ' ' || code[1] != 'D' {
			t.Errorf("foo/bar.txt status = '%v'; want ' D'", code)
		}
	}
	if !found {
		t.Errorf("File foo/bar.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestRemove_RecursiveMissingAfter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := os.Mkdir(filepath.Join(env.root, "foo"), 0777); err != nil {
		t.Fatal(err)
	}
	relpath := filepath.Join("foo", "bar.txt")
	err = ioutil.WriteFile(
		filepath.Join(env.root, relpath),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "init", env.root); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", relpath); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "committed"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(env.root, "foo")); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "rm", "-r", "-after", "foo"); err != nil {
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
		if ent.Name() != "foo/bar.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'D' || code[1] != ' ' {
			t.Errorf("foo/bar.txt status = '%v'; want 'D '", code)
		}
	}
	if !found {
		t.Errorf("File foo/bar.txt unmodified")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func stageRemoveTest(ctx context.Context, git *gittool.Tool, repo string) error {
	if err := git.Run(ctx, "init", repo); err != nil {
		return err
	}
	err := ioutil.WriteFile(
		filepath.Join(repo, removeTestFileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		return err
	}
	git = git.WithDir(repo)
	if err := git.Run(ctx, "add", removeTestFileName); err != nil {
		return err
	}
	if err := git.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		return err
	}
	return nil
}
