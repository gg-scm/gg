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

// Package escape provides functions for escaping strings for different contexts.
package escape

import (
	"strings"
)

// Bash quotes s such that it can be used as a literal argument in a
// bash command line.
func Bash(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for i := 0; i < len(s); i++ {
		if !isShellSafe(s[i]) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	sb := new(strings.Builder)
	sb.Grow(len(s) + 2)
	sb.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			sb.WriteString(`'\''`)
		} else {
			sb.WriteByte(s[i])
		}
	}
	sb.WriteByte('\'')
	return sb.String()
}

func isShellSafe(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' || b == '_' || b == '/' || b == '.'
}

// GitConfig quotes s such that it can be used as a Git configuration value.
func GitConfig(s string) string {
	sb := new(strings.Builder)
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			sb.WriteString(`\n`)
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		default:
			sb.WriteByte(s[i])
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
