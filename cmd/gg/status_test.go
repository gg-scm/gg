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
	"io"
	"strings"
	"testing"
)

func TestReadStatusEntry(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		ent       statusEntry
		err       func(error) bool
		remaining string
	}{
		{
			name: "Empty",
			data: "",
			err:  func(e error) bool { return e == io.EOF },
		},
		{
			name: "ModifiedWorkTree",
			data: " M foo.txt\x00",
			ent: statusEntry{
				code: [2]byte{' ', 'M'},
				name: "foo.txt",
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
			ent: statusEntry{
				code: [2]byte{'M', 'M'},
				name: "foo.txt",
			},
		},
		{
			name: "Renamed",
			data: "R  bar.txt\x00foo.txt\x00",
			ent: statusEntry{
				code: [2]byte{'R', ' '},
				name: "bar.txt",
				from: "foo.txt",
			},
		},
		{
			name: "Multiple",
			data: "R  bar.txt\x00foo.txt\x00MM baz.txt\x00",
			ent: statusEntry{
				code: [2]byte{'R', ' '},
				name: "bar.txt",
				from: "foo.txt",
			},
			remaining: "MM baz.txt\x00",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.data)
			ent, err := readStatusEntry(r)
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
			if ent.code != test.ent.code || ent.name != test.ent.name || ent.from != test.ent.from {
				t.Fatalf("readStatusEntry(...) = %#v; want %#v", ent, test.ent)
			}
		})
	}
}
