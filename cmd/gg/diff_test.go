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

package main

import (
	"bytes"
	"context"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Create a commit with foo.txt.
	const oldLine = "Hello, World!"
	if err := env.root.Apply(filesystem.Write("foo.txt", oldLine+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Modify foo.txt in the working copy without committing.
	const newLine = "Good bye, World!"
	if err := env.root.Apply(filesystem.Write("foo.txt", newLine+"\n")); err != nil {
		t.Fatal(err)
	}
	if err != nil {
		t.Fatal(err)
	}

	// Verify that diff contains both the old content and the new content.
	out, err := env.gg(ctx, env.root.String(), "diff")
	if err != nil {
		t.Error(err)
	}
	if !bytes.Contains(out, []byte(oldLine)) || !bytes.Contains(out, []byte(newLine)) {
		t.Errorf("diff does not contain either %q or %q. Output:\n%s", oldLine, newLine, out)
	}
}

func TestDiff_NoChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Create a commit with foo.txt.
	const oldLine = "Hello, World!"
	if err := env.root.Apply(filesystem.Write("foo.txt", oldLine+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Verify that diff is empty (working copy is clean).
	out, err := env.gg(ctx, env.root.String(), "diff")
	if err != nil {
		t.Error(err)
	}
	if len(out) != 0 {
		t.Errorf("diff is not empty. Output:\n%s", out)
	}
}

func TestDiff_AfterInit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Verify that diff is empty (working copy is clean).
	out, err := env.gg(ctx, env.root.String(), "diff")
	if err != nil {
		t.Error(err)
	}
	if len(out) != 0 {
		t.Errorf("diff is not empty. Output:\n%s", out)
	}
}

func TestDiff_BeforeFirstCommit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}

	// Create a file foo.txt and add it to the index. Do not commit.
	const line = "Hello, World!"
	if err := env.root.Apply(filesystem.Write("foo.txt", line+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}

	// Verify that diff contains the line written to foo.txt.
	out, err := env.gg(ctx, env.root.String(), "diff")
	if err != nil {
		t.Error(err)
	}
	if !bytes.Contains(out, []byte(line)) {
		t.Errorf("diff does not contain %q. Output:\n%s", line, out)
	}
}
