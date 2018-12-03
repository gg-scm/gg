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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
	"github.com/google/go-cmp/cmp"
)

func TestCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	tests := []struct {
		name string
		env  []string
	}{
		{
			name: "NilEnv",
			env:  nil,
		},
		{
			name: "FooEnv",
			env:  []string{"FOO=bar"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			dir, err := ioutil.TempDir("", "gg_gittool_test")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Remove(dir); err != nil {
					t.Error("cleaning up directory:", err)
				}
			}()
			var hookArgs []string
			var env []string
			if test.env != nil {
				env = append([]string(nil), test.env...)
			}
			git, err := New(gitPath, dir, &Options{
				LogHook: func(_ context.Context, args []string) {
					hookArgs = append([]string(nil), args...)
				},
				Env: env,
			})
			if err != nil {
				t.Fatal(err)
			}
			c := git.Command(ctx, "commit", "-m", "Hello, World!")
			if c.Path != gitPath {
				t.Errorf("c.Path = %q; want %q", c.Path, gitPath)
			}
			if len(c.Args) == 0 {
				t.Error("len(c.Args) = 0; want 4")
			} else {
				if got, want := filepath.Base(c.Args[0]), filepath.Base(gitPath); got != want {
					t.Errorf("c.Args[0], filepath.Base(c.Args[0]) = %q, %q; want %q, %q", c.Args[0], got, gitPath, want)
				}
				if got, want := c.Args[1:], ([]string{"commit", "-m", "Hello, World!"}); !cmp.Equal(got, want) {
					t.Errorf("c.Args[1:] = %q; want %q", got, want)
				}
			}
			if !cmp.Equal(c.Env, test.env) {
				t.Errorf("c.Env = %q; want %q", c.Env, test.env)
			}
			if c.Dir != dir {
				t.Errorf("c.Dir = %q; want %q", c.Dir, dir)
			}
			if want := ([]string{"commit", "-m", "Hello, World!"}); !cmp.Equal(hookArgs, want) {
				t.Errorf("log hook args = %q; want %q", hookArgs, want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping due to -short")
	}
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx, gitPath)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.g.Run(ctx, "init", "repo"); err != nil {
		t.Fatal(err)
	}
	gitDir := env.root.FromSlash("repo/.git")
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
	gitPath, err := findGit()
	if err != nil {
		t.Skip("git not found:", err)
	}
	ctx := context.Background()
	env, err := newTestEnv(ctx, gitPath)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()

	if err := env.g.Run(ctx, "init", "repo"); err != nil {
		t.Fatal(err)
	}
	g := env.g.WithDir(env.root.FromSlash("repo"))
	if err := env.root.Apply(filesystem.Write("repo/foo.txt", "Hi!\n")); err != nil {
		t.Fatal(err)
	}
	if err := g.Run(ctx, "add", "foo.txt"); err != nil {
		t.Fatal(err)
	}
	if err := g.Run(ctx, "commit", "-m", "first commit"); err != nil {
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
		got, err := g.Query(ctx, "cat-file", "-e", test.obj)
		if got != test.want || (err != nil) != test.err || !strings.Contains(fmt.Sprint(err), test.errMsg) {
			errStr := "<nil>"
			if test.err {
				errStr = fmt.Sprintf("<error containing %q>", test.errMsg)
			}
			t.Errorf("g.Query(ctx, \"cat-file\", \"-e\", %q) = %t, %v; want %t, %s", test.obj, got, err, test.want, errStr)
		}
	}
}

type testEnv struct {
	top  filesystem.Dir
	root filesystem.Dir
	g    *Git
}

func newTestEnv(ctx context.Context, gitPath string) (*testEnv, error) {
	topPath, err := ioutil.TempDir("", "gg_git_test")
	if err != nil {
		return nil, err
	}
	top := filesystem.Dir(topPath)
	if err := top.Apply(filesystem.Mkdir("scratch")); err != nil {
		os.RemoveAll(topPath)
		return nil, err
	}
	root := filesystem.Dir(top.FromSlash("scratch"))
	g, err := New(gitPath, root.String(), &Options{
		Env: []string{
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME=" + topPath,
			"TERM=xterm-color", // stops git from assuming output is to a "dumb" terminal
		},
	})
	if err != nil {
		os.RemoveAll(topPath)
		return nil, err
	}
	const miniConfig = "[user]\nname = User\nemail = foo@example.com\n"
	if err := top.Apply(filesystem.Write(".gitconfig", miniConfig)); err != nil {
		os.RemoveAll(topPath)
		return nil, err
	}
	return &testEnv{top: top, root: root, g: g}, nil
}

func (env *testEnv) cleanup() {
	os.RemoveAll(env.top.String())
}

var gitPathCache struct {
	mu  sync.Mutex
	val string
}

func findGit() (string, error) {
	defer gitPathCache.mu.Unlock()
	gitPathCache.mu.Lock()
	if gitPathCache.val != "" {
		return gitPathCache.val, nil
	}
	path, err := exec.LookPath("git")
	if err != nil {
		return "", err
	}
	gitPathCache.val = path
	return path, nil
}
