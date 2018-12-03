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
	"testing"
)

func TestBranch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initRepoWithHistory(ctx, "."); err != nil {
		t.Fatal(err)
	}
	first, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "branch", "foo", "bar"); err != nil {
		t.Fatal(err)
	}
	if r, err := env.git.Head(ctx); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != first.Commit() {
			t.Errorf("HEAD = %s; want %s", r.Commit(), first.Commit())
		}
		if r.Ref() != "refs/heads/foo" {
			t.Errorf("HEAD refname = %q; want refs/heads/foo", r.Ref())
		}
	}
	if r, err := env.git.ParseRev(ctx, "bar"); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != first.Commit() {
			t.Errorf("bar = %s; want %s", r.Commit(), first.Commit())
		}
		if r.Ref() != "refs/heads/bar" {
			t.Errorf("bar refname = %q; want refs/heads/bar", r.Ref())
		}
	}
}

func TestBranch_Upstream(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.initRepoWithHistory(ctx, "repo1"); err != nil {
		t.Fatal(err)
	}
	git1 := env.git.WithDir(env.root.FromSlash("repo1"))
	first, err := git1.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "clone", "repo1", "repo2"); err != nil {
		t.Fatal(err)
	}

	repoPath2 := env.root.FromSlash("repo2")
	if _, err := env.gg(ctx, repoPath2, "branch", "foo"); err != nil {
		t.Fatal(err)
	}
	git2 := env.git.WithDir(repoPath2)
	if r, err := git2.Head(ctx); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != first.Commit() {
			t.Errorf("HEAD = %s; want %s", r.Commit(), first.Commit())
		}
		if r.Ref() != "refs/heads/foo" {
			t.Errorf("HEAD refname = %q; want refs/heads/foo", r.Ref())
		}
	}
	cfg, err := git2.ReadConfig(ctx)
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
