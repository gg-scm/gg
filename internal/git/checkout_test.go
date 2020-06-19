// Copyright 2018 The gg Authors
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

	// Create a template repository with two branches: main and foo.
	if err := env.g.Init(ctx, "template"); err != nil {
		t.Fatal(err)
	}
	const mainContent = "content A\n"
	if err := env.root.Apply(filesystem.Write("template/file.txt", mainContent)); err != nil {
		t.Fatal(err)
	}
	templateGit := env.g.WithDir(env.root.FromSlash("template"))
	if err := templateGit.Add(ctx, []Pathspec{"file.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := templateGit.Commit(ctx, dummyContent, CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	main, err := templateGit.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := templateGit.NewBranch(ctx, "foo", BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	const fooContent = mainContent + "content B\n"
	if err := env.root.Apply(filesystem.Write("template/file.txt", fooContent)); err != nil {
		t.Fatal(err)
	}
	if err := templateGit.CommitAll(ctx, dummyContent, CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	foo, err := templateGit.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Use raw command to avoid depending on system-under-test.
	if err := templateGit.Run(ctx, "checkout", "--quiet", "main"); err != nil {
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
			branch:       "main",
			localContent: mainContent,
			wantHead:     *main,
			wantContent:  mainContent,
		},
		{
			name:         "DifferentBranch",
			branch:       "foo",
			localContent: mainContent,
			wantHead:     *foo,
			wantContent:  fooContent,
		},
		{
			name:         "DoesNotExist",
			branch:       "bar",
			localContent: mainContent,
			wantErr:      true,
			wantHead:     *main,
			wantContent:  mainContent,
		},
		{
			name:         "CommitHash",
			branch:       foo.Commit.String(),
			localContent: mainContent,
			wantErr:      true,
			wantHead:     *main,
			wantContent:  mainContent,
		},
		{
			name:         "Ref",
			branch:       "refs/heads/foo",
			localContent: mainContent,
			wantErr:      true,
			wantHead:     *main,
			wantContent:  mainContent,
		},
		{
			name:         "LocalModifications",
			branch:       "foo",
			localContent: "content C\n" + mainContent,
			wantErr:      true,
			wantHead:     *main,
			wantContent:  "content C\n" + mainContent,
		},
		{
			name:         "Merge",
			branch:       "foo",
			opts:         CheckoutOptions{ConflictBehavior: MergeLocal},
			localContent: "content C\n" + mainContent,
			wantHead:     *foo,
			wantContent:  "content C\n" + fooContent,
		},
		{
			name:         "Force",
			branch:       "foo",
			opts:         CheckoutOptions{ConflictBehavior: DiscardLocal},
			localContent: "content C\n",
			wantHead:     *foo,
			wantContent:  fooContent,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clone from template to local working copy.
			if err := env.g.Run(ctx, "clone", "template", "wc"); err != nil {
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

func TestCheckoutRev(t *testing.T) {
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

	// Create a template repository with two branches: main and foo.
	if err := env.g.Init(ctx, "template"); err != nil {
		t.Fatal(err)
	}
	const mainContent = "content A\n"
	if err := env.root.Apply(filesystem.Write("template/file.txt", mainContent)); err != nil {
		t.Fatal(err)
	}
	templateGit := env.g.WithDir(env.root.FromSlash("template"))
	if err := templateGit.Add(ctx, []Pathspec{"file.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := templateGit.Commit(ctx, dummyContent, CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	main, err := templateGit.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := templateGit.NewBranch(ctx, "foo", BranchOptions{Checkout: true}); err != nil {
		t.Fatal(err)
	}
	const fooContent = mainContent + "content B\n"
	if err := env.root.Apply(filesystem.Write("template/file.txt", fooContent)); err != nil {
		t.Fatal(err)
	}
	if err := templateGit.CommitAll(ctx, dummyContent, CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	foo, err := templateGit.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := templateGit.CheckoutBranch(ctx, "main", CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}

	// Run checkout tests on a cloned repository.
	tests := []struct {
		name         string
		rev          string
		opts         CheckoutOptions
		localContent string

		wantErr     bool
		wantHead    Rev
		wantContent string
	}{
		{
			name:         "SameBranch",
			rev:          "main",
			localContent: mainContent,
			wantHead:     Rev{Commit: main.Commit, Ref: "HEAD"},
			wantContent:  mainContent,
		},
		{
			name:         "DifferentBranch",
			rev:          "foo",
			localContent: mainContent,
			wantHead:     Rev{Commit: foo.Commit, Ref: "HEAD"},
			wantContent:  fooContent,
		},
		{
			name:         "DoesNotExist",
			rev:          "bar",
			localContent: mainContent,
			wantErr:      true,
			wantHead:     *main,
			wantContent:  mainContent,
		},
		{
			name:         "CommitHash",
			rev:          foo.Commit.String(),
			localContent: mainContent,
			wantHead:     Rev{Commit: foo.Commit, Ref: "HEAD"},
			wantContent:  fooContent,
		},
		{
			name:         "Ref",
			rev:          "refs/heads/foo",
			localContent: mainContent,
			wantHead:     Rev{Commit: foo.Commit, Ref: "HEAD"},
			wantContent:  fooContent,
		},
		{
			name:         "LocalModifications",
			rev:          "foo",
			localContent: "content C\n" + mainContent,
			wantErr:      true,
			wantHead:     *main,
			wantContent:  "content C\n" + mainContent,
		},
		{
			name:         "Merge",
			rev:          "foo",
			opts:         CheckoutOptions{ConflictBehavior: MergeLocal},
			localContent: "content C\n" + mainContent,
			wantHead:     Rev{Commit: foo.Commit, Ref: "HEAD"},
			wantContent:  "content C\n" + fooContent,
		},
		{
			name:         "Force",
			rev:          "foo",
			opts:         CheckoutOptions{ConflictBehavior: DiscardLocal},
			localContent: "content C\n",
			wantHead:     Rev{Commit: foo.Commit, Ref: "HEAD"},
			wantContent:  fooContent,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clone from template to local working copy.
			if err := env.g.Run(ctx, "clone", "template", "wc"); err != nil {
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

			// Call CheckoutRev.
			if err := g.CheckoutRev(ctx, test.rev, test.opts); err != nil && !test.wantErr {
				t.Error("CheckoutRev:", err)
			} else if err == nil && test.wantErr {
				t.Error("CheckoutRev did not return error")
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
