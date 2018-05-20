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
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"zombiezen.com/go/gg/internal/gittool"
)

func TestPush(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, pushEnv.repoA, "push"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("refs/heads/master = %s (first commit); want %s", r.CommitHex(), pushEnv.commit2)
		} else if r.CommitHex() != pushEnv.commit2 {
			t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit2)
		}
	}
}

func TestPush_Arg(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "clone", "--bare", "repoB", "repoC"); err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, pushEnv.repoA, "push", filepath.Join(env.root, "repoC")); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("origin refs/heads/master = %s (pushed commit); want %s", r.CommitHex(), pushEnv.commit1)
		} else if r.CommitHex() != pushEnv.commit1 {
			t.Errorf("origin refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit1)
		}
	}
	gitC := env.git.WithDir(filepath.Join(env.root, "repoC"))
	if r, err := gittool.ParseRev(ctx, gitC, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("named remote refs/heads/master = %s (first commit); want %s", r.CommitHex(), pushEnv.commit2)
		} else if r.CommitHex() != pushEnv.commit2 {
			t.Errorf("named remote refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit2)
		}
	}
}

func TestPush_FailUnknownRef(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, pushEnv.repoA, "push", "-d", "foo"); err == nil {
		t.Error("push of new ref did not return error")
	} else if _, isUsage := err.(*usageError); isUsage {
		t.Errorf("push of new ref returned usage error: %v", err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("refs/heads/master = %s (pushed commit); want %s", r.CommitHex(), pushEnv.commit1)
		} else if r.CommitHex() != pushEnv.commit1 {
			t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit1)
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "foo"); err == nil {
		if ref := r.RefName(); ref != "" {
			t.Errorf("foo resolved to %s", ref)
		}
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("foo = %s (first commit); want to not exist", r.CommitHex())
		} else if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("foo = %s (pushed commit); want to not exist", r.CommitHex())
		} else {
			t.Errorf("foo = %s; want to not exist", r.CommitHex())
		}
	}
}

func TestPush_CreateRef(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := env.gg(ctx, pushEnv.repoA, "push", "-d", "foo", "--create"); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("refs/heads/master = %s (pushed commit); want %s", r.CommitHex(), pushEnv.commit1)
		} else if r.CommitHex() != pushEnv.commit1 {
			t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit1)
		}
	}
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/foo"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("refs/heads/foo = %s (first commit); want %s", r.CommitHex(), pushEnv.commit2)
		} else if r.CommitHex() != pushEnv.commit2 {
			t.Errorf("refs/heads/foo = %s; want %s", r.CommitHex(), pushEnv.commit2)
		}
	}
}

func TestPush_RewindFails(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Push second commit to B
	if err := env.git.WithDir(pushEnv.repoA).Run(ctx, "push", "origin", "master"); err != nil {
		t.Fatal(err)
	}

	// Push rewind
	if _, err := env.gg(ctx, pushEnv.repoA, "push", "-d", "master", "-r", pushEnv.commit1); err == nil {
		t.Error("push of parent rev did not return error")
	} else if _, isUsage := err.(*usageError); isUsage {
		t.Errorf("push of parent rev returned usage error: %v", err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit1 {
			t.Errorf("refs/heads/master = %s (commit 1); want %s (commit 2)", r.CommitHex(), pushEnv.commit2)
		} else if r.CommitHex() != pushEnv.commit2 {
			t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit2)
		}
	}
}

func TestPush_RewindForce(t *testing.T) {
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	pushEnv, err := stagePushTest(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	// Push second commit to B
	if err := env.git.WithDir(pushEnv.repoA).Run(ctx, "push", "origin", "master"); err != nil {
		t.Fatal(err)
	}

	// Push rewind
	if _, err := env.gg(ctx, pushEnv.repoA, "push", "-f", "-d", "master", "-r", pushEnv.commit1); err != nil {
		t.Fatal(err)
	}
	gitB := env.git.WithDir(pushEnv.repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		t.Error(err)
	} else {
		if r.CommitHex() == pushEnv.commit2 {
			t.Errorf("refs/heads/master = %s (commit 2); want %s (commit 1)", r.CommitHex(), pushEnv.commit1)
		} else if r.CommitHex() != pushEnv.commit1 {
			t.Errorf("refs/heads/master = %s; want %s", r.CommitHex(), pushEnv.commit1)
		}
	}
}

func TestGerritPushRef(t *testing.T) {
	tests := []struct {
		branch string
		opts   *gerritOptions

		wantRef  string
		wantOpts map[string][]string
	}{
		{
			branch:   "master",
			wantRef:  "refs/for/master",
			wantOpts: map[string][]string{"no-publish-comments": nil},
		},
		{
			branch: "master",
			opts: &gerritOptions{
				publishComments: true,
			},
			wantRef:  "refs/for/master",
			wantOpts: map[string][]string{"publish-comments": nil},
		},
		{
			branch: "master",
			opts: &gerritOptions{
				message: "This is a rebase on master!",
			},
			wantRef: "refs/for/master",
			wantOpts: map[string][]string{
				"m": {"This is a rebase on master!"},
				"no-publish-comments": nil,
			},
		},
		{
			branch: "master",
			opts: &gerritOptions{
				reviewers: []string{"a@a.com", "c@r.com"},
				cc:        []string{"b@o.com", "d@zombo.com"},
			},
			wantRef: "refs/for/master",
			wantOpts: map[string][]string{
				"r":  {"a@a.com", "c@r.com"},
				"cc": {"b@o.com", "d@zombo.com"},
				"no-publish-comments": nil,
			},
		},
		{
			branch: "master",
			opts: &gerritOptions{
				reviewers: []string{"a@a.com,c@r.com"},
				cc:        []string{"b@o.com,d@zombo.com"},
			},
			wantRef: "refs/for/master",
			wantOpts: map[string][]string{
				"r":  {"a@a.com", "c@r.com"},
				"cc": {"b@o.com", "d@zombo.com"},
				"no-publish-comments": nil,
			},
		},
		{
			branch: "master",
			opts: &gerritOptions{
				notify:    "NONE",
				notifyTo:  []string{"a@a.com"},
				notifyCC:  []string{"b@b.com"},
				notifyBCC: []string{"c@c.com"},
			},
			wantRef: "refs/for/master",
			wantOpts: map[string][]string{
				"notify":              {"NONE"},
				"notify-to":           {"a@a.com"},
				"notify-cc":           {"b@b.com"},
				"notify-bcc":          {"c@c.com"},
				"no-publish-comments": nil,
			},
		},
	}
	for _, test := range tests {
		out := gerritPushRef(test.branch, test.opts)
		ref, opts, err := parseGerritRef(out)
		if err != nil {
			t.Errorf("gerritPushRef(%q, %+v) = %q; cannot parse: %v", test.branch, test.opts, out, err)
			continue
		}
		if ref != test.wantRef || !gerritOptionsEqual(opts, test.wantOpts) {
			t.Errorf("gerritPushRef(%q, %+v) = %q; want ref %q and options %q", test.branch, test.opts, out, test.wantRef, test.wantOpts)
		}
	}
}

func TestParseGerritRef(t *testing.T) {
	tests := []struct {
		ref  string
		base string
		opts map[string][]string
	}{
		{
			ref:  "refs/for/master",
			base: "refs/for/master",
		},
		{
			ref:  "refs/for/master%no-publish-comments",
			base: "refs/for/master",
			opts: map[string][]string{"no-publish-comments": nil},
		},
		{
			ref:  "refs/for/expiremental%topic=driver/i42",
			base: "refs/for/expiremental",
			opts: map[string][]string{"topic": {"driver/i42"}},
		},
		{
			ref:  "refs/for/master%notify=NONE,notify-to=a@a.com",
			base: "refs/for/master",
			opts: map[string][]string{"notify": {"NONE"}, "notify-to": {"a@a.com"}},
		},
		{
			ref:  "refs/for/master%m=This_is_a_rebase_on_master%21",
			base: "refs/for/master",
			opts: map[string][]string{"m": {"This is a rebase on master!"}},
		},
		{
			ref:  "refs/for/master%m=This+is+a+rebase+on+master%21",
			base: "refs/for/master",
			opts: map[string][]string{"m": {"This is a rebase on master!"}},
		},
		{
			ref:  "refs/for/master%l=Code-Review+1,l=Verified+1",
			base: "refs/for/master",
			opts: map[string][]string{"l": {"Code-Review+1", "Verified+1"}},
		},
		{
			ref:  "refs/for/master%r=a@a.com,cc=b@o.com",
			base: "refs/for/master",
			opts: map[string][]string{"r": {"a@a.com"}, "cc": {"b@o.com"}},
		},
		{
			ref:  "refs/for/master%r=a@a.com,cc=b@o.com,r=c@r.com",
			base: "refs/for/master",
			opts: map[string][]string{"r": {"a@a.com", "c@r.com"}, "cc": {"b@o.com"}},
		},
	}
	for _, test := range tests {
		base, opts, err := parseGerritRef(test.ref)
		if err != nil {
			t.Errorf("parseGerritRef(%q) = _, _, %v; want no error", test.ref, err)
			continue
		}
		if base != test.base || !gerritOptionsEqual(opts, test.opts) {
			t.Errorf("parseGerritRef(%q) = %q, %q, <nil>; want %q, %q, <nil>", test.ref, base, opts, test.base, test.opts)
		}
	}
}

func parseGerritRef(ref string) (string, map[string][]string, error) {
	start := strings.IndexByte(ref, '%')
	if start == -1 {
		return ref, nil, nil
	}
	opts := make(map[string][]string)
	q := ref[start+1:]
	for len(q) > 0 {
		sep := strings.IndexByte(q, ',')
		if sep == -1 {
			sep = len(q)
		}
		if eq := strings.IndexByte(q[:sep], '='); eq != -1 {
			k := q[:eq]
			v := q[eq+1 : sep]
			if k == "m" || k == "message" { // special-cased in Gerrit (see ReceiveCommits.java)
				var err error
				v, err = url.QueryUnescape(strings.Replace(q[eq+1:sep], "_", "+", -1))
				if err != nil {
					return "", nil, err
				}
			}
			opts[k] = append(opts[k], v)
		} else {
			k := q[:sep]
			if v := opts[k]; v != nil {
				opts[k] = append(v, "")
			} else {
				opts[k] = nil
			}
		}
		if sep >= len(q) {
			break
		}
		q = q[sep+1:]
	}
	return ref[:start], opts, nil
}

func gerritOptionsEqual(m1, m2 map[string][]string) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		v2, ok := m2[k]
		if !ok || len(v1) != len(v2) || (v1 == nil) != (v2 == nil) {
			return false
		}
		for i := range v1 {
			if v1[i] != v2[i] {
				return false
			}
		}
	}
	return true
}

type pushEnv struct {
	repoA, repoB     string
	commit1, commit2 string
}

func stagePushTest(ctx context.Context, env *testEnv) (*pushEnv, error) {
	repoA := filepath.Join(env.root, "repoA")
	if err := env.git.Run(ctx, "init", repoA); err != nil {
		return nil, err
	}
	repoB := filepath.Join(env.root, "repoB")
	if err := env.git.Run(ctx, "init", "--bare", repoB); err != nil {
		return nil, err
	}

	gitA := env.git.WithDir(repoA)
	commit1, err := dummyRev(ctx, gitA, repoA, "master", "foo.txt", "initial commit")
	if err != nil {
		return nil, err
	}

	if err := gitA.Run(ctx, "remote", "add", "origin", repoB); err != nil {
		return nil, err
	}
	if err := gitA.Run(ctx, "push", "--set-upstream", "origin", "master"); err != nil {
		return nil, err
	}
	if r, err := gittool.ParseRev(ctx, gitA, "refs/remotes/origin/master"); err != nil {
		return nil, err
	} else if r.CommitHex() != commit1 {
		return nil, fmt.Errorf("source repository origin/master = %v; want %v", r.CommitHex(), commit1)
	}
	gitB := env.git.WithDir(repoB)
	if r, err := gittool.ParseRev(ctx, gitB, "refs/heads/master"); err != nil {
		return nil, err
	} else if r.CommitHex() != commit1 {
		return nil, fmt.Errorf("destination repository master = %v; want %v", r.CommitHex(), commit1)
	}

	commit2, err := dummyRev(ctx, gitA, repoA, "master", "bar.txt", "second commit")
	if err != nil {
		return nil, err
	}
	return &pushEnv{
		repoA:   repoA,
		repoB:   repoB,
		commit1: commit1,
		commit2: commit2,
	}, nil
}
