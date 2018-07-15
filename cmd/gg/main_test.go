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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gg-scm.io/pkg/internal/gitobj"
	"gg-scm.io/pkg/internal/gittool"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewXDGDirs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		environ []string
		want    xdgDirs
	}{
		{
			name: "Empty",
			want: xdgDirs{
				configDirs: []string{"/etc/xdg"},
			},
		},
		{
			name:    "JustHome",
			environ: []string{"HOME=/home/foo"},
			want: xdgDirs{
				configHome: filepath.Join("/home/foo", ".config"),
				configDirs: []string{"/etc/xdg"},
			},
		},
		{
			name:    "ConfigHome",
			environ: []string{"XDG_CONFIG_HOME=/on/the/range"},
			want: xdgDirs{
				configHome: "/on/the/range",
				configDirs: []string{"/etc/xdg"},
			},
		},
		{
			name:    "OneConfigPath",
			environ: []string{"XDG_CONFIG_DIRS=/on/the/range"},
			want: xdgDirs{
				configDirs: []string{"/on/the/range"},
			},
		},
		{
			name: "TwoConfigPaths",
			environ: []string{
				"XDG_CONFIG_DIRS=" + strings.Join([]string{
					"/on/the/range",
					"/discouraging/words",
				}, string(filepath.ListSeparator)),
			},
			want: xdgDirs{
				configDirs: []string{"/on/the/range", "/discouraging/words"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			x := newXDGDirs(test.environ)
			diff := cmp.Diff(&test.want, x,
				cmp.AllowUnexported(xdgDirs{}),
				cmpopts.EquateEmpty())
			if diff != "" {
				t.Errorf("newXDGDirs(%q) = (-want +got):\n%s", test.environ, diff)
			}
		})
	}
}

type testEnv struct {
	// root is the path to a directory guaranteed to be empty at the
	// beginning of the test.
	root string

	// git is a Git tool configured to operate in root.
	git *gittool.Tool

	// roundTripper is the HTTP transport to use when invoking gg.
	// It defaults to a stub.
	roundTripper http.RoundTripper

	// The following are fields managed by testEnv, and should not be
	// referred to in tests.

	// topDir is the path to the temporary directory created by newTestEnv.
	topDir string

	stderr   *bytes.Buffer
	tb       testing.TB
	editFile int
}

var (
	gitPathOnce  sync.Once
	gitPath      string
	gitPathError error
)

func newTestEnv(ctx context.Context, tb testing.TB) (*testEnv, error) {
	if testing.Short() {
		tb.Skipf("skipping integration test due to -short")
	}
	gitPathOnce.Do(func() {
		gitPath, gitPathError = exec.LookPath("git")
	})
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
	xdgConfigDir := filepath.Join(topDir, "xdgconfig")
	stderr := new(bytes.Buffer)
	git, err := gittool.New(gitPath, root, &gittool.Options{
		Env: append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+topDir,
			"XDG_CONFIG_HOME="+xdgConfigDir,
			"XDG_CONFIG_DIRS="+xdgConfigDir,
		),
		Stderr: stderr,
	})
	if err != nil {
		os.RemoveAll(topDir)
		return nil, err
	}
	env := &testEnv{
		topDir:       topDir,
		root:         root,
		git:          git,
		roundTripper: stubRoundTripper{},
		stderr:       stderr,
		tb:           tb,
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

var (
	cpPathOnce  sync.Once
	cpPath      string
	cpPathError error
)

// editorCmd returns a shell command that will write the given bytes to
// an edited file, suitable for the content of the core.editor
// configuration setting.
func (env *testEnv) editorCmd(content []byte) (string, error) {
	cpPathOnce.Do(func() {
		cpPath, cpPathError = exec.LookPath("cp")
	})
	if cpPathError != nil {
		return "", fmt.Errorf("editor command: cp not found: %v", cpPathError)
	}
	dst := filepath.Join(env.topDir, fmt.Sprintf("msg%02d", env.editFile))
	env.editFile++
	if err := ioutil.WriteFile(dst, content, 0666); err != nil {
		return "", fmt.Errorf("editor command: %v", err)
	}
	return fmt.Sprintf("%s %s", cpPath, shellEscape(dst)), nil
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
	xdgConfigDir := filepath.Join(env.topDir, "xdgconfig")
	pctx := &processContext{
		dir: dir,
		env: []string{
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME=" + env.topDir,
			"XDG_CONFIG_HOME=" + xdgConfigDir,
			"XDG_CONFIG_DIRS=" + xdgConfigDir,
		},
		stdout:     out,
		stderr:     env.stderr,
		httpClient: &http.Client{Transport: env.roundTripper},
		lookPath: func(name string) (string, error) {
			if name == "git" {
				return gitPath, gitPathError
			}
			return "", errors.New("look path stubbed")
		},
	}
	err := run(ctx, pctx, args)
	return out.Bytes(), err
}

// rel resolves a slash-separated path relative to env.root.
func (env *testEnv) rel(path string) string {
	return filepath.Join(env.root, filepath.FromSlash(path))
}

// initEmptyRepo creates a repository at the slash-separated path
// relative to env.root.
func (env *testEnv) initEmptyRepo(ctx context.Context, dir string) error {
	return env.git.Run(ctx, "init", env.rel(dir))
}

// initRepoWithHistory creates a repository with some dummy commits but
// a blank, clean working copy. dir is a slash-separated path relative
// to env.root.
func (env *testEnv) initRepoWithHistory(ctx context.Context, dir string) error {
	repoDir := env.rel(dir)
	if err := env.git.Run(ctx, "init", repoDir); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(repoDir, ".dummy"), nil, 0666); err != nil {
		return err
	}
	repoGit := env.git.WithDir(repoDir)
	if err := repoGit.Run(ctx, "add", ".dummy"); err != nil {
		return err
	}
	if err := repoGit.Run(ctx, "commit", "-m", "initial import"); err != nil {
		return err
	}
	if err := repoGit.Run(ctx, "rm", ".dummy"); err != nil {
		return err
	}
	if err := repoGit.Run(ctx, "commit", "-m", "removed dummy file"); err != nil {
		return err
	}
	return nil
}

// readFile reads the file with the given content at the
// slash-separated path relative to env.root.
func (env *testEnv) readFile(path string) (string, error) {
	data, err := ioutil.ReadFile(env.rel(path))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// newFile creates a file with some non-empty dummy content at the
// slash-separated path relative to env.root.
func (env *testEnv) newFile(path string) error {
	return env.writeFile(path, "Hello, World!\n")
}

// writeFile creates a file with the given content at the
// slash-separated path relative to env.root.
func (env *testEnv) writeFile(path string, content string) error {
	return ioutil.WriteFile(env.rel(path), []byte(content), 0666)
}

// mkdir creates a directory at the slash-separated path relative to
// env.root.
func (env *testEnv) mkdir(path string) error {
	return os.Mkdir(env.rel(path), 0777)
}

// addFiles runs `git add -N` with the slash-separated paths relative to
// env.root.
func (env *testEnv) addFiles(ctx context.Context, files ...string) error {
	// Use the first file's directory as the Git working directory.
	anchor := env.rel(files[0])
	info, err := os.Stat(anchor)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		anchor = filepath.Dir(anchor)
	}

	// Run git add.
	args := make([]string, 0, 2+len(files))
	args = append(args, "add", "--")
	for i := range files {
		args = append(args, env.rel(files[i]))
	}
	return env.git.WithDir(anchor).Run(ctx, args...)
}

// trackFiles runs `git add -N` with the slash-separated paths relative to
// env.root.
func (env *testEnv) trackFiles(ctx context.Context, files ...string) error {
	// Use the first file's directory as the Git working directory.
	anchor := env.rel(files[0])
	info, err := os.Stat(anchor)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		anchor = filepath.Dir(anchor)
	}

	// Run git add.
	args := make([]string, 0, 3+len(files))
	args = append(args, "add", "-N", "--")
	for i := range files {
		args = append(args, env.rel(files[i]))
	}
	return env.git.WithDir(anchor).Run(ctx, args...)
}

// newCommit runs `git commit -a` with some dummy commit message at the
// slash-separated path relative to env.root.
func (env *testEnv) newCommit(ctx context.Context, dir string) (gitobj.Hash, error) {
	git := env.git.WithDir(env.rel(dir))
	if err := git.Run(ctx, "commit", "-am", "did stuff"); err != nil {
		return gitobj.Hash{}, err
	}
	r, err := gittool.ParseRev(ctx, git, "HEAD")
	if err != nil {
		return gitobj.Hash{}, err
	}
	return r.Commit(), nil
}

// dummyRev creates a new revision in a repository that adds the given file.
// If the branch is not the same as the current branch, that branch is either
// checked out or created.
func dummyRev(ctx context.Context, git *gittool.Tool, dir string, branch string, file string, msg string) (gitobj.Hash, error) {
	git = git.WithDir(dir)
	curr, err := gittool.ParseRev(ctx, git, "HEAD")
	if err != nil {
		// First commit
		if branch != "master" {
			return gitobj.Hash{}, fmt.Errorf("make dummy rev: %v", err)
		}
	} else if curr.Ref().Branch() != branch {
		if _, err := gittool.ParseRev(ctx, git, "refs/heads/"+branch); err != nil {
			// Branch doesn't exist, create it.
			if err := git.Run(ctx, "branch", "--", branch); err != nil {
				return gitobj.Hash{}, fmt.Errorf("make dummy rev: %v", err)
			}
			if err := git.Run(ctx, "branch", "--set-upstream-to="+curr.Ref().String(), "--", branch); err != nil {
				return gitobj.Hash{}, fmt.Errorf("make dummy rev: %v", err)
			}
		}
		if err := git.Run(ctx, "checkout", "--quiet", branch); err != nil {
			return gitobj.Hash{}, fmt.Errorf("make dummy rev: %v", err)
		}
	}
	err = ioutil.WriteFile(filepath.Join(dir, file), []byte("dummy content"), 0666)
	if err != nil {
		return gitobj.Hash{}, fmt.Errorf("make dummy rev: %v", err)
	}
	if err := git.Run(ctx, "add", file); err != nil {
		return gitobj.Hash{}, fmt.Errorf("make dummy rev: %v", err)
	}
	if err := git.Run(ctx, "commit", "-m", msg); err != nil {
		return gitobj.Hash{}, fmt.Errorf("make dummy rev: %v", err)
	}
	curr, err = gittool.ParseRev(ctx, git, "HEAD")
	if err != nil {
		return gitobj.Hash{}, fmt.Errorf("make dummy rev: %v", err)
	}
	return curr.Commit(), nil
}

// prettyCommit annotates the hex-encoded hash with a name if present
// in the given map.
func prettyCommit(h gitobj.Hash, names map[gitobj.Hash]string) string {
	n := names[h]
	if n == "" {
		return h.String()
	}
	return h.String() + " (" + n + ")"
}

// configEscape quotes s such that it can be used as a git configuration value.
func configEscape(s string) string {
	sb := new(strings.Builder)
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			sb.WriteString(`\n`)
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		default:
			sb.WriteByte(s[i])
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

// stubRoundTripper returns a Bad Gateway response for any incoming request.
type stubRoundTripper struct{}

// RoundTrip returns a Bad Gateway response.
func (stubRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body != nil {
		r.Body.Close() // RoundTripper must call Close.
	}
	body := strings.NewReader("stub")
	return &http.Response{
		StatusCode: http.StatusBadGateway,
		Status:     fmt.Sprintf("%d %s", http.StatusBadGateway, http.StatusText(http.StatusBadGateway)),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			http.CanonicalHeaderKey("Content-Type"):   {"text/plain; charset=utf-8"},
			http.CanonicalHeaderKey("Content-Length"): {fmt.Sprint(body.Len())},
		},
		Body:          ioutil.NopCloser(body),
		ContentLength: int64(body.Len()),
		Request:       r,
	}, nil
}
