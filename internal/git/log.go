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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gg-scm.io/pkg/internal/sigterm"
)

// CommitInfo stores information about a single commit.
type CommitInfo struct {
	Hash       Hash
	Parents    []Hash
	Author     User
	Committer  User
	AuthorTime time.Time
	CommitTime time.Time
	Message    string
}

// User identifies an author or committer.
type User struct {
	// Name is the user's full name.
	Name string
	// Email is the user's email address.
	Email string
}

// String returns the user information as a string in the
// form "User Name <foo@example.com>".
func (u User) String() string {
	return fmt.Sprintf("%s <%s>", u.Name, u.Email)
}

// CommitInfo obtains information about a single commit.
func (g *Git) CommitInfo(ctx context.Context, rev string) (*CommitInfo, error) {
	errPrefix := fmt.Sprintf("git log %q", rev)
	if err := validateRev(rev); err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	if strings.HasPrefix(rev, "^") {
		return nil, fmt.Errorf("%s: revision cannot be an exclusion", errPrefix)
	}
	if strings.Contains(rev, "..") {
		return nil, fmt.Errorf("%s: revision cannot be a range", errPrefix)
	}
	if strings.HasSuffix(rev, "^@") {
		return nil, fmt.Errorf("%s: revision cannot use parent shorthand", errPrefix)
	}

	out, err := g.output(ctx, errPrefix, []string{
		g.exe,
		"log",
		"--max-count=1",
		"-z",
		"--pretty=tformat:%H%x00%P%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI%x00%B",
		rev,
		"--",
	})
	if err != nil {
		return nil, err
	}
	info, err := parseCommitInfo(out)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	return info, nil
}

const (
	commitInfoPrettyFormat = "tformat:%H%x00%P%x00%an%x00%ae%x00%aI%x00%cn%x00%ce%x00%cI%x00%B"
	commitInfoFieldCount   = 9
)

func parseCommitInfo(out string) (*CommitInfo, error) {
	if !strings.HasSuffix(out, "\x00") {
		return nil, errors.New("parse commit: invalid format")
	}
	fields := strings.Split(out[:len(out)-1], "\x00")
	if len(fields) != commitInfoFieldCount {
		return nil, errors.New("parse commit: invalid format")
	}
	hash, err := ParseHash(fields[0])
	if err != nil {
		return nil, fmt.Errorf("parse commit: hash: %v", err)
	}

	var parents []Hash
	if parentStrings := strings.Fields(fields[1]); len(parentStrings) > 0 {
		parents = make([]Hash, 0, len(parentStrings))
		for _, s := range parentStrings {
			p, err := ParseHash(s)
			if err != nil {
				return nil, fmt.Errorf("parse commit: parents: %v", err)
			}
			parents = append(parents, p)
		}
	}
	authorTime, err := time.Parse(time.RFC3339, fields[4])
	if err != nil {
		return nil, fmt.Errorf("parse commit: author time: %v", err)
	}
	commitTime, err := time.Parse(time.RFC3339, fields[7])
	if err != nil {
		return nil, fmt.Errorf("parse commit: commit time: %v", err)
	}
	return &CommitInfo{
		Hash:    hash,
		Parents: parents,
		Author: User{
			Name:  fields[2],
			Email: fields[3],
		},
		Committer: User{
			Name:  fields[5],
			Email: fields[6],
		},
		AuthorTime: authorTime,
		CommitTime: commitTime,
		Message:    fields[8],
	}, nil
}

// LogOptions specifies filters and ordering on a log listing.
type LogOptions struct {
	// Revs specifies the set of commits to list. When empty, it defaults
	// to all commits reachable from HEAD.
	Revs []string

	// MaxParents sets an inclusive upper limit on the number of parents
	// on revisions to return from Log. If MaxParents is zero, then it is
	// treated as no limit unless AllowZeroMaxParents is true.
	MaxParents          int
	AllowZeroMaxParents bool

	// If FirstParent is true, then follow only the first parent commit
	// upon seeing a merge commit.
	FirstParent bool

	// Limit specifies the upper bound on the number of revisions to return from Log.
	// Zero means no limit.
	Limit int

	// If Reverse is true, then commits will be returned in reverse order.
	Reverse bool
}

// Log starts fetching information about a set of commits. The context's
// deadline and cancelation will apply to the entire read from the Log.
func (g *Git) Log(ctx context.Context, opts LogOptions) (*Log, error) {
	// TODO(someday): Add an example for this method.

	const errPrefix = "git log"
	for _, rev := range opts.Revs {
		if err := validateRev(rev); err != nil {
			return nil, fmt.Errorf("%s: %v", errPrefix, err)
		}
	}
	args := []string{g.exe, "log", "-z", "--pretty=" + commitInfoPrettyFormat}
	if opts.MaxParents > 0 || opts.AllowZeroMaxParents {
		args = append(args, fmt.Sprintf("--max-parents=%d", opts.MaxParents))
	}
	if opts.FirstParent {
		args = append(args, "--first-parent")
	}
	if opts.Limit > 0 {
		args = append(args, fmt.Sprintf("--max-count=%d", opts.Limit))
	}
	if opts.Reverse {
		args = append(args, "--reverse")
	}
	args = append(args, opts.Revs...)
	args = append(args, "--")
	c := g.command(ctx, args)
	pipe, err := c.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	stderr := new(bytes.Buffer)
	c.Stderr = &limitWriter{w: stderr, n: 4096}
	ctx, cancel := context.WithCancel(ctx)
	wait, err := sigterm.Start(ctx, c)
	if err != nil {
		cancel()
		pipe.Close()
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	r := bufio.NewReaderSize(pipe, 1<<20 /* 1 MiB */)
	if _, err := r.Peek(1); err != nil && err != io.EOF {
		cancel()
		waitErr := wait()
		return nil, commandError(errPrefix, waitErr, stderr.Bytes())
	}
	return &Log{
		r:      r,
		cancel: cancel,
		wait:   wait,
	}, nil
}

// Log is an open handle to a `git log` subprocess. Closing the Log
// stops the subprocess.
type Log struct {
	r      *bufio.Reader
	cancel context.CancelFunc
	wait   func() error

	scanErr  error
	scanDone bool
	info     *CommitInfo
}

// Next attempts to scan the next log entry and returns whether there is a new entry.
func (l *Log) Next() bool {
	if l.scanDone {
		return false
	}

	// Continue growing buffer until we've fit a log entry.
	end := -1
	for n := l.r.Buffered(); n < l.r.Size(); {
		data, err := l.r.Peek(n)
		end = findCommitInfoEnd(data)
		if end != -1 {
			break
		}
		if err != nil {
			switch {
			case err == io.EOF && l.r.Buffered() == 0:
				l.abort(nil)
			case err == io.EOF && l.r.Buffered() > 0:
				l.abort(io.ErrUnexpectedEOF)
			default:
				l.abort(err)
			}
			return false
		}
		if l.r.Buffered() > n {
			n = l.r.Buffered()
		} else {
			n++
		}
	}
	if end == -1 {
		l.abort(bufio.ErrBufferFull)
		return false
	}

	// Parse entry.
	data, err := l.r.Peek(end)
	if err != nil {
		// Should already be buffered.
		panic(err)
	}
	info, err := parseCommitInfo(string(data))
	if err != nil {
		l.abort(err)
		return false
	}
	if _, err := l.r.Discard(end); err != nil {
		// Should already be buffered.
		panic(err)
	}
	l.info = info
	return true
}

func (l *Log) abort(e error) {
	l.r = nil
	l.scanErr = e
	l.scanDone = true
	l.info = nil
	l.cancel()
}

func findCommitInfoEnd(b []byte) int {
	nuls := 0
	for i := range b {
		if b[i] != 0 {
			continue
		}
		nuls++
		if nuls == commitInfoFieldCount {
			return i + 1
		}
	}
	return -1
}

// CommitInfo returns the most recently scanned log entry.
// Next must be called at least once before calling CommitInfo.
func (l *Log) CommitInfo() *CommitInfo {
	return l.info
}

// Close ends the log subprocess and waits for it to finish.
// Close returns an error if Next returned false due to a parse failure.
func (l *Log) Close() error {
	// Not safe to call multiple times, but interface for Close doesn't
	// require this to be supported.

	l.cancel()
	l.wait() // Ignore error, since it's probably from interrupting.
	return l.scanErr
}
