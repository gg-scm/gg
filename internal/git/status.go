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

package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// StatusOptions specifies the command-line arguments for `git status`.
type StatusOptions struct {
	// IncludeIgnored specifies whether to emit ignored files.
	IncludeIgnored bool
	// DisableRenames will force Git to disable rename/copy detection.
	DisableRenames bool
	// Pathspecs filters the output to the given pathspecs.
	Pathspecs []Pathspec
}

// Status returns any differences the working copy has from the files at HEAD.
func (g *Git) Status(ctx context.Context, opts StatusOptions) ([]StatusEntry, error) {
	renameBug := false
	if version, err := g.getVersion(ctx); err == nil && affectedByStatusRenameBug(version) {
		renameBug = true
	}
	args := []string{g.exe}
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
	stdout, err := g.output(ctx, "git status", args)
	if err != nil {
		return nil, err
	}
	var entries []StatusEntry
	for len(stdout) > 0 {
		var ent StatusEntry
		var err error
		ent, stdout, err = readStatusEntry(stdout, renameBug)
		if err != nil {
			return entries, err
		}
		entries = append(entries, ent)
	}
	return entries, nil
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

// A StatusEntry describes the state of a single file in the working copy.
type StatusEntry struct {
	// Code is the two-letter code from the Git status short format.
	// More details in the Output section of git-status(1).
	Code StatusCode
	// Name is the path of the file.
	Name TopPath
	// From is the path of the file that this file was renamed or
	// copied from, otherwise an empty string.
	From TopPath
}

func readStatusEntry(data string, renameBug bool) (StatusEntry, string, error) {
	// Read status code and space.
	if len(data) == 0 {
		return StatusEntry{}, "", io.EOF
	}
	if len(data) < 4 { // 2 bytes + 1 space + 1 NUL
		return StatusEntry{}, data, errors.New("read status entry: unexpected EOF")
	}
	var ent StatusEntry
	copy(ent.Code[:], data)
	if data[2] != ' ' {
		return StatusEntry{}, data, fmt.Errorf("read status entry: expected ' ', got %q", data[2])
	}

	// Read name and from.
	i := strings.IndexByte(data[3:], 0)
	if i == -1 {
		return StatusEntry{}, "", errors.New("read status entry: unexpected EOF reading name")
	}
	ent.Name = TopPath(data[3 : 3+i])
	data = data[4+i:]
	if renameBug && ent.Code[0] == ' ' && ent.Code[1] == 'R' {
		// See doc for affectedByStatusRenameBug for explanation.
		ent.From = ent.Name
		ent.Name = ""
		return ent, data, nil
	}
	if ent.Code[0] == 'R' || ent.Code[0] == 'C' || ent.Code[1] == 'R' || ent.Code[1] == 'C' {
		i := strings.IndexByte(data, 0)
		if i == -1 {
			return StatusEntry{}, "", errors.New("read status entry: unexpected EOF reading from")
		}
		ent.From = TopPath(data[:i])
		data = data[i+1:]
	}

	// Check code validity at very end in order to consume as much as possible.
	if !ent.Code.isValid() {
		return StatusEntry{}, data, fmt.Errorf("read status entry: invalid code %q %q", ent.Code[0], ent.Code[1])
	}
	return ent, data, nil
}

// String returns the entry in short format.
func (ent StatusEntry) String() string {
	if ent.From != "" {
		return ent.Code.String() + " " + ent.From.String() + " -> " + ent.Name.String()
	}
	return ent.Code.String() + " " + ent.Name.String()
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

// DiffStatus compares the working copy with a commit using `git diff --name-status`.
//
// See https://git-scm.com/docs/git-diff#git-diff---name-status for more
// details.
func (g *Git) DiffStatus(ctx context.Context, opts DiffStatusOptions) ([]DiffStatusEntry, error) {
	if opts.Commit1 != "" {
		if err := validateRev(opts.Commit1); err != nil {
			return nil, fmt.Errorf("diff status: %w", err)
		}
	}
	if opts.Commit2 != "" {
		if opts.Commit1 == "" {
			return nil, errors.New("diff status: Commit2 set without Commit1 being set")
		}
		if err := validateRev(opts.Commit2); err != nil {
			return nil, fmt.Errorf("diff status: %w", err)
		}
	}
	args := []string{g.exe, "diff", "--name-status", "-z"}
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
	stdout, err := g.output(ctx, "diff status", args)
	if err != nil {
		return nil, err
	}
	var entries []DiffStatusEntry
	for len(stdout) > 0 {
		var ent DiffStatusEntry
		var err error
		ent, stdout, err = readDiffStatusEntry(stdout)
		if err != nil {
			return entries, err
		}
		entries = append(entries, ent)
	}
	return entries, nil
}

// A DiffStatusEntry describes the state of a single file in a diff.
type DiffStatusEntry struct {
	Code DiffStatusCode
	Name TopPath
}

func readDiffStatusEntry(data string) (DiffStatusEntry, string, error) {
	// Read status code.
	if len(data) == 0 {
		return DiffStatusEntry{}, "", io.EOF
	}
	if len(data) < 2 {
		return DiffStatusEntry{}, data, errors.New("read diff entry: unexpected EOF")
	}
	var ent DiffStatusEntry
	ent.Code = DiffStatusCode(data[0])
	hasFrom := ent.Code == DiffStatusRenamed || ent.Code == DiffStatusCopied

	// Read NUL.
	if hasFrom {
		foundNul := false
		for i := 1; i < 4 && i < len(data); i++ {
			if data[i] == 0 {
				foundNul = true
				data = data[i+1:]
				break
			}
		}
		if !foundNul {
			return DiffStatusEntry{}, data, errors.New("read diff entry: expected '\\x00' after 'R' or 'C', but not found")
		}
	} else {
		if data[1] != 0 {
			return DiffStatusEntry{}, data, fmt.Errorf("read diff entry: expected '\\x00', got %q", data[1])
		}
		data = data[2:]
	}

	// Read name.
	if hasFrom {
		i := strings.IndexByte(data, 0)
		if i == -1 {
			return DiffStatusEntry{}, "", errors.New("read diff entry: unexpected EOF")
		}
		// TODO(someday): Persist this value. Until then, just skip.
		data = data[i+1:]
	}
	i := strings.IndexByte(data, 0)
	if i == -1 {
		return DiffStatusEntry{}, "", errors.New("read diff entry: unexpected EOF")
	}
	ent.Name = TopPath(data[:i])
	data = data[i+1:]

	// Check code validity at very end in order to consume as much as possible.
	if !ent.Code.isValid() {
		return DiffStatusEntry{}, data, fmt.Errorf("read diff entry: invalid code %v", ent.Code)
	}
	return ent, data, nil
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
