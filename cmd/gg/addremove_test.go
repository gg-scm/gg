// Copyright 2020 The gg Authors
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
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/filesystem"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestAddRemove(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tests := []struct {
		name         string
		beforeCommit []filesystem.Operation
		afterCommit  []filesystem.Operation
		dir          string
		args         []string
		status       []git.StatusEntry
	}{
		{
			name: "NoArgs",
			beforeCommit: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			afterCommit: []filesystem.Operation{
				filesystem.Remove("foo.txt"),
				filesystem.Write("bar.txt", dummyContent),
			},
			status: []git.StatusEntry{
				{Code: git.StatusCode{'D', ' '}, Name: "foo.txt"},
				{Code: git.StatusCode{' ', 'A'}, Name: "bar.txt"},
			},
		},
		{
			name: "RemovedFile",
			beforeCommit: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			afterCommit: []filesystem.Operation{
				filesystem.Remove("foo.txt"),
				filesystem.Write("bar.txt", dummyContent),
			},
			args: []string{"foo.txt"},
			status: []git.StatusEntry{
				{Code: git.StatusCode{'D', ' '}, Name: "foo.txt"},
				{Code: git.StatusCode{'?', '?'}, Name: "bar.txt"},
			},
		},
		{
			name: "NewFile",
			beforeCommit: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			afterCommit: []filesystem.Operation{
				filesystem.Remove("foo.txt"),
				filesystem.Write("bar.txt", dummyContent),
			},
			args: []string{"bar.txt"},
			status: []git.StatusEntry{
				{Code: git.StatusCode{' ', 'D'}, Name: "foo.txt"},
				{Code: git.StatusCode{' ', 'A'}, Name: "bar.txt"},
			},
		},
		{
			name: "AllFiles",
			beforeCommit: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			afterCommit: []filesystem.Operation{
				filesystem.Remove("foo.txt"),
				filesystem.Write("bar.txt", dummyContent),
			},
			args: []string{"foo.txt", "bar.txt"},
			status: []git.StatusEntry{
				{Code: git.StatusCode{'D', ' '}, Name: "foo.txt"},
				{Code: git.StatusCode{' ', 'A'}, Name: "bar.txt"},
			},
		},
		{
			name: "Dir",
			beforeCommit: []filesystem.Operation{
				filesystem.Write("foo/bar.txt", dummyContent),
			},
			afterCommit: []filesystem.Operation{
				filesystem.Remove("foo/bar.txt"),
				filesystem.Write("foo/baz.txt", dummyContent),
			},
			args: []string{"."},
			status: []git.StatusEntry{
				{Code: git.StatusCode{'D', ' '}, Name: "foo/bar.txt"},
				{Code: git.StatusCode{' ', 'A'}, Name: "foo/baz.txt"},
			},
		},
		{
			name: "NoArgs/FromSubdir",
			beforeCommit: []filesystem.Operation{
				filesystem.Write("foo.txt", dummyContent),
			},
			afterCommit: []filesystem.Operation{
				filesystem.Write("bar/baz.txt", dummyContent),
				filesystem.Write("quux.txt", dummyContent),
			},
			dir: "bar",
			status: []git.StatusEntry{
				{Code: git.StatusCode{' ', 'A'}, Name: "bar/baz.txt"},
				{Code: git.StatusCode{' ', 'A'}, Name: "quux.txt"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}

			if err := env.initEmptyRepo(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.root.Apply(test.beforeCommit...); err != nil {
				t.Fatal(err)
			}
			if err := env.addFiles(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if _, err := env.newCommit(ctx, "."); err != nil {
				t.Fatal(err)
			}
			err = env.root.Apply(test.afterCommit...)
			if err != nil {
				t.Fatal(err)
			}

			args := append([]string{"addremove"}, test.args...)
			if _, err := env.gg(ctx, env.root.FromSlash(test.dir), args...); err != nil {
				t.Fatal(err)
			}

			st, err := env.git.Status(ctx, git.StatusOptions{
				DisableRenames: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			diff := cmp.Diff(test.status, st,
				cmpopts.SortSlices(func(ent1, ent2 git.StatusEntry) bool {
					return ent1.Name < ent2.Name
				}),
				// Force String-ification in diff output.
				cmp.Comparer(func(s1, s2 git.StatusCode) bool {
					return s1 == s2
				}),
			)
			if diff != "" {
				t.Errorf("status (-want +got):\n%s", diff)
			}
		})
	}
}
