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

package filesystem

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestApply(t *testing.T) {
	t.Run("Write/Top", func(t *testing.T) {
		t.Parallel()
		dir, err := ioutil.TempDir("", "gg_filesystem")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error("clean up temp dir:", err)
			}
		}()
		const wantContent = "Hello, World!\n"
		err = Dir(dir).Apply([]Operation{{
			Op:      Write,
			Name:    "foo.txt",
			Content: wantContent,
		}})
		if err != nil {
			t.Error("Apply(...) =", err)
		}
		got, err := ioutil.ReadFile(filepath.Join(dir, "foo.txt"))
		if err != nil {
			t.Error(err)
		} else if string(got) != wantContent {
			t.Errorf("foo.txt content = %q; want %q", got, wantContent)
		}
	})
	t.Run("Write/SubDir", func(t *testing.T) {
		t.Parallel()
		dir, err := ioutil.TempDir("", "gg_filesystem")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error("clean up temp dir:", err)
			}
		}()
		const wantContent = "Hello, World!\n"
		err = Dir(dir).Apply([]Operation{{
			Op:      Write,
			Name:    "foo/bar/baz.txt",
			Content: wantContent,
		}})
		if err != nil {
			t.Error("Apply(...) =", err)
		}
		got, err := ioutil.ReadFile(filepath.Join(dir, "foo", "bar", "baz.txt"))
		if err != nil {
			t.Error(err)
		} else if string(got) != wantContent {
			t.Errorf("foo/bar/baz.txt content = %q; want %q", got, wantContent)
		}
	})
	t.Run("Mkdir/New", func(t *testing.T) {
		t.Parallel()
		dir, err := ioutil.TempDir("", "gg_filesystem")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error("clean up temp dir:", err)
			}
		}()
		err = Dir(dir).Apply([]Operation{{
			Op:   Mkdir,
			Name: "foo/bar/baz",
		}})
		if err != nil {
			t.Error("Apply(...) =", err)
		}
		madeDir := filepath.Join(dir, "foo", "bar", "baz")
		got, err := ioutil.ReadDir(madeDir)
		if err != nil {
			t.Error(err)
		} else if len(got) > 0 {
			t.Errorf("ReadDir(%q) = %v; want []", madeDir, got)
		}
	})
	t.Run("Mkdir/Exists", func(t *testing.T) {
		t.Parallel()
		dir, err := ioutil.TempDir("", "gg_filesystem")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error("clean up temp dir:", err)
			}
		}()
		if err := os.MkdirAll(filepath.Join(dir, "foo", "bar"), 0777); err != nil {
			t.Fatal(err)
		}
		err = Dir(dir).Apply([]Operation{{
			Op:   Mkdir,
			Name: "foo/bar",
		}})
		if err == nil {
			t.Error("Apply(...) = nil; want error")
		}
	})
	t.Run("Remove/Exists", func(t *testing.T) {
		t.Parallel()
		dir, err := ioutil.TempDir("", "gg_filesystem")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error("clean up temp dir:", err)
			}
		}()
		parent := filepath.Join(dir, "foo")
		if err := os.Mkdir(parent, 0777); err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(filepath.Join(parent, "bar.txt"), []byte("sup"), 0666); err != nil {
			t.Fatal(err)
		}
		err = Dir(dir).Apply([]Operation{{
			Op:   Remove,
			Name: "foo/bar.txt",
		}})
		if err != nil {
			t.Error("Apply(...) =", err)
		}
		got, err := ioutil.ReadDir(parent)
		if err != nil {
			t.Error(err)
		} else if len(got) > 0 {
			t.Errorf("ReadDir(%q) = %v; want []", parent, got)
		}
	})
	t.Run("Remove/DoesNotExist", func(t *testing.T) {
		t.Parallel()
		dir, err := ioutil.TempDir("", "gg_filesystem")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Error("clean up temp dir:", err)
			}
		}()
		parent := filepath.Join(dir, "foo")
		if err := os.Mkdir(parent, 0777); err != nil {
			t.Fatal(err)
		}
		err = Dir(dir).Apply([]Operation{{
			Op:   Remove,
			Name: "foo/bar.txt",
		}})
		if err == nil {
			t.Error("Apply(...) = nil; want error")
		}
		got, err := ioutil.ReadDir(parent)
		if err != nil {
			t.Error(err)
		} else if len(got) > 0 {
			t.Errorf("ReadDir(%q) = %v; want []", parent, got)
		}
	})
}

func TestFromSlash(t *testing.T) {
	tests := []struct {
		name string
		dir  Dir
		path string
		want string
	}{
		{
			name: "Empty",
			dir:  "foo",
			path: "",
			want: "foo",
		},
		{
			name: "SingleName",
			dir:  "foo",
			path: "bar.txt",
			want: filepath.Join("foo", "bar.txt"),
		},
		{
			name: "SubDir",
			dir:  "foo",
			path: "bar/baz.txt",
			want: filepath.Join("foo", "bar", "baz.txt"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.dir.FromSlash(test.path)
			if got != test.want {
				t.Errorf("Dir(%q).FromSlash(%q) = %q; want %q", string(test.dir), test.path, got, test.want)
			}
		})
	}
}
