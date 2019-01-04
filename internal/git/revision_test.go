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
	"strings"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"github.com/google/go-cmp/cmp"
)

func TestHash(t *testing.T) {
	tests := []struct {
		h     Hash
		s     string
		short string
	}{
		{
			h:     Hash{},
			s:     "0000000000000000000000000000000000000000",
			short: "00000000",
		},
		{
			h: Hash{
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67,
			},
			s:     "0123456789abcdef0123456789abcdef01234567",
			short: "01234567",
		},
	}
	for _, test := range tests {
		if got := test.h.String(); got != test.s {
			t.Errorf("Hash(%x).String() = %q; want %q", test.h[:], got, test.s)
		}
		if got := test.h.Short(); got != test.short {
			t.Errorf("Hash(%x).Short() = %q; want %q", test.h[:], got, test.short)
		}
	}
}

func TestParseHash(t *testing.T) {
	tests := []struct {
		s       string
		want    Hash
		wantErr bool
	}{
		{s: "", wantErr: true},
		{s: "0000000000000000000000000000000000000000", want: Hash{}},
		{
			s: "0123456789abcdef0123456789abcdef01234567",
			want: Hash{
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67,
			},
		},
		{
			s:       "0123456789abcdef0123456789abcdef0123456",
			wantErr: true,
		},
		{
			s:       "0123456789abcdef0123456789abcdef012345678",
			wantErr: true,
		},
		{
			s:       "01234567",
			wantErr: true,
		},
	}
	for _, test := range tests {
		switch got, err := ParseHash(test.s); {
		case err == nil && !test.wantErr && got != test.want:
			t.Errorf("ParseHash(%q) = %v, <nil>; want %v, <nil>", test.s, got, test.want)
		case err == nil && test.wantErr:
			t.Errorf("ParseHash(%q) = %v, <nil>; want error", test.s, got)
		case err != nil && !test.wantErr:
			t.Errorf("ParseHash(%q) = _, %v; want %v, <nil>", test.s, err, test.want)
		}
	}
}

func TestRef(t *testing.T) {
	tests := []struct {
		ref      Ref
		invalid  bool
		isBranch bool
		branch   string
		isTag    bool
		tag      string
	}{
		{
			ref:     "",
			invalid: true,
		},
		{
			ref:     "-",
			invalid: true,
		},
		{ref: "master"},
		{ref: "HEAD"},
		{ref: "FETCH_HEAD"},
		{ref: "ORIG_HEAD"},
		{ref: "MERGE_HEAD"},
		{ref: "CHERRY_PICK_HEAD"},
		{ref: "FOO"},
		{
			ref:     "-refs/heads/master",
			invalid: true,
		},
		{
			ref:      "refs/heads/master",
			isBranch: true,
			branch:   "master",
		},
		{
			ref:   "refs/tags/v1.2.3",
			isTag: true,
			tag:   "v1.2.3",
		},
		{ref: "refs/for/master"},
	}
	for _, test := range tests {
		if got := test.ref.String(); got != string(test.ref) {
			t.Errorf("Ref(%q).String() = %q; want %q", string(test.ref), got, string(test.ref))
		}
		if got := test.ref.IsValid(); got != !test.invalid {
			t.Errorf("Ref(%q).IsValid() = %t; want %t", string(test.ref), got, !test.invalid)
		}
		if got := test.ref.IsBranch(); got != test.isBranch {
			t.Errorf("Ref(%q).IsBranch() = %t; want %t", string(test.ref), got, test.isBranch)
		}
		if got := test.ref.Branch(); got != test.branch {
			t.Errorf("Ref(%q).Branch() = %s; want %s", string(test.ref), got, test.branch)
		}
		if got := test.ref.IsTag(); got != test.isTag {
			t.Errorf("Ref(%q).IsTag() = %t; want %t", string(test.ref), got, test.isTag)
		}
		if got := test.ref.Tag(); got != test.tag {
			t.Errorf("Ref(%q).Tag() = %s; want %s", string(test.ref), got, test.tag)
		}
	}
}

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

	repoPath := env.root.FromSlash("repo")
	if err := env.g.Init(ctx, repoPath); err != nil {
		t.Fatal(err)
	}
	g := env.g.WithDir(repoPath)

	// First commit
	const fileName = "foo.txt"
	if err := env.root.Apply(filesystem.Write("repo/foo.txt", "Hello, World!\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Run(ctx, "add", fileName); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Run(ctx, "commit", "-m", "first commit"); err != nil {
		t.Fatal(err)
	}
	commit1Hex, err := g.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit1, err := ParseHash(strings.TrimSuffix(commit1Hex, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Run(ctx, "tag", "initial"); err != nil {
		t.Fatal(err)
	}

	// Second commit
	if err := env.root.Apply(filesystem.Write("repo/foo.txt", "Some more thoughts...\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Run(ctx, "commit", "-a", "-m", "second commit"); err != nil {
		t.Fatal(err)
	}
	commit2Hex, err := g.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit2, err := ParseHash(strings.TrimSuffix(commit2Hex, "\n"))
	if err != nil {
		t.Fatal(err)
	}

	// Run fetch (to write FETCH_HEAD)
	if _, err := g.Run(ctx, "fetch", repoPath, "HEAD"); err != nil {
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
		rev, err := g.ParseRev(ctx, test.refspec)
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
		if got := rev.Commit; got != test.commit {
			t.Errorf("ParseRev(ctx, g, %q).Commit() = %v; want %v", test.refspec, got, test.commit)
		}
		if got := rev.Ref; got != test.ref {
			t.Errorf("ParseRev(ctx, g, %q).RefName() = %q; want %q", test.refspec, got, test.ref)
		}
	}
}

func TestListRefs(t *testing.T) {
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

	// Since ListRefs may be used to check state of other commands,
	// everything here uses raw commands.

	// Create the first master commit.
	if err := env.g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "commit", "-m", "first commit"); err != nil {
		t.Fatal(err)
	}
	revMaster, err := env.g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Create a new commit on branch abc.
	if _, err := env.g.Run(ctx, "checkout", "--quiet", "-b", "abc"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "add", "bar.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "commit", "-m", "abc commit"); err != nil {
		t.Fatal(err)
	}
	revABC, err := env.g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Create a two new commits on branch def.
	if _, err := env.g.Run(ctx, "checkout", "--quiet", "-b", "def", "master"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("baz.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "add", "baz.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "commit", "-m", "def commit 1"); err != nil {
		t.Fatal(err)
	}
	revDEF1, err := env.g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("baz.txt", dummyContent+"abc\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.g.Run(ctx, "commit", "-a", "-m", "def commit 2"); err != nil {
		t.Fatal(err)
	}
	revDEF2, err := env.g.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Tag the def branch as "ghi".
	if _, err := env.g.Run(ctx, "tag", "-a", "-m", "tests gonna tag", "ghi", "HEAD~"); err != nil {
		t.Fatal(err)
	}

	// Call env.g.ListRefs().
	got, err := env.g.ListRefs(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Verify that refs match what we expect.
	want := map[Ref]Hash{
		"refs/heads/master": revMaster.Commit,
		"refs/heads/abc":    revABC.Commit,
		"refs/heads/def":    revDEF2.Commit,
		"refs/tags/ghi":     revDEF1.Commit,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("refs (-want +got):\n%s", diff)
	}
}
