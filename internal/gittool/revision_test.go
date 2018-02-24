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

package gittool

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestParseRev(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
	if gitPathError != nil {
		t.Skip("git not found:", gitPathError)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := env.git.Run(ctx, "init", repoPath); err != nil {
		t.Fatal(err)
	}
	git := env.git.WithDir(repoPath)

	// First commit
	const fileName = "foo.txt"
	filePath := filepath.Join(repoPath, fileName)
	err = ioutil.WriteFile(filePath, []byte("Hello, World!\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.Run(ctx, "add", fileName); err != nil {
		t.Fatal(err)
	}
	if err := git.Run(ctx, "commit", "-m", "first commit"); err != nil {
		t.Fatal(err)
	}
	commit1, err := git.RunOneLiner(ctx, '\n', "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commit1) != 40 {
		t.Fatalf("rev-parse returned %q; need 40-digit hash", commit1)
	}
	if err := git.Run(ctx, "tag", "initial"); err != nil {
		t.Fatal(err)
	}

	// Second commit
	err = ioutil.WriteFile(filePath, []byte("Some more thoughts...\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.Run(ctx, "commit", "-a", "-m", "second commit"); err != nil {
		t.Fatal(err)
	}
	commit2, err := git.RunOneLiner(ctx, '\n', "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(commit2) != 40 {
		t.Fatalf("rev-parse returned %q; need 40-digit hash", commit2)
	}

	// Now verify:
	tests := []struct {
		refspec   string
		commitHex string
		refname   string
		branch    string
		err       bool
	}{
		{
			refspec: "",
			err:     true,
		},
		{
			refspec: "-",
			err:     true,
		},
		{
			refspec: "-HEAD",
			err:     true,
		},
		{
			refspec:   "HEAD",
			commitHex: string(commit2),
			refname:   "refs/heads/master",
			branch:    "master",
		},
		{
			refspec:   "master",
			commitHex: string(commit2),
			refname:   "refs/heads/master",
			branch:    "master",
		},
		{
			refspec:   string(commit1),
			commitHex: string(commit1),
		},
		{
			refspec:   string(commit2),
			commitHex: string(commit2),
		},
		{
			refspec:   "initial",
			commitHex: string(commit1),
			refname:   "refs/tags/initial",
		},
	}
	for _, test := range tests {
		rev, err := ParseRev(ctx, git, test.refspec)
		if err != nil {
			if !test.err {
				t.Errorf("ParseRev(ctx, git, %q) error: %v", test.refspec, err)
			}
			continue
		}
		if test.err {
			t.Errorf("ParseRev(ctx, git, %q) = %v; want error", test.refspec, rev)
			continue
		}
		if got := rev.CommitHex(); got != test.commitHex {
			t.Errorf("ParseRev(ctx, git, %q).CommitHex() = %q; want %q", test.refspec, got, test.commitHex)
		}
		if got := rev.RefName(); got != test.refname {
			t.Errorf("ParseRev(ctx, git, %q).RefName() = %q; want %q", test.refspec, got, test.refname)
		}
		if got := rev.Branch(); got != test.branch {
			t.Errorf("ParseRev(ctx, git, %q).Branch() = %q; want %q", test.refspec, got, test.branch)
		}
	}
}
