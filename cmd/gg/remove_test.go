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

const removeTestFileName = "foo.txt"

func TestRemove(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRemoveTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, repoPath, "rm", removeTestFileName); err != nil {
		t.Fatal(err)
	}
	git := env.git.WithDir(repoPath)
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
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
		if ent.name != removeTestFileName {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if ent.code[0] != 'D' || ent.code[1] != ' ' {
			t.Errorf("status = '%c%c'; want 'D '", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file %s unmodified", removeTestFileName)
	}
}

func TestRemove_AddedFails(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := env.git.Run(ctx, "init", repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, removeTestFileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	git := env.git.WithDir(repoPath)
	if err := git.Run(ctx, "add", removeTestFileName); err != nil {
		t.Fatal(err)
	}

	if _, err = env.gg(ctx, repoPath, "rm", removeTestFileName); err == nil {
		t.Error("`gg rm` returned success on added file")
	} else if _, isUsage := err.(*usageError); isUsage {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
	}
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
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
		if ent.name != removeTestFileName {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if ent.code[0] != 'A' || ent.code[1] != ' ' {
			t.Errorf("status = '%c%c'; want 'A '", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file %s removed", removeTestFileName)
	}
}

func TestRemove_AddedForce(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := env.git.Run(ctx, "init", repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, removeTestFileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	git := env.git.WithDir(repoPath)
	if err := git.Run(ctx, "add", removeTestFileName); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, repoPath, "rm", "-f", removeTestFileName); err != nil {
		t.Fatal(err)
	}
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
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

func TestRemove_ModifiedFails(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRemoveTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, removeTestFileName),
		[]byte("The world has changed...\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = env.gg(ctx, repoPath, "rm", removeTestFileName); err == nil {
		t.Error("`gg rm` returned success on modified file")
	} else if _, isUsage := err.(*usageError); isUsage {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
	}
	git := env.git.WithDir(repoPath)
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
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
		if ent.name != removeTestFileName {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if ent.code[0] != ' ' || ent.code[1] != 'M' {
			t.Errorf("status = '%c%c'; want ' M'", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file %s reverted", removeTestFileName)
	}
}

func TestRemove_ModifiedForce(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRemoveTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(repoPath, removeTestFileName),
		[]byte("The world has changed...\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, repoPath, "rm", "-f", removeTestFileName); err != nil {
		t.Fatal(err)
	}
	git := env.git.WithDir(repoPath)
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
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
		if ent.name != removeTestFileName {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if ent.code[0] != 'D' || ent.code[1] != ' ' {
			t.Errorf("status = '%c%c'; want 'D '", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file %s unmodified", removeTestFileName)
	}
}

func TestRemove_MissingFails(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRemoveTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repoPath, removeTestFileName)); err != nil {
		t.Fatal(err)
	}

	if _, err = env.gg(ctx, repoPath, "rm", removeTestFileName); err == nil {
		t.Error("`gg rm` returned success on missing file")
	} else if _, isUsage := err.(*usageError); isUsage {
		t.Errorf("`gg rm` error: %v; want failure, not usage", err)
	}
	git := env.git.WithDir(repoPath)
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
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
		if ent.name != removeTestFileName {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if ent.code[0] != ' ' || ent.code[1] != 'D' {
			t.Errorf("status = '%c%c'; want ' D'", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file %s reverted", removeTestFileName)
	}
}

func TestRemove_MissingAfter(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := stageRemoveTest(ctx, env.git, repoPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repoPath, removeTestFileName)); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, repoPath, "rm", "-after", removeTestFileName); err != nil {
		t.Fatal(err)
	}
	git := env.git.WithDir(repoPath)
	p, err := git.Start(ctx, "status", "--porcelain", "-z", "-unormal")
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
		if ent.name != removeTestFileName {
			t.Errorf("unknown line in status: '%c%c' %s", ent.code[0], ent.code[1], ent.name)
			continue
		}
		found = true
		if ent.code[0] != 'D' || ent.code[1] != ' ' {
			t.Errorf("status = '%c%c'; want 'D '", ent.code[0], ent.code[1])
		}
	}
	if !found {
		t.Errorf("file %s reverted", removeTestFileName)
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
