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

import "testing"

func TestHash(t *testing.T) {
	tests := []struct {
		h     Hash
		s     string
		short string
	}{
		{
			h:     Hash{},
			s:     "0000000000000000000000000000000000000000",
			short: "00000000",
		},
		{
			h: Hash{
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67,
			},
			s:     "0123456789abcdef0123456789abcdef01234567",
			short: "01234567",
		},
	}
	for _, test := range tests {
		if got := test.h.String(); got != test.s {
			t.Errorf("Hash(%x).String() = %q; want %q", test.h[:], got, test.s)
		}
		if got := test.h.Short(); got != test.short {
			t.Errorf("Hash(%x).Short() = %q; want %q", test.h[:], got, test.short)
		}
	}
}

func TestParseHash(t *testing.T) {
	tests := []struct {
		s       string
		want    Hash
		wantErr bool
	}{
		{s: "", wantErr: true},
		{s: "0000000000000000000000000000000000000000", want: Hash{}},
		{
			s: "0123456789abcdef0123456789abcdef01234567",
			want: Hash{
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
				0x01, 0x23, 0x45, 0x67,
			},
		},
		{
			s:       "0123456789abcdef0123456789abcdef0123456",
			wantErr: true,
		},
		{
			s:       "0123456789abcdef0123456789abcdef012345678",
			wantErr: true,
		},
		{
			s:       "01234567",
			wantErr: true,
		},
	}
	for _, test := range tests {
		switch got, err := ParseHash(test.s); {
		case err == nil && !test.wantErr && got != test.want:
			t.Errorf("ParseHash(%q) = %v, <nil>; want %v, <nil>", test.s, got, test.want)
		case err == nil && test.wantErr:
			t.Errorf("ParseHash(%q) = %v, <nil>; want error", test.s, got)
		case err != nil && !test.wantErr:
			t.Errorf("ParseHash(%q) = _, %v; want %v, <nil>", test.s, err, test.want)
		}
	}
}

func TestRef(t *testing.T) {
	tests := []struct {
		ref      Ref
		invalid  bool
		isBranch bool
		branch   string
		isTag    bool
		tag      string
	}{
		{
			ref:     "",
			invalid: true,
		},
		{
			ref:     "-",
			invalid: true,
		},
		{ref: "master"},
		{ref: "HEAD"},
		{ref: "FETCH_HEAD"},
		{ref: "ORIG_HEAD"},
		{ref: "MERGE_HEAD"},
		{ref: "CHERRY_PICK_HEAD"},
		{ref: "FOO"},
		{
			ref:     "-refs/heads/master",
			invalid: true,
		},
		{
			ref:      "refs/heads/master",
			isBranch: true,
			branch:   "master",
		},
		{
			ref:   "refs/tags/v1.2.3",
			isTag: true,
			tag:   "v1.2.3",
		},
		{ref: "refs/for/master"},
	}
	for _, test := range tests {
		if got := test.ref.String(); got != string(test.ref) {
			t.Errorf("Ref(%q).String() = %q; want %q", string(test.ref), got, string(test.ref))
		}
		if got := test.ref.IsValid(); got != !test.invalid {
			t.Errorf("Ref(%q).IsValid() = %t; want %t", string(test.ref), got, !test.invalid)
		}
		if got := test.ref.IsBranch(); got != test.isBranch {
			t.Errorf("Ref(%q).IsBranch() = %t; want %t", string(test.ref), got, test.isBranch)
		}
		if got := test.ref.Branch(); got != test.branch {
			t.Errorf("Ref(%q).Branch() = %s; want %s", string(test.ref), got, test.branch)
		}
		if got := test.ref.IsTag(); got != test.isTag {
			t.Errorf("Ref(%q).IsTag() = %t; want %t", string(test.ref), got, test.isTag)
		}
		if got := test.ref.Tag(); got != test.tag {
			t.Errorf("Ref(%q).Tag() = %s; want %s", string(test.ref), got, test.tag)
		}
	}
}
