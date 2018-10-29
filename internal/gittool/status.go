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
type StatusReader struct {
	p         *Process
	r         *bufio.Reader
	cancel    context.CancelFunc
	renameBug bool

	scanned bool
	ent     StatusEntry
	err     error
}

// StatusOptions specifies the command-line arguments for `git status`.
type StatusOptions struct {
	// IncludeIgnored specifies whether to emit ignored files.
	IncludeIgnored bool
	// DisableRenames will force Git to disable rename/copy detection.
	DisableRenames bool
	// Pathspecs filters the output to the given pathspecs.
	Pathspecs []Pathspec
}

// Status starts a `git status` subprocess.
func Status(ctx context.Context, git *Tool, opts StatusOptions) (*StatusReader, error) {
	renameBug := false
	if version, err := git.getVersion(ctx); err == nil && affectedByStatusRenameBug(version) {
		renameBug = true
	}
	ctx, cancel := context.WithCancel(ctx)
	args := make([]string, 0, 8+len(opts.Pathspecs))
	if opts.DisableRenames {
		args = append(args, "-c", "status.renames=false")
	}
	args = append(args, "status", "--porcelain", "-z", "-unormal")
	if opts.IncludeIgnored {
		args = append(args, "--ignored")
	}
	if len(opts.Pathspecs) > 0 {
		args = append(args, "--")
		for _, spec := range opts.Pathspecs {
			args = append(args, string(spec))
		}
	}
	p, err := git.Start(ctx, args...)
	if err != nil {
		return nil, err
	}
	return &StatusReader{
		p:         p,
		r:         bufio.NewReader(p),
		cancel:    cancel,
		renameBug: renameBug,
	}, nil
}

// affectedByStatusRenameBug reports whether `git status --porcelain`
// emits incorrect output for locally renamed files.
//
// In the affected versions, Git will only list the missing source file,
// not the new added file. See https://github.com/zombiezen/gg/issues/60
// for a full explanation.
func affectedByStatusRenameBug(version string) bool {
	prefixes := []string{
		"git version 2.11",
		"git version 2.12",
		"git version 2.13",
		"git version 2.14",
		"git version 2.15",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(version, p) && (len(version) == len(p) || version[len(p)] == '.') {
			return true
		}
	}
	return false
}

// Scan reads the next entry in the status output.
func (sr *StatusReader) Scan() bool {
	sr.err = readStatusEntry(&sr.ent, sr.r, sr.renameBug)
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
	name TopPath
	from TopPath
}

func readStatusEntry(out *StatusEntry, r io.ByteReader, renameBug bool) error {
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
	name, err := readString(r, 2048)
	if err != nil {
		return fmt.Errorf("read status entry: %v", err)
	}
	out.name = TopPath(name)
	if renameBug && out.code[0] == ' ' && out.code[1] == 'R' {
		// See doc for affectedByStatusRenameBug for explanation.
		out.from = out.name
		out.name = ""
		return nil
	}
	if out.code[0] == 'R' || out.code[0] == 'C' || out.code[1] == 'R' || out.code[1] == 'C' {
		from, err := readString(r, 2048)
		if err != nil {
			return fmt.Errorf("read status entry: %v", err)
		}
		out.from = TopPath(from)
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
		return ent.code.String() + " " + ent.from.String() + " -> " + ent.name.String()
	}
	return ent.code.String() + " " + ent.name.String()
}

// Code returns the two-letter code from the git status short format.
//
// More details in the Output section of git-status(1).
func (ent *StatusEntry) Code() StatusCode {
	return ent.code
}

// Name returns the path of the file.
func (ent *StatusEntry) Name() TopPath {
	return ent.name
}

// From returns the path of the file that this file was renamed or
// copied from, otherwise an empty string.
func (ent *StatusEntry) From() TopPath {
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

// IsIgnored returns true if the file is being ignored by Git.
func (code StatusCode) IsIgnored() bool {
	return code[0] == '!' && code[1] == '!'
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

// DiffStatusReader is a handle to a running `git diff --name-status`
// command.
//
// See https://git-scm.com/docs/git-diff#git-diff---name-status for more
// details.
type DiffStatusReader struct {
	p      *Process
	r      *bufio.Reader
	cancel context.CancelFunc

	scanned bool
	ent     DiffStatusEntry
	err     error
}

// DiffStatusOptions specifies the command-line arguments for `git diff --status`.
type DiffStatusOptions struct {
	// Commit1 specifies the earlier commit to compare with. If empty,
	// then DiffStatus compares against the index.
	Commit1 string
	// Commit2 specifies the later commit to compare with. If empty, then
	// DiffStatus compares against the working tree. Callers must not set
	// Commit2 if Commit1 is empty.
	Commit2 string
	// Pathspecs filters the output to the given pathspecs.
	Pathspecs []Pathspec
	// DisableRenames will force Git to disable rename/copy detection.
	DisableRenames bool
}

// DiffStatus compares the working copy with a commit,
// optionally restricting to the given pathspec.
func DiffStatus(ctx context.Context, git *Tool, opts DiffStatusOptions) (*DiffStatusReader, error) {
	if opts.Commit1 == "" && opts.Commit2 != "" {
		panic("Commit2 set without Commit1 being set")
	}
	if strings.HasPrefix(opts.Commit1, "-") {
		return nil, fmt.Errorf("diff status: commit %q should not start with '-'", opts.Commit1)
	}
	if strings.HasPrefix(opts.Commit2, "-") {
		return nil, fmt.Errorf("diff status: commit %q should not start with '-'", opts.Commit2)
	}
	ctx, cancel := context.WithCancel(ctx)
	args := make([]string, 0, 6+len(opts.Pathspecs))
	args = append(args, "diff", "--name-status", "-z")
	if opts.DisableRenames {
		args = append(args, "--no-renames")
	}
	if opts.Commit1 != "" {
		args = append(args, opts.Commit1)
	}
	if opts.Commit2 != "" {
		args = append(args, opts.Commit2)
	}
	if len(opts.Pathspecs) > 0 {
		args = append(args, "--")
		for _, p := range opts.Pathspecs {
			args = append(args, string(p))
		}
	}
	p, err := git.Start(ctx, args...)
	if err != nil {
		return nil, err
	}
	return &DiffStatusReader{
		p:      p,
		r:      bufio.NewReader(p),
		cancel: cancel,
	}, nil
}

// Scan reads the next entry in the diff output.
func (dr *DiffStatusReader) Scan() bool {
	dr.err = readDiffStatusEntry(&dr.ent, dr.r)
	if dr.err != nil {
		return false
	}
	dr.scanned = true
	return true
}

// Err returns the first non-EOF error encountered during Scan.
func (dr *DiffStatusReader) Err() error {
	if dr.err == io.EOF {
		return nil
	}
	return dr.err
}

// Entry returns the most recent entry parsed by a call to Scan.
// The pointer may point to data that will be overwritten by a
// subsequent call to Scan.
func (dr *DiffStatusReader) Entry() *DiffStatusEntry {
	if !dr.scanned || dr.err != nil {
		return nil
	}
	return &dr.ent
}

// Close finishes reading from the Git subprocess and waits for it to
// terminate. The behavior of calling methods on a DiffStatusReader
// after Close is undefined.
//
// If the subprocess exited due to a signal, Close will not return an
// error, as it usually means that Close terminated the process. In the
// case that another signal terminated the subprocess, this usually
// results in a scan error.
func (dr *DiffStatusReader) Close() error {
	dr.cancel()
	err := dr.p.Wait()
	*dr = DiffStatusReader{}
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

// A DiffStatusEntry describes the state of a single file in a diff.
type DiffStatusEntry struct {
	code DiffStatusCode
	name TopPath
}

func readDiffStatusEntry(out *DiffStatusEntry, r io.ByteReader) error {
	// Read status code.
	code, err := r.ReadByte()
	if err == io.EOF {
		return io.EOF
	}
	if err != nil {
		return fmt.Errorf("read diff entry: %v", err)
	}
	out.code = DiffStatusCode(code)
	hasFrom := out.code == DiffStatusRenamed || out.code == DiffStatusCopied

	// Read NUL.
	if hasFrom {
		foundNul := false
		for i := 0; i < 3; i++ {
			nul, err := r.ReadByte()
			if err != nil {
				return fmt.Errorf("read diff entry: %v", dontExpectEOF(err))
			}
			if nul == 0 {
				foundNul = true
				break
			}
		}
		if !foundNul {
			return errors.New("read diff entry: expected '\\x00' after 'R' or 'C', but not found")
		}
	} else {
		nul, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("read diff entry: %v", dontExpectEOF(err))
		}
		if nul != 0 {
			return fmt.Errorf("read diff entry: expected '\\x00', got %q", nul)
		}
	}

	// Read name.
	if hasFrom {
		// TODO(someday): Persist this value.
		if _, err := readString(r, 2048); err != nil {
			return fmt.Errorf("read diff entry: %v", err)
		}
	}
	name, err := readString(r, 2048)
	if err != nil {
		return fmt.Errorf("read diff entry: %v", err)
	}
	out.name = TopPath(name)

	// Check code validity at very end in order to consume as much as possible.
	if !out.code.isValid() {
		return fmt.Errorf("read diff entry: invalid code %v", out.code)
	}
	return nil
}

// Code returns the letter code from the entry.
func (ent *DiffStatusEntry) Code() DiffStatusCode {
	return ent.code
}

// Name returns the path of the file.
func (ent *DiffStatusEntry) Name() TopPath {
	return ent.name
}

// DiffStatusCode is a single-letter code from the `git diff --name-status` format.
//
// See https://git-scm.com/docs/git-diff#git-diff---diff-filterACDMRTUXB82308203
// for a description of each of the codes.
type DiffStatusCode byte

// Diff status codes.
const (
	DiffStatusAdded       DiffStatusCode = 'A'
	DiffStatusCopied      DiffStatusCode = 'C'
	DiffStatusDeleted     DiffStatusCode = 'D'
	DiffStatusModified    DiffStatusCode = 'M'
	DiffStatusRenamed     DiffStatusCode = 'R'
	DiffStatusChangedMode DiffStatusCode = 'T'
	DiffStatusUnmerged    DiffStatusCode = 'U'
	DiffStatusUnknown     DiffStatusCode = 'X'
	DiffStatusBroken      DiffStatusCode = 'B'
)

func (code DiffStatusCode) isValid() bool {
	return code == DiffStatusAdded ||
		code == DiffStatusCopied ||
		code == DiffStatusDeleted ||
		code == DiffStatusModified ||
		code == DiffStatusRenamed ||
		code == DiffStatusChangedMode ||
		code == DiffStatusUnmerged ||
		code == DiffStatusUnknown ||
		code == DiffStatusBroken
}

// String returns the code letter as a string.
func (code DiffStatusCode) String() string {
	return string(code)
}

func dontExpectEOF(e error) error {
	if e == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return e
}
