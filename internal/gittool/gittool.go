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
	exe        string
	dir        string
	configHome string
	log        func(context.Context, []string)
}

// New returns a Tool that uses the given absolute path.
func New(path string) (*Tool, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path to git must be absolute (got %q)", path)
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
	return &Tool{exe: path}, nil
}

// Find searches for the git executable in PATH.
func Find() (*Tool, error) {
	exe, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("find git: %v", err)
	}
	return &Tool{exe: exe}, nil
}

func (t *Tool) cmd(ctx context.Context, args []string) *exec.Cmd {
	c := exec.CommandContext(ctx, t.exe, args...)
	c.Stderr = os.Stderr
	c.Dir = t.dir
	if t.configHome != "" {
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+t.configHome)
	}
	return c
}

// SetLogHook sets a function to be called at the start of every git
// subprocess.
func (t *Tool) SetLogHook(hook func(ctx context.Context, args []string)) {
	t.log = hook
}

// SetDir sets the tool's working directory.
func (t *Tool) SetDir(dir string) {
	t.dir = dir
}

// SetConfigHome instructs the tool to use global configuration from
// $HOME/.gitconfig and $HOME/.config/git/config.
//
// This is primarily useful for setting up hermetic git test cases.
func (t *Tool) SetConfigHome(home string) {
	t.configHome = home
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
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
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
