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
	"path/filepath"
	"testing"

	"zombiezen.com/go/gg/internal/gittool"
)

const cloneFileMsg = "Hello, World!\n"

func TestClone(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	head, err := setupCloneTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.gg(ctx, env.root, "clone", "repoA", "repoB"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(filepath.Join(env.root, "repoB"))
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != head {
			t.Errorf("HEAD = %s; want %s", r.CommitHex(), head)
		}
		if r.RefName() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.RefName())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/foo"); err != nil {
		t.Error(err)
	} else if r.CommitHex() != head {
		t.Errorf("refs/heads/foo = %s; want %s", r.CommitHex(), head)
	}
	if r, err := gittool.ParseRev(ctx, gitB, "refs/remotes/origin/master"); err != nil {
		t.Error(err)
	} else if r.CommitHex() != head {
		t.Errorf("refs/remotes/origin/master = %s; want %s", r.CommitHex(), head)
	}
	if r, err := gittool.ParseRev(ctx, gitB, "refs/remotes/origin/foo"); err != nil {
		t.Error(err)
	} else if r.CommitHex() != head {
		t.Errorf("refs/remotes/origin/foo = %s; want %s", r.CommitHex(), head)
	}
	got, err := ioutil.ReadFile(filepath.Join(env.root, "repoB", "foo.txt"))
	if err != nil {
		t.Error(err)
	} else if string(got) != cloneFileMsg {
		t.Errorf("repoB/foo.txt content = %q; want %q", got, cloneFileMsg)
	}
}

func TestClone_Branch(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	head, err := setupCloneTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.gg(ctx, env.root, "clone", "-b=foo", "repoA", "repoB"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(filepath.Join(env.root, "repoB"))
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != head {
			t.Errorf("HEAD = %s; want %s", r.CommitHex(), head)
		}
		if r.RefName() != "refs/heads/foo" {
			t.Errorf("HEAD refname = %q; want refs/heads/foo", r.RefName())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else if r.CommitHex() != head {
		t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), head)
	}
	if r, err := gittool.ParseRev(ctx, gitB, "refs/remotes/origin/master"); err != nil {
		t.Error(err)
	} else if r.CommitHex() != head {
		t.Errorf("refs/remotes/origin/master = %s; want %s", r.CommitHex(), head)
	}
	if r, err := gittool.ParseRev(ctx, gitB, "refs/remotes/origin/foo"); err != nil {
		t.Error(err)
	} else if r.CommitHex() != head {
		t.Errorf("refs/remotes/origin/foo = %s; want %s", r.CommitHex(), head)
	}
	got, err := ioutil.ReadFile(filepath.Join(env.root, "repoB", "foo.txt"))
	if err != nil {
		t.Error(err)
	} else if string(got) != cloneFileMsg {
		t.Errorf("repoB/foo.txt content = %q; want %q", got, cloneFileMsg)
	}
}

func setupCloneTest(ctx context.Context, env *testEnv) (head string, _ error) {
	repoA := filepath.Join(env.root, "repoA")
	if err := env.git.Run(ctx, "init", repoA); err != nil {
		return "", err
	}
	gitA := env.git.WithDir(repoA)
	const fileName = "foo.txt"
	err := ioutil.WriteFile(
		filepath.Join(repoA, fileName),
		[]byte(cloneFileMsg),
		0666)
	if err != nil {
		return "", err
	}
	if err := gitA.Run(ctx, "add", fileName); err != nil {
		return "", err
	}
	if err := gitA.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		return "", err
	}
	if err := gitA.Run(ctx, "branch", "foo"); err != nil {
		return "", err
	}
	r, err := gittool.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		return "", err
	}
	return r.CommitHex(), nil
}
