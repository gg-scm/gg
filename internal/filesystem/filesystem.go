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

// Package filesystem provides concise data structures for filesystem
// operations and functions to apply them to local files.
package filesystem

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// An Operation describes a single step of a Dir.Apply.
type Operation struct {
	// Op specifies what Apply should do.
	Op Op
	// Name is a slash-separated path relative to the directory.
	Name string
	// Content is the content of the file created for a Write Op.
	Content string
}

// String returns a readable description of an operation like "remove foo/bar".
func (o *Operation) String() string {
	if o.Op == Write {
		return fmt.Sprintf("write %q to %q", o.Content, o.Name)
	}
	return fmt.Sprintf("%s %q", o.Op, o.Name)
}

// Op is an operation code.
type Op int

// Operation codes.
const (
	Write Op = iota
	Mkdir
	Remove
)

// String returns the lowercased constant name of op.
func (op Op) String() string {
	switch op {
	case Write:
		return "write"
	case Mkdir:
		return "mkdir"
	case Remove:
		return "remove"
	default:
		return fmt.Sprintf("Op(%d)", int(op))
	}
}

// A Dir is a filesystem path to a directory from which to apply operations.
type Dir string

// Apply applies the sequence of filesystem operations given. It stops
// at the first operation to fail.
func (dir Dir) Apply(ops ...Operation) error {
	for _, o := range ops {
		p := dir.FromSlash(o.Name)
		switch o.Op {
		case Write:
			if err := os.MkdirAll(filepath.Dir(p), 0777); err != nil {
				return err
			}
			if err := ioutil.WriteFile(p, []byte(o.Content), 0666); err != nil {
				return err
			}
		case Mkdir:
			if err := os.MkdirAll(filepath.Dir(p), 0777); err != nil {
				return err
			}
			if err := os.Mkdir(p, 0777); err != nil {
				return err
			}
		case Remove:
			if _, err := os.Lstat(p); os.IsNotExist(err) {
				return err
			}
			if err := os.RemoveAll(p); err != nil {
				return err
			}
		default:
			return fmt.Errorf("apply: unknown operation %v", o.Op)
		}
	}
	return nil
}

// FromSlash resolves the given slash-separated path relative to dir.
// path must not be an absolute path.
func (dir Dir) FromSlash(path string) string {
	if strings.HasPrefix(path, "/") {
		panic("absolute path to filesystem.Dir.FromSlash")
	}
	return filepath.Join(string(dir), filepath.FromSlash(path))
}

// String returns the directory path.
func (dir Dir) String() string {
	return string(dir)
}
