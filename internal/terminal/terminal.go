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

// Package terminal provides functions for querying and manipulating an
// interactive terminal.
package terminal // import "gg-scm.io/pkg/internal/terminal"

import (
	"io"
	"os"
)

// IsTerminal reports whether w writes directly to a terminal.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isTerminal(f.Fd())
}

// ResetTextStyle clears any text styles on the writer. The behavior of
// calling this function on a non-terminal is undefined.
func ResetTextStyle(w io.Writer) error {
	return resetTextStyle(w)
}
