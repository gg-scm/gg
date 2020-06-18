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
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"gg-scm.io/pkg/internal/sigterm"
)

// hashSize is the number of bytes in a hash.
const hashSize = 20

// A Hash is the SHA-1 hash of a Git object.
type Hash [hashSize]byte

// ParseHash parses a hex-encoded hash.
func ParseHash(s string) (Hash, error) {
	if len(s) != hex.EncodedLen(hashSize) {
		return Hash{}, fmt.Errorf("parse hash %q: wrong size", s)
	}
	var h Hash
	if _, err := hex.Decode(h[:], []byte(s)); err != nil {
		return Hash{}, fmt.Errorf("parse hash %q: %w", s, err)
	}
	return h, nil
}

// String returns the hex-encoded hash.
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// Short returns the first 4 hex-encoded bytes of the hash.
func (h Hash) Short() string {
	return hex.EncodeToString(h[:4])
}

// A Ref is a Git reference to a commit.
type Ref string

// Top-level refs.
const (
	// Head names the commit on which the changes in the working tree
	// are based.
	Head Ref = "HEAD"

	// FetchHead records the branch which was fetched from a remote
	// repository with the last git fetch invocation.
	FetchHead Ref = "FETCH_HEAD"
)

// BranchRef returns a ref for the given branch name.
func BranchRef(b string) Ref {
	return branchPrefix + Ref(b)
}

// TagRef returns a ref for the given tag name.
func TagRef(t string) Ref {
	return tagPrefix + Ref(t)
}

// IsValid reports whether r is a valid reference.
func (r Ref) IsValid() bool {
	// See https://git-scm.com/docs/git-check-ref-format for details.
	return r != "" &&
		r[0] != '-' && r[0] != '.' &&
		r[len(r)-1] != '.' &&
		!strings.ContainsAny(string(r), " :~^\\") &&
		!strings.Contains(string(r), "..") &&
		!strings.Contains(string(r), "@{") &&
		!strings.Contains(string(r), "//") &&
		!strings.Contains(string(r), "/.")
}

// String returns the ref as a string.
func (r Ref) String() string {
	return string(r)
}

// IsBranch reports whether r starts with "refs/heads/".
func (r Ref) IsBranch() bool {
	return r.IsValid() && strings.HasPrefix(string(r), branchPrefix)
}

// Branch returns the string after "refs/heads/" or an empty string
// if the ref does not start with "refs/heads/".
func (r Ref) Branch() string {
	if !r.IsBranch() {
		return ""
	}
	return string(r[len(branchPrefix):])
}

// IsTag reports whether r starts with "refs/tags/".
func (r Ref) IsTag() bool {
	return r.IsValid() && strings.HasPrefix(string(r), tagPrefix)
}

// Tag returns the string after "refs/tags/" or an empty string
// if the ref does not start with "refs/tags/".
func (r Ref) Tag() string {
	if !r.IsTag() {
		return ""
	}
	return string(r[len(tagPrefix):])
}

// Ref prefixes.
const (
	branchPrefix = "refs/heads/"
	tagPrefix    = "refs/tags/"
)

// Head returns the working copy's branch revision. If the branch does
// not point to a valid commit (such as when the repository is first
// created), then Head returns an error.
func (g *Git) Head(ctx context.Context) (*Rev, error) {
	return g.ParseRev(ctx, Head.String())
}

// HeadRef returns the working copy's branch. If the working copy is in
// detached HEAD state, then HeadRef returns an empty string and no
// error. The ref may not point to a valid commit.
func (g *Git) HeadRef(ctx context.Context) (Ref, error) {
	const errPrefix = "head ref"
	c := g.command(ctx, []string{g.exe, "symbolic-ref", "--quiet", "HEAD"})
	stdout := new(strings.Builder)
	c.Stdout = &limitWriter{w: stdout, n: dataOutputLimit}
	stderr := new(bytes.Buffer)
	c.Stderr = &limitWriter{w: stderr, n: errorOutputLimit}
	if err := sigterm.Run(ctx, c); err != nil {
		if err, ok := err.(*exec.ExitError); ok && exitStatus(err.ProcessState) == 1 {
			// Not a symbolic ref: detached HEAD.
			return "", nil
		}
		return "", commandError(errPrefix, err, stderr.Bytes())
	}
	name, err := oneLine(stdout.String())
	if err != nil {
		return "", fmt.Errorf("%s: %w", errPrefix, err)
	}
	return Ref(name), nil
}

// ParseRev parses a revision.
func (g *Git) ParseRev(ctx context.Context, refspec string) (*Rev, error) {
	errPrefix := fmt.Sprintf("parse revision %q", refspec)
	if err := validateRev(refspec); err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}

	out, err := g.output(ctx, errPrefix, []string{g.exe, "rev-parse", "-q", "--verify", "--revs-only", refspec + "^0"})
	if err != nil {
		return nil, err
	}
	commitHex, err := oneLine(out)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}
	h, err := ParseHash(commitHex)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}

	out, err = g.output(ctx, errPrefix, []string{g.exe, "rev-parse", "-q", "--verify", "--revs-only", "--symbolic-full-name", refspec})
	if err != nil {
		return nil, err
	}
	if out == "" {
		// No associated ref name, but is a valid commit.
		return &Rev{Commit: h}, nil
	}
	refName, err := oneLine(out)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPrefix, err)
	}
	return &Rev{
		Commit: h,
		Ref:    Ref(refName),
	}, nil
}

// ListRefs lists all of the refs in the repository with tags dereferenced.
func (g *Git) ListRefs(ctx context.Context) (map[Ref]Hash, error) {
	const errPrefix = "git show-ref"
	out, err := g.output(ctx, errPrefix, []string{g.exe, "show-ref", "--dereference", "--head"})
	if err != nil {
		return nil, err
	}
	refs, err := parseRefs(out, false)
	if err != nil {
		return refs, fmt.Errorf("%s: %w", errPrefix, err)
	}
	return refs, nil
}

// ListRefsVerbatim lists all of the refs in the repository. Tags will not be
// dereferenced.
func (g *Git) ListRefsVerbatim(ctx context.Context) (map[Ref]Hash, error) {
	const errPrefix = "git show-ref"
	out, err := g.output(ctx, errPrefix, []string{g.exe, "show-ref", "--head"})
	if err != nil {
		return nil, err
	}
	refs, err := parseRefs(out, true)
	if err != nil {
		return refs, fmt.Errorf("%s: %w", errPrefix, err)
	}
	return refs, nil
}

func parseRefs(out string, ignoreDerefs bool) (map[Ref]Hash, error) {
	refs := make(map[Ref]Hash)
	tags := make(map[Ref]bool)
	isSpace := func(c rune) bool { return c == ' ' || c == '\t' }
	for len(out) > 0 {
		eol := strings.IndexByte(out, '\n')
		if eol == -1 {
			return refs, errors.New("parse refs: unexpected EOF")
		}
		line := out[:eol]
		out = out[eol+1:]

		sp := strings.IndexFunc(line, isSpace)
		if sp == -1 {
			return refs, fmt.Errorf("parse refs: could not parse line %q", line)
		}
		h, err := ParseHash(line[:sp])
		if err != nil {
			return refs, fmt.Errorf("parse refs: hash of ref %q: %w", line[sp+1:], err)
		}
		ref := Ref(strings.TrimLeftFunc(line[sp+1:], isSpace))
		if strings.HasSuffix(string(ref), "^{}") {
			// Dereferenced tag. This takes precedence over the previous hash stored in the map.
			if ignoreDerefs {
				continue
			}
			ref = ref[:len(ref)-3]
			if tags[ref] {
				return refs, fmt.Errorf("parse refs: multiple hashes found for tag %v", ref)
			}
			tags[ref] = true
		} else if _, exists := refs[ref]; exists {
			return refs, fmt.Errorf("parse refs: multiple hashes found for %v", ref)
		}
		refs[ref] = h
	}
	return refs, nil
}

// A RefMutation describes an operation to perform on a ref. The zero value is
// a no-op.
type RefMutation struct {
	command  string
	newvalue string
	oldvalue string
}

// DeleteRef returns a RefMutation that unconditionally deletes a ref.
func DeleteRef() RefMutation {
	return RefMutation{command: "delete"}
}

// DeleteRefIfMatches returns a RefMutation that attempts to delete a ref, but
// fails if it has the given value.
func DeleteRefIfMatches(oldvalue string) RefMutation {
	return RefMutation{command: "delete", oldvalue: oldvalue}
}

// String returns the mutation in a form similar to a line of input to
// `git update-ref --stdin`.
func (mut RefMutation) String() string {
	switch mut.command {
	case "":
		return ""
	case "update", "create":
		if mut.oldvalue != "" {
			return mut.command + " <ref> " + mut.newvalue + " " + mut.oldvalue
		}
		return mut.command + " <ref> " + mut.newvalue
	case "delete", "verify":
		if mut.oldvalue != "" {
			return mut.command + " <ref> " + mut.oldvalue
		}
		return mut.command + " <ref>"
	default:
		return mut.command + " <ref>"
	}
}

// MutateRefs atomically modifies zero or more refs. If there are no non-zero
// mutations, then MutateRefs returns nil without running Git.
func (g *Git) MutateRefs(ctx context.Context, muts map[Ref]RefMutation) error {
	input := new(bytes.Buffer)
	for ref, mut := range muts {
		if mut.command == "" {
			continue
		}
		input.WriteString(mut.command)
		input.WriteByte(' ')
		input.WriteString(ref.String())
		input.WriteByte(0)
		switch mut.command {
		case "update":
			input.WriteString(mut.newvalue)
			input.WriteByte(0)
			input.WriteString(mut.oldvalue)
			input.WriteByte(0)
		case "create":
			input.WriteString(mut.newvalue)
			input.WriteByte(0)
		case "delete", "verify":
			input.WriteString(mut.oldvalue)
			input.WriteByte(0)
		default:
			panic("unknown command " + mut.command)
		}
	}
	if input.Len() == 0 {
		return nil
	}

	c := g.command(ctx, []string{g.exe, "update-ref", "--stdin", "-z"})
	c.Stdin = input
	output := new(bytes.Buffer)
	c.Stderr = &limitWriter{w: output, n: errorOutputLimit}
	c.Stdout = c.Stderr
	if err := sigterm.Run(ctx, c); err != nil {
		return commandError("git update-ref", err, output.Bytes())
	}
	return nil
}

// Rev is a parsed reference to a single commit.
type Rev struct {
	Commit Hash
	Ref    Ref
}

// String returns the shortest symbolic name if possible, falling back
// to the commit hash.
func (r *Rev) String() string {
	if b := r.Ref.Branch(); b != "" {
		return b
	}
	if r.Ref.IsValid() {
		return r.Ref.String()
	}
	return r.Commit.String()
}
