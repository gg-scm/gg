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

// Package git provides a high-level interface for interacting with
// a Git subprocess.
package git // import "gg-scm.io/pkg/internal/git"

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gg-scm.io/pkg/internal/sigterm"
	"golang.org/x/xerrors"
)

// Git is a context for performing Git version control operations.
// Broadly, it consists of a path to an installed copy of Git and a
// working directory path.
type Git struct {
	exe string
	dir string
	env []string // cap(env) == len(env), guaranteed by New.
	log func(context.Context, []string)

	versionMu   sync.Mutex
	versionCond chan struct{}
	version     string
}

// Options specifies optional parameters to New.
type Options struct {
	// LogHook is a function that will be called at the start of every Git
	// subprocess.
	LogHook func(ctx context.Context, args []string)

	// Env specifies the environment of the subprocess.
	// If len(Env) == 0, then no environment variables will be set.
	Env []string
}

// New creates a new Git context.
func New(path string, wd string, opts Options) (*Git, error) {
	if !filepath.IsAbs(path) {
		return nil, xerrors.Errorf("path to git must be absolute (got %q)", path)
	}
	if wd == "" {
		return nil, xerrors.New("init git: working directory must not be blank")
	}

	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return nil, xerrors.Errorf("stat git: %w", err)
	}
	m := info.Mode()
	if m.IsDir() || m&0111 == 0 {
		return nil, xerrors.Errorf("stat git: not an executable file")
	}

	wd, err = filepath.Abs(wd)
	if err != nil {
		return nil, xerrors.Errorf("init git: resolve working directory: %w", err)
	}

	g := &Git{
		exe: path,
		dir: wd,
	}
	g.log = opts.LogHook
	g.env = make([]string, len(opts.Env))
	copy(g.env, opts.Env)
	return g, nil
}

// Command creates a new *exec.Cmd that will invoke Git with the given
// arguments. The returned command does not obey the given Context's deadline
// or cancelation.
func (g *Git) Command(ctx context.Context, args ...string) *exec.Cmd {
	c := g.command(ctx, append([]string{g.exe}, args...))
	c.Env = append([]string{}, c.Env...) // Defensive copy.
	return c
}

// command returns a new *exec.Cmd for the given arguments, whose first
// element must be g.exe. g.env will be used directly, so make a copy if
// the return value is going to be exposed (usually just in Command).
func (g *Git) command(ctx context.Context, argv []string) *exec.Cmd {
	if g.log != nil {
		g.log(ctx, argv[1:])
	}
	if len(argv) == 0 || argv[0] != g.exe {
		panic("argv[0] != g.exe")
	}
	return &exec.Cmd{
		Path: g.exe,
		Args: argv,
		Env:  g.env,
		Dir:  g.dir,
	}
}

func (g *Git) getVersion(ctx context.Context) (string, error) {
	g.versionMu.Lock()
	for g.versionCond != nil {
		c := g.versionCond
		g.versionMu.Unlock()
		select {
		case <-c:
			g.versionMu.Lock()
		case <-ctx.Done():
			return "", xerrors.Errorf("git --version: %w", ctx.Err())
		}
	}
	if g.version != "" {
		// Cached version string available.
		v := g.version
		g.versionMu.Unlock()
		return v, nil
	}
	g.versionCond = make(chan struct{})
	g.versionMu.Unlock()

	// Run git --version.
	v, err := g.output(ctx, "git --version", []string{g.exe, "--version"})
	g.versionMu.Lock()
	close(g.versionCond)
	g.versionCond = nil
	if err != nil {
		g.versionMu.Unlock()
		return "", err
	}
	g.version = v
	g.versionMu.Unlock()
	return v, nil
}

// Path returns the absolute path to the Git executable.
func (g *Git) Path() string {
	return g.exe
}

// WithDir returns a new instance that is changed to use dir as its working directory.
// Any relative paths will be interpreted relative to g's working directory.
func (g *Git) WithDir(dir string) *Git {
	if filepath.IsAbs(dir) {
		dir = filepath.Clean(dir)
	} else {
		dir = filepath.Join(g.dir, dir)
	}
	g2 := new(Git)
	*g2 = *g
	g2.dir = dir
	return g2
}

const (
	dataOutputLimit  = 10 << 20 // 10 MiB
	errorOutputLimit = 1 << 20  // 1 MiB
)

// Run runs Git with the given arguments. If an error occurs, the
// combined stdout and stderr will be returned in the error.
func (g *Git) Run(ctx context.Context, args ...string) error {
	return g.run(ctx, errorSubject(args), append([]string{g.exe}, args...))
}

// run runs the specified Git subcommand.  If an error occurs, the
// combined stdout and stderr will be returned in the error. argv is
// interpreted the same as in command(). run will use the given error
// prefix instead of one derived from the arguments.
func (g *Git) run(ctx context.Context, errPrefix string, argv []string) error {
	c := g.command(ctx, argv)
	output := new(bytes.Buffer)
	c.Stderr = &limitWriter{w: output, n: errorOutputLimit}
	c.Stdout = c.Stderr
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError(errPrefix, err, output.Bytes())
	}
	return nil
}

// Output runs Git with the given arguments and returns its stdout.
func (g *Git) Output(ctx context.Context, args ...string) (string, error) {
	return g.output(ctx, errorSubject(args), append([]string{g.exe}, args...))
}

// output runs the specified Git subcommand, returning its stdout.
// argv is interpreted the same as in command().
// output will use the given error prefix instead of one derived from the arguments.
func (g *Git) output(ctx context.Context, errPrefix string, argv []string) (string, error) {
	c := g.command(ctx, argv)
	stdout := new(strings.Builder)
	c.Stdout = &limitWriter{w: stdout, n: dataOutputLimit}
	stderr := new(bytes.Buffer)
	c.Stderr = &limitWriter{w: stderr, n: errorOutputLimit}
	if err := sigterm.Run(ctx, c); err != nil {
		return stdout.String(), commandError(errPrefix, err, stderr.Bytes())
	}
	return stdout.String(), nil
}

// oneLine verifies that s contains a single line delimited by '\n' and
// trims the trailing '\n'.
func oneLine(s string) (string, error) {
	if s == "" {
		return "", io.EOF
	}
	i := strings.IndexByte(s, '\n')
	if i == -1 {
		return "", io.ErrUnexpectedEOF
	}
	if i < len(s)-1 {
		return "", xerrors.New("multiple lines present")
	}
	return s[:len(s)-1], nil
}

func errorSubject(args []string) string {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") && (i == 0 || args[i-1] != "-c") {
			return "git " + a
		}
	}
	return "git"
}

type limitWriter struct {
	w io.Writer
	n int64
}

func (lw *limitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > lw.n {
		n, err := lw.w.Write(p[:int(lw.n)])
		lw.n -= int64(n)
		if err != nil {
			return n, err
		}
		return n, xerrors.New("buffer full")
	}
	n, err := lw.w.Write(p)
	lw.n -= int64(n)
	return n, err
}
