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

package flag

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string

		stopAtFirstArg bool
		args           []string
		xDefault       bool
		forceDefault   bool
		oDefault       string
		revDefault     string

		parsedArgs []string
		x          bool
		force      bool
		o          string
		rev        string
	}{
		{
			name: "Empty",
		},
		{
			name:       "ArgsOnly",
			args:       []string{"a", "b", "c"},
			parsedArgs: []string{"a", "b", "c"},
		},
		{
			name: "BoolFlag",
			args: []string{"-x"},
			x:    true,
		},
		{
			name:     "BoolFlagDefaultTrue",
			xDefault: true,
			args:     []string{"-x"},
			x:        true,
		},
		{
			name:     "BoolFlagZero",
			xDefault: true,
			args:     []string{"-x=0"},
			x:        false,
		},
		{
			name:  "LongBoolFlag",
			args:  []string{"-force"},
			force: true,
		},
		{
			name:  "LongBoolFlagDashDash",
			args:  []string{"--force"},
			force: true,
		},
		{
			name: "StringFlagSameArg",
			args: []string{"-o=foo"},
			o:    "foo",
		},
		{
			name: "StringFlagNextArg",
			args: []string{"-o", "foo"},
			o:    "foo",
		},
		{
			name: "LongStringFlagSameArg",
			args: []string{"-rev=foo"},
			rev:  "foo",
		},
		{
			name: "LongStringFlagNextArg",
			args: []string{"-rev", "foo"},
			rev:  "foo",
		},
		{
			name: "LongStringFlagDashDashSameArg",
			args: []string{"--rev=foo"},
			rev:  "foo",
		},
		{
			name: "LongStringFlagDashDashNextArg",
			args: []string{"--rev", "foo"},
			rev:  "foo",
		},
		{
			name: "StringFlagMultiple",
			args: []string{"-o", "foo", "-o=bar"},
			o:    "bar",
		},
		{
			name:           "StringFlagMultiple_StopAtFirstArg",
			stopAtFirstArg: true,
			args:           []string{"-o", "foo", "baz", "-o=bar"},
			o:              "foo",
			parsedArgs:     []string{"baz", "-o=bar"},
		},
		{
			name:       "StringFlagMultiple_ArgInBetween",
			args:       []string{"-o", "foo", "baz", "-o=bar"},
			o:          "bar",
			parsedArgs: []string{"baz"},
		},
		{
			name: "Alias",
			args: []string{"-out=foo"},
			o:    "foo",
		},
		{
			name:       "Divider",
			args:       []string{"-o", "foo", "--", "-o=bar"},
			o:          "foo",
			parsedArgs: []string{"-o=bar"},
		},
		{
			name:           "Divider_StopAtFirstArg",
			stopAtFirstArg: true,
			args:           []string{"-o", "foo", "--", "-o=bar"},
			o:              "foo",
			parsedArgs:     []string{"-o=bar"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fset := NewFlagSet(!test.stopAtFirstArg, "", "")
			x := fset.Bool("x", test.xDefault, "")
			force := fset.Bool("force", test.forceDefault, "")
			o := fset.String("o", test.oDefault, "")
			fset.Alias("o", "out")
			rev := fset.String("rev", test.revDefault, "")
			if err := fset.Parse(test.args); err != nil {
				t.Fatal(err)
			}
			if *x != test.x {
				t.Errorf("x = %t; want %t", *x, test.x)
			}
			if *force != test.force {
				t.Errorf("force = %t; want %t", *force, test.force)
			}
			if *o != test.o {
				t.Errorf("o = %q; want %q", *o, test.o)
			}
			if *rev != test.rev {
				t.Errorf("rev = %q; want %q", *rev, test.rev)
			}
			if args := fset.Args(); !stringsEqual(args, test.parsedArgs) {
				t.Errorf("fset.Args() = %q; want %q", args, test.parsedArgs)
			}
		})
	}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
