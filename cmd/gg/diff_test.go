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
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestDiff(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	const oldLine = "Hello, World!"
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte(oldLine+"\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "first post"); err != nil {
		t.Fatal(err)
	}
	const newLine = "Good bye, World!"
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte(newLine+"\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	out, err := env.gg(ctx, env.root, "diff")
	if err != nil {
		t.Error(err)
	}
	if !bytes.Contains(out, []byte(oldLine)) || !bytes.Contains(out, []byte(newLine)) {
		t.Errorf("diff does not contain either %q or %q. Output:\n%s", oldLine, newLine, out)
	}
}

func TestDiff_NoChange(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte("Hello, World!\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "first post"); err != nil {
		t.Fatal(err)
	}

	out, err := env.gg(ctx, env.root, "diff")
	if err != nil {
		t.Error(err)
	}
	if len(out) != 0 {
		t.Errorf("diff is not empty. Output:\n%s", out)
	}
}
