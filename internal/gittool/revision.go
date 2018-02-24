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
	"context"
	"encoding/hex"
	"fmt"
	"strings"
)

const hashSize = 20

// Rev is a reference to a commit.
type Rev struct {
	commit  [hashSize]byte
	refname string
}

// ParseRev parses a revision.
func ParseRev(ctx context.Context, git *Tool, refspec string) (*Rev, error) {
	if strings.HasPrefix(refspec, "-") {
		return nil, fmt.Errorf("parse revision %q: refspec cannot start with '-'", refspec)
	}

	commitHex, err := git.RunOneLiner(ctx, '\n', "rev-parse", "-q", "--verify", "--revs-only", refspec)
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}
	if len(commitHex) != hex.EncodedLen(hashSize) {
		return nil, fmt.Errorf("parse revision %q: invalid output from git rev-parse", refspec)
	}
	r := new(Rev)
	if _, err := hex.Decode(r.commit[:], commitHex); err != nil {
		return nil, fmt.Errorf("parse revision %q: invalid output from git rev-parse", refspec)
	}

	refname, err := git.RunOneLiner(ctx, '\n', "rev-parse", "-q", "--verify", "--revs-only", "--symbolic-full-name", refspec)
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}
	r.refname = string(refname)
	return r, nil
}

// CommitHex returns the full hex-encoded commit hash.
func (r *Rev) CommitHex() string {
	return hex.EncodeToString(r.commit[:])
}

// RefName returns the full refname or empty if r is not a symbolic revision.
func (r *Rev) RefName() string {
	return r.refname
}

// Branch parses the branch name from r or empty if r does not reference
// a branch.
//
// If Branch returns a non-empty string, it implies that RefName will
// also return a non-empty string.
func (r *Rev) Branch() string {
	const prefix = "refs/heads/"
	if strings.HasPrefix(r.refname, prefix) {
		return r.refname[len(prefix):]
	}
	return ""
}

// String returns the shortest symbolic name if possible, falling back
// to the commit hash.
func (r *Rev) String() string {
	if b := r.Branch(); b != "" {
		return b
	}
	if r.refname != "" {
		return r.refname
	}
	return r.CommitHex()
}
