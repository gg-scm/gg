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
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "init"); err != nil {
		t.Error(err)
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

func TestInit_Arg(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, env.root.String(), "init", "repo"); err != nil {
		t.Error(err)
	}
	gitDirPath := env.root.FromSlash("repo/.git")
	info, err := os.Stat(gitDirPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a directory", gitDirPath)
	}
}
