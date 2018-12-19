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

func TestIsAncestor(t *testing.T) {
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
	if err := env.g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Commit(ctx, "commit 1"); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Run(ctx, "checkout", "--quiet", "-b", "a"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent+"a\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.g.CommitAll(ctx, "commit 2"); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Run(ctx, "checkout", "--quiet", "-b", "b", "master"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent+"b\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.g.CommitAll(ctx, "commit 3"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		rev1 string
		rev2 string
		want bool
	}{
		{"master", "a", true},
		{"master", "b", true},
		{"a", "b", false},
		{"b", "a", false},
		{"a", "master", false},
		{"b", "master", false},
	}
	for _, test := range tests {
		got, err := env.g.IsAncestor(ctx, test.rev1, test.rev2)
		if err != nil {
			t.Errorf("IsAncestor(ctx, %q, %q) error: %v", test.rev1, test.rev2, err)
			continue
		}
		if got != test.want {
			t.Errorf("IsAncestor(ctx, %q, %q) = %t, <nil>; want %t, <nil>", test.rev1, test.rev2, got, test.want)
		}
	}
}
