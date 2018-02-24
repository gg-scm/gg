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
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"zombiezen.com/go/gut/internal/gittool"
)

var (
	exePath      string
	exePathError error

	gitPath      string
	gitPathError error
)

func TestMain(m *testing.M) {
	if os.Getenv("GUT_TEST_SUBPROCESS") == "1" {
		main()
		return
	}
	exePath, exePathError = os.Executable()
	gitPath, gitPathError = exec.LookPath("git")
	os.Exit(m.Run())
}

type testEnv struct {
	root string
	git  *gittool.Tool
}

func newTestEnv(ctx context.Context, s skipper) (*testEnv, error) {
	if testing.Short() {
		s.Skipf("skipping integration test due to -short")
	}
	if gitPathError != nil {
		s.Skipf("could not find git, skipping (error: %v)", gitPathError)
	}
	if exePathError != nil {
		return nil, fmt.Errorf("could not determine executable path: %v", exePathError)
	}
	root, err := ioutil.TempDir("", "gut_integration_test")
	if err != nil {
		return nil, err
	}
	git, err := gittool.New(gitPath, root, &gittool.Options{
		Env: append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+root),
	})
	if err != nil {
		return nil, err
	}
	return &testEnv{root: root, git: git}, nil
}

func (env *testEnv) cleanup() {
	os.RemoveAll(env.root)
}

func (env *testEnv) gut(ctx context.Context, dir string, args ...string) error {
	pctx := &processContext{
		dir: dir,
		env: []string{"GIT_CONFIG_NOSYSTEM=1", "HOME=" + env.root},
	}
	return run(ctx, pctx, append([]string{"-git=" + gitPath}, args...))
}

type skipper interface {
	Skipf(string, ...interface{})
}
