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

import (
	"io"
	"testing"
)

func TestReadStatusEntry(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		renameBug bool

		want      StatusEntry
		remaining string
		err       func(error) bool
	}{
		{
			name: "Empty",
			data: "",
			err:  func(e error) bool { return e == io.EOF },
		},
		{
			name: "ModifiedWorkTree",
			data: " M foo.txt\x00",
			want: StatusEntry{
				Code: StatusCode{' ', 'M'},
				Name: "foo.txt",
			},
		},
		{
			name: "MissingNul",
			data: " M foo.txt",
			err:  func(e error) bool { return e != nil && e != io.EOF },
		},
		{
			name: "ModifiedIndex",
			data: "MM foo.txt\x00",
			want: StatusEntry{
				Code: StatusCode{'M', 'M'},
				Name: "foo.txt",
			},
		},
		{
			name: "Renamed",
			data: "R  bar.txt\x00foo.txt\x00",
			want: StatusEntry{
				Code: StatusCode{'R', ' '},
				Name: "bar.txt",
				From: "foo.txt",
			},
		},
		{
			// Regression test for https://github.com/zombiezen/gg/issues/44
			name:      "RenamedLocally",
			data:      " R bar.txt\x00foo.txt\x00",
			renameBug: false,
			want: StatusEntry{
				Code: StatusCode{' ', 'R'},
				Name: "bar.txt",
				From: "foo.txt",
			},
		},
		{
			// Test for Git bug described in https://github.com/zombiezen/gg/issues/60
			name:      "RenamedLocally_GoodInputWithGitBug",
			data:      " R bar.txt\x00foo.txt\x00",
			renameBug: true,
			want: StatusEntry{
				Code: StatusCode{' ', 'R'},
				Name: "",
				From: "bar.txt",
			},
			remaining: "foo.txt\x00",
		},
		{
			// Test for Git bug described in https://github.com/zombiezen/gg/issues/60
			name:      "RenamedLocally_GitBug",
			data:      " R bar.txt\x00 A foo.txt\x00",
			renameBug: true,
			want: StatusEntry{
				Code: StatusCode{' ', 'R'},
				Name: "",
				From: "bar.txt",
			},
			remaining: " A foo.txt\x00",
		},
		{
			name: "Multiple",
			data: "R  bar.txt\x00foo.txt\x00MM baz.txt\x00",
			want: StatusEntry{
				Code: StatusCode{'R', ' '},
				Name: "bar.txt",
				From: "foo.txt",
			},
			remaining: "MM baz.txt\x00",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, remaining, err := readStatusEntry(test.data, test.renameBug)
			if err == nil {
				if test.err != nil {
					t.Fatalf("readStatusEntry(%q, %t) = %+v, %q, <nil>; want %+v, %q, <non-nil>", test.data, test.renameBug, got, remaining, test.want, test.remaining)
				}
				if got != test.want || remaining != test.remaining {
					t.Fatalf("readStatusEntry(%q, %t) = %+v, %q, <nil>; want %+v, %q, <nil>", test.data, test.renameBug, got, remaining, test.want, test.remaining)
				}
			} else {
				if test.err == nil {
					t.Fatalf("readStatusEntry(%q, %t) = _, %q, %v; want %+v, %q, <nil>", test.data, test.renameBug, remaining, err, test.want, test.remaining)
				}
				if remaining != test.remaining || !test.err(err) {
					t.Fatalf("readStatusEntry(%q, %t) = _, %q, %v; want _, %q, <non-nil>", test.data, test.renameBug, remaining, err, test.remaining)
				}
			}
		})
	}
}

func TestReadDiffStatusEntry(t *testing.T) {
	tests := []struct {
		name string
		data string

		want      DiffStatusEntry
		remaining string
		err       func(error) bool
	}{
		{
			name: "Empty",
			data: "",
			err:  func(e error) bool { return e == io.EOF },
		},
		{
			name: "Modified",
			data: "M\x00foo.txt\x00",
			want: DiffStatusEntry{
				Code: 'M',
				Name: "foo.txt",
			},
		},
		{
			name: "MissingNul",
			data: "M\x00foo.txt",
			err:  func(e error) bool { return e != nil && e != io.EOF },
		},
		{
			name: "Renamed",
			data: "R00\x00foo.txt\x00bar.txt\x00",
			want: DiffStatusEntry{
				Code: 'R',
				Name: "bar.txt",
			},
		},
		{
			name:      "RenamedScoreTooLong",
			data:      "R000\x00foo.txt\x00bar.txt\x00",
			err:       func(e error) bool { return e != nil && e != io.EOF },
			remaining: "R000\x00foo.txt\x00bar.txt\x00",
		},
		{
			name: "Multiple",
			data: "A\x00foo.txt\x00D\x00bar.txt\x00",
			want: DiffStatusEntry{
				Code: 'A',
				Name: "foo.txt",
			},
			remaining: "D\x00bar.txt\x00",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, remaining, err := readDiffStatusEntry(test.data)
			if err == nil {
				if test.err != nil {
					t.Fatalf("readDiffStatusEntry(%q) = %+v, %q, <nil>; want %+v, %q, <non-nil>", test.data, got, remaining, test.want, test.remaining)
				}
				if got != test.want || remaining != test.remaining {
					t.Fatalf("readDiffStatusEntry(%q) = %+v, %q, <nil>; want %+v, %q, <nil>", test.data, got, remaining, test.want, test.remaining)
				}
			} else {
				if test.err == nil {
					t.Fatalf("readDiffStatusEntry(%q) = _, %q, %v; want %+v, %q, <nil>", test.data, remaining, err, test.want, test.remaining)
				}
				if remaining != test.remaining || !test.err(err) {
					t.Fatalf("readDiffStatusEntry(%q) = _, %q, %v; want _, %q, <non-nil>", test.data, remaining, err, test.remaining)
				}
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
