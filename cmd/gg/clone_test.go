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

	"gg-scm.io/pkg/internal/filesystem"
)

const cloneFileMsg = "Hello, World!\n"

func TestClone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.initEmptyRepo(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	const fileContent = "wut up\n"
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", fileContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	head, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}
	gitA := env.git.WithDir(env.root.FromSlash("repoA"))
	if err := gitA.Run(ctx, "branch", "foo"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "clone", "repoA", "repoB"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(env.root.FromSlash("repoB"))
	if r, err := gitB.Head(ctx); err != nil {
		t.Error(err)
	} else {
		if r.Commit != head {
			t.Errorf("HEAD = %s; want %s", r.Commit, head)
		}
		if r.Ref != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.Ref)
		}
	}
	if r, err := gitB.ParseRev(ctx, "refs/heads/foo"); err != nil {
		t.Error(err)
	} else if r.Commit != head {
		t.Errorf("refs/heads/foo = %s; want %s", r.Commit, head)
	}
	if r, err := gitB.ParseRev(ctx, "refs/remotes/origin/master"); err != nil {
		t.Error(err)
	} else if r.Commit != head {
		t.Errorf("refs/remotes/origin/master = %s; want %s", r.Commit, head)
	}
	if r, err := gitB.ParseRev(ctx, "refs/remotes/origin/foo"); err != nil {
		t.Error(err)
	} else if r.Commit != head {
		t.Errorf("refs/remotes/origin/foo = %s; want %s", r.Commit, head)
	}
	if got, err := env.root.ReadFile("repoB/foo.txt"); err != nil {
		t.Error(err)
	} else if got != fileContent {
		t.Errorf("repoB/foo.txt content = %q; want %q", got, fileContent)
	}
}

func TestClone_Branch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.initEmptyRepo(ctx, "repoA"); err != nil {
		t.Fatal(err)
	}
	const fileContent = "wut up\n"
	if err := env.root.Apply(filesystem.Write("repoA/foo.txt", fileContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "repoA/foo.txt"); err != nil {
		t.Fatal(err)
	}
	head, err := env.newCommit(ctx, "repoA")
	if err != nil {
		t.Fatal(err)
	}
	gitA := env.git.WithDir(env.root.FromSlash("repoA"))
	if err := gitA.Run(ctx, "branch", "foo"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "clone", "-b=foo", "repoA", "repoB"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(env.root.FromSlash("repoB"))
	if r, err := gitB.Head(ctx); err != nil {
		t.Error(err)
	} else {
		if r.Commit != head {
			t.Errorf("HEAD = %s; want %s", r.Commit, head)
		}
		if r.Ref != "refs/heads/foo" {
			t.Errorf("HEAD refname = %q; want refs/heads/foo", r.Ref)
		}
	}
	if r, err := gitB.ParseRev(ctx, "refs/heads/master"); err != nil {
		t.Error(err)
	} else if r.Commit != head {
		t.Errorf("refs/heads/master = %s; want %s", r.Commit, head)
	}
	if r, err := gitB.ParseRev(ctx, "refs/remotes/origin/master"); err != nil {
		t.Error(err)
	} else if r.Commit != head {
		t.Errorf("refs/remotes/origin/master = %s; want %s", r.Commit, head)
	}
	if r, err := gitB.ParseRev(ctx, "refs/remotes/origin/foo"); err != nil {
		t.Error(err)
	} else if r.Commit != head {
		t.Errorf("refs/remotes/origin/foo = %s; want %s", r.Commit, head)
	}
	if got, err := env.root.ReadFile("repoB/foo.txt"); err != nil {
		t.Error(err)
	} else if got != fileContent {
		t.Errorf("repoB/foo.txt content = %q; want %q", got, fileContent)
	}
}

func TestDefaultCloneDest(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{
			url:  "git@github.com:google/go-cloud.git",
			want: "go-cloud",
		},
		{
			url:  "https://github.com/google/go-cloud.git",
			want: "go-cloud",
		},
		{
			url:  "https://github.com/google/go-cloud",
			want: "go-cloud",
		},
		{
			url:  "foo",
			want: "foo",
		},
		{
			url:  "bar",
			want: "bar",
		},
		{
			url:  "foo/bar/.git",
			want: "bar",
		},
		{
			url:  "foo/bar.git",
			want: "bar",
		},
		{
			url:  `C:\Users\Ross\Documents\foo.git`,
			want: "foo",
		},
	}
	for _, test := range tests {
		got := defaultCloneDest(test.url)
		if got != test.want {
			t.Errorf("defaultCloneDest(%q) = %q; want %q", test.url, got, test.want)
		}
	}
}
