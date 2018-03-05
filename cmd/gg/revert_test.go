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
	"bufio"
	"context"
	"io"
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

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRevertTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, revertTestFileName1),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, revertTestFileName2),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	// Stage changes
	git := env.git.WithDir(repoPath)
	if err := git.Run(ctx, "add", revertTestFileName2); err != nil {
		t.Fatal(err)
	}

	if err := env.gg(ctx, repoPath, "revert", revertTestFileName1); err != nil {
		t.Fatal(err)
	}
	data1, err := ioutil.ReadFile(filepath.Join(repoPath, revertTestFileName1))
	if err != nil {
		t.Error(err)
	} else if string(data1) != revertTestContent1 {
		t.Errorf("unstaged modified file content = %q after revert; want %q", data1, revertTestContent1)
	}
	data2, err := ioutil.ReadFile(filepath.Join(repoPath, revertTestFileName2))
	if err != nil {
		t.Error(err)
	} else if string(data2) == revertTestContent2 {
		t.Error("unrelated file was reverted")
	}

	if err := env.gg(ctx, repoPath, "revert", revertTestFileName2); err != nil {
		t.Fatal(err)
	}
	data2, err = ioutil.ReadFile(filepath.Join(repoPath, revertTestFileName2))
	if err != nil {
		t.Error(err)
	} else if string(data2) != revertTestContent2 {
		t.Errorf("staged modified file content = %q after revert; want %q", data2, revertTestContent2)
	}
	p, err := git.Start(ctx, "status", "--porcelain=v1", "-z", "-unormal")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("read status entry:", err)
		}
		t.Errorf("found status: '%c%c' %s; want clean working copy", ent.code[0], ent.code[1], ent.name)
	}
}

func TestRevert_All(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRevertTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, revertTestFileName1),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, revertTestFileName2),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	// Stage changes
	git := env.git.WithDir(repoPath)
	if err := git.Run(ctx, "add", revertTestFileName2); err != nil {
		t.Fatal(err)
	}

	if err := env.gg(ctx, repoPath, "revert", "--all"); err != nil {
		t.Fatal(err)
	}
	data1, err := ioutil.ReadFile(filepath.Join(repoPath, revertTestFileName1))
	if err != nil {
		t.Error(err)
	} else if string(data1) != revertTestContent1 {
		t.Errorf("unstaged modified file content = %q after revert; want %q", data1, revertTestContent1)
	}
	data2, err := ioutil.ReadFile(filepath.Join(repoPath, revertTestFileName2))
	if err != nil {
		t.Error(err)
	} else if string(data2) != revertTestContent2 {
		t.Errorf("staged modified file content = %q after revert; want %q", data2, revertTestContent2)
	}

	p, err := git.Start(ctx, "status", "--porcelain=v1", "-z", "-unormal")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("read status entry:", err)
		}
		t.Errorf("found status: '%c%c' %s; want clean working copy", ent.code[0], ent.code[1], ent.name)
	}
}

func TestRevert_Rev(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRevertTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, revertTestFileName1),
		[]byte("mumble mumble"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	git := env.git.WithDir(repoPath)
	if err := git.Run(ctx, "commit", "-a", "-m", "second commit"); err != nil {
		t.Fatal(err)
	}

	if err := env.gg(ctx, repoPath, "revert", "-r", "HEAD^", revertTestFileName1); err != nil {
		t.Fatal(err)
	}
	data1, err := ioutil.ReadFile(filepath.Join(repoPath, revertTestFileName1))
	if err != nil {
		t.Error(err)
	} else if string(data1) != revertTestContent1 {
		t.Errorf("file content = %q after revert; want %q", data1, revertTestContent1)
	}

	p, err := git.Start(ctx, "status", "--porcelain=v1", "-z", "-unormal")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	found := false
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("read status entry:", err)
		}
		if ent.name != revertTestFileName1 {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if !(ent.code[0] == ' ' && ent.code[1] == 'M') && !(ent.code[0] == 'M' || ent.code[1] == ' ') {
			t.Errorf("status = '%c%c'; want ' M' or 'M '", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file %s unmodified", revertTestFileName1)
	}
}

func TestRevert_Missing(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRevertTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repoPath, revertTestFileName1)); err != nil {
		t.Fatal(err)
	}

	if err := env.gg(ctx, repoPath, "revert", revertTestFileName1); err != nil {
		t.Fatal(err)
	}
	data1, err := ioutil.ReadFile(filepath.Join(repoPath, revertTestFileName1))
	if err != nil {
		t.Error(err)
	} else if string(data1) != revertTestContent1 {
		t.Errorf("file content = %q after revert; want %q", data1, revertTestContent1)
	}

	git := env.git.WithDir(repoPath)
	p, err := git.Start(ctx, "status", "--porcelain=v1", "-z", "-unormal")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Wait()
	r := bufio.NewReader(p)
	for {
		ent, err := readStatusEntry(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("read status entry:", err)
		}
		t.Errorf("found status: '%c%c' %s; want clean working copy", ent.code[0], ent.code[1], ent.name)
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
