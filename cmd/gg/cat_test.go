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

package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCat(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	if err := env.git.Run(ctx, "init"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte("foo 1\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "bar.txt"),
		[]byte("bar 1\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Mkdir(filepath.Join(env.root, "baz"), 0777)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "add", "."); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-m", "first"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte("foo 2\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "bar.txt"),
		[]byte("bar 2\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "commit", "-a", "-m", "second"); err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "foo.txt"),
		[]byte("dirty foo\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile(
		filepath.Join(env.root, "bar.txt"),
		[]byte("dirty bar\n"),
		0666)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		dir  string
		args []string
		out  string
	}{
		{
			name: "NoRevFlag",
			args: []string{"foo.txt"},
			out:  "foo 2\n",
		},
		{
			name: "RevFlag",
			args: []string{"-r", "HEAD~", "foo.txt"},
			out:  "foo 1\n",
		},
		{
			name: "MultipleFiles",
			args: []string{"foo.txt", "bar.txt"},
			out:  "foo 2\nbar 2\n",
		},
		{
			name: "MultipleFilesRevFlag",
			args: []string{"-r", "HEAD~", "foo.txt", "bar.txt"},
			out:  "foo 1\nbar 1\n",
		},
		{
			name: "InSubdir",
			dir:  "baz",
			args: []string{"../foo.txt"},
			out:  "foo 2\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"cat"}, test.args...)
			out, err := env.gg(ctx, filepath.Join(env.root, test.dir), args...)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.out {
				t.Errorf("output = %q; want %q", out, test.out)
			}
		})
	}
}
