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
	"fmt"
	"strings"

	"zombiezen.com/go/gg/internal/gitobj"
)

// Rev is a parsed reference to a single commit.
type Rev struct {
	commit  gitobj.Hash
	refname gitobj.Ref
}

// ParseRev parses a revision.
func ParseRev(ctx context.Context, git *Tool, refspec string) (*Rev, error) {
	if strings.HasPrefix(refspec, "-") {
		return nil, fmt.Errorf("parse revision %q: cannot start with '-'", refspec)
	}

	commitHex, err := git.RunOneLiner(ctx, '\n', "rev-parse", "-q", "--verify", "--revs-only", refspec)
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}
	h, err := gitobj.ParseHash(string(commitHex))
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}

	refname, err := git.RunOneLiner(ctx, '\n', "rev-parse", "-q", "--verify", "--revs-only", "--symbolic-full-name", refspec)
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}
	return &Rev{
		commit:  h,
		refname: gitobj.Ref(refname),
	}, nil
}

// CommitHex returns the full hex-encoded commit hash.
func (r *Rev) CommitHex() string {
	return r.commit.String()
}

// Ref returns the full refname or empty if r is not a symbolic revision.
func (r *Rev) Ref() gitobj.Ref {
	return r.refname
}

// Branch parses the branch name from r or empty if r does not reference
// a branch.
//
// If Branch returns a non-empty string, it implies that RefName will
// also return a non-empty string.
func (r *Rev) Branch() string {
	return r.refname.Branch()
}

// String returns the shortest symbolic name if possible, falling back
// to the commit hash.
func (r *Rev) String() string {
	if b := r.Branch(); b != "" {
		return b
	}
	if r.refname.IsValid() {
		return r.refname.String()
	}
	return r.commit.String()
}
