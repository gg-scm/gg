// Copyright 2019 The gg Authors
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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestLiteralPath(t *testing.T) {
	tests := []struct {
		in   string
		want Pathspec
	}{
		{"", ""},
		{":", ":():"}, // the "no pathspec" pattern
		{":foo.txt", ":():foo.txt"},
		{":*.txt", ":(literal):*.txt"},
		{"foo.txt", "foo.txt"},
		{"foo/bar.txt", "foo/bar.txt"},
		{"foo\\bar.txt", "foo\\bar.txt"},
		{"*.txt", ":(literal)*.txt"},
		{"foo?.txt", ":(literal)foo?.txt"},
		{"foo[0-9].txt", ":(literal)foo[0-9].txt"},
		{"foo*/bar.txt", ":(literal)foo*/bar.txt"},
	}
	for _, test := range tests {
		got := LiteralPath(test.in)
		if got != test.want {
			t.Errorf("LiteralPath(%q) = Pathspec(%q); want Pathspec(%q)", test.in, got, test.want)
		}
	}
}

func TestTopPathPathspec(t *testing.T) {
	tests := []struct {
		in   TopPath
		want Pathspec
	}{
		{"", ":(top)"},
		{":", ":(top):"}, // the "no pathspec" pattern
		{":foo.txt", ":(top):foo.txt"},
		{":*.txt", ":(top,literal):*.txt"},
		{"foo.txt", ":(top)foo.txt"},
		{"foo/bar.txt", ":(top)foo/bar.txt"},
		{"foo\\bar.txt", ":(top)foo\\bar.txt"},
		{"*.txt", ":(top,literal)*.txt"},
		{"foo?.txt", ":(top,literal)foo?.txt"},
		{"foo[0-9].txt", ":(top,literal)foo[0-9].txt"},
		{"foo*/bar.txt", ":(top,literal)foo*/bar.txt"},
	}
	for _, test := range tests {
		got := test.in.Pathspec()
		if got != test.want {
			t.Errorf("TopPath(%q).Pathspec() = Pathspec(%q); want Pathspec(%q)", test.in, got, test.want)
		}
	}
}

func TestPathspecSplitMagic(t *testing.T) {
	tests := []struct {
		p     Pathspec
		magic PathspecMagic
		pat   string
	}{
		{"", PathspecMagic{}, ""},
		{":", PathspecMagic{}, ":"}, // the "no pathspec" pattern
		{"foo.txt", PathspecMagic{}, "foo.txt"},
		{":!foo.txt", PathspecMagic{Exclude: true}, "foo.txt"},
		{":^foo.txt", PathspecMagic{Exclude: true}, "foo.txt"},
		{":/foo.txt", PathspecMagic{Top: true}, "foo.txt"},
		{":!/foo.txt", PathspecMagic{Top: true, Exclude: true}, "foo.txt"},
		{":/!foo.txt", PathspecMagic{Top: true, Exclude: true}, "foo.txt"},
		{"::foo.txt", PathspecMagic{}, "foo.txt"},
		{":::foo.txt", PathspecMagic{}, ":foo.txt"},
		{":!:foo.txt", PathspecMagic{Exclude: true}, "foo.txt"},
		{":^:foo.txt", PathspecMagic{Exclude: true}, "foo.txt"},
		{":/:foo.txt", PathspecMagic{Top: true}, "foo.txt"},
		{":!/:foo.txt", PathspecMagic{Top: true, Exclude: true}, "foo.txt"},
		{":/!:foo.txt", PathspecMagic{Top: true, Exclude: true}, "foo.txt"},
		{":()foo.txt", PathspecMagic{}, "foo.txt"},
		{":():foo.txt", PathspecMagic{}, ":foo.txt"},
		{":(top)foo.txt", PathspecMagic{Top: true}, "foo.txt"},
		{":(literal)foo.txt", PathspecMagic{Literal: true}, "foo.txt"},
		{":(icase)foo.txt", PathspecMagic{CaseInsensitive: true}, "foo.txt"},
		{":(glob)foo.txt", PathspecMagic{Glob: true}, "foo.txt"},
		{":(attr:foo -bar)foo.txt", PathspecMagic{AttributeRequirements: []string{"foo", "-bar"}}, "foo.txt"},
		{":(exclude)foo.txt", PathspecMagic{Exclude: true}, "foo.txt"},
		{":(top,literal,icase,glob,exclude)foo.txt", PathspecMagic{Top: true, Literal: true, CaseInsensitive: true, Glob: true, Exclude: true}, "foo.txt"},
	}
	for _, test := range tests {
		magic, pat := test.p.SplitMagic()
		if !cmp.Equal(magic, test.magic, cmpopts.EquateEmpty()) || pat != test.pat {
			t.Errorf("Pathspec(%q).SplitMagic() = %v, Pathspec(%q); want %v, Pathspec(%q)", test.p, magic, pat, test.magic, test.pat)
		}
	}
}
