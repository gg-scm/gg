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
	errPrefix := fmt.Sprintf("parse revision %q", refspec)
	if err := validateRev(refspec); err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}

	out, err := g.run(ctx, errPrefix, []string{"rev-parse", "-q", "--verify", "--revs-only", refspec})
	if err != nil {
		return nil, err
	}
	commitHex, err := oneLine(out)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	h, err := ParseHash(commitHex)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}

	out, err = g.run(ctx, errPrefix, []string{"rev-parse", "-q", "--verify", "--revs-only", "--symbolic-full-name", refspec})
	if err != nil {
		return nil, err
	}
	if out == "" {
		// No associated ref name, but is a valid commit.
		return &Rev{Commit: h}, nil
	}
	refName, err := oneLine(out)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", errPrefix, err)
	}
	return &Rev{
		Commit: h,
		Ref:    Ref(refName),
	}, nil
}

// ListRefs lists all of the refs in the repository.
func (g *Git) ListRefs(ctx context.Context) (map[Ref]*Rev, error) {
	const errPrefix = "git show-ref"
	out, err := g.run(ctx, errPrefix, []string{"show-ref", "--dereference"})
	if err != nil {
		return nil, err
	}
	refs := make(map[Ref]*Rev)
	tags := make(map[Ref]bool)
	for len(out) > 0 {
		eol := strings.IndexByte(out, '\n')
		if eol == -1 {
			return refs, fmt.Errorf("%s: unexpected EOF", errPrefix)
		}
		line := out[:eol]
		out = out[eol+1:]

		sp := strings.IndexByte(line, ' ')
		if sp == -1 {
			return refs, fmt.Errorf("%s: could not parse line %q", errPrefix, line)
		}
		h, err := ParseHash(line[:sp])
		if err != nil {
			return refs, fmt.Errorf("%s: parse hash of ref %q: %v", errPrefix, line[sp+1:], err)
		}
		ref := Ref(line[sp+1:])
		if strings.HasSuffix(string(ref), "^{}") {
			// Dereferenced tag. This takes precedence over the previous hash stored in the map.
			ref = ref[:len(ref)-3]
			if tags[ref] {
				return refs, fmt.Errorf("%s: multiple hashes found for tag %v", errPrefix, ref)
			}
			tags[ref] = true
		} else if refs[ref] != nil {
			return refs, fmt.Errorf("%s: multiple hashes found for %v", errPrefix, ref)
		}
		refs[ref] = &Rev{
			Commit: h,
			Ref:    ref,
		}
	}
	return refs, nil
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
