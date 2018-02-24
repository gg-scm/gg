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

	"zombiezen.com/go/gut/internal/gittool"
)

func TestPush(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoA := filepath.Join(env.root, "repoA")
	if err := env.git.Run(ctx, "init", repoA); err != nil {
		t.Fatal(err)
	}
	repoB := filepath.Join(env.root, "repoB")
	if err := env.git.Run(ctx, "init", "--bare", repoB); err != nil {
		t.Fatal(err)
	}
	gitA := env.git.WithDir(repoA)
	const fileName = "foo.txt"
	err = ioutil.WriteFile(
		filepath.Join(repoA, fileName),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "add", fileName); err != nil {
		t.Fatal(err)
	}
	if err := gitA.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		t.Fatal(err)
	}
	first, err := gittool.ParseRev(ctx, gitA, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	if err := env.gut(ctx, repoA, "push", repoB); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "HEAD"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != first.CommitHex() {
			t.Errorf("HEAD = %s; want %s", r.CommitHex(), first.CommitHex())
		}
		if r.RefName() != "refs/heads/master" {
			t.Errorf("HEAD refname = %q; want refs/heads/master", r.RefName())
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() != first.CommitHex() {
			t.Errorf("master = %s; want %s", r.CommitHex(), first.CommitHex())
		}
		if r.RefName() != "refs/heads/master" {
			t.Errorf("master refname = %q; want refs/heads/master", r.RefName())
		}
	}
}
