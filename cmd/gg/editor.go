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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/escape"
	"gg-scm.io/tool/internal/sigterm"
)

// editor allows editing text content interactively.
type editor struct {
	git      *git.Git
	log      func(error)
	tempRoot string

	env    []string // empty means no environment
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// open opens the default Git editor with the given initial
// content and waits for it to return.
func (e *editor) open(ctx context.Context, basename string, initial []byte) ([]byte, error) {
	editor, err := e.git.Output(ctx, "var", "GIT_EDITOR")
	if err != nil {
		return nil, fmt.Errorf("open editor: %w", err)
	}
	editor = strings.TrimSuffix(editor, "\n")
	dir, err := ioutil.TempDir(e.tempRoot, "gg_editor")
	if err != nil {
		return nil, fmt.Errorf("open editor: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			e.log(fmt.Errorf("clean up editor: %w", err))
		}
	}()
	path := filepath.Join(dir, basename)
	if err := ioutil.WriteFile(path, initial, 0600); err != nil {
		return nil, fmt.Errorf("open editor: %w", err)
	}
	c := exec.Command("/bin/sh", "-c", string(editor)+" "+escape.Shell(path))
	c.Stdin = e.stdin
	c.Stdout = e.stdout
	c.Stderr = e.stderr
	c.Env = e.env
	if len(c.Env) == 0 {
		c.Env = []string{} // force empty
	}
	if err := sigterm.Run(ctx, c); err != nil {
		return nil, fmt.Errorf("open editor: %w", err)
	}
	edited, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open editor: read result: %w", err)
	}
	return edited, nil
}
