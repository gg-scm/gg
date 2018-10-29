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

package gittool

import (
	"io"
	"strings"
	"testing"
)

func TestReadStatusEntry(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		renameBug bool

		code      StatusCode
		entName   TopPath
		from      TopPath
		err       func(error) bool
		remaining string
	}{
		{
			name: "Empty",
			data: "",
			err:  func(e error) bool { return e == io.EOF },
		},
		{
			name:    "ModifiedWorkTree",
			data:    " M foo.txt\x00",
			code:    StatusCode{' ', 'M'},
			entName: "foo.txt",
		},
		{
			name: "MissingNul",
			data: " M foo.txt",
			err:  func(e error) bool { return e != nil && e != io.EOF },
		},
		{
			name:    "ModifiedIndex",
			data:    "MM foo.txt\x00",
			code:    StatusCode{'M', 'M'},
			entName: "foo.txt",
		},
		{
			name:    "Renamed",
			data:    "R  bar.txt\x00foo.txt\x00",
			code:    StatusCode{'R', ' '},
			entName: "bar.txt",
			from:    "foo.txt",
		},
		{
			// Regression test for https://github.com/zombiezen/gg/issues/44
			name:      "RenamedLocally",
			data:      " R bar.txt\x00foo.txt\x00",
			renameBug: false,
			code:      StatusCode{' ', 'R'},
			entName:   "bar.txt",
			from:      "foo.txt",
		},
		{
			// Test for Git bug described in https://github.com/zombiezen/gg/issues/60
			name:      "RenamedLocally_GoodInputWithGitBug",
			data:      " R bar.txt\x00foo.txt\x00",
			renameBug: true,
			code:      StatusCode{' ', 'R'},
			entName:   "",
			from:      "bar.txt",
			remaining: "foo.txt\x00",
		},
		{
			// Test for Git bug described in https://github.com/zombiezen/gg/issues/60
			name:      "RenamedLocally_GitBug",
			data:      " R bar.txt\x00 A foo.txt\x00",
			renameBug: true,
			code:      StatusCode{' ', 'R'},
			entName:   "",
			from:      "bar.txt",
			remaining: " A foo.txt\x00",
		},
		{
			name:      "Multiple",
			data:      "R  bar.txt\x00foo.txt\x00MM baz.txt\x00",
			code:      StatusCode{'R', ' '},
			entName:   "bar.txt",
			from:      "foo.txt",
			remaining: "MM baz.txt\x00",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.data)
			var ent StatusEntry
			err := readStatusEntry(&ent, r, test.renameBug)
			if remaining := test.data[len(test.data)-r.Len():]; remaining != test.remaining {
				t.Errorf("after readStatusEntry, remaining = %q; want %q", remaining, test.remaining)
			}
			if err != nil {
				if test.err == nil {
					t.Fatalf("readStatusEntry(...) = _, %v; want <nil>", err)
				}
				if !test.err(err) {
					t.Fatalf("readStatusEntry(...) = _, %v", err)
				}
				return
			}
			if test.err != nil {
				t.Fatal("readStatusEntry(...) = _, <nil>; want error")
			}
			if got, want := ent.Code(), test.code; got != want {
				t.Errorf("readStatusEntry(...).Code() = '%v'; want '%v'", got, want)
			}
			if got, want := ent.Name(), test.entName; got != want {
				t.Errorf("readStatusEntry(...).Name() = %q; want %q", got, want)
			}
			if got, want := ent.From(), test.from; got != want {
				t.Errorf("readStatusEntry(...).From() = %q; want %q", got, want)
			}
		})
	}
}

func TestReadDiffStatusEntry(t *testing.T) {
	tests := []struct {
		name string
		data string

		code      DiffStatusCode
		entName   TopPath
		err       func(error) bool
		remaining string
	}{
		{
			name: "Empty",
			data: "",
			err:  func(e error) bool { return e == io.EOF },
		},
		{
			name:    "Modified",
			data:    "M\x00foo.txt\x00",
			code:    'M',
			entName: "foo.txt",
		},
		{
			name: "MissingNul",
			data: "M\x00foo.txt",
			err:  func(e error) bool { return e != nil && e != io.EOF },
		},
		{
			name:    "Renamed",
			data:    "R00\x00foo.txt\x00bar.txt\x00",
			code:    'R',
			entName: "bar.txt",
		},
		{
			name:      "RenamedScoreTooLong",
			data:      "R000\x00foo.txt\x00bar.txt\x00",
			err:       func(e error) bool { return e != nil && e != io.EOF },
			remaining: "\x00foo.txt\x00bar.txt\x00",
		},
		{
			name:      "Multiple",
			data:      "A\x00foo.txt\x00D\x00bar.txt\x00",
			code:      'A',
			entName:   "foo.txt",
			remaining: "D\x00bar.txt\x00",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.data)
			var ent DiffStatusEntry
			err := readDiffStatusEntry(&ent, r)
			if remaining := test.data[len(test.data)-r.Len():]; remaining != test.remaining {
				t.Errorf("after readDiffStatusEntry, remaining = %q; want %q", remaining, test.remaining)
			}
			if err != nil {
				if test.err == nil {
					t.Fatalf("readDiffStatusEntry(...) = _, %v; want <nil>", err)
				}
				if !test.err(err) {
					t.Fatalf("readDiffStatusEntry(...) = _, %v", err)
				}
				return
			}
			if test.err != nil {
				t.Fatal("readDiffStatusEntry(...) = _, <nil>; want error")
			}
			if got, want := ent.Code(), test.code; got != want {
				t.Errorf("readDiffStatusEntry(...).Code() = '%v'; want '%v'", got, want)
			}
			if got, want := ent.Name(), test.entName; got != want {
				t.Errorf("readDiffStatusEntry(...).Name() = %q; want %q", got, want)
			}
		})
	}
}

func TestAffectedByStatusRenameBug(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"", false},
		{"git version 2.10", false},
		{"git version 2.10.0", false},
		{"git version 2.10.0.foobarbaz", false},
		{"git version 2.11", true},
		{"git version 2.11.0", true},
		{"git version 2.11.0.foobarbaz", true},
		{"git version 2.12", true},
		{"git version 2.12.0", true},
		{"git version 2.12.0.foobarbaz", true},
		{"git version 2.13", true},
		{"git version 2.13.0", true},
		{"git version 2.13.0.foobarbaz", true},
		{"git version 2.14", true},
		{"git version 2.14.0", true},
		{"git version 2.14.0.foobarbaz", true},
		{"git version 2.15", true},
		{"git version 2.15.0", true},
		{"git version 2.15.0.foobarbaz", true},
		{"git version 2.16", false},
		{"git version 2.16.0", false},
		{"git version 2.16.0.foobarbaz", false},
	}
	for _, test := range tests {
		if got := affectedByStatusRenameBug(test.version); got != test.want {
			t.Errorf("affectedByStatusRenameBug(%q) = %t; want %t", test.version, got, test.want)
		}
	}
}
