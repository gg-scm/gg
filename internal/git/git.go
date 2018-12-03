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

// Package git provides a high-level interface for interacting with
// a Git subprocess.
package git // import "gg-scm.io/pkg/internal/git"

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gg-scm.io/pkg/internal/sigterm"
)

// Git is a context for performing Git version control operations.
// Broadly, it consists of a path to an installed copy of Git and a
// working directory path.
type Git struct {
	exe string
	dir string

	env    []string
	log    func(context.Context, []string)
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

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
	Env []string

	// Stderr will receive the stderr from the Git subprocess.
	Stderr io.Writer

	// Stdin and Stdout are hooked up to the Git subprocess during
	// RunInteractive.
	Stdin  io.Reader
	Stdout io.Writer
}

// New creates a new Git context.
func New(path string, wd string, opts *Options) (*Git, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path to git must be absolute (got %q)", path)
	}
	if wd == "" {
		return nil, errors.New("init git: working directory must not be blank")
	}

	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat git: %v", err)
	}
	m := info.Mode()
	if m.IsDir() || m&0111 == 0 {
		return nil, fmt.Errorf("stat git: not an executable file")
	}

	wd, err = filepath.Abs(wd)
	if err != nil {
		return nil, fmt.Errorf("init git: resolve working directory: %v", err)
	}

	g := &Git{
		exe: path,
		dir: wd,
	}
	if opts != nil {
		g.log = opts.LogHook
		g.env = append([]string(nil), opts.Env...)
		g.stdin = opts.Stdin
		g.stdout = opts.Stdout
		g.stderr = opts.Stderr
	} else {
		g.env = []string{}
	}
	return g, nil
}

// Command creates a new *exec.Cmd that will invoke Git with the given
// arguments. The returned command does not obey the given Context's deadline
// or cancelation.
func (g *Git) Command(ctx context.Context, args ...string) *exec.Cmd {
	if g.log != nil {
		g.log(ctx, args)
	}
	c := exec.Command(g.exe, args...)
	c.Env = g.env
	c.Dir = g.dir
	return c
}

func (g *Git) cmd(ctx context.Context, args []string) *exec.Cmd {
	c := g.Command(ctx, args...)
	c.Stderr = g.stderr
	return c
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
			return "", wrapError("git --version", ctx.Err())
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
	args := []string{"--version"}
	c := g.cmd(ctx, args)
	sb := new(strings.Builder)
	c.Stdout = &limitWriter{w: sb, n: 4096}
	if err := sigterm.Run(ctx, c); err != nil {
		g.versionMu.Lock()
		close(g.versionCond)
		g.versionCond = nil
		g.versionMu.Unlock()
		return "", wrapError("git --version", err)
	}
	v := sb.String()

	g.versionMu.Lock()
	g.version = v
	close(g.versionCond)
	g.versionCond = nil
	g.versionMu.Unlock()
	return v, nil
}

// Path returns the absolute path to the Git executable.
func (g *Git) Path() string {
	return g.exe
}

// WithDir returns a new instance that is changed to use dir as its working directory.
func (g *Git) WithDir(dir string) *Git {
	g2 := new(Git)
	*g2 = *g
	g2.dir = dir
	return g2
}

// Run starts the specified Git subcommand and waits for it to finish.
//
// stderr will be sent to the writer specified in the tool's options.
// stdin and stdout will be connected to the null device.
func (g *Git) Run(ctx context.Context, args ...string) error {
	if err := sigterm.Run(ctx, g.cmd(ctx, args)); err != nil {
		return wrapError(errorSubject(args), err)
	}
	return nil
}

// Query starts the specified Git subcommand and waits for it to exit
// with code zero (returns true) or one (returns false).
//
// stderr will be buffered, being returned as part of the error if the
// tool does not exit with zero or one. stdin and stdout will be
// connected to the null device.
func (g *Git) Query(ctx context.Context, args ...string) (bool, error) {
	c := g.cmd(ctx, args)
	stderr := new(bytes.Buffer)
	c.Stderr = stderr
	if err := sigterm.Run(ctx, c); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return false, wrapError(errorSubject(args), err)
		}
		if exitStatus(exitErr.ProcessState) == 1 {
			return false, nil
		}
		var msg string
		if errOut := bytes.TrimRight(stderr.Bytes(), "\n"); len(errOut) > 0 {
			msg = fmt.Sprintf("run %s: %s (%v)", errorSubject(args), errOut, exitErr)
		} else {
			msg = fmt.Sprintf("run %s: %v", errorSubject(args), exitErr)
		}
		return false, &exitError{
			msg:      msg,
			signaled: wasSignaled(exitErr.ProcessState),
		}
	}
	return true, nil
}

// RunInteractive starts the specified Git subcommand and waits for it
// to finish.  All standard streams will be attached to the
// corresponding streams specified in the tool's options.
func (g *Git) RunInteractive(ctx context.Context, args ...string) error {
	c := g.cmd(ctx, args)
	c.Stdin = g.stdin
	c.Stdout = g.stdout
	if err := sigterm.Run(ctx, c); err != nil {
		return wrapError(errorSubject(args), err)
	}
	return nil
}

// RunOneLiner starts the specified Git subcommand, reads a single
// "line" delimited by the given byte from stdout, and waits for it to
// finish.
//
// RunOneLiner will return (nil, nil) iff the output is entirely empty.
// Any data after the first occurrence of the delimiter byte will be
// considered an error.
//
// stderr will be sent to the writer specified in the tool's options.
// stdin will be connected to the null device.
func (g *Git) RunOneLiner(ctx context.Context, delim byte, args ...string) ([]byte, error) {
	const max = 4096
	p, err := g.Start(ctx, args...)
	if err != nil {
		return nil, err
	}
	br := bufio.NewReaderSize(p, max)
	buf, peekErr := br.Peek(max)
	i := bytes.IndexByte(buf, delim)
	if i == -1 {
		if err := p.Wait(); IsExitError(err) {
			// Not finding the delimiter is probably due to a command failure.
			return nil, err
		}
		if len(buf) == 0 && peekErr == io.EOF {
			return nil, nil
		}
		if peekErr != nil {
			return nil, fmt.Errorf("run %s: %v", errorSubject(args), peekErr)
		}
		return nil, fmt.Errorf("run %s: delimiter not found in first %d bytes of output", errorSubject(args), max)
	}
	out := make([]byte, i)
	copy(out, buf)
	if _, err := br.Discard(i + 1); err != nil {
		panic(err)
	}
	_, overErr := br.ReadByte()
	waitErr := p.Wait()
	if overErr == nil {
		return nil, fmt.Errorf("run %s: found data past delimiter", errorSubject(args))
	}
	return out, waitErr
}

// Start starts the specified Git subcommand and pipes its stdout.
//
// stderr will be sent to the writer specified in the tool's options.
// stdin will be connected to the null device.
func (g *Git) Start(ctx context.Context, args ...string) (*Process, error) {
	c := g.cmd(ctx, args)
	rc, err := c.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("run %s: %v", errorSubject(args), err)
	}
	wait, err := sigterm.Start(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("run %s: %v", errorSubject(args), err)
	}
	return &Process{
		wait:    wait,
		pipe:    rc,
		subject: errorSubject(args),
	}, nil
}

// GitDir determines the absolute path of the ".git" directory given the
// tool's configuration, resolving any symlinks.
func (g *Git) GitDir(ctx context.Context) (string, error) {
	line, err := g.RunOneLiner(ctx, '\n', "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(string(line))
}

// WorkTree determines the absolute path of the root of the working
// tree given the tool's configuration, resolving any symlinks.
func (g *Git) WorkTree(ctx context.Context) (string, error) {
	line, err := g.RunOneLiner(ctx, '\n', "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(string(line))
}

// Process is a running Git subprocess that can be read from.
type Process struct {
	wait    func() error
	pipe    io.ReadCloser
	subject string
}

// Read reads from the process's stdout.
func (p *Process) Read(b []byte) (int, error) {
	return p.pipe.Read(b)
}

// Wait waits for the Git subprocess to exit and consumes any remaining
// data from the subprocess's stdout.
func (p *Process) Wait() error {
	io.Copy(ioutil.Discard, p.pipe)
	p.pipe.Close()
	if err := p.wait(); err != nil {
		return wrapError(p.subject, err)
	}
	return nil
}

type exitError struct {
	msg      string
	signaled bool // Terminated by signal.
}

func wrapError(subject string, e error) error {
	msg := fmt.Sprintf("run %s: %v", subject, e)
	if e, ok := e.(*exec.ExitError); ok {
		return &exitError{
			msg:      msg,
			signaled: wasSignaled(e.ProcessState),
		}
	}
	return errors.New(msg)
}

// IsExitError reports whether e indicates an unsuccessful exit by a
// Git command.
func IsExitError(e error) bool {
	_, ok := e.(*exitError)
	return ok
}

func (ee *exitError) Error() string {
	return ee.msg
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
		return n, errors.New("buffer full")
	}
	n, err := lw.w.Write(p)
	lw.n -= int64(n)
	return n, err
}
