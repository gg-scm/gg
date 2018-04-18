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

func TestBranch(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	const fileName = "foo.txt"
	err = ioutil.WriteFile(
		filepath.Join(env.root, fileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", fileName); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		t.Fatal(err)
	}
	first, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root, "branch", "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	if r, err := gittool.ParseRev(ctx, env.git, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != first.CommitHex() {
			t.Errorf("HEAD = %s; want %s", r.CommitHex(), first.CommitHex())
		}
		if r.RefName() != "refs/heads/foo" {
			t.Errorf("HEAD refname = %q; want refs/heads/foo", r.RefName())
		}
	}
	if r, err := gittool.ParseRev(ctx, env.git, "bar"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != first.CommitHex() {
			t.Errorf("bar = %s; want %s", r.CommitHex(), first.CommitHex())
		}
		if r.RefName() != "refs/heads/bar" {
			t.Errorf("bar refname = %q; want refs/heads/bar", r.RefName())
		}
	}
}

func TestBranch_Upstream(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath1 := filepath.Join(env.root, "repo1")
	if err := env.git.Run(ctx, "init", repoPath1); err != nil {
		t.Fatal(err)
	}
	const fileName = "foo.txt"
	err = ioutil.WriteFile(
		filepath.Join(repoPath1, fileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	git1 := env.git.WithDir(repoPath1)
	if err := git1.Run(ctx, "add", fileName); err != nil {
		t.Fatal(err)
	}
	if err := git1.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		t.Fatal(err)
	}
	first, err := gittool.ParseRev(ctx, git1, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "clone", "repo1", "repo2"); err != nil {
		t.Fatal(err)
	}

	repoPath2 := filepath.Join(env.root, "repo2")
	if _, err := env.gg(ctx, repoPath2, "branch", "foo"); err != nil {
		t.Fatal(err)
	}
	git2 := env.git.WithDir(repoPath2)
	if r, err := gittool.ParseRev(ctx, git2, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != first.CommitHex() {
			t.Errorf("HEAD = %s; want %s", r.CommitHex(), first.CommitHex())
		}
		if r.RefName() != "refs/heads/foo" {
			t.Errorf("HEAD refname = %q; want refs/heads/foo", r.RefName())
		}
	}
	cfg, err := gittool.ReadConfig(ctx, git2)
	if err != nil {
		t.Fatal(err)
	}
	if remote := cfg.Value("branch.foo.remote"); remote != "origin" {
		t.Errorf("branch.foo.remote = %q; want \"origin\"", remote)
	}
	if mergeBranch := cfg.Value("branch.foo.merge"); mergeBranch != "refs/heads/master" {
		t.Errorf("branch.foo.remote = %q; want \"refs/heads/master\"", mergeBranch)
	}
}
