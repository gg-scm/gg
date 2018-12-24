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
	"os"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestCheckoutBranch(t *testing.T) {
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

	// Create a template repository with two branches: master and foo.
	if err := env.g.Init(ctx, "template"); err != nil {
		t.Fatal(err)
	}
	const masterContent = "content A\n"
	if err := env.root.Apply(filesystem.Write("template/file.txt", masterContent)); err != nil {
		t.Fatal(err)
	}
	templateGit := env.g.WithDir(env.root.FromSlash("template"))
	if err := templateGit.Add(ctx, []Pathspec{"file.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := templateGit.Commit(ctx, dummyContent); err != nil {
		t.Fatal(err)
	}
	master, err := templateGit.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := templateGit.NewBranch(ctx, "foo", BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	const fooContent = masterContent + "content B\n"
	if err := env.root.Apply(filesystem.Write("template/file.txt", fooContent)); err != nil {
		t.Fatal(err)
	}
	if err := templateGit.CommitAll(ctx, dummyContent); err != nil {
		t.Fatal(err)
	}
	foo, err := templateGit.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Use raw command to avoid depending on system-under-test.
	if _, err := templateGit.Run(ctx, "checkout", "--quiet", "master"); err != nil {
		t.Fatal(err)
	}

	// Run checkout tests on a cloned repository.
	tests := []struct {
		name         string
		branch       string
		opts         CheckoutOptions
		localContent string

		wantErr     bool
		wantHead    Rev
		wantContent string
	}{
		{
			name:         "SameBranch",
			branch:       "master",
			localContent: masterContent,
			wantHead:     *master,
			wantContent:  masterContent,
		},
		{
			name:         "DifferentBranch",
			branch:       "foo",
			localContent: masterContent,
			wantHead:     *foo,
			wantContent:  fooContent,
		},
		{
			name:         "DoesNotExist",
			branch:       "bar",
			localContent: masterContent,
			wantErr:      true,
			wantHead:     *master,
			wantContent:  masterContent,
		},
		{
			name:         "CommitHash",
			branch:       foo.Commit.String(),
			localContent: masterContent,
			wantErr:      true,
			wantHead:     *master,
			wantContent:  masterContent,
		},
		{
			name:         "Ref",
			branch:       "refs/heads/foo",
			localContent: masterContent,
			wantErr:      true,
			wantHead:     *master,
			wantContent:  masterContent,
		},
		{
			name:         "LocalModifications",
			branch:       "foo",
			localContent: "content C\n" + masterContent,
			wantErr:      true,
			wantHead:     *master,
			wantContent:  "content C\n" + masterContent,
		},
		{
			name:         "Merge",
			branch:       "foo",
			opts:         CheckoutOptions{Merge: true},
			localContent: "content C\n" + masterContent,
			wantHead:     *foo,
			wantContent:  "content C\n" + fooContent,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clone from template to local working copy.
			if _, err := env.g.Run(ctx, "clone", "template", "wc"); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.RemoveAll(env.root.FromSlash("wc")); err != nil {
					t.Error("cleanup:", err)
				}
			}()
			// Reconstruct local branches, since clone will only make the branch for HEAD.
			g := env.g.WithDir(env.root.FromSlash("wc"))
			if err := g.NewBranch(ctx, "foo", BranchOptions{Track: true, StartPoint: "origin/foo"}); err != nil {
				t.Fatal(err)
			}
			// Make local changes to file.
			if err := env.root.Apply(filesystem.Write("wc/file.txt", test.localContent)); err != nil {
				t.Fatal(err)
			}

			// Call CheckoutBranch.
			if err := g.CheckoutBranch(ctx, test.branch, test.opts); err != nil && !test.wantErr {
				t.Error("CheckoutBranch:", err)
			} else if err == nil && test.wantErr {
				t.Error("CheckoutBranch did not return error")
			}

			// Verify that HEAD matches expectations.
			if rev, err := g.Head(ctx); err != nil {
				t.Error(err)
			} else if rev.Commit != test.wantHead.Commit || rev.Ref != test.wantHead.Ref {
				t.Errorf("HEAD = %v (ref = %v); want %v (ref = %v)", rev.Commit, rev.Ref, test.wantHead.Commit, test.wantHead.Ref)
			}

			// Verify that local content matches expectations.
			if got, err := env.root.ReadFile("wc/file.txt"); err != nil {
				t.Error(err)
			} else if got != test.wantContent {
				t.Errorf("wc/file.txt content = %q; want %q", got, test.wantContent)
			}
		})
	}
}
