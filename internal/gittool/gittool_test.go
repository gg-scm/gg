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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestQuery(t *testing.T) {
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
	git := env.git.WithDir(filepath.Join(env.root, "repo"))
	err = ioutil.WriteFile(filepath.Join(env.root, "repo", "foo.txt"), []byte("Hi!\n"), 0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := git.Run(ctx, "commit", "-m", "first commit"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		obj    string
		want   bool
		err    bool
		errMsg string
	}{
		{obj: "master", want: true},
		{obj: "761b1f6130847a33f580c515aad3594f0127e564", want: false},
		{obj: "xyzzy", err: true, errMsg: "Not a valid object name xyzzy"},
	}
	for _, test := range tests {
		got, err := git.Query(ctx, "cat-file", "-e", test.obj)
		if got != test.want || (err != nil) != test.err || !strings.Contains(fmt.Sprint(err), test.errMsg) {
			errStr := "<nil>"
			if test.err {
				errStr = fmt.Sprintf("<error containing %q>", test.errMsg)
			}
			t.Errorf("git.Query(ctx, \"cat-file\", \"-e\", %q) = %t, %v; want %t, %s", test.obj, got, err, test.want, errStr)
		}
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
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME=" + root,
			"TERM=xterm-color", // stops git from assuming output is to a "dumb" terminal
		},
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
