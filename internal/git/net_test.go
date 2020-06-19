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

import "testing"

func TestFetchRefspecParse(t *testing.T) {
	tests := []struct {
		spec FetchRefspec
		src  RefPattern
		dst  RefPattern
		plus bool
	}{
		{spec: "", src: "", dst: "", plus: false},
		{spec: "foo", src: "foo", dst: "", plus: false},
		{spec: "foo:", src: "foo", dst: "", plus: false},
		{spec: "foo:bar", src: "foo", dst: "bar", plus: false},
		{spec: "refs/heads/*:refs/remotes/origin/*", src: "refs/heads/*", dst: "refs/remotes/origin/*", plus: false},
		{spec: "tag baz", src: "refs/tags/baz", dst: "refs/tags/baz", plus: false},
		{spec: "+", src: "", dst: "", plus: true},
		{spec: "+foo", src: "foo", dst: "", plus: true},
		{spec: "+foo:", src: "foo", dst: "", plus: true},
		{spec: "+foo:bar", src: "foo", dst: "bar", plus: true},
		{spec: "+refs/heads/*:refs/remotes/origin/*", src: "refs/heads/*", dst: "refs/remotes/origin/*", plus: true},
		{spec: "+tag baz", src: "refs/tags/baz", dst: "refs/tags/baz", plus: true},
	}
	for _, test := range tests {
		src, dst, plus := test.spec.Parse()
		if src != test.src || dst != test.dst || plus != test.plus {
			t.Errorf("FetchRefspec(%q).Parse() = %q, %q, %t; want %q, %q, %t", test.spec, src, dst, plus, test.src, test.dst, test.plus)
		}
	}
}

func TestFetchRefspecMap(t *testing.T) {
	tests := []struct {
		spec   FetchRefspec
		local  Ref
		remote Ref
	}{
		{
			spec:   "+refs/heads/*:refs/remotes/origin/*",
			local:  "refs/heads/main",
			remote: "refs/remotes/origin/main",
		},
		{
			spec:   "+refs/heads/*:refs/remotes/origin/*",
			local:  "refs/tags/v1.0.0",
			remote: "",
		},
		{
			spec:   "+refs/heads/*:refs/special-remote",
			local:  "refs/heads/main",
			remote: "refs/special-remote",
		},
		{
			spec:   "+main:refs/special-remote",
			local:  "refs/heads/main",
			remote: "refs/special-remote",
		},
		{
			spec:   "+main:refs/special-remote",
			local:  "refs/heads/feature",
			remote: "",
		},
	}
	for _, test := range tests {
		if remote := test.spec.Map(test.local); remote != test.remote {
			t.Errorf("FetchRefspec(%q).Map(%q) = %q; want %q", test.spec, test.local, remote, test.remote)
		}
	}
}

func TestRefPatternPrefix(t *testing.T) {
	tests := []struct {
		pat    RefPattern
		prefix string
		ok     bool
	}{
		{pat: "", ok: false},
		{pat: "*", ok: true, prefix: ""},
		{pat: "/*", ok: false, prefix: ""},
		{pat: "er", ok: false},
		{pat: "main", ok: false},
		{pat: "/main", ok: false},
		{pat: "heads/main", ok: false},
		{pat: "refs/heads/main", ok: false},
		{pat: "refs", ok: false},
		{pat: "refs/qa*", ok: false},
		{pat: "refs/*", ok: true, prefix: "refs/"},
		{pat: "refs/heads/*", ok: true, prefix: "refs/heads/"},
	}
	for _, test := range tests {
		prefix, ok := test.pat.Prefix()
		if prefix != test.prefix || ok != test.ok {
			t.Errorf("RefPattern(%q).Prefix() = %q, %t; want %q, %t", test.pat, prefix, ok, test.prefix, test.ok)
		}
	}
}

func TestRefPatternMatches(t *testing.T) {
	tests := []struct {
		pat    RefPattern
		ref    Ref
		suffix string
		ok     bool
	}{
		{pat: "", ref: "refs/heads/main", ok: false},
		{pat: "*", ref: "refs/heads/main", ok: true, suffix: "refs/heads/main"},
		{pat: "er", ref: "refs/heads/main", ok: false},
		{pat: "main", ref: "refs/heads/main", ok: true},
		{pat: "/main", ref: "refs/heads/main", ok: false},
		{pat: "heads/main", ref: "refs/heads/main", ok: true},
		{pat: "refs/heads/main", ref: "refs/heads/main", ok: true},
		{pat: "refs", ref: "refs/heads/main", ok: false},
		{pat: "refs/heads/*", ref: "refs/heads/main", ok: true, suffix: "main"},
		{pat: "refs/*", ref: "refs/heads/main", ok: true, suffix: "heads/main"},
	}
	for _, test := range tests {
		suffix, ok := test.pat.Match(test.ref)
		if suffix != test.suffix || ok != test.ok {
			t.Errorf("RefPattern(%q).Match(Ref(%q)) = %q, %t; want %q, %t", test.pat, test.ref, suffix, ok, test.suffix, test.ok)
		}
	}
}
