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

package gittool

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// StatusReader is a handle to a running `git status` command.
// It does not return clean or ignored files.
type StatusReader struct {
	p      *Process
	r      *bufio.Reader
	cancel context.CancelFunc

	scanned bool
	ent     StatusEntry
	err     error
}

// Status starts a `git status` subprocess.
func Status(ctx context.Context, git *Tool, args []string) (*StatusReader, error) {
	ctx, cancel := context.WithCancel(ctx)
	gitArgs := make([]string, 0, 5+len(args))
	gitArgs = append(gitArgs, "status", "--porcelain", "-z", "-unormal", "--")
	gitArgs = append(gitArgs, args...)
	p, err := git.Start(ctx, gitArgs...)
	if err != nil {
		return nil, err
	}
	return &StatusReader{
		p:      p,
		r:      bufio.NewReader(p),
		cancel: cancel,
	}, nil
}

// Scan reads the next entry in the status output.
func (sr *StatusReader) Scan() bool {
	sr.err = readStatusEntry(&sr.ent, sr.r)
	if sr.err != nil {
		return false
	}
	sr.scanned = true
	return true
}

// Err returns the first non-EOF error encountered during Scan.
func (sr *StatusReader) Err() error {
	if sr.err == io.EOF {
		return nil
	}
	return sr.err
}

// Entry returns the most recent entry parsed by a call to Scan.
// The pointer may point to data that will be overwritten by a
// subsequent call to Scan.
func (sr *StatusReader) Entry() *StatusEntry {
	if !sr.scanned || sr.err != nil {
		return nil
	}
	return &sr.ent
}

// Close finishes reading from the Git subprocess and waits for it to
// terminate. The behavior of calling methods on a StatusReader after
// Close is undefined.
//
// If the subprocess exited due to a signal, Close will not return an
// error, as it usually means that Close terminated the process. In the
// case that another signal terminated the subprocess, this usually
// results in a scan error.
func (sr *StatusReader) Close() error {
	sr.cancel()
	err := sr.p.Wait()
	*sr = StatusReader{}
	switch err := err.(type) {
	case nil:
		return nil
	case *exitError:
		if err.signaled {
			return nil
		}
		return err
	default:
		return err
	}
}

// A StatusEntry describes the state of a single file in the working copy.
type StatusEntry struct {
	code StatusCode
	name string
	from string
}

func readStatusEntry(out *StatusEntry, r io.ByteReader) error {
	var err error
	// Read status code.
	out.code[0], err = r.ReadByte()
	if err == io.EOF {
		return err
	}
	if err != nil {
		return fmt.Errorf("read status entry: %v", err)
	}
	out.code[1], err = r.ReadByte()
	if err != nil {
		return fmt.Errorf("read status entry: %v", dontExpectEOF(err))
	}

	// Read space.
	sp, err := r.ReadByte()
	if err != nil {
		return fmt.Errorf("read status entry: %v", dontExpectEOF(err))
	}
	if sp != ' ' {
		return fmt.Errorf("read status entry: expected ' ', got %q", sp)
	}

	// Read name and from.
	out.name, err = readString(r, 2048)
	if err != nil {
		return fmt.Errorf("read status entry: %v", err)
	}
	if out.code[0] == 'R' || out.code[0] == 'C' || out.code[1] == 'R' || out.code[1] == 'C' {
		out.from, err = readString(r, 2048)
		if err != nil {
			return fmt.Errorf("read status entry: %v", err)
		}
	}

	// Check code validity at very end in order to consume as much as possible.
	if !out.code.isValid() {
		return fmt.Errorf("read status entry: invalid code %q %q", out.code[0], out.code[1])
	}
	return nil
}

// readString reads a NUL-terminated string from r.
func readString(r io.ByteReader, limit int) (string, error) {
	var sb strings.Builder
	for sb.Len() < limit {
		b, err := r.ReadByte()
		if err != nil {
			return "", dontExpectEOF(err)
		}
		if b == 0 {
			return sb.String(), nil
		}
		sb.WriteByte(b)
	}
	b, err := r.ReadByte()
	if err != nil {
		return "", dontExpectEOF(err)
	}
	if b != 0 {
		return "", errors.New("string too long")
	}
	return sb.String(), nil
}

// String returns the entry in short format.
func (ent *StatusEntry) String() string {
	if ent.from != "" {
		return ent.code.String() + " " + ent.from + " -> " + ent.name
	}
	return ent.code.String() + " " + ent.name
}

// Code returns the two-letter code from the git status short format.
//
// More details in the Output section of git-status(1).
func (ent *StatusEntry) Code() StatusCode {
	return ent.code
}

// Name returns the path of the file. The path will always be relative
// to the top of the repository.
func (ent *StatusEntry) Name() string {
	return ent.name
}

// From returns the path of the file that this file was renamed or
// copied from, otherwise an empty string. The path will always be
/// relative to the top of the repository.
func (ent *StatusEntry) From() string {
	return ent.from
}

// A StatusCode is a two-letter code from the `git status` short format.
// For paths with no merge conflicts, the first letter is the status of
// the index and the second letter is the status of the work tree.
//
// More details at https://git-scm.com/docs/git-status#_short_format
type StatusCode [2]byte

// String returns the code's bytes as a string.
func (code StatusCode) String() string {
	return string(code[:])
}

// IsMissing reports whether the file has been deleted in the work tree.
func (code StatusCode) IsMissing() bool {
	return code[1] == 'D'
}

// IsModified reports whether the file has been modified in either the
// index or the work tree.
func (code StatusCode) IsModified() bool {
	return code[0] == 'M' && code[1] == ' ' ||
		code[0] == ' ' && code[1] == 'M' ||
		code[0] == 'M' && code[1] == 'M'
}

// IsRemoved reports whether the file has been deleted in the index.
func (code StatusCode) IsRemoved() bool {
	return code[0] == 'D' && code[1] == ' '
}

// IsRenamed reports whether the file is the result of a rename.
func (code StatusCode) IsRenamed() bool {
	return code[0] == 'R' && (code[1] == ' ' || code[1] == 'M')
}

// IsOriginalMissing reports whether the file has been detected as a
// rename in the work tree, but neither this file or its original have
// been updated in the index. If IsOriginalMissing is true, then IsAdded
// returns true.
func (code StatusCode) IsOriginalMissing() bool {
	return code[0] == ' ' && code[1] == 'R'
}

// IsCopied reports whether the file has been copied from elsewhere.
func (code StatusCode) IsCopied() bool {
	return code[0] == 'C' && (code[1] == ' ' || code[1] == 'M') ||
		// TODO(someday): Is this even possible?
		code[0] == ' ' && code[1] == 'C'
}

// IsAdded reports whether the file is new to the index (including
// copies, but not renames).
func (code StatusCode) IsAdded() bool {
	return code[0] == 'A' && (code[1] == ' ' || code[1] == 'M') ||
		code[0] == ' ' && code[1] == 'A' ||
		code.IsOriginalMissing() ||
		code.IsCopied()
}

// IsUntracked returns true if the file is not being tracked by Git.
func (code StatusCode) IsUntracked() bool {
	return code[0] == '?' && code[1] == '?'
}

// IsUnmerged reports whether the file has unresolved merge conflicts.
func (code StatusCode) IsUnmerged() bool {
	return code[0] == 'D' && code[1] == 'D' ||
		code[0] == 'A' && code[1] == 'U' ||
		code[0] == 'U' && code[1] == 'D' ||
		code[0] == 'U' && code[1] == 'A' ||
		code[0] == 'D' && code[1] == 'U' ||
		code[0] == 'A' && code[1] == 'A' ||
		code[0] == 'U' && code[1] == 'U'
}

func (code StatusCode) isValid() bool {
	const codes = "??!!" +
		" M D A R" +
		"M MMMD" +
		"A AMAD" +
		"D " +
		"R RMRD" +
		"C CMCD" +
		"DDAUUDUADUAAUU"
	for i := 0; i < len(codes); i += 2 {
		if code[0] == codes[i] && code[1] == codes[i+1] {
			return true
		}
	}
	return false
}

func dontExpectEOF(e error) error {
	if e == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return e
}
