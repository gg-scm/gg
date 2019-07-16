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
	"io/ioutil"
	"strings"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestCat(t *testing.T) {
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

	// Create a repository with a few commits to foo.txt.
	// Uses raw commands, as cat is used to verify the state of other APIs.
	const (
		content1  = "Hello, World!\n"
		content2  = "Wut up, world?\n"
		wcContent = "This is foo.txt\n"
	)
	if err := env.g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", content1)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Run(ctx, "commit", "-m", "commit 1"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", content2)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Run(ctx, "commit", "-a", "-m", "commit 2"); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", wcContent)); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		rev  string
		want string
	}{
		{"HEAD", content2},
		{"HEAD~", content1},
	}
	for _, test := range tests {
		t.Run(test.rev+":foo.txt", func(t *testing.T) {
			r, err := env.g.Cat(ctx, test.rev, "foo.txt")
			if err != nil {
				t.Fatal("Cat error:", err)
			}
			got, err := ioutil.ReadAll(r)
			if string(got) != test.want {
				t.Errorf("read content = %q; want %q", got, test.want)
			}
			if err != nil {
				t.Error("Read error:", err)
			}
			if err := r.Close(); err != nil {
				t.Error("Close error:", err)
			}
		})
	}
}

func TestCatDoesNotExist(t *testing.T) {
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

	// Create a repository with a single commit.
	// Uses raw commands, as cat is used to verify the state of other APIs.
	if err := env.g.Init(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.g.Run(ctx, "commit", "-m", "initial commit"); err != nil {
		t.Fatal(err)
	}
	r, err := env.g.Cat(ctx, "HEAD", "bar.txt")
	if err == nil {
		t.Error("Cat did not return an error")
	} else if got := err.Error(); !strings.Contains(got, "bar.txt") {
		t.Errorf("error = %v; want to contain \"bar.txt\"", got)
	}
	if r != nil {
		t.Error("reader != nil")
		r.Close()
	}
}
