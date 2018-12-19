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

func TestLog(t *testing.T) {
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
	if err := env.root.Apply(filesystem.Write("foo.txt", dummyContent)); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "foo.txt"); err != nil {
		t.Fatal(err)
	}
	const wantMsg = "First post!!"
	if err := env.git.Commit(ctx, wantMsg); err != nil {
		t.Fatal(err)
	}
	rev, err := env.git.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}

	out, err := env.gg(ctx, env.root.String(), "log")
	if err != nil {
		t.Error(err)
	}
	hex := rev.Commit().Short()
	if !bytes.Contains(out, []byte(hex)) || !bytes.Contains(out, []byte(wantMsg)) {
		t.Errorf("log does not contain either %q or %q. Output:\n%s", hex, wantMsg, out)
	}
}
