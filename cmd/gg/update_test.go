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

	"zombiezen.com/go/gg/internal/gitobj"
	"zombiezen.com/go/gg/internal/gittool"
)

func TestUpdate_NoArgsFastForward(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	h1, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Commit 1")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := dummyRev(ctx, env.git, env.root, "upstream", "bar.txt", "Commit 2")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "--quiet", "master"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "branch", "--quiet", "--set-upstream-to=upstream"); err != nil {
		t.Fatal(err)
	}

	_, err = env.gg(ctx, env.root, "update")
	if err != nil {
		t.Error(err)
	}
	if r, err := gittool.ParseRev(ctx, env.git, "HEAD"); err != nil {
		t.Fatal(err)
	} else if r.Commit() != h2 {
		names := map[gitobj.Hash]string{
			h1: "first commit",
			h2: "second commit",
		}
		t.Errorf("after update, HEAD = %s; want %s",
			prettyCommit(r.Commit(), names),
			prettyCommit(h2, names))
	}
}

func TestUpdate_SwitchBranch(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	h1, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Commit 1")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := dummyRev(ctx, env.git, env.root, "foo", "bar.txt", "Commit 2")
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.gg(ctx, env.root, "update", "master")
	if err != nil {
		t.Error(err)
	}
	if r, err := gittool.ParseRev(ctx, env.git, "HEAD"); err != nil {
		t.Fatal(err)
	} else {
		if r.Commit() != h1 {
			names := map[gitobj.Hash]string{
				h1: "first commit",
				h2: "second commit",
			}
			t.Errorf("after update master, HEAD = %s; want %s",
				prettyCommit(r.Commit(), names),
				prettyCommit(h1, names))
		}
		if got, want := r.Ref(), gitobj.Ref("refs/heads/master"); got != want {
			t.Errorf("after update master, HEAD ref = %s; want %s", got, want)
		}
	}
}

func TestUpdate_ToCommit(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	h1, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Commit 1")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := dummyRev(ctx, env.git, env.root, "master", "bar.txt", "Commit 2")
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.gg(ctx, env.root, "update", h1.String())
	if err != nil {
		t.Error(err)
	}
	if r, err := gittool.ParseRev(ctx, env.git, "HEAD"); err != nil {
		t.Fatal(err)
	} else {
		if r.Commit() != h1 {
			names := map[gitobj.Hash]string{
				h1: "first commit",
				h2: "second commit",
			}
			t.Errorf("after update master, HEAD = %s; want %s",
				prettyCommit(r.Commit(), names),
				prettyCommit(h1, names))
		}
		if got := r.Ref(); got != gitobj.Head {
			t.Errorf("after update master, HEAD ref = %s; want %s", got, gitobj.Head)
		}
	}
}
