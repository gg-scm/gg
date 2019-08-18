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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gg-scm.io/pkg/internal/escape"
	"gg-scm.io/pkg/internal/filesystem"
	"gg-scm.io/pkg/internal/git"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/xerrors"
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
				cacheHome:  filepath.Join("/home/foo", ".cache"),
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
		{
			name:    "CacheHome",
			environ: []string{"XDG_CACHE_HOME=/on/the/range"},
			want: xdgDirs{
				cacheHome:  "/on/the/range",
				configDirs: []string{"/etc/xdg"},
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
	root filesystem.Dir

	// git is a Git tool configured to operate in root.
	git *git.Git

	// roundTripper is the HTTP transport to use when invoking gg.
	// It defaults to a stub.
	roundTripper http.RoundTripper

	// The following are fields managed by testEnv, and should not be
	// referred to in tests.

	// topDir is the path to the temporary directory created by newTestEnv.
	topDir filesystem.Dir

	stderr   bytes.Buffer
	tb       testing.TB
	editFile int
}

var (
	gitPathOnce  sync.Once
	gitPath      string
	gitPathError error
)

func newTestEnv(ctx context.Context, tb testing.TB) (*testEnv, error) {
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
	// Always evaluate symlinks in the root directory path so as to make path
	// comparisons easier (simple equality). This is mostly relevant on macOS.
	topDir, err = filepath.EvalSymlinks(topDir)
	if err != nil {
		return nil, err
	}
	topFS := filesystem.Dir(topDir)
	err = topFS.Apply(
		filesystem.Mkdir("scratch"),
		filesystem.Mkdir("temp"),
	)
	if err != nil {
		os.RemoveAll(topDir)
		return nil, err
	}
	root := topFS.FromSlash("scratch")
	xdgConfigDir := topFS.FromSlash("xdgconfig")
	xdgCacheDir := topFS.FromSlash("xdgcache")
	git, err := git.New(gitPath, root, git.Options{
		Env: append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+topDir,
			"XDG_CONFIG_HOME="+xdgConfigDir,
			"XDG_CONFIG_DIRS="+xdgConfigDir,
			"XDG_CACHE_HOME="+xdgCacheDir,
		),
	})
	if err != nil {
		os.RemoveAll(topDir)
		return nil, err
	}
	env := &testEnv{
		topDir:       topFS,
		root:         filesystem.Dir(root),
		git:          git,
		roundTripper: stubRoundTripper{},
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
	fullConfig := "[user]\nname = User\nemail = foo@example.com\n" + string(config)
	err := env.topDir.Apply(filesystem.Write(".gitconfig", fullConfig))
	if err != nil {
		return xerrors.Errorf("write git config: %w", err)
	}
	return nil
}

// writeGitHubAuth writes a new file at $XDG_CONFIG_DIR/gg/github_token.
func (env *testEnv) writeGitHubAuth(tokenFile []byte) error {
	err := env.topDir.Apply(filesystem.Write("xdgconfig/gg/github_token", string(tokenFile)))
	if err != nil {
		return xerrors.Errorf("write GitHub auth: %w", err)
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
		return "", xerrors.Errorf("editor command: cp not found: %w", cpPathError)
	}
	fname := fmt.Sprintf("msg%02d", env.editFile)
	env.editFile++
	err := env.topDir.Apply(filesystem.Write(fname, string(content)))
	if err != nil {
		return "", xerrors.Errorf("editor command: %w", err)
	}
	dst := env.topDir.FromSlash(fname)
	return fmt.Sprintf("%s %s", cpPath, escape.Shell(dst)), nil
}

func (env *testEnv) cleanup() {
	if env.tb.Failed() && env.stderr.Len() > 0 {
		env.tb.Log("stderr:", env.stderr.String())
	}
	if err := os.RemoveAll(string(env.topDir)); err != nil {
		env.tb.Error("cleanup:", err)
	}
}

func (env *testEnv) gg(ctx context.Context, dir string, args ...string) ([]byte, error) {
	out := new(bytes.Buffer)
	xdgConfigDir := env.topDir.FromSlash("xdgconfig")
	pctx := &processContext{
		dir: dir,
		env: []string{
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME=" + env.topDir.String(),
			"XDG_CONFIG_HOME=" + xdgConfigDir,
			"XDG_CONFIG_DIRS=" + xdgConfigDir,
		},
		tempDir:    env.topDir.FromSlash("temp"),
		stdout:     out,
		stderr:     &env.stderr,
		httpClient: &http.Client{Transport: env.roundTripper},
		lookPath: func(name string) (string, error) {
			if name == "git" {
				return gitPath, gitPathError
			}
			return "", xerrors.New("look path stubbed")
		},
	}
	err := run(ctx, pctx, args)
	return out.Bytes(), err
}

// initEmptyRepo creates a repository at the slash-separated path
// relative to env.root.
func (env *testEnv) initEmptyRepo(ctx context.Context, dir string) error {
	return env.git.Init(ctx, env.root.FromSlash(dir))
}

// initRepoWithHistory creates a repository with some dummy commits but
// a blank, clean working copy. dir is a slash-separated path relative
// to env.root.
func (env *testEnv) initRepoWithHistory(ctx context.Context, dir string) error {
	repoDir := env.root.FromSlash(dir)
	if err := env.git.Init(ctx, repoDir); err != nil {
		return err
	}
	err := env.root.Apply(filesystem.Write(dir+"/.dummy", ""))
	if err != nil {
		return err
	}
	repoGit := env.git.WithDir(repoDir)
	if err := repoGit.Add(ctx, []git.Pathspec{".dummy"}, git.AddOptions{}); err != nil {
		return err
	}
	if err := repoGit.Commit(ctx, "initial import", git.CommitOptions{}); err != nil {
		return err
	}
	if err := repoGit.Remove(ctx, []git.Pathspec{".dummy"}, git.RemoveOptions{}); err != nil {
		return err
	}
	if err := repoGit.Commit(ctx, "removed dummy file", git.CommitOptions{}); err != nil {
		return err
	}
	return nil
}

// addFiles runs `git add` with the slash-separated paths relative to env.root.
func (env *testEnv) addFiles(ctx context.Context, files ...string) error {
	// Use the first file's directory as the Git working directory.
	anchor := env.root.FromSlash(files[0])
	info, err := os.Stat(anchor)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		anchor = filepath.Dir(anchor)
	}

	// Run git add.
	var pathspecs []git.Pathspec
	for _, f := range files {
		pathspecs = append(pathspecs, git.LiteralPath(env.root.FromSlash(f)))
	}
	return env.git.WithDir(anchor).Add(ctx, pathspecs, git.AddOptions{})
}

// trackFiles runs `git add -N` with the slash-separated paths relative to
// env.root.
func (env *testEnv) trackFiles(ctx context.Context, files ...string) error {
	// Use the first file's directory as the Git working directory.
	anchor := env.root.FromSlash(files[0])
	info, err := os.Stat(anchor)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		anchor = filepath.Dir(anchor)
	}

	// Run git add.
	var pathspecs []git.Pathspec
	for _, f := range files {
		pathspecs = append(pathspecs, git.LiteralPath(env.root.FromSlash(f)))
	}
	return env.git.WithDir(anchor).Add(ctx, pathspecs, git.AddOptions{
		IntentToAdd: true,
	})
}

// newCommit runs `git commit -a` with some dummy commit message at the
// slash-separated path relative to env.root.
func (env *testEnv) newCommit(ctx context.Context, dir string) (git.Hash, error) {
	g := env.git.WithDir(env.root.FromSlash(dir))
	if err := g.CommitAll(ctx, "did stuff", git.CommitOptions{}); err != nil {
		return git.Hash{}, err
	}
	r, err := g.Head(ctx)
	if err != nil {
		return git.Hash{}, err
	}
	return r.Commit, nil
}

// prettyCommit annotates the hex-encoded hash with a name if present
// in the given map.
func prettyCommit(h git.Hash, names map[git.Hash]string) string {
	n := names[h]
	if n == "" {
		return h.String()
	}
	return h.String() + " (" + n + ")"
}

// dummyContent is a non-empty string that is used in tests where the
// exact data is not relevant to the test.
const dummyContent = "Hello, World!\n"

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
