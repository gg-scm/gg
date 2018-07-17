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

// Package gitobj provides Git data types.
package gitobj // import "gg-scm.io/pkg/internal/gitobj"

import (
	"encoding/hex"
	"fmt"
	"strings"
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
		return Hash{}, fmt.Errorf("parse hash %q: %v", s, err)
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

// IsValid reports whether r is a valid reference.
func (r Ref) IsValid() bool {
	return r != "" && r[0] != '-'
}

// String returns the ref as a string.
func (r Ref) String() string {
	return string(r)
}

// IsBranch reports whether r starts with "refs/heads/".
func (r Ref) IsBranch() bool {
	return strings.HasPrefix(string(r), branchPrefix)
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
	return strings.HasPrefix(string(r), tagPrefix)
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
