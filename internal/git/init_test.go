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
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	ctx := context.Background()
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	env, err := newTestEnv(ctx, gitPath)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.g.Init(ctx, "."); err != nil {
		t.Error("Init returned error:", err)
	}
	gitDirPath := env.root.FromSlash(".git")
	info, err := os.Stat(gitDirPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", gitDirPath)
	}
}

func TestInitBare(t *testing.T) {
	ctx := context.Background()
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	env, err := newTestEnv(ctx, gitPath)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.g.InitBare(ctx, "."); err != nil {
		t.Error("Init returned error:", err)
	}
	if exists, err := env.root.Exists("HEAD"); err != nil {
		t.Error(err)
	} else if !exists {
		t.Error("HEAD file does not exist")
	}
}
