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
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	repoPath := filepath.Join(env.root, "repo")
	if err := env.gut(ctx, env.root, "init", repoPath); err != nil {
		t.Fatal(err)
	}
	gitDirPath := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitDirPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", gitDirPath)
	}
}
