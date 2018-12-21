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
	"github.com/google/go-cmp/cmp"
)

func TestListTree(t *testing.T) {
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

	// Create a repository with one commit with only foo.txt and another commit
	// with both foo.txt and bar.txt. Uses raw commands, as ListTree is used to
	// verify the state of other APIs.
	if err := env.g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "commit", "-m", "commit 1"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "add", "bar.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "commit", "-m", "commit 2"); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		rev       string
		pathspecs []Pathspec
		want      map[TopPath]struct{}
	}{
		{
			name: "SingleFile",
			rev:  "HEAD~",
			want: map[TopPath]struct{}{"foo.txt": {}},
		},
		{
			name: "MultipleFiles",
			rev:  "HEAD",
			want: map[TopPath]struct{}{"foo.txt": {}, "bar.txt": {}},
		},
		{
			name:      "MultipleFilesFiltered",
			rev:       "HEAD",
			pathspecs: []Pathspec{"foo.txt"},
			want:      map[TopPath]struct{}{"foo.txt": {}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := env.g.ListTree(ctx, test.rev, test.pathspecs)
			if err != nil {
				t.Fatal("ListTree error:", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("ListTree (-want +got)\n%s", diff)
			}
		})
	}
}
