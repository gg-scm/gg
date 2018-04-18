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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"zombiezen.com/go/gg/internal/gittool"
)

var (
	gitPath      string
	gitPathError error
)

func TestMain(m *testing.M) {
	gitPath, gitPathError = exec.LookPath("git")
	os.Exit(m.Run())
}

type testEnv struct {
	topDir string
	root   string
	git    *gittool.Tool
	stderr *bytes.Buffer
	tb     testing.TB
}

func newTestEnv(ctx context.Context, tb testing.TB) (*testEnv, error) {
	if testing.Short() {
		tb.Skipf("skipping integration test due to -short")
	}
	if gitPathError != nil {
		tb.Skipf("could not find git, skipping (error: %v)", gitPathError)
	}
	topDir, err := ioutil.TempDir("", "gg_integration_test")
	if err != nil {
		return nil, err
	}
	root := filepath.Join(topDir, "scratch")
	if err := os.Mkdir(root, 0777); err != nil {
		os.RemoveAll(topDir)
	}
	stderr := new(bytes.Buffer)
	git, err := gittool.New(gitPath, root, &gittool.Options{
		Env:    append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+topDir),
		Stderr: stderr,
	})
	if err != nil {
		os.RemoveAll(topDir)
		return nil, err
	}
	env := &testEnv{
		topDir: topDir,
		root:   root,
		git:    git,
		stderr: stderr,
		tb:     tb,
	}
	if err := env.writeConfig(nil); err != nil {
		os.RemoveAll(topDir)
		return nil, err
	}
	return env, nil
}

// writeConfig writes a new configuration file with the given content.
// The test harness may write some baseline settings as well, but any
// settings in the argument take precedence.
func (env *testEnv) writeConfig(config []byte) error {
	path := filepath.Join(env.topDir, ".gitconfig")
	fullConfig := []byte("[user]\nname = User\nemail = foo@example.com\n")
	fullConfig = append(fullConfig, config...)
	if err := ioutil.WriteFile(path, fullConfig, 0666); err != nil {
		return fmt.Errorf("write git config: %v", err)
	}
	return nil
}

func (env *testEnv) cleanup() {
	if env.tb.Failed() && env.stderr.Len() > 0 {
		env.tb.Log("stderr:", env.stderr)
	}
	if err := os.RemoveAll(env.topDir); err != nil {
		env.tb.Error("cleanup:", err)
	}
}

func (env *testEnv) gg(ctx context.Context, dir string, args ...string) ([]byte, error) {
	out := new(bytes.Buffer)
	pctx := &processContext{
		dir:    dir,
		env:    []string{"GIT_CONFIG_NOSYSTEM=1", "HOME=" + env.root},
		stdout: out,
		stderr: env.stderr,
	}
	err := run(ctx, pctx, append([]string{"-git=" + gitPath}, args...))
	return out.Bytes(), err
}
