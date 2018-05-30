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

	"zombiezen.com/go/gg/internal/gitobj"
	"zombiezen.com/go/gg/internal/gittool"
)

func TestMerge(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Initial import")
	if err != nil {
		t.Fatal(err)
	}
	feature, err := dummyRev(ctx, env.git, env.root, "feature", "bar.txt", "Feature branch change")
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := dummyRev(ctx, env.git, env.root, "master", "baz.txt", "Upstream change")
	if err != nil {
		t.Fatal(err)
	}

	out, err := env.gg(ctx, env.root, "merge", "feature")
	if len(out) > 0 {
		t.Logf("merge output:\n%s", out)
	}
	if err != nil {
		t.Error("merge:", err)
	}
	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	names := map[gitobj.Hash]string{
		base:     "initial commit",
		upstream: "master commit",
		feature:  "branch commit",
	}
	if curr.Commit() != upstream {
		t.Errorf("after merge, HEAD = %s; want %s",
			prettyCommit(curr.Commit(), names),
			prettyCommit(upstream, names))
	}
	mergeHead, err := gittool.ParseRev(ctx, env.git, "MERGE_HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if mergeHead.Commit() != feature {
		t.Errorf("after merge, MERGE_HEAD = %s; want %s",
			prettyCommit(curr.Commit(), names),
			prettyCommit(feature, names))
	}
	if _, err := os.Stat(filepath.Join(env.root, "bar.txt")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(env.root, "baz.txt")); err != nil {
		t.Error(err)
	}
}

func TestMerge_Conflict(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	base, err := dummyRev(ctx, env.git, env.root, "master", "foo.txt", "Initial import")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "--quiet", "-b", "feature"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(env.root, "foo.txt"), []byte("feature content\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "Made a change myself"); err != nil {
		t.Fatal(err)
	}
	feature, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "checkout", "--quiet", "master"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(filepath.Join(env.root, "foo.txt"), []byte("boring text\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "Upstream change"); err != nil {
		t.Fatal(err)
	}
	upstream, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	out, err := env.gg(ctx, env.root, "merge", "feature")
	if len(out) > 0 {
		t.Logf("merge output:\n%s", out)
	}
	if err == nil {
		t.Error("merge did not return error")
	} else if isUsage(err) {
		t.Errorf("merge returned usage error: %v", err)
	}
	curr, err := gittool.ParseRev(ctx, env.git, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	names := map[gitobj.Hash]string{
		base:              "initial commit",
		upstream.Commit(): "master commit",
		feature.Commit():  "branch commit",
	}
	if curr.Commit() != upstream.Commit() {
		t.Errorf("after merge, HEAD = %s; want %s",
			prettyCommit(curr.Commit(), names),
			prettyCommit(upstream.Commit(), names))
	}
	mergeHead, err := gittool.ParseRev(ctx, env.git, "MERGE_HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if mergeHead.Commit() != feature.Commit() {
		t.Errorf("after merge, MERGE_HEAD = %s; want %s",
			prettyCommit(curr.Commit(), names),
			prettyCommit(feature.Commit(), names))
	}
}
