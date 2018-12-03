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
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"gg-scm.io/pkg/internal/sigterm"
)

// WorkTree determines the absolute path of the root of the current
// working tree given the configuration. Any symlinks are resolved.
func (g *Git) WorkTree(ctx context.Context) (string, error) {
	line, err := g.RunOneLiner(ctx, '\n', "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(string(line))
}

// CommonDir determines the absolute path of the Git directory, possibly
// shared among different working trees, given the configuration. Any
// symlinks are resolved.
func (g *Git) CommonDir(ctx context.Context) (string, error) {
	line, err := g.RunOneLiner(ctx, '\n', "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	path := string(line)
	if filepath.IsAbs(path) {
		path = filepath.Clean(path)
	} else {
		path = filepath.Join(g.dir, path)
	}
	return filepath.EvalSymlinks(path)
}

// IsMerging reports whether the index has a pending merge commit.
func (g *Git) IsMerging(ctx context.Context) (bool, error) {
	c := g.Command(ctx, "cat-file", "-e", "MERGE_HEAD")
	stderr := new(bytes.Buffer)
	c.Stderr = stderr
	if err := sigterm.Run(ctx, c); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return false, fmt.Errorf("check git merge: %v", err)
		}
		if exitStatus(exitErr.ProcessState) == 1 {
			return false, nil
		}
		errOut := bytes.TrimRight(stderr.Bytes(), "\n")
		if len(errOut) == 0 {
			return false, fmt.Errorf("check git merge: %v", exitErr)
		}
		return false, fmt.Errorf("check git merge: %s (%v)", errOut, exitErr)
	}
	return true, nil
}
