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

import "strings"

// NoPathspec is the special "there is no pathspec" form. If present,
// it should be the only pathspec in a list of pathspecs.
const NoPathspec Pathspec = ":"

// A Pathspec is a Git path pattern. It will be passed through literally to Git.
type Pathspec string

// LiteralPath escapes a string from any special characters.
func LiteralPath(p string) Pathspec {
	if !isGlobSafe(p) {
		return ":(literal)" + Pathspec(p)
	}
	if strings.HasPrefix(p, ":") {
		return Pathspec(":()" + p)
	}
	return Pathspec(p)
}

func isGlobSafe(s string) bool {
	return !strings.ContainsAny(s, "*?[")
}

// JoinPathspecMagic combines a set of magic options and a pattern into
// a pathspec.
func JoinPathspecMagic(magic PathspecMagic, pattern string) Pathspec {
	if magic.IsZero() && !strings.HasPrefix(pattern, ":") {
		return Pathspec(pattern)
	}
	return Pathspec(magic.String() + pattern)
}

// String returns the pathspec as a string.
func (p Pathspec) String() string {
	return string(p)
}

// SplitMagic splits the pathspec into the magic signature and the pattern.
// Unrecognized options are ignored, so this is a lossy operation.
func (p Pathspec) SplitMagic() (PathspecMagic, string) {
	if p == NoPathspec {
		return PathspecMagic{}, string(p)
	}
	switch {
	case strings.HasPrefix(string(p), ":("):
		// Long form.
		end := strings.IndexByte(string(p), ')')
		if end == -1 {
			return PathspecMagic{}, ""
		}
		var magic PathspecMagic
		for _, word := range strings.Split(string(p[2:end]), ",") {
			if strings.HasPrefix(word, "attr:") {
				// TODO(maybe): I don't know what the behavior of specifying
				// "attr:" multiple times is (i.e. additive or clears each time).
				reqs := strings.Fields(word[5:])
				magic.AttributeRequirements = append(magic.AttributeRequirements, reqs...)
				continue
			}
			switch word {
			case "top":
				magic.Top = true
			case "literal":
				magic.Literal = true
			case "icase":
				magic.CaseInsensitive = true
			case "glob":
				magic.Glob = true
			case "exclude":
				magic.Exclude = true
			}
		}
		return magic, string(p[end+1:])
	case strings.HasPrefix(string(p), ":"):
		// Short form.
		end := strings.IndexFunc(string(p[1:]), func(c rune) bool {
			return !isMagicChar(c)
		})
		if end == -1 {
			return PathspecMagic{}, ""
		}
		end += 1 // for prefix
		var magic PathspecMagic
		for _, c := range p[1:end] {
			switch c {
			case '!', '^':
				magic.Exclude = true
			case '/':
				magic.Top = true
			}
		}
		return magic, strings.TrimPrefix(string(p[end:]), ":")
	default:
		// Just a pattern.
		return PathspecMagic{}, string(p)
	}
}

// isMagicChar reports whether c is a magic signature character as
// defined in gitglossary(7) under pathspec.
func isMagicChar(c rune) bool {
	// Caret '^' is the special case where it is a regex special character, but is a magic character.
	return 31 < c && c < 128 && c != ':' && !('A' <= c && c <= 'Z') && !('a' <= c && c <= 'z') && !('0' <= c && c <= '9') && c != '|' && c != '*' && c != '+' && c != '?' && c != '[' && c != '{' && c != '(' && c != ')' && c != '.' && c != '$' && c != '\\'
}

// PathspecMagic specifies all the "magic" pathspec options.
// See pathspec under gitglossary(7) for more details.
type PathspecMagic struct {
	Top                   bool
	Literal               bool
	CaseInsensitive       bool
	Glob                  bool
	AttributeRequirements []string
	Exclude               bool
}

// IsZero reports whether all the fields are unset.
func (magic PathspecMagic) IsZero() bool {
	return !magic.Top && !magic.Literal && !magic.CaseInsensitive && len(magic.AttributeRequirements) == 0 && !magic.Exclude
}

// String returns the magic in long form like ":(top,literal)".
// The zero value returns ":()".
func (magic PathspecMagic) String() string {
	words := make([]string, 0, 5)
	if magic.Top {
		words = append(words, "top")
	}
	if magic.Literal {
		words = append(words, "literal")
	}
	if magic.CaseInsensitive {
		words = append(words, "icase")
	}
	if magic.Glob {
		words = append(words, "glob")
	}
	if magic.Exclude {
		words = append(words, "exclude")
	}
	return ":(" + strings.Join(words, ",") + ")"
}

// A TopPath is a slash-separated path relative to the top level of the repository.
type TopPath string

// Pathspec converts a top-level path to a pathspec.
func (tp TopPath) Pathspec() Pathspec {
	if !isGlobSafe(string(tp)) {
		return ":(top,literal)" + Pathspec(tp)
	}
	return ":(top)" + Pathspec(tp)
}

// String returns the path as a string.
func (tp TopPath) String() string {
	return string(tp)
}
