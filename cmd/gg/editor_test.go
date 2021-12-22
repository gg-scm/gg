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

package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"gg-scm.io/tool/internal/escape"
	"gg-scm.io/tool/internal/filesystem"
)

func TestEditor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	const want = "I edited it!\n"
	cmd, err := env.editorCmd([]byte(want))
	if err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("[core]\neditor = %s\n", escape.GitConfig(cmd))
	if err := env.writeConfig([]byte(config)); err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}

	stderr := new(bytes.Buffer)
	e := &editor{
		git:      env.git,
		tempRoot: env.root.String(),
		log: func(e error) {
			t.Error("Editor error:", e)
		},
		stderr: stderr,
	}
	got, err := e.open(ctx, "foo.txt", []byte("This is the initial content.\n"))
	if stderr.Len() > 0 {
		t.Log(stderr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("open(...) = %q; want %q", got, want)
	}
}

// Test for https://github.com/gg-scm/gg/issues/152
func TestEditorDirectory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("[core]\neditor = %s\n", escape.GitConfig("pwd | tee"))
	if err := env.writeConfig([]byte(config)); err != nil {
		t.Fatal(err)
	}
	if err := env.initEmptyRepo(ctx, "."); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Mkdir("foo")); err != nil {
		t.Fatal(err)
	}

	stderr := new(bytes.Buffer)
	e := &editor{
		git:      env.git.WithDir("foo"),
		tempRoot: env.topDir.FromSlash("temp"),
		log: func(e error) {
			t.Error("Editor error:", e)
		},
		stderr: stderr,
	}
	got, err := e.open(ctx, "foo.txt", []byte("This is the initial content.\n"))
	if stderr.Len() > 0 {
		t.Log(stderr)
	}
	if err != nil {
		t.Fatal(err)
	}
	gotPath := strings.TrimSuffix(string(got), "\n")
	gotPath, err = filepath.EvalSymlinks(gotPath)
	if err != nil {
		t.Fatal(err)
	}
	wantPath, err := filepath.EvalSymlinks(env.root.String())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != wantPath {
		t.Errorf("open(...) = %q; want %q", gotPath, wantPath)
	}
}
