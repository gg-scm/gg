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

// Package gittool provides a high-level interface for interacting with
// a git subprocess.
package gittool

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
)

// Tool is an installed copy of git.
type Tool struct {
	exe string
	dir string

	env    []string
	log    func(context.Context, []string)
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// Options specifies optional parameters to New.
type Options struct {
	// LogHook is a function that will be called at the start of every git
	// subprocess.
	LogHook func(ctx context.Context, args []string)

	// Env specifies the environment of the subprocess.
	Env []string

	// Stderr will receive the stderr from the git subprocess.
	Stderr io.Writer

	// Stdin and Stdout are hooked up to the git subprocess during
	// RunInteractive.
	Stdin  io.Reader
	Stdout io.Writer
}

// New creates a new tool.
func New(path string, wd string, opts *Options) (*Tool, error) {
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

	t := &Tool{
		exe: path,
		dir: wd,
	}
	if opts != nil {
		t.log = opts.LogHook
		t.env = append([]string(nil), opts.Env...)
		t.stdin = opts.Stdin
		t.stdout = opts.Stdout
		t.stderr = opts.Stderr
	} else {
		t.env = []string{}
	}
	return t, nil
}

func (t *Tool) cmd(ctx context.Context, args []string) *exec.Cmd {
	c := exec.CommandContext(ctx, t.exe, args...)
	c.Env = t.env
	c.Stderr = t.stderr
	c.Dir = t.dir
	return c
}

// WithDir returns a new tool that is changed to use dir as its working directory.
func (t *Tool) WithDir(dir string) *Tool {
	t2 := new(Tool)
	*t2 = *t
	t2.dir = dir
	return t2
}

// Run starts the specified git subcommand and waits for it to finish.
//
// stderr will be sent to the current process's stderr.
func (t *Tool) Run(ctx context.Context, args ...string) error {
	if t.log != nil {
		t.log(ctx, args)
	}
	if err := t.cmd(ctx, args).Run(); err != nil {
		return wrapError(errorSubject(args), err)
	}
	return nil
}

// RunInteractive starts the specified git subcommand and waits for it
// to finish.  All standard streams will be attached to the
// corresponding streams of the current process.
func (t *Tool) RunInteractive(ctx context.Context, args ...string) error {
	c := t.cmd(ctx, args)
	c.Stdin = t.stdin
	c.Stdout = t.stdout
	if t.log != nil {
		t.log(ctx, args)
	}
	if err := c.Run(); err != nil {
		return wrapError(errorSubject(args), err)
	}
	return nil
}

// RunOneLiner starts the specified git subcommand, reads a single
// "line" delimited by the given byte from stdout, and waits for it to
// finish.
//
// RunOneLiner will return (nil, nil) iff the output is entirely empty.
// Any data after the first occurrence of the delimiter byte will be
// considered an error.
func (t *Tool) RunOneLiner(ctx context.Context, delim byte, args ...string) ([]byte, error) {
	const max = 4096
	p, err := t.Start(ctx, args...)
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

// Start starts the specified git subcommand and pipes its stdout.
//
// stderr will be sent to the current process's stderr.
func (t *Tool) Start(ctx context.Context, args ...string) (*Process, error) {
	c := t.cmd(ctx, args)
	rc, err := c.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("run %s: %v", errorSubject(args), err)
	}
	if t.log != nil {
		t.log(ctx, args)
	}
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("run %s: %v", errorSubject(args), err)
	}
	return &Process{
		cmd:     c,
		pipe:    rc,
		subject: errorSubject(args),
	}, nil
}

// Config reads the string value of a git configuration variable.
func Config(ctx context.Context, git *Tool, name string) (string, error) {
	line, err := git.RunOneLiner(ctx, 0, "config", "-z", "--get", "--", name)
	if err != nil {
		return "", err
	}
	return string(line), nil
}

// Process is a running git subprocess that can be read from.
type Process struct {
	cmd     *exec.Cmd
	pipe    io.ReadCloser
	subject string
}

// Read reads from the process's stdout.
func (p *Process) Read(b []byte) (int, error) {
	return p.pipe.Read(b)
}

// Wait waits for the git subprocess to exit and consumes any remaining
// data from the subprocess's stdout.
func (p *Process) Wait() error {
	io.Copy(ioutil.Discard, p.pipe)
	p.pipe.Close()
	if err := p.cmd.Wait(); err != nil {
		return wrapError(p.subject, err)
	}
	return nil
}

type exitError string

func wrapError(subject string, e error) error {
	msg := fmt.Sprintf("run %s: %v", subject, e)
	if _, ok := e.(*exec.ExitError); ok {
		return (*exitError)(&msg)
	}
	return errors.New(msg)
}

// IsExitError reports whether e indicates an unsuccessful exit by a
// git command.
func IsExitError(e error) bool {
	_, ok := e.(*exitError)
	return ok
}

func (ee *exitError) Error() string {
	return string(*ee)
}

func errorSubject(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return "git " + a
		}
	}
	return "git"
}
