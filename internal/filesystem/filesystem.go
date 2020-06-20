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

// Package filesystem provides concise data structures for filesystem
// operations and functions to apply them to local files.
package filesystem

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// An Operation describes a single step of a Dir.Apply. The zero value
// is a no-op.
type Operation struct {
	code opCode
	name string
	arg  string
}

// Write returns a new write operation. The name is a slash-separated
// path relative to the Dir.
func Write(name, content string) Operation {
	return Operation{code: opWrite, name: name, arg: content}
}

// Mkdir returns a new make directory operation. The name is a
// slash-separated path relative to the Dir.
func Mkdir(name string) Operation {
	return Operation{code: opMkdir, name: name}
}

// Remove returns a new remove operation. The name is a slash-separated
// path relative to the Dir. Remove operations on directories are recursive.
func Remove(name string) Operation {
	return Operation{code: opRemove, name: name}
}

// Rename returns a new rename operation. old and new are slash-separated
// paths relative to the Dir.
func Rename(old, new string) Operation {
	return Operation{code: opRename, name: new, arg: old}
}

// Symlink returns a new symlink operation. new is a slash-separated
// path relative to the Dir. old is a slash-separated path relative to new.
func Symlink(old, new string) Operation {
	return Operation{code: opSymlink, name: new, arg: old}
}

// String returns a readable description of an operation like "remove foo/bar".
func (o Operation) String() string {
	switch o.code {
	case nop:
		return nop.String()
	case opWrite:
		return fmt.Sprintf("write %q to %q", o.arg, o.name)
	case opRename:
		return fmt.Sprintf("rename %q to %q", o.arg, o.name)
	case opSymlink:
		return fmt.Sprintf("symlink %q as %q", o.arg, o.name)
	default:
		return fmt.Sprintf("%s %q", o.code, o.name)
	}
}

type opCode int

const (
	nop opCode = iota
	opWrite
	opMkdir
	opRemove
	opRename
	opSymlink
)

// String returns the human-readable name of code.
func (code opCode) String() string {
	switch code {
	case nop:
		return "no-op"
	case opWrite:
		return "write"
	case opMkdir:
		return "mkdir"
	case opRemove:
		return "remove"
	case opRename:
		return "rename"
	case opSymlink:
		return "symlink"
	default:
		return fmt.Sprintf("opCode(%d)", int(code))
	}
}

// A Dir is a filesystem path to a directory from which to apply operations.
type Dir string

// Apply applies the sequence of filesystem operations given. It stops
// at the first operation to fail.
func (dir Dir) Apply(ops ...Operation) error {
	for _, o := range ops {
		p, err := dir.fromSlash(o.code.String(), o.name)
		if err != nil {
			return err
		}
		switch o.code {
		case nop:
			// Do nothing.
		case opWrite:
			if err := os.MkdirAll(filepath.Dir(p), 0777); err != nil {
				return err
			}
			if err := ioutil.WriteFile(p, []byte(o.arg), 0666); err != nil {
				return err
			}
		case opMkdir:
			if err := os.MkdirAll(filepath.Dir(p), 0777); err != nil {
				return err
			}
			if err := os.Mkdir(p, 0777); err != nil {
				return err
			}
		case opRemove:
			if _, err := os.Lstat(p); os.IsNotExist(err) {
				return err
			}
			if err := os.RemoveAll(p); err != nil {
				return err
			}
		case opRename:
			oldPath, err := dir.fromSlash(o.code.String(), o.arg)
			if err != nil {
				return err
			}
			if err := os.Rename(oldPath, p); err != nil {
				return err
			}
		case opSymlink:
			if err := os.Symlink(filepath.FromSlash(o.arg), p); err != nil {
				return err
			}
		default:
			panic("invalid operation code")
		}
	}
	return nil
}

// ReadFile reads the content of the given slash-separated path relative
// to dir.
func (dir Dir) ReadFile(path string) (string, error) {
	fpath, err := dir.fromSlash("read", path)
	if err != nil {
		return "", err
	}
	f, err := os.Open(fpath)
	if err != nil {
		return "", err
	}
	sb := new(strings.Builder)
	_, cpErr := io.Copy(sb, f)
	closeErr := f.Close()
	if cpErr != nil {
		return "", cpErr
	}
	return sb.String(), closeErr
}

// Exists tests the existence of the given relative slash-separated path.
// The path is interpreted relative to dir.
func (dir Dir) Exists(path string) (bool, error) {
	fpath, err := dir.fromSlash("check", path)
	if err != nil {
		return false, err
	}
	switch _, err := os.Stat(fpath); {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, err
	}
}

// FromSlash resolves the given slash-separated path relative to dir.
// path must not be an absolute path.
func (dir Dir) FromSlash(path string) string {
	fpath, err := dir.fromSlash("resolve", path)
	if err != nil {
		panic(err)
	}
	return fpath
}

func (dir Dir) fromSlash(op, path string) (string, error) {
	if strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("filesystem: %s %q: absolute path not permitted", op, path)
	}
	return filepath.Join(string(dir), filepath.FromSlash(path)), nil
}

// String returns the directory path.
func (dir Dir) String() string {
	return string(dir)
}
