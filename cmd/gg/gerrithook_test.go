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
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

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
