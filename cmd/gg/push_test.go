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
	"testing"

	"zombiezen.com/go/gg/internal/gittool"
)

func TestPush(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)

	if err := env.gg(ctx, pushEnv.repoA, "push"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("refs/heads/master = %s (first commit); want %s", r.CommitHex(), pushEnv.commit2)
		} else if r.CommitHex() != pushEnv.commit2 {
			t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit2)
		}
	}
}

func TestPush_Arg(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)
	if err := env.git.Run(ctx, "clone", "--bare", "repoB", "repoC"); err != nil {
		t.Fatal(err)
	}

	if err := env.gg(ctx, pushEnv.repoA, "push", filepath.Join(env.root, "repoC")); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("origin refs/heads/master = %s (pushed commit); want %s", r.CommitHex(), pushEnv.commit1)
		} else if r.CommitHex() != pushEnv.commit1 {
			t.Errorf("origin refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit1)
		}
	}
	gitC := env.git.WithDir(filepath.Join(env.root, "repoC"))
	if r, err := gittool.ParseRev(ctx, gitC, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("named remote refs/heads/master = %s (first commit); want %s", r.CommitHex(), pushEnv.commit2)
		} else if r.CommitHex() != pushEnv.commit2 {
			t.Errorf("named remote refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit2)
		}
	}
}

func TestPush_FailUnknownRef(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)

	if err := env.gg(ctx, pushEnv.repoA, "push", "-d", "foo"); err == nil {
		t.Error("push of new ref did not return error")
	} else if _, isUsage := err.(*usageError); isUsage {
		t.Errorf("push of new ref returned usage error: %v", err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("refs/heads/master = %s (pushed commit); want %s", r.CommitHex(), pushEnv.commit1)
		} else if r.CommitHex() != pushEnv.commit1 {
			t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit1)
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "foo"); err == nil {
		if ref := r.RefName(); ref != "" {
			t.Errorf("foo resolved to %s", ref)
		}
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("foo = %s (first commit); want to not exist", r.CommitHex())
		} else if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("foo = %s (pushed commit); want to not exist", r.CommitHex())
		} else {
			t.Errorf("foo = %s; want to not exist", r.CommitHex())
		}
	}
}

func TestPush_CreateRef(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)

	if err := env.gg(ctx, pushEnv.repoA, "push", "-d", "foo", "--create"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("refs/heads/master = %s (pushed commit); want %s", r.CommitHex(), pushEnv.commit1)
		} else if r.CommitHex() != pushEnv.commit1 {
			t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit1)
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/foo"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("refs/heads/foo = %s (first commit); want %s", r.CommitHex(), pushEnv.commit2)
		} else if r.CommitHex() != pushEnv.commit2 {
			t.Errorf("refs/heads/foo = %s; want %s", r.CommitHex(), pushEnv.commit2)
		}
	}
}

type pushEnv struct {
	repoA, repoB     string
	commit1, commit2 string
}

func stagePushTest(ctx context.Context, env *testEnv) (*pushEnv, error) {
	repoA := filepath.Join(env.root, "repoA")
	if err := env.git.Run(ctx, "init", repoA); err != nil {
		return nil, err
	}
	repoB := filepath.Join(env.root, "repoB")
	if err := env.git.Run(ctx, "init", "--bare", repoB); err != nil {
		return nil, err
	}

	gitA := env.git.WithDir(repoA)
	const fileName = "foo.txt"
	err := ioutil.WriteFile(
		filepath.Join(repoA, fileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "add", fileName); err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		return nil, err
	}
	commit1, err := gittool.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		return nil, err
	}

	if err := gitA.Run(ctx, "remote", "add", "origin", repoB); err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "push", "origin", "master"); err != nil {
		return nil, err
	}
	gitB := env.git.WithDir(repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		return nil, err
	} else if r.CommitHex() != commit1.CommitHex() {
		return nil, fmt.Errorf("destination repository master = %v; want %v", r.CommitHex(), commit1.CommitHex())
	}

	err = ioutil.WriteFile(
		filepath.Join(repoA, fileName),
		[]byte("Hello, World!\nI've learned some things...\n"),
		0666)
	if err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "commit", "-a", "-m", "second commit"); err != nil {
		return nil, err
	}
	commit2, err := gittool.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		return nil, err
	}
	return &pushEnv{
		repoA:   repoA,
		repoB:   repoB,
		commit1: commit1.CommitHex(),
		commit2: commit2.CommitHex(),
	}, nil
}
