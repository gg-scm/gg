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
	"context"
	"fmt"
	"strings"
)

// Head returns the working copy's branch revision.
func (g *Git) Head(ctx context.Context) (*Rev, error) {
	return g.ParseRev(ctx, Head.String())
}

// ParseRev parses a revision.
func (g *Git) ParseRev(ctx context.Context, refspec string) (*Rev, error) {
	if strings.HasPrefix(refspec, "-") {
		return nil, fmt.Errorf("parse revision %q: cannot start with '-'", refspec)
	}

	commitHex, err := g.RunOneLiner(ctx, '\n', "rev-parse", "-q", "--verify", "--revs-only", refspec)
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}
	h, err := ParseHash(string(commitHex))
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}

	refname, err := g.RunOneLiner(ctx, '\n', "rev-parse", "-q", "--verify", "--revs-only", "--symbolic-full-name", refspec)
	if err != nil {
		return nil, fmt.Errorf("parse revision %q: %v", refspec, err)
	}
	return &Rev{
		Commit: h,
		Ref:    Ref(refname),
	}, nil
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
