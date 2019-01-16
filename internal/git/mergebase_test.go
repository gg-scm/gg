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

package git

import (
	"context"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestMergeBase(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx, gitPath)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	// Create a repository with the following commits:
	//
	// master -- a
	//       \
	//        -- b
	// orphan
	if err := env.g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Commit(ctx, "commit 1", CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	master, err := env.g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.g.NewBranch(ctx, "a", BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent+"a\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.g.CommitAll(ctx, "commit 2", CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	a, err := env.g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.g.NewBranch(ctx, "b", BranchOptions{StartPoint: "master", Checkout: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent+"b\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.g.CommitAll(ctx, "commit 3", CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	b, err := env.g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.g.Run(ctx, "checkout", "--orphan", "orphan"); err != nil {
		t.Fatal(err)
	}
	if err := env.g.CommitAll(ctx, "disconnected commit", CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	orphan, err := env.g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := map[Hash]string{
		master.Commit: "master",
		a.Commit:      "a",
		b.Commit:      "b",
		orphan.Commit: "orphan",
	}

	tests := []struct {
		rev1       string
		rev2       string
		mergeBase  Hash
		isAncestor bool
	}{
		{"master", "master", master.Commit, true},
		{"master", "a", master.Commit, true},
		{"master", "b", master.Commit, true},
		{"a", "b", master.Commit, false},
		{"b", "a", master.Commit, false},
		{"a", "master", master.Commit, false},
		{"b", "master", master.Commit, false},
		{"master", "orphan", Hash{}, false},
		{"orphan", "master", Hash{}, false},
	}
	for _, test := range tests {
		got, err := env.g.MergeBase(ctx, test.rev1, test.rev2)
		if err != nil {
			if test.mergeBase != (Hash{}) {
				t.Errorf("MergeBase(ctx, %q, %q) error: %v", test.rev1, test.rev2, err)
			}
			continue
		}
		if test.mergeBase == (Hash{}) {
			t.Errorf("MergeBase(ctx, %q, %q) = %s, <nil>; want error", test.rev1, test.rev2, prettyCommit(got, names))
		}
		if got != test.mergeBase {
			t.Errorf("MergeBase(ctx, %q, %q) = %s, <nil>; want %s, <nil>", test.rev1, test.rev2, prettyCommit(got, names), prettyCommit(test.mergeBase, names))
		}
	}
	t.Run("IsAncestor", func(t *testing.T) {
		for _, test := range tests {
			got, err := env.g.IsAncestor(ctx, test.rev1, test.rev2)
			if err != nil {
				t.Errorf("IsAncestor(ctx, %q, %q) error: %v", test.rev1, test.rev2, err)
				continue
			}
			if got != test.isAncestor {
				t.Errorf("IsAncestor(ctx, %q, %q) = %t, <nil>; want %t, <nil>", test.rev1, test.rev2, got, test.isAncestor)
			}
		}
	})
}
