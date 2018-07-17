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

	"gg-scm.io/pkg/internal/gitobj"
	"gg-scm.io/pkg/internal/gittool"
)

func TestPull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	pullEnv, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, pullEnv.repoB, "pull"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pullEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != pullEnv.commit1 {
			names := pullEnv.commitNames()
			t.Errorf("HEAD = %s; want %s",
				prettyCommit(r.Commit(), names),
				prettyCommit(pullEnv.commit1, names))
		}
		if r.Ref() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.Ref())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "origin/master"); err != nil {
		t.Error(err)
	} else if r.Commit() != pullEnv.commit2 {
		names := pullEnv.commitNames()
		t.Errorf("origin/master = %s; want %s",
			prettyCommit(r.Commit(), names),
			prettyCommit(pullEnv.commit2, names))
	}
	if r, err := gittool.ParseRev(ctx, gitB, "first"); err != nil {
		t.Error(err)
	} else if r.Commit() != pullEnv.commit1 {
		names := pullEnv.commitNames()
		t.Errorf("origin/master = %s; want %s",
			prettyCommit(r.Commit(), names),
			prettyCommit(pullEnv.commit1, names))
	}
}

func TestPullWithArgument(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	pullEnv, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	// Move HEAD away.  We want to check that the corresponding branch is pulled.
	if err := env.git.WithDir(pullEnv.repoA).Run(ctx, "checkout", "--detach", "HEAD^"); err != nil {
		t.Fatal(err)
	}
	repoC := filepath.Join(env.root, "repoC")
	if err := os.Rename(pullEnv.repoA, repoC); err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, pullEnv.repoB, "pull", repoC); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pullEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != pullEnv.commit1 {
			names := pullEnv.commitNames()
			t.Errorf("HEAD = %s; want %s",
				prettyCommit(r.Commit(), names),
				prettyCommit(pullEnv.commit1, names))
		}
		if r.Ref() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.Ref())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "FETCH_HEAD"); err != nil {
		t.Error(err)
	} else if r.Commit() != pullEnv.commit2 {
		names := pullEnv.commitNames()
		t.Errorf("FETCH_HEAD = %s; want %s",
			prettyCommit(r.Commit(), names),
			prettyCommit(pullEnv.commit2, names))
	}
	if r, err := gittool.ParseRev(ctx, gitB, "first"); err != nil {
		t.Error(err)
	} else if r.Commit() != pullEnv.commit1 {
		names := pullEnv.commitNames()
		t.Errorf("origin/master = %s; want %s",
			prettyCommit(r.Commit(), names),
			prettyCommit(pullEnv.commit1, names))
	}
}

func TestPullUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	pullEnv, err := setupPullTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.gg(ctx, pullEnv.repoB, "pull", "-u"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pullEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.Commit() != pullEnv.commit2 {
			names := pullEnv.commitNames()
			t.Errorf("HEAD = %s; want %s",
				prettyCommit(r.Commit(), names),
				prettyCommit(pullEnv.commit1, names))
		}
		if r.Ref() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.Ref())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "origin/master"); err != nil {
		t.Error(err)
	} else if r.Commit() != pullEnv.commit2 {
		names := pullEnv.commitNames()
		t.Errorf("origin/master = %s; want %s",
			prettyCommit(r.Commit(), names),
			prettyCommit(pullEnv.commit2, names))
	}
}

func TestInferUpstream(t *testing.T) {
	t.Parallel()
	tests := []struct {
		localBranch string
		merge       gitobj.Ref
		want        gitobj.Ref
	}{
		{localBranch: "", want: "HEAD"},
		{localBranch: "master", want: "refs/heads/master"},
		{localBranch: "master", merge: "refs/heads/master", want: "refs/heads/master"},
		{localBranch: "foo", want: "refs/heads/foo"},
		{localBranch: "foo", merge: "refs/heads/bar", want: "refs/heads/bar"},
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		if test.merge != "" {
			if err := env.git.Run(ctx, "config", "--local", "branch."+test.localBranch+".merge", test.merge.String()); err != nil {
				t.Errorf("for localBranch = %q, merge = %q: %v", test.localBranch, test.merge, err)
				continue
			}
		}
		cfg, err := gittool.ReadConfig(ctx, env.git)
		if test.merge != "" {
			// Cleanup
			if err := env.git.Run(ctx, "config", "--local", "--unset", "branch."+test.localBranch+".merge"); err != nil {
				t.Errorf("cleaning up localBranch = %q, merge = %q: %v", test.localBranch, test.merge, err)
			}
		}
		if err != nil {
			t.Errorf("for localBranch = %q, merge = %q: %v", test.localBranch, test.merge, err)
			continue
		}
		got := inferUpstream(cfg, test.localBranch)
		if got != test.want {
			t.Errorf("inferUpstream(ctx, env.git, %q) (with branch.%s.merge = %q) = %q; want %q", test.localBranch, test.localBranch, test.merge, got, test.want)
		}
	}
}

type pullEnv struct {
	repoA, repoB     string
	commit1, commit2 gitobj.Hash
}

func setupPullTest(ctx context.Context, env *testEnv) (*pullEnv, error) {
	repoA := filepath.Join(env.root, "repoA")
	if err := env.git.Run(ctx, "init", repoA); err != nil {
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
	repoB := filepath.Join(env.root, "repoB")
	if err := env.git.Run(ctx, "clone", repoA, repoB); err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(
		filepath.Join(repoA, fileName),
		[]byte("Hello, World!\nI learned some things...\n"),
		0666)
	if err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "tag", "first"); err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "commit", "-a", "-m", "second commit"); err != nil {
		return nil, err
	}
	commit2, err := gittool.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		return nil, err
	}
	return &pullEnv{
		repoA:   repoA,
		repoB:   repoB,
		commit1: commit1.Commit(),
		commit2: commit2.Commit(),
	}, nil
}

func (env *pullEnv) commitNames() map[gitobj.Hash]string {
	return map[gitobj.Hash]string{
		env.commit1: "shared commit",
		env.commit2: "remote commit",
	}
}
