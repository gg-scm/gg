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
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestAdd(t *testing.T) {
	ctx := context.Background()
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}

	tests := []struct {
		name      string
		fsOps     []filesystem.Operation
		pathspecs []Pathspec
		opts      AddOptions
		want      []StatusEntry
		allowed   [][]StatusEntry
		wantErr   bool
	}{
		{
			name: "NoPathspecs",
			fsOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{},
			want: []StatusEntry{
				{
					Code: StatusCode{'?', '?'},
					Name: "foo.txt",
				},
			},
		},
		{
			name: "WrongFile",
			fsOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"bar.txt"},
			wantErr:   true,
			want: []StatusEntry{
				{
					Code: StatusCode{'?', '?'},
					Name: "foo.txt",
				},
			},
		},
		{
			name: "Untracked/DefaultOptions",
			fsOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo.txt"},
			want: []StatusEntry{
				{
					Code: StatusCode{'A', ' '},
					Name: "foo.txt",
				},
			},
		},
		{
			name: "Untracked/IncludeIgnoredOnUntracked",
			fsOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo.txt"},
			opts: AddOptions{
				IncludeIgnored: true,
			},
			want: []StatusEntry{
				{
					Code: StatusCode{'A', ' '},
					Name: "foo.txt",
				},
			},
		},
		{
			name: "Untracked/IntentToAdd",
			fsOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo.txt"},
			opts: AddOptions{
				IntentToAdd: true,
			},
			want: []StatusEntry{
				{
					Code: StatusCode{' ', 'A'},
					Name: "foo.txt",
				},
			},
			// Git 2.11 and before.
			allowed: [][]StatusEntry{{
				{
					Code: StatusCode{'A', 'M'},
					Name: "foo.txt",
				},
			}},
		},
		{
			name: "Untracked/Dot",
			fsOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"."},
			want: []StatusEntry{
				{
					Code: StatusCode{'A', ' '},
					Name: "foo.txt",
				},
			},
		},
		{
			name: "Ignored/DefaultOptions",
			fsOps: []filesystem.Operation{
				filesystem.Write(".gitignore", "foo.txt\n"),
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo.txt"},
			want: []StatusEntry{
				{
					Code: StatusCode{'?', '?'},
					Name: ".gitignore",
				},
				{
					Code: StatusCode{'!', '!'},
					Name: "foo.txt",
				},
			},
			wantErr: true, // Git exits with an error saying that the paths are ignored.
		},
		{
			name: "Ignored/DefaultOptionsWithUntrackedFile",
			fsOps: []filesystem.Operation{
				filesystem.Write(".gitignore", "foo.txt\n"),
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo.txt", ".gitignore"},
			want: []StatusEntry{
				{
					Code: StatusCode{'A', ' '},
					Name: ".gitignore",
				},
				{
					Code: StatusCode{'!', '!'},
					Name: "foo.txt",
				},
			},
			wantErr: true, // Git exits with an error saying that the paths are ignored.
		},
		{
			name: "Ignored/IncludeIgnored",
			fsOps: []filesystem.Operation{
				filesystem.Write(".gitignore", "foo.txt\n"),
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo.txt"},
			opts: AddOptions{
				IncludeIgnored: true,
			},
			want: []StatusEntry{
				{
					Code: StatusCode{'?', '?'},
					Name: ".gitignore",
				},
				{
					Code: StatusCode{'A', ' '},
					Name: "foo.txt",
				},
			},
		},
		{
			name: "Ignored/DotWithDefaults",
			fsOps: []filesystem.Operation{
				filesystem.Write(".gitignore", "foo.txt\n"),
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"."},
			want: []StatusEntry{
				{
					Code: StatusCode{'A', ' '},
					Name: ".gitignore",
				},
				{
					Code: StatusCode{'!', '!'},
					Name: "foo.txt",
				},
			},
		},
		{
			name: "Ignored/DotIncludeIgnored",
			fsOps: []filesystem.Operation{
				filesystem.Write(".gitignore", "foo.txt\n"),
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"."},
			opts: AddOptions{
				IncludeIgnored: true,
			},
			want: []StatusEntry{
				{
					Code: StatusCode{'A', ' '},
					Name: ".gitignore",
				},
				{
					Code: StatusCode{'A', ' '},
					Name: "foo.txt",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(ctx, gitPath)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()

			// Create an empty repository and then apply test's filesystem operations.
			if err := env.g.Init(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.root.Apply(test.fsOps...); err != nil {
				t.Fatal(err)
			}

			// Call Add with the test arguments.
			if err := env.g.Add(ctx, test.pathspecs, test.opts); err != nil && !test.wantErr {
				t.Error("unexpected Add error:", err)
			} else if err == nil && test.wantErr {
				t.Error("Add did not return error; want error")
			}

			// Compare status.
			got, err := env.g.Status(ctx, StatusOptions{
				IncludeIgnored: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			opts := []cmp.Option{
				cmp.Transformer("String", StatusCode.String),
				cmpopts.SortSlices(func(a, b StatusEntry) bool {
					return a.Name < b.Name
				}),
			}
			diff := cmp.Diff(test.want, got, opts...)
			if diff != "" {
				foundAlt := false
				for _, alt := range test.allowed {
					if cmp.Equal(alt, got, opts...) {
						foundAlt = true
						break
					}
				}
				if !foundAlt {
					t.Errorf("status (-want +got):\n%s", diff)
				}
			}
		})
	}
}
