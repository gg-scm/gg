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

package gittool

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var (
	gitPath      string
	gitPathError error
)

func TestMain(m *testing.M) {
	gitPath, gitPathError = exec.LookPath("git")
	os.Exit(m.Run())
}

func TestRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
	if gitPathError != nil {
		t.Skip("git not found:", gitPathError)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.git.Run(ctx, "init", "repo"); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(env.root, "repo", ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Errorf("%s is not a git directory", gitDir)
	}
}

func TestConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
	if gitPathError != nil {
		t.Skip("git not found:", gitPathError)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	err = ioutil.WriteFile(
		filepath.Join(env.root, ".gitconfig"),
		[]byte("[user]\n\temail = foo@example.com\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if email, err := Config(ctx, env.git, "user.email"); err != nil {
		t.Error("for user.email:", err)
	} else if want := "foo@example.com"; email != want {
		t.Errorf("user.email = %q; want %q", email, want)
	}
	if notfound, err := Config(ctx, env.git, "foo.notfound"); err != nil {
		t.Error("for foo.notfound:", err)
	} else if notfound != "" {
		t.Errorf("foo.notfound = %q; want empty", notfound)
	}
}

type testEnv struct {
	root string
	git  *Tool
}

func newTestEnv(ctx context.Context) (*testEnv, error) {
	root, err := ioutil.TempDir("", "gg_gittool_test")
	if err != nil {
		return nil, err
	}
	git, err := New(gitPath, root, &Options{
		Env: []string{"GIT_CONFIG_NOSYSTEM=1", "HOME=" + root},
	})
	if err != nil {
		os.Remove(root)
		return nil, err
	}
	gitConfigPath := filepath.Join(root, ".gitconfig")
	gitConfig := []byte("[user]\nname = User\nemail = foo@example.com\n")
	if err := ioutil.WriteFile(gitConfigPath, gitConfig, 0666); err != nil {
		os.RemoveAll(root)
		return nil, err
	}
	return &testEnv{root: root, git: git}, nil
}

func (env *testEnv) cleanup() {
	os.RemoveAll(env.root)
}
