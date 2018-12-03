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
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestParseRev(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
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

	repoPath := filepath.Join(env.root, "repo")
	if err := env.g.Run(ctx, "init", repoPath); err != nil {
		t.Fatal(err)
	}
	g := env.g.WithDir(repoPath)

	// First commit
	const fileName = "foo.txt"
	filePath := filepath.Join(repoPath, fileName)
	err = ioutil.WriteFile(filePath, []byte("Hello, World!\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Run(ctx, "add", fileName); err != nil {
		t.Fatal(err)
	}
	if err := g.Run(ctx, "commit", "-m", "first commit"); err != nil {
		t.Fatal(err)
	}
	commit1Hex, err := g.RunOneLiner(ctx, '\n', "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit1, err := ParseHash(string(commit1Hex))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Run(ctx, "tag", "initial"); err != nil {
		t.Fatal(err)
	}

	// Second commit
	err = ioutil.WriteFile(filePath, []byte("Some more thoughts...\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Run(ctx, "commit", "-a", "-m", "second commit"); err != nil {
		t.Fatal(err)
	}
	commit2Hex, err := g.RunOneLiner(ctx, '\n', "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit2, err := ParseHash(string(commit2Hex))
	if err != nil {
		t.Fatal(err)
	}

	// Run fetch (to write FETCH_HEAD)
	if err := g.Run(ctx, "fetch", repoPath, "HEAD"); err != nil {
		t.Fatal(err)
	}

	// Now verify:
	tests := []struct {
		refspec string
		commit  Hash
		ref     Ref
		err     bool
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
			refspec: "HEAD",
			commit:  commit2,
			ref:     "refs/heads/master",
		},
		{
			refspec: "FETCH_HEAD",
			commit:  commit2,
			ref:     "FETCH_HEAD",
		},
		{
			refspec: "master",
			commit:  commit2,
			ref:     "refs/heads/master",
		},
		{
			refspec: commit1.String(),
			commit:  commit1,
		},
		{
			refspec: commit2.String(),
			commit:  commit2,
		},
		{
			refspec: "initial",
			commit:  commit1,
			ref:     "refs/tags/initial",
		},
	}
	for _, test := range tests {
		rev, err := ParseRev(ctx, g, test.refspec)
		if err != nil {
			if !test.err {
				t.Errorf("ParseRev(ctx, g, %q) error: %v", test.refspec, err)
			}
			continue
		}
		if test.err {
			t.Errorf("ParseRev(ctx, g, %q) = %v; want error", test.refspec, rev)
			continue
		}
		if got := rev.Commit(); got != test.commit {
			t.Errorf("ParseRev(ctx, g, %q).Commit() = %v; want %v", test.refspec, got, test.commit)
		}
		if got := rev.Ref(); got != test.ref {
			t.Errorf("ParseRev(ctx, g, %q).RefName() = %q; want %q", test.refspec, got, test.ref)
		}
	}
}
