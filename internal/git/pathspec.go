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

// A Pathspec is a Git path pattern. It will be passed through literally to Git.
type Pathspec string

// LiteralPath escapes a string from any special characters.
func LiteralPath(p string) Pathspec {
	return ":(literal)" + Pathspec(p)
}

// String returns the pathspec as a string.
func (p Pathspec) String() string {
	return string(p)
}

// A TopPath is a slash-separated path relative to the top level of the repository.
type TopPath string

// Pathspec converts a top-level path to a pathspec.
func (tp TopPath) Pathspec() Pathspec {
	return ":(top,literal)" + Pathspec(tp)
}

// String returns the path as a string.
func (tp TopPath) String() string {
	return string(tp)
}
