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

//+build darwin dragonfly freebsd linux netbsd openbsd plan9 solaris

package gittool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
)

// Color returns the ANSI escape sequence for the given configuration
// setting.
func Color(ctx context.Context, git *Tool, name string, default_ string) ([]byte, error) {
	args := []string{"config", "--get-color", name}
	if default_ != "" {
		args = append(args, default_)
	}
	p, err := git.Start(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("get git color %s: %v", name, err)
	}
	seq, err := ioutil.ReadAll(&limitedReader{p, 1024})
	waitErr := p.Wait()
	if waitErr != nil {
		// An exit error is usually more interesting than I/O.
		return nil, fmt.Errorf("get git color %s: %v", name, err)
	}
	if err != nil {
		return nil, fmt.Errorf("get git color %s: %v", name, err)
	}
	return seq, nil
}

// ColorBool finds the color configuration setting is true or false.
// isTerm indicates whether the eventual output will be a terminal.
func ColorBool(ctx context.Context, git *Tool, name string, isTerm bool) (bool, error) {
	args := []string{"config", "--get-colorbool", name}
	if isTerm {
		args = append(args, "true")
	} else {
		args = append(args, "false")
	}
	out, err := git.RunOneLiner(ctx, '\n', args...)
	if err != nil {
		return false, fmt.Errorf("git config %s: %v", name, err)
	}
	switch {
	case bytes.Equal(out, []byte("true")):
		return true, nil
	case bytes.Equal(out, []byte("false")):
		return false, nil
	default:
		return false, fmt.Errorf("git config %s: expected true/false, got %q", name, out)
	}
}

type limitedReader struct {
	R io.Reader // underlying reader
	N int64     // max bytes remaining
}

func (l *limitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, errors.New("read limit reached")
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}
