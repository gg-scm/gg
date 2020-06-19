// Copyright 2019 The gg Authors
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

// ListRemoteRefs lists all of the refs in a remote repository.
// remote may be a URL or the name of a remote.
//
// This function may block on user input if the remote requires
// credentials.
func (g *Git) ListRemoteRefs(ctx context.Context, remote string) (map[Ref]Hash, error) {
	// TODO(now): Add tests.

	errPrefix := fmt.Sprintf("git ls-remote %q", remote)
	out, err := g.output(ctx, errPrefix, []string{g.exe, "ls-remote", "--quiet", "--", remote})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	refs, err := parseRefs(out, true)
	if err != nil {
		return refs, fmt.Errorf("%s: %w", errPrefix, err)
	}
	return refs, nil
}

// A FetchRefspec specifies a mapping from remote refs to local refs.
type FetchRefspec string

// String returns the refspec as a string.
func (spec FetchRefspec) String() string {
	return string(spec)
}

// Parse parses the refspec into its parts.
func (spec FetchRefspec) Parse() (src, dst RefPattern, plus bool) {
	plus = strings.HasPrefix(string(spec), "+")
	s := string(spec)
	if plus {
		s = s[1:]
	}
	if i := strings.IndexByte(s, ':'); i != -1 {
		return RefPattern(s[:i]), RefPattern(s[i+1:]), plus
	}
	if strings.HasPrefix(s, "tag ") {
		name := s[len("tag "):]
		return RefPattern("refs/tags/" + name), RefPattern("refs/tags/" + name), plus
	}
	return RefPattern(s), "", plus
}

// Map maps a remote ref into a local ref. If there is no mapping, then
// Map returns an empty Ref.
func (spec FetchRefspec) Map(remote Ref) Ref {
	srcPattern, dstPattern, _ := spec.Parse()
	suffix, ok := srcPattern.Match(remote)
	if !ok {
		return ""
	}
	if prefix, ok := dstPattern.Prefix(); ok {
		return Ref(prefix + suffix)
	}
	return Ref(dstPattern)
}

// A RefPattern is a part of a refspec. It may be either a literal
// suffix match (e.g. "main" matches "refs/head/main"), or the last
// component may be a wildcard ('*'), which indicates a prefix match.
type RefPattern string

// String returns the pattern string.
func (pat RefPattern) String() string {
	return string(pat)
}

// Prefix returns the prefix before the wildcard if it's a wildcard
// pattern. Otherwise it returns "", false.
func (pat RefPattern) Prefix() (_ string, ok bool) {
	if pat == "*" {
		return "", true
	}
	const wildcard = "/*"
	if strings.HasSuffix(string(pat), wildcard) && len(pat) > len(wildcard) {
		return string(pat[:len(pat)-1]), true
	}
	return "", false
}

// Match reports whether a ref matches the pattern.
// If the pattern is a prefix match, then suffix is the string matched by the wildcard.
func (pat RefPattern) Match(ref Ref) (suffix string, ok bool) {
	prefix, ok := pat.Prefix()
	if ok {
		if !strings.HasPrefix(string(ref), prefix) {
			return "", false
		}
		return string(ref[len(prefix):]), true
	}
	return "", string(ref) == string(pat) || strings.HasSuffix(string(ref), string("/"+pat))
}
