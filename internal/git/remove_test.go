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

func TestRemove(t *testing.T) {
	ctx := context.Background()
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	tests := []struct {
		name      string
		commitOps []filesystem.Operation
		localOps  []filesystem.Operation
		pathspecs []Pathspec
		opts      RemoveOptions
		want      []StatusEntry
		wantErr   bool
		wantWC    map[string]bool
	}{
		{
			name: "NoPathspecs",
			commitOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{},
			want:      []StatusEntry{},
			wantWC:    map[string]bool{"foo.txt": true},
		},
		{
			name: "UnmodifiedFile",
			commitOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo.txt"},
			want: []StatusEntry{
				{
					Code: StatusCode{'D', ' '},
					Name: "foo.txt",
				},
			},
			wantWC: map[string]bool{"foo.txt": false},
		},
		{
			name: "ModifiedFile",
			commitOps: []filesystem.Operation{
				filesystem.Write("foo.txt", "Hello\n"),
			},
			localOps: []filesystem.Operation{
				filesystem.Write("foo.txt", "And now for something completely different\n"),
			},
			pathspecs: []Pathspec{"foo.txt"},
			wantErr:   true,
			want: []StatusEntry{
				{
					Code: StatusCode{' ', 'M'},
					Name: "foo.txt",
				},
			},
			wantWC: map[string]bool{"foo.txt": true},
		},
		{
			name: "ModifiedFileWithAllowed",
			commitOps: []filesystem.Operation{
				filesystem.Write("foo.txt", "Hello\n"),
			},
			localOps: []filesystem.Operation{
				filesystem.Write("foo.txt", "And now for something completely different\n"),
			},
			pathspecs: []Pathspec{"foo.txt"},
			opts: RemoveOptions{
				Modified: true,
			},
			want: []StatusEntry{
				{
					Code: StatusCode{'D', ' '},
					Name: "foo.txt",
				},
			},
			wantWC: map[string]bool{"foo.txt": false},
		},
		{
			name: "WrongFile",
			commitOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"bar.txt"},
			wantErr:   true,
			wantWC:    map[string]bool{"foo.txt": true},
		},
		{
			name: "NonRecursiveDirectory",
			commitOps: []filesystem.Operation{
				filesystem.Write("foo/bar.txt", dummyContent),
				filesystem.Write("foo/baz.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo"},
			wantErr:   true,
			wantWC: map[string]bool{
				"foo/bar.txt": true,
				"foo/baz.txt": true,
			},
		},
		{
			name: "RecursiveDirectory",
			commitOps: []filesystem.Operation{
				filesystem.Write("foo/bar.txt", dummyContent),
				filesystem.Write("foo/baz.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo"},
			opts: RemoveOptions{
				Recursive: true,
			},
			want: []StatusEntry{
				{
					Code: StatusCode{'D', ' '},
					Name: "foo/bar.txt",
				},
				{
					Code: StatusCode{'D', ' '},
					Name: "foo/baz.txt",
				},
			},
			wantWC: map[string]bool{
				"foo/bar.txt": false,
				"foo/baz.txt": false,
			},
		},
		{
			name: "KeepWorkingCopy",
			commitOps: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			pathspecs: []Pathspec{"foo.txt"},
			opts: RemoveOptions{
				KeepWorkingCopy: true,
			},
			want: []StatusEntry{
				{
					Code: StatusCode{'D', ' '},
					Name: "foo.txt",
				},
				{
					Code: StatusCode{'?', '?'},
					Name: "foo.txt",
				},
			},
			wantWC: map[string]bool{"foo.txt": true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(ctx, gitPath)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()

			// Create an empty repository, apply test's filesystem operations, then commit.
			if err := env.g.Init(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.root.Apply(test.commitOps...); err != nil {
				t.Fatal(err)
			}
			if err := env.g.Add(ctx, []Pathspec{"."}, AddOptions{}); err != nil {
				t.Fatal(err)
			}
			if err := env.g.Commit(ctx, "initial import", CommitOptions{}); err != nil {
				t.Fatal(err)
			}

			// Make any local modifications for the test.
			if err := env.root.Apply(test.localOps...); err != nil {
				t.Fatal(err)
			}

			// Call Remove with the test arguments.
			if err := env.g.Remove(ctx, test.pathspecs, test.opts); err != nil && !test.wantErr {
				t.Error("unexpected Remove error:", err)
			} else if err == nil && test.wantErr {
				t.Error("Remove did not return error; want error")
			}

			// Compare status.
			got, err := env.g.Status(ctx, StatusOptions{})
			if err != nil {
				t.Fatal(err)
			}
			opts := []cmp.Option{
				cmp.Transformer("String", StatusCode.String),
				cmpopts.SortSlices(func(a, b StatusEntry) bool {
					if a.Name == b.Name {
						if a.Code[0] == b.Code[0] {
							return a.Code[1] < b.Code[1]
						}
						return a.Code[0] < b.Code[0]
					}
					return a.Name < b.Name
				}),
				cmpopts.EquateEmpty(),
			}
			diff := cmp.Diff(test.want, got, opts...)
			if diff != "" {
				t.Errorf("status (-want +got):\n%s", diff)
			}

			// Compare working copy.
			for path, want := range test.wantWC {
				got, err := env.root.Exists(path)
				if err != nil {
					t.Error(err)
					continue
				}
				if !got && want {
					t.Errorf("%s does not exist; want to exist", path)
				}
				if got && !want {
					t.Errorf("%s exists; want not to exist", path)
				}
			}
		})
	}
}
