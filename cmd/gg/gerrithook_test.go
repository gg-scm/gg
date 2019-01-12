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

package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"gg-scm.io/pkg/internal/filesystem"
)

func TestGerritHook(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("Off", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.git.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		if err := env.root.Apply(filesystem.Write(".git/hooks/commit-msg", dummyContent)); err != nil {
			t.Fatal(err)
		}

		if _, err := env.gg(ctx, env.root.String(), "gerrithook", "off"); err != nil {
			t.Error(err)
		}
		if exists, err := env.root.Exists(".git/hooks/commit-msg"); err != nil {
			t.Error(err)
		} else if exists {
			t.Error(".git/hooks/commit-msg still exists")
		}
	})
	t.Run("On", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.git.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		const wantContent = "#!/bin/bash\necho Hello World\n"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if want := "/foo/commit-msg"; r.URL.Path != want {
				t.Errorf("HTTP path = %q; want %q", r.URL.Path, want)
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(wantContent)))
			io.WriteString(w, wantContent)
		}))
		defer srv.Close()
		env.roundTripper = srv.Client().Transport

		if _, err := env.gg(ctx, env.root.String(), "gerrithook", "--url="+srv.URL+"/foo/commit-msg", "on"); err != nil {
			t.Error(err)
		}
		if got, err := env.root.ReadFile(".git/hooks/commit-msg"); err != nil {
			t.Error(err)
		} else if got != wantContent {
			t.Errorf(".git/hooks/commit-msg content = %q; want %q", got, wantContent)
		}
	})
	t.Run("Cached", func(t *testing.T) {
		env, err := newTestEnv(ctx, t)
		if err != nil {
			t.Fatal(err)
		}
		defer env.cleanup()
		if err := env.git.Init(ctx, "."); err != nil {
			t.Fatal(err)
		}
		const wantContent = "#!/bin/bash\necho Hello World\n"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if want := "/foo/commit-msg"; r.URL.Path != want {
				t.Errorf("HTTP path = %q; want %q", r.URL.Path, want)
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(wantContent)))
			io.WriteString(w, wantContent)
		}))
		defer srv.Close()

		// First time to cache it.
		env.roundTripper = srv.Client().Transport
		scriptURL := srv.URL + "/foo/commit-msg"
		if _, err := env.gg(ctx, env.root.String(), "gerrithook", "--url="+scriptURL, "on"); err != nil {
			t.Error(err)
		}
		if got, err := env.root.ReadFile(".git/hooks/commit-msg"); err != nil {
			t.Error("After first gerrithook on:", err)
		} else if got != wantContent {
			t.Errorf(".git/hooks/commit-msg content after first gerrithook = %q; want %q", got, wantContent)
		}

		// Remove the hook between runs.
		if err := env.root.Apply(filesystem.Remove(".git/hooks/commit-msg")); err != nil {
			t.Fatal(err)
		}

		// Run gerrithook again with roundTripper stubbed (simulating a network drop).
		env.roundTripper = stubRoundTripper{}
		if _, err := env.gg(ctx, env.root.String(), "gerrithook", "--url="+scriptURL, "on"); err != nil {
			t.Error(err)
		}
		if got, err := env.root.ReadFile(".git/hooks/commit-msg"); err != nil {
			t.Error(err)
		} else if got != wantContent {
			t.Errorf(".git/hooks/commit-msg content after second gerrithook = %q; want %q", got, wantContent)
		}
	})
}

func TestCommitMsgHookPath(t *testing.T) {
	topDir, commonDir, other := "/foo", "/foo/.git", "/bar"
	if runtime.GOOS == "windows" {
		topDir, commonDir, other = `C:\foo`, `C:\foo\.git`, `C:\bar`
	}
	tests := []struct {
		name string
		cfg  dummyConfig
		want string
	}{
		{
			name: "Default",
			want: filepath.Join(commonDir, "hooks", "commit-msg"),
		},
		{
			name: "HooksPathAbsolute",
			cfg:  dummyConfig{"core.hooksPath": other},
			want: other,
		},
		{
			name: "HooksPathRelative",
			cfg:  dummyConfig{"core.hooksPath": "baz"},
			want: filepath.Join(topDir, "baz", "commit-msg"),
		},
		{
			name: "HooksPathRelativeBare",
			cfg:  dummyConfig{"core.hooksPath": "baz", "core.bare": "true"},
			want: filepath.Join(commonDir, "baz", "commit-msg"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := commitMsgHookPath(context.Background(), test.cfg, dummyGitDirs{
				common: commonDir,
				top:    topDir,
			})
			if err != nil {
				t.Fatal("commitMsgHookPath error:", err)
			}
			if got != test.want {
				t.Errorf("commitMsgHookPath(...) = %q; want %q", got, test.want)
			}
		})
	}
}

type dummyConfig map[string]string

func (cfg dummyConfig) Value(key string) string {
	return cfg[key]
}

func (cfg dummyConfig) Bool(key string) (bool, error) {
	v := cfg[key]
	if v == "" {
		return false, nil
	}
	return strconv.ParseBool(v)
}

type dummyGitDirs struct {
	common string
	top    string
}

func (g dummyGitDirs) CommonDir(ctx context.Context) (string, error) {
	return g.common, nil
}

func (g dummyGitDirs) WorkTree(ctx context.Context) (string, error) {
	return g.top, nil
}
