// Copyright 2019 Google LLC
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
