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

	"zombiezen.com/go/gg/internal/gittool"
)

func TestAdd(t *testing.T) {
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
		filepath.Join(env.root, "foo.txt"),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	st, err := gittool.Status(ctx, env.git, nil)
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'A' && code[1] != 'A' {
			t.Errorf("foo.txt status = '%v'; want to contain 'A'", code)
		}
	}
	if !found {
		t.Error("File foo.txt not in git status")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestAdd_DoesNotStageModified(t *testing.T) {
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
		filepath.Join(env.root, "foo.txt"),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "commit"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte("Something different\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	st, err := gittool.Status(ctx, env.git, nil)
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != ' ' || code[1] != 'M' {
			t.Errorf("foo.txt status = '%v'; want ' M'", code)
		}
	}
	if !found {
		t.Error("File foo.txt not in git status")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestAdd_WholeRepo(t *testing.T) {
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
		filepath.Join(env.root, "foo.txt"),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "add", "."); err != nil {
		t.Error(err)
	}
	st, err := gittool.Status(ctx, env.git, nil)
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'A' && code[1] != 'A' {
			t.Errorf("foo.txt status = '%v'; want to contain 'A'", code)
		}
	}
	if !found {
		t.Error("File foo.txt not in git status")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestAdd_ResolveUnmerged(t *testing.T) {
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
		filepath.Join(env.root, "foo.txt"),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "commit"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte("Change A\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "branch A"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "-b", "feature", "HEAD~"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte("Change B\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "branch B"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "master"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "merge", "--no-ff", "feature"); err == nil {
		t.Fatal("Merge did not exit; want conflict")
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte("I resolved it!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "add", "foo.txt"); err != nil {
		t.Error("gg:", err)
	}
	st, err := gittool.Status(ctx, env.git, nil)
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
		if ent.Name() != "foo.txt" {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); code[0] != 'M' || code[1] != ' ' {
			t.Errorf("foo.txt status = '%v'; want 'M '", code)
		}
	}
	if !found {
		t.Error("File foo.txt not in git status")
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func TestAdd_Directory(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}

	dirPath := filepath.Join(env.root, "foo")
	if err := os.Mkdir(dirPath, 0777); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(dirPath, "bar.txt"),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", "."); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "commit"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(dirPath, "bar.txt"),
		[]byte("Change A\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "branch A"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "-b", "feature", "HEAD~"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(dirPath, "bar.txt"),
		[]byte("Change B\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "branch B"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "master"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "merge", "--no-ff", "feature"); err == nil {
		t.Fatal("Merge did not exit; want conflict")
	}
	err = ioutil.WriteFile(
		filepath.Join(dirPath, "bar.txt"),
		[]byte("I resolved it!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(dirPath, "newfile.txt"),
		[]byte("Another file!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "add", "foo"); err != nil {
		t.Error("gg:", err)
	}
	st, err := gittool.Status(ctx, env.git, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := st.Close(); err != nil {
			t.Error("st.Close():", err)
		}
	}()
	foundBar, foundNewFile := false, false
	for st.Scan() {
		ent := st.Entry()
		code := ent.Code()
		switch ent.Name() {
		case "foo/bar.txt":
			foundBar = true
			if code[0] != 'M' || code[1] != ' ' {
				t.Errorf("foo/bar.txt status = '%v'; want 'M '", code)
			}
		case "foo/newfile.txt":
			foundNewFile = true
			if code[0] != 'A' && code[1] != 'A' {
				t.Errorf("foo/newfile.txt status = '%v'; want to contain 'A'", code)
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
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}
