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

const (
	revertTestFileName1 = "foo.txt"
	revertTestFileName2 = "bar.txt"

	revertTestContent1 = "Hello, World!\n"
	revertTestContent2 = "Hello again, World!\n"
)

func TestRevert(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := stageRevertTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, revertTestFileName1),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, revertTestFileName2),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	// Stage changes
	if err := env.git.Run(ctx, "add", revertTestFileName2); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", revertTestFileName1); err != nil {
		t.Fatal(err)
	}
	data1, err := ioutil.ReadFile(filepath.Join(env.root, revertTestFileName1))
	if err != nil {
		t.Error(err)
	} else if string(data1) != revertTestContent1 {
		t.Errorf("unstaged modified file content = %q after revert; want %q", data1, revertTestContent1)
	}
	data2, err := ioutil.ReadFile(filepath.Join(env.root, revertTestFileName2))
	if err != nil {
		t.Error(err)
	} else if string(data2) == revertTestContent2 {
		t.Error("unrelated file was reverted")
	}

	if _, err := env.gg(ctx, env.root, "revert", revertTestFileName2); err != nil {
		t.Fatal(err)
	}
	data2, err = ioutil.ReadFile(filepath.Join(env.root, revertTestFileName2))
	if err != nil {
		t.Error(err)
	} else if string(data2) != revertTestContent2 {
		t.Errorf("staged modified file content = %q after revert; want %q", data2, revertTestContent2)
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
	for st.Scan() {
		t.Errorf("Found status: %v; want clean working copy", st.Entry())
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

	if err := stageRevertTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, revertTestFileName1),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, revertTestFileName2),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	// Stage changes
	if err := env.git.Run(ctx, "add", revertTestFileName2); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", "--all"); err != nil {
		t.Fatal(err)
	}
	data1, err := ioutil.ReadFile(filepath.Join(env.root, revertTestFileName1))
	if err != nil {
		t.Error(err)
	} else if string(data1) != revertTestContent1 {
		t.Errorf("unstaged modified file content = %q after revert; want %q", data1, revertTestContent1)
	}
	data2, err := ioutil.ReadFile(filepath.Join(env.root, revertTestFileName2))
	if err != nil {
		t.Error(err)
	} else if string(data2) != revertTestContent2 {
		t.Errorf("staged modified file content = %q after revert; want %q", data2, revertTestContent2)
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
	for st.Scan() {
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

	if err := stageRevertTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, revertTestFileName1),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "second commit"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", "-r", "HEAD^", revertTestFileName1); err != nil {
		t.Fatal(err)
	}
	data1, err := ioutil.ReadFile(filepath.Join(env.root, revertTestFileName1))
	if err != nil {
		t.Error(err)
	} else if string(data1) != revertTestContent1 {
		t.Errorf("File content = %q after revert; want %q", data1, revertTestContent1)
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
		if ent.Name() != revertTestFileName1 {
			t.Errorf("Unknown line in status: %v", ent)
			continue
		}
		found = true
		if code := ent.Code(); !(code[0] == ' ' && code[1] == 'M') && !(code[0] == 'M' || code[1] == ' ') {
			t.Errorf("%s status = '%v'; want ' M' or 'M '", revertTestFileName1, code)
		}
	}
	if !found {
		t.Errorf("File %s unmodified", revertTestFileName1)
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

	if err := stageRevertTest(ctx, env.git, env.root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(env.root, revertTestFileName1)); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "revert", revertTestFileName1); err != nil {
		t.Fatal(err)
	}
	data1, err := ioutil.ReadFile(filepath.Join(env.root, revertTestFileName1))
	if err != nil {
		t.Error(err)
	} else if string(data1) != revertTestContent1 {
		t.Errorf("file content = %q after revert; want %q", data1, revertTestContent1)
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
	for st.Scan() {
		t.Errorf("Found status: %v; want clean working copy", st.Entry())
	}
	if err := st.Err(); err != nil {
		t.Error(err)
	}
}

func stageRevertTest(ctx context.Context, git *gittool.Tool, repo string) error {
	if err := git.Run(ctx, "init", repo); err != nil {
		return err
	}
	err := ioutil.WriteFile(
		filepath.Join(repo, revertTestFileName1),
		[]byte(revertTestContent1),
		0666)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(
		filepath.Join(repo, revertTestFileName2),
		[]byte(revertTestContent2),
		0666)
	if err != nil {
		return err
	}
	git = git.WithDir(repo)
	if err := git.Run(ctx, "add", revertTestFileName1, revertTestFileName2); err != nil {
		return err
	}
	if err := git.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		return err
	}
	return nil
}
