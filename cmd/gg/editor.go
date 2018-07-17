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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"gg-scm.io/pkg/internal/gittool"
	"gg-scm.io/pkg/internal/sigterm"
)

// editor allows editing text content interactively.
type editor struct {
	git      *gittool.Tool
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
	editor, err := e.git.RunOneLiner(ctx, '\n', "var", "GIT_EDITOR")
	if err != nil {
		return nil, fmt.Errorf("open editor: %v", err)
	}
	dir, err := ioutil.TempDir(e.tempRoot, "gg_editor")
	if err != nil {
		return nil, fmt.Errorf("open editor: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			e.log(fmt.Errorf("clean up editor: %v", err))
		}
	}()
	path := filepath.Join(dir, basename)
	if err := ioutil.WriteFile(path, initial, 0600); err != nil {
		return nil, fmt.Errorf("open editor: %v", err)
	}
	c := exec.Command("/bin/sh", "-c", string(editor)+" "+shellEscape(path))
	c.Stdin = e.stdin
	c.Stdout = e.stdout
	c.Stderr = e.stderr
	c.Env = e.env
	if len(c.Env) == 0 {
		c.Env = []string{} // force empty
	}
	if err := sigterm.Run(ctx, c); err != nil {
		return nil, fmt.Errorf("open editor: %v", err)
	}
	edited, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open editor: read result: %v", err)
	}
	return edited, nil
}
