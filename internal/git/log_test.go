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
	"time"

	"gg-scm.io/pkg/internal/filesystem"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCommitInfo(t *testing.T) {
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()

	// The commits created in these tests are entirely deterministic
	// because the dates and users are fixed, so their hashes will always
	// be the same.

	t.Run("EmptyMaster", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		_, err = env.g.CommitInfo(ctx, "master")
		if err == nil {
			t.Error("CommitInfo did not return error", err)
		}
	})
	t.Run("FirstCommit", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with a single commit to foo.txt.
		// Uses raw commands, as CommitInfo is used to verify the state of other APIs.
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
			t.Fatal(err)
		}
		// Message does not have trailing newline to verify verbatim processing.
		const (
			wantAuthorName     = "Lisbeth Salander"
			wantAuthorEmail    = "lisbeth@example.com"
			wantCommitterName  = "Octo Cat"
			wantCommitterEmail = "noreply@github.com"
			wantMsg            = "\t foobarbaz  \r\n\n  initial import  "
		)
		wantAuthorTime := time.Date(2018, time.February, 20, 15, 47, 42, 0, time.FixedZone("UTC-8", -8*60*60))
		wantCommitTime := time.Date(2018, time.December, 29, 8, 58, 24, 0, time.FixedZone("UTC-8", -8*60*60))
		{
			c := env.g.Command(ctx, "commit", "--cleanup=verbatim", "--file=-")
			c.Env = append(c.Env,
				"GIT_AUTHOR_NAME="+wantAuthorName,
				"GIT_AUTHOR_EMAIL="+wantAuthorEmail,
				"GIT_AUTHOR_DATE=2018-02-20T15:47:42-08:00",
				"GIT_COMMITTER_NAME="+wantCommitterName,
				"GIT_COMMITTER_EMAIL="+wantCommitterEmail,
				"GIT_COMMITTER_DATE=2018-12-29T08:58:24-08:00",
			)
			c.Stdin = strings.NewReader(wantMsg)
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}
		got, err := env.g.CommitInfo(ctx, "HEAD")
		if err != nil {
			t.Fatal("CommitInfo:", err)
		}
		want := &CommitInfo{
			Hash: Hash{0x73, 0xcd, 0x46, 0xa9, 0x21, 0x98, 0xf2, 0x88, 0x49, 0x18, 0x85, 0xd2, 0x8b, 0x6e, 0xf8, 0x26, 0xd3, 0x9c, 0x96, 0x08},
			Author: User{
				Name:  wantAuthorName,
				Email: wantAuthorEmail,
			},
			AuthorTime: wantAuthorTime,
			Committer: User{
				Name:  wantCommitterName,
				Email: wantCommitterEmail,
			},
			CommitTime: wantCommitTime,
			Message:    wantMsg,
		}
		if diff := cmp.Diff(want, got, equateTruncatedTime(time.Second), cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("CommitInfo(ctx, \"HEAD\") diff (-want +got):\n%s", diff)
		}
	})
	t.Run("SecondCommit", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with two commits.
		// Uses raw commands, as CommitInfo is used to verify the state of other APIs.
		const (
			wantAuthorName     = "Lisbeth Salander"
			wantAuthorEmail    = "lisbeth@example.com"
			wantCommitterName  = "Octo Cat"
			wantCommitterEmail = "noreply@github.com"
		)
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
			t.Fatal(err)
		}
		{
			c := env.g.Command(ctx, "commit", "--cleanup=verbatim", "--file=-")
			c.Env = append(c.Env,
				"GIT_AUTHOR_NAME="+wantAuthorName,
				"GIT_AUTHOR_EMAIL="+wantAuthorEmail,
				"GIT_AUTHOR_DATE=2018-02-20T15:47:42-08:00",
				"GIT_COMMITTER_NAME="+wantCommitterName,
				"GIT_COMMITTER_EMAIL="+wantCommitterEmail,
				"GIT_COMMITTER_DATE=2018-12-29T08:58:24-08:00",
			)
			c.Stdin = strings.NewReader("initial import")
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}
		if err := env.g.Remove(ctx, []Pathspec{"foo.txt"}, RemoveOptions{}); err != nil {
			t.Fatal(err)
		}
		// Message does not have trailing newline to verify verbatim processing.
		const wantMsg = "\t foobarbaz  \r\n\n  the second commit  "
		wantAuthorTime := time.Date(2018, time.March, 21, 16, 26, 9, 0, time.FixedZone("UTC-7", -7*60*60))
		wantCommitTime := time.Date(2018, time.December, 29, 8, 58, 24, 0, time.FixedZone("UTC-8", -8*60*60))
		{
			c := env.g.Command(ctx, "commit", "--quiet", "--cleanup=verbatim", "--file=-")
			c.Env = append(c.Env,
				"GIT_AUTHOR_NAME="+wantAuthorName,
				"GIT_AUTHOR_EMAIL="+wantAuthorEmail,
				"GIT_AUTHOR_DATE=2018-03-21T16:26:09-07:00",
				"GIT_COMMITTER_NAME="+wantCommitterName,
				"GIT_COMMITTER_EMAIL="+wantCommitterEmail,
				"GIT_COMMITTER_DATE=2018-12-29T08:58:24-08:00",
			)
			c.Stdin = strings.NewReader(wantMsg)
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}

		got, err := env.g.CommitInfo(ctx, "HEAD")
		if err != nil {
			t.Fatal("CommitInfo:", err)
		}
		want := &CommitInfo{
			Hash: Hash{0x71, 0x56, 0xda, 0x2b, 0x51, 0x58, 0x7d, 0xbf, 0x35, 0x5f, 0x31, 0x29, 0x93, 0xc7, 0x9e, 0x20, 0xbc, 0x28, 0x10, 0xb4},
			Parents: []Hash{
				{0x27, 0x8b, 0x79, 0xae, 0xdc, 0xa2, 0x4c, 0x9f, 0x85, 0xdb, 0x56, 0xe3, 0x19, 0x9a, 0x14, 0xdb, 0x2b, 0x6e, 0x9a, 0x8d},
			},
			Author: User{
				Name:  wantAuthorName,
				Email: wantAuthorEmail,
			},
			AuthorTime: wantAuthorTime,
			Committer: User{
				Name:  wantCommitterName,
				Email: wantCommitterEmail,
			},
			CommitTime: wantCommitTime,
			Message:    wantMsg,
		}
		if diff := cmp.Diff(want, got, equateTruncatedTime(time.Second), cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("CommitInfo(ctx, \"HEAD\") diff (-want +got):\n%s", diff)
		}
	})
	t.Run("MergeCommit", func(t *testing.T) {
		env, err := newTestEnv(ctx, gitPath)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()

		// Create a repository with a merge commit.
		// Uses raw commands, as CommitInfo is used to verify the state of other APIs.
		const (
			wantAuthorName     = "Lisbeth Salander"
			wantAuthorEmail    = "lisbeth@example.com"
			wantCommitterName  = "Octo Cat"
			wantCommitterEmail = "noreply@github.com"
		)
		if err := env.g.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
			t.Fatal(err)
		}
		{
			c := env.g.Command(ctx, "commit", "--cleanup=verbatim", "--file=-")
			c.Env = append(c.Env,
				"GIT_AUTHOR_NAME="+wantAuthorName,
				"GIT_AUTHOR_EMAIL="+wantAuthorEmail,
				"GIT_AUTHOR_DATE=2018-02-20T15:47:42-08:00",
				"GIT_COMMITTER_NAME="+wantCommitterName,
				"GIT_COMMITTER_EMAIL="+wantCommitterEmail,
				"GIT_COMMITTER_DATE=2018-12-29T08:58:24-08:00",
			)
			c.Stdin = strings.NewReader("initial import")
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}
		if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "bar.txt"); err != nil {
			t.Fatal(err)
		}
		{
			c := env.g.Command(ctx, "commit", "--cleanup=verbatim", "--file=-")
			c.Env = append(c.Env,
				"GIT_AUTHOR_NAME="+wantAuthorName,
				"GIT_AUTHOR_EMAIL="+wantAuthorEmail,
				"GIT_AUTHOR_DATE=2018-02-21T15:49:58-08:00",
				"GIT_COMMITTER_NAME="+wantCommitterName,
				"GIT_COMMITTER_EMAIL="+wantCommitterEmail,
				"GIT_COMMITTER_DATE=2018-12-29T08:58:24-08:00",
			)
			c.Stdin = strings.NewReader("first parent")
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}
		if err := env.g.Run(ctx, "checkout", "--quiet", "-b", "diverge", "HEAD~"); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write("baz.txt", dummyContent)); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Run(ctx, "add", "baz.txt"); err != nil {
			t.Fatal(err)
		}
		{
			c := env.g.Command(ctx, "commit", "--cleanup=verbatim", "--file=-")
			c.Env = append(c.Env,
				"GIT_AUTHOR_NAME="+wantAuthorName,
				"GIT_AUTHOR_EMAIL="+wantAuthorEmail,
				"GIT_AUTHOR_DATE=2018-02-21T17:07:53-08:00",
				"GIT_COMMITTER_NAME="+wantCommitterName,
				"GIT_COMMITTER_EMAIL="+wantCommitterEmail,
				"GIT_COMMITTER_DATE=2018-12-29T08:58:24-08:00",
			)
			c.Stdin = strings.NewReader("second parent")
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}
		if err := env.g.Run(ctx, "checkout", "--quiet", "master"); err != nil {
			t.Fatal(err)
		}
		if err := env.g.Merge(ctx, []string{"diverge"}); err != nil {
			t.Fatal(err)
		}
		const wantMsg = "Merge branch 'diverge' into branch master\n"
		wantAuthorTime := time.Date(2018, time.February, 21, 19, 37, 26, 0, time.FixedZone("UTC-8", -8*60*60))
		wantCommitTime := time.Date(2018, time.December, 29, 8, 58, 24, 0, time.FixedZone("UTC-8", -8*60*60))
		{
			c := env.g.Command(ctx, "commit", "--cleanup=verbatim", "--file=-")
			c.Env = append(c.Env,
				"GIT_AUTHOR_NAME="+wantAuthorName,
				"GIT_AUTHOR_EMAIL="+wantAuthorEmail,
				"GIT_AUTHOR_DATE=2018-02-21T19:37:26-08:00",
				"GIT_COMMITTER_NAME="+wantCommitterName,
				"GIT_COMMITTER_EMAIL="+wantCommitterEmail,
				"GIT_COMMITTER_DATE=2018-12-29T08:58:24-08:00",
			)
			c.Stdin = strings.NewReader(wantMsg)
			if err := c.Run(); err != nil {
				t.Fatal(err)
			}
		}

		got, err := env.g.CommitInfo(ctx, "HEAD")
		if err != nil {
			t.Fatal("CommitInfo:", err)
		}
		want := &CommitInfo{
			Hash: Hash{0xa7, 0xaf, 0xbf, 0x02, 0x90, 0x27, 0xd6, 0x61, 0xfb, 0x06, 0x3c, 0x9c, 0x49, 0xa5, 0xfa, 0x44, 0x38, 0x53, 0xfa, 0x40},
			Parents: []Hash{
				{0xba, 0xd7, 0xc7, 0xc9, 0xa3, 0x69, 0x03, 0x68, 0xae, 0xb2, 0x51, 0x97, 0x7f, 0x84, 0x12, 0xd1, 0xee, 0x55, 0x11, 0x56},
				{0xac, 0xb6, 0xec, 0xe2, 0xdb, 0xc7, 0x5b, 0x42, 0x4d, 0x7d, 0x39, 0x11, 0x14, 0x48, 0xf8, 0xba, 0xca, 0x7d, 0x84, 0x07},
			},
			Author: User{
				Name:  wantAuthorName,
				Email: wantAuthorEmail,
			},
			AuthorTime: wantAuthorTime,
			Committer: User{
				Name:  wantCommitterName,
				Email: wantCommitterEmail,
			},
			CommitTime: wantCommitTime,
			Message:    wantMsg,
		}
		if diff := cmp.Diff(want, got, equateTruncatedTime(time.Second), cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("CommitInfo(ctx, \"HEAD\") diff (-want +got):\n%s", diff)
		}
	})
}

func TestLog(t *testing.T) {
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

	// Create a repository with a merge commit.
	//
	// The commits created in this test are entirely deterministic
	// because the dates and users are fixed, so their hashes will always
	// be the same.
	const (
		wantAuthorName     = "Lisbeth Salander"
		wantAuthorEmail    = "lisbeth@example.com"
		wantCommitterName  = "Octo Cat"
		wantCommitterEmail = "noreply@github.com"
	)
	commitOpts := func(t time.Time) CommitOptions {
		return CommitOptions{
			Author: User{
				Name:  wantAuthorName,
				Email: wantAuthorEmail,
			},
			AuthorTime: t,
			Committer: User{
				Name:  wantCommitterName,
				Email: wantCommitterEmail,
			},
			CommitTime: t,
		}
	}
	if err := env.g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Add(ctx, []Pathspec{"foo.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	const wantMessage0 = "initial import\n"
	wantTime0 := time.Date(2018, time.February, 20, 15, 47, 42, 0, time.FixedZone("UTC-8", -8*60*60))
	if err := env.g.Commit(ctx, wantMessage0, commitOpts(wantTime0)); err != nil {
		t.Fatal(err)
	}

	if err := env.root.Apply(filesystem.Write("bar.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Add(ctx, []Pathspec{"bar.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	const wantMessage1 = "first parent\n"
	wantTime1 := time.Date(2018, time.February, 21, 15, 49, 58, 0, time.FixedZone("UTC-8", -8*60*60))
	if err := env.g.Commit(ctx, wantMessage1, commitOpts(wantTime1)); err != nil {
		t.Fatal(err)
	}

	if err := env.g.NewBranch(ctx, "diverge", BranchOptions{Checkout: true, StartPoint: "HEAD~"}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("baz.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Add(ctx, []Pathspec{"baz.txt"}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	const wantMessage2 = "second parent\n"
	wantTime2 := time.Date(2018, time.February, 21, 17, 7, 53, 0, time.FixedZone("UTC-8", -8*60*60))
	if err := env.g.Commit(ctx, wantMessage2, commitOpts(wantTime2)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.CheckoutBranch(ctx, "master", CheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Merge(ctx, []string{"diverge"}); err != nil {
		t.Fatal(err)
	}
	const wantMessage3 = "i merged\n"
	wantTime3 := time.Date(2018, time.February, 21, 19, 37, 26, 0, time.FixedZone("UTC-8", -8*60*60))
	if err := env.g.Commit(ctx, wantMessage3, commitOpts(wantTime3)); err != nil {
		t.Fatal(err)
	}

	log, err := env.g.Log(ctx, LogOptions{})
	if err != nil {
		t.Fatal("Log:", err)
	}
	var got []*CommitInfo
	for log.Next() {
		got = append(got, log.CommitInfo())
	}
	if err := log.Close(); err != nil {
		t.Error("Close:", err)
	}
	author := User{Name: wantAuthorName, Email: wantAuthorEmail}
	committer := User{Name: wantCommitterName, Email: wantCommitterEmail}
	hash0 := Hash{0x3b, 0x1b, 0x50, 0xc3, 0xe4, 0x27, 0xa6, 0x8f, 0x73, 0x3f, 0xe8, 0x7a, 0x74, 0x0f, 0x8f, 0x74, 0x3c, 0xe3, 0x13, 0xb7}
	hash1 := Hash{0x9d, 0x74, 0xc2, 0x89, 0x91, 0x5c, 0x29, 0xbc, 0xda, 0xc1, 0x74, 0xc4, 0x77, 0x85, 0x8c, 0x51, 0x0c, 0x64, 0x7c, 0xf0}
	hash2 := Hash{0x87, 0x48, 0x1b, 0x55, 0x8a, 0xb3, 0xa3, 0xe4, 0xe1, 0x02, 0xd3, 0x9a, 0x1f, 0x32, 0x2d, 0xd9, 0xe9, 0xf0, 0x23, 0x42}
	hash3 := Hash{0x1f, 0x56, 0x59, 0x25, 0xf9, 0x90, 0xfb, 0xb2, 0x9a, 0x20, 0xbc, 0x0b, 0x18, 0xa0, 0xac, 0x19, 0xba, 0x2c, 0x19, 0xc8}
	want := []*CommitInfo{
		{
			Hash:       hash3,
			Parents:    []Hash{hash1, hash2},
			Author:     author,
			Committer:  committer,
			AuthorTime: wantTime3,
			CommitTime: wantTime3,
			Message:    wantMessage3,
		},
		{
			Hash:       hash2,
			Parents:    []Hash{hash0},
			Author:     author,
			Committer:  committer,
			AuthorTime: wantTime2,
			CommitTime: wantTime2,
			Message:    wantMessage2,
		},
		{
			Hash:       hash1,
			Parents:    []Hash{hash0},
			Author:     author,
			Committer:  committer,
			AuthorTime: wantTime1,
			CommitTime: wantTime1,
			Message:    wantMessage1,
		},
		{
			Hash:       hash0,
			Author:     author,
			Committer:  committer,
			AuthorTime: wantTime0,
			CommitTime: wantTime0,
			Message:    wantMessage0,
		},
	}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("log diff (-want +got):\n%s", diff)
	}
}

func equateTruncatedTime(d time.Duration) cmp.Option {
	return cmp.Comparer(func(t1, t2 time.Time) bool {
		return t1.Truncate(d).Equal(t2.Truncate(d))
	})
}
