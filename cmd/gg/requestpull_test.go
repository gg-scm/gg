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
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"
)

func TestRequestPull(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		branch      string
		upstreamURL string
		forkURL     string
		args        []string

		headOwner string
		headRef   string
		title     string
		body      string
	}{
		{
			name:        "Shared",
			branch:      "shared",
			upstreamURL: "https://github.com/example/foo.git",

			headOwner: "example",
			headRef:   "shared",
			title:     "Commit title",
			body:      "Commit description",
		},
		{
			name:        "Fork",
			branch:      "myfork",
			upstreamURL: "https://github.com/example/foo.git",
			forkURL:     "https://github.com/exampleuser/foo.git",

			headOwner: "exampleuser",
			headRef:   "myfork",
			title:     "Commit title",
			body:      "Commit description",
		},
		{
			name:        "ForkFromOtherBranch",
			branch:      "shared",
			upstreamURL: "https://github.com/example/foo.git",
			forkURL:     "https://github.com/exampleuser/foo.git",
			args:        []string{"myfork"},

			headOwner: "exampleuser",
			headRef:   "myfork",
			title:     "Commit title",
			body:      "Commit description",
		},
		{
			name:        "SetTitle",
			branch:      "shared",
			upstreamURL: "https://github.com/example/foo.git",
			args:        []string{"--title=Title from CLI"},

			headOwner: "example",
			headRef:   "shared",
			title:     "Title from CLI",
		},
		{
			name:        "SetTitleAndBody",
			branch:      "shared",
			upstreamURL: "https://github.com/example/foo.git",
			args:        []string{"--title=Title from CLI", "--body=Body from CLI"},

			headOwner: "example",
			headRef:   "shared",
			title:     "Title from CLI",
			body:      "Body from CLI",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()
			const authToken = "xyzzy12345"
			if err := env.writeGitHubAuth([]byte(authToken + "\n")); err != nil {
				t.Fatal(err)
			}
			api := &fakeGitHubPullRequestAPI{
				logger:         t,
				errorer:        t,
				permittedToken: authToken,
			}
			fakeGitHub := httptest.NewServer(api)
			defer fakeGitHub.Close()
			fakeGitHubTransport := &http.Transport{
				DialTLS: func(network, addr string) (net.Conn, error) {
					hostport := strings.TrimPrefix(fakeGitHub.URL, "http://")
					return net.Dial("tcp", hostport)
				},
			}
			defer fakeGitHubTransport.CloseIdleConnections()
			env.roundTripper = fakeGitHubTransport

			if err := env.initRepoWithHistory(ctx, "origin"); err != nil {
				t.Fatal(err)
			}
			if err := env.git.Run(ctx, "clone", "--quiet", "origin", "local"); err != nil {
				t.Fatal(err)
			}
			localDir := env.rel("local")
			if err := env.git.WithDir(localDir).Run(ctx, "branch", "--track", "shared", "origin/master"); err != nil {
				t.Fatal(err)
			}
			if err := env.git.WithDir(localDir).Run(ctx, "branch", "--track", "myfork", "origin/master"); err != nil {
				t.Fatal(err)
			}
			if err := env.git.WithDir(localDir).Run(ctx, "remote", "add", "forkremote", test.forkURL); err != nil {
				t.Fatal(err)
			}
			if err := env.git.WithDir(localDir).Run(ctx, "remote", "set-url", "origin", test.upstreamURL); err != nil {
				t.Fatal(err)
			}
			if test.forkURL != "" {
				if err := env.git.WithDir(localDir).Run(ctx, "config", "branch.myfork.pushRemote", "forkremote"); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := env.git.WithDir(localDir).Run(ctx, "config", "--unset", "branch.myfork.pushRemote"); err != nil {
						t.Error(err)
					}
				}()
			}
			for _, b := range []string{"shared", "myfork"} {
				if err := env.git.WithDir(localDir).Run(ctx, "checkout", "--quiet", b); err != nil {
					t.Fatal(err)
				}
				if err := env.newFile("local/blah.txt"); err != nil {
					t.Fatal(err)
				}
				if err := env.addFiles(ctx, "local/blah.txt"); err != nil {
					t.Fatal(err)
				}
				if err := env.git.WithDir(localDir).Run(ctx, "commit", "-m", "Commit title\n\nCommit description"); err != nil {
					t.Fatal(err)
				}
			}
			if err := env.git.WithDir(localDir).Run(ctx, "checkout", "--quiet", test.branch); err != nil {
				t.Fatal(err)
			}

			args := append([]string{"requestpull", "--edit=0"}, test.args...)
			if _, err := env.gg(ctx, localDir, args...); err != nil {
				t.Fatal(err)
			}
			api.mu.Lock()
			prs := api.prs
			api.prs = nil
			api.mu.Unlock()
			if len(prs) == 0 {
				t.Fatal("No PRs created")
			}
			if len(prs) > 1 {
				t.Errorf("Created %d PRs; want 1. Only looking at first.", len(prs))
			}
			if prs[0].owner != "example" || prs[0].repo != "foo" {
				t.Errorf("Opened on %s/%s; want example/foo", prs[0].owner, prs[0].repo)
			}
			if got, want := prs[0].baseRef, "master"; got != want {
				t.Errorf("Base ref = %q; want %q", got, want)
			}
			if got, want := prs[0].headOwner, test.headOwner; got != want {
				t.Errorf("Head owner = %q; want %q", got, want)
			}
			if got, want := prs[0].headRef, test.headRef; got != want {
				t.Errorf("Head ref = %q; want %q", got, want)
			}
			if got, want := prs[0].title, test.title; got != want {
				t.Errorf("Title = %q; want %q", got, want)
			}
			if got, want := prs[0].body, test.body; got != want {
				t.Errorf("Body = %q; want %q", got, want)
			}
			if got, want := prs[0].maintainerCanModify, true; got != want {
				t.Errorf("Maintainer can modify = %t; want %t", got, want)
			}
		})
	}
}

func TestRequestPull_BodyWithoutTitleUsageError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	const authToken = "xyzzy12345"
	if err := env.writeGitHubAuth([]byte(authToken + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.initRepoWithHistory(ctx, "origin"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "clone", "--quiet", "origin", "local"); err != nil {
		t.Fatal(err)
	}
	localDir := env.rel("local")
	if err := env.git.WithDir(localDir).Run(ctx, "remote", "set-url", "origin", "https://github.com/example/foo.git"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.WithDir(localDir).Run(ctx, "checkout", "--quiet", "--track", "-b", "feature", "origin/master"); err != nil {
		t.Fatal(err)
	}
	if err := env.newFile("local/blah.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "local/blah.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "local"); err != nil {
		t.Fatal(err)
	}

	api := &fakeGitHubPullRequestAPI{
		logger:         t,
		errorer:        t,
		permittedToken: authToken,
	}
	fakeGitHub := httptest.NewServer(api)
	defer fakeGitHub.Close()
	fakeGitHubTransport := &http.Transport{
		DialTLS: func(network, addr string) (net.Conn, error) {
			hostport := strings.TrimPrefix(fakeGitHub.URL, "http://")
			return net.Dial("tcp", hostport)
		},
	}
	defer fakeGitHubTransport.CloseIdleConnections()
	env.roundTripper = fakeGitHubTransport

	if _, err := env.gg(ctx, localDir, "requestpull", "--body=ohai"); err == nil {
		t.Error("gg did not return error")
	} else if !isUsage(err) {
		t.Errorf("Error = %v; want usage", err)
	}
	api.mu.Lock()
	prs := api.prs
	api.prs = nil
	api.mu.Unlock()
	if len(prs) > 0 {
		t.Errorf("Created %d PRs; want 0", len(prs))
	}
}

func TestRequestPull_Editor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env, err := newTestEnv(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	defer env.cleanup()
	const authToken = "xyzzy12345"
	if err := env.writeGitHubAuth([]byte(authToken + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := env.initRepoWithHistory(ctx, "origin"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.Run(ctx, "clone", "--quiet", "origin", "local"); err != nil {
		t.Fatal(err)
	}
	localDir := env.rel("local")
	if err := env.git.WithDir(localDir).Run(ctx, "remote", "set-url", "origin", "https://github.com/example/foo.git"); err != nil {
		t.Fatal(err)
	}
	if err := env.git.WithDir(localDir).Run(ctx, "checkout", "--quiet", "--track", "-b", "feature", "origin/master"); err != nil {
		t.Fatal(err)
	}
	if err := env.newFile("local/blah.txt"); err != nil {
		t.Fatal(err)
	}
	if err := env.addFiles(ctx, "local/blah.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.newCommit(ctx, "local"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		editorContent string
		title         string
		body          string
		fail          bool
	}{
		{
			name:          "Empty",
			editorContent: "",
			fail:          true,
		},
		{
			name:          "OnlyWhiteSpace",
			editorContent: "      \n  \n",
			fail:          true,
		},
		{
			name:          "TitleEmpty",
			editorContent: "\nMore content",
			fail:          true,
		},
		{
			name:          "TitleOnlySpace",
			editorContent: "    \nMore content",
			fail:          true,
		},
		{
			name:          "OneLineNoEOL",
			editorContent: "Hello, World",
			title:         "Hello, World",
		},
		{
			name:          "OneLine",
			editorContent: "Hello, World\n",
			title:         "Hello, World",
		},
		{
			name:          "TwoLines",
			editorContent: "Hello, World\nThis is TMI",
			title:         "Hello, World",
			body:          "This is TMI",
		},
		{
			name:          "TwoLinesWithBlankSep",
			editorContent: "Hello, World\n\nThis is TMI",
			title:         "Hello, World",
			body:          "This is TMI",
		},
		{
			name:          "ThreeLines",
			editorContent: "Hello, World\nThis is TMI\nAnd even more\n",
			title:         "Hello, World",
			body:          "This is TMI\nAnd even more",
		},
		{
			name:          "Indented",
			editorContent: "Hello, World\n\n\n\tThis is TMI\n\tAnd even more\n",
			title:         "Hello, World",
			body:          "\tThis is TMI\n\tAnd even more",
		},
		{
			name:          "Comment",
			editorContent: "Hello, World\n# First line is the title, rest is body.\n",
			title:         "Hello, World",
		},
		{
			name:          "OnlyComments",
			editorContent: "# First line is the title, rest is body.\n",
			fail:          true,
		},
		{
			name:          "CommentFirstLine",
			editorContent: "# First line is the title, rest is body.\nHello, World",
			title:         "Hello, World",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			api := &fakeGitHubPullRequestAPI{
				logger:         t,
				errorer:        t,
				permittedToken: authToken,
			}
			fakeGitHub := httptest.NewServer(api)
			defer fakeGitHub.Close()
			fakeGitHubTransport := &http.Transport{
				DialTLS: func(network, addr string) (net.Conn, error) {
					hostport := strings.TrimPrefix(fakeGitHub.URL, "http://")
					return net.Dial("tcp", hostport)
				},
			}
			defer fakeGitHubTransport.CloseIdleConnections()
			env.roundTripper = fakeGitHubTransport
			cmd, err := env.editorCmd([]byte(test.editorContent))
			if err != nil {
				t.Fatal(err)
			}
			config := fmt.Sprintf("[core]\neditor = %s\n", configEscape(cmd))
			if err := env.writeConfig([]byte(config)); err != nil {
				t.Fatal(err)
			}

			if _, err := env.gg(ctx, localDir, "requestpull", "--edit"); err != nil && !test.fail {
				t.Fatal(err)
			} else if err == nil && test.fail {
				t.Error()
			}
			api.mu.Lock()
			prs := api.prs
			api.prs = nil
			api.mu.Unlock()
			if len(prs) == 0 {
				if !test.fail {
					t.Error("No PRs created")
				}
				return
			}
			if test.fail {
				t.Fatalf("Created %d PRs; want 0", len(prs))
			}
			if len(prs) > 1 {
				t.Errorf("Created %d PRs; want 1. Only looking at first.", len(prs))
			}
			if got, want := prs[0].title, test.title; got != want {
				t.Errorf("Title = %q; want %q", got, want)
			}
			if got, want := prs[0].body, test.body; got != want {
				t.Errorf("Body = %q; want %q", got, want)
			}
		})
	}
}

type fakePullRequest struct {
	id        int64
	num       int
	owner     string
	repo      string
	baseRef   string
	headOwner string
	headRef   string

	title string
	body  string

	maintainerCanModify bool
}

type fakeGitHubPullRequestAPI struct {
	logger         logger
	errorer        errorer
	permittedToken string

	mu  sync.Mutex
	prs []fakePullRequest
}

func (api *fakeGitHubPullRequestAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	pathParts := strings.Split(strings.TrimPrefix(path.Clean(r.URL.Path), "/"), "/")
	if r.Method != "POST" || len(pathParts) != 4 || pathParts[0] != "repos" || pathParts[3] != "pulls" || r.Host != "api.github.com" {
		api.logger.Logf("%s received unhandled API request %s %s", r.Host, r.Method, r.URL.Path)
		http.Error(w, `{"message":"Not implemented"}`, http.StatusNotFound)
		return
	}
	if got, want := r.Header.Get("Accept"), "application/vnd.github.v3+json"; got != want {
		api.errorer.Errorf("Accept header = %q; want %q", got, want)
	}
	if got, want := r.Header.Get("Content-Type"), "application/json"; parseContentType(got) != want {
		api.errorer.Errorf("Content-Type header = %q; want %q", got, want)
	}
	if got, want := r.Header.Get("Authorization"), "token "+api.permittedToken; got != want {
		api.errorer.Errorf("Authorization header = %q; want %q", got, want)
		http.Error(w, `{"message":"Bad auth token"}`, http.StatusUnauthorized)
		return
	}
	var body map[string]interface{} // Struct field matches are always case-insensitive.
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.errorer.Errorf("Decode body: %v", err)
		http.Error(w, `{"message":"Could not parse body"}`, http.StatusBadRequest)
		return
	}
	owner := pathParts[1]
	repo := pathParts[2]
	title, head, base := jsonString(body["title"]), jsonString(body["head"]), jsonString(body["base"])
	if title == "" || head == "" || base == "" {
		api.errorer.Errorf("Missing one or more of the required fields: title = %q, head = %q, base = %q", title, head, base)
		http.Error(w, `{"message":"Missing required fields"}`, http.StatusUnprocessableEntity)
		return
	}
	headOwner, headRef := owner, head
	if i := strings.IndexByte(head, ':'); i != -1 {
		headOwner, headRef = head[:i], head[i+1:]
	}
	api.mu.Lock()
	id := int64(12345 + len(api.prs))
	num := 1 + len(api.prs)
	api.prs = append(api.prs, fakePullRequest{
		id:                  id,
		num:                 num,
		owner:               owner,
		repo:                repo,
		baseRef:             base,
		headOwner:           headOwner,
		headRef:             headRef,
		title:               title,
		body:                jsonString(body["body"]),
		maintainerCanModify: jsonBool(body["maintainer_can_modify"], true),
	})
	api.mu.Unlock()

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, num)
	response, err := json.Marshal(map[string]interface{}{
		"id":       id,
		"number":   num,
		"url":      url,
		"html_url": fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, num),
		"state":    "open",
		"title":    title,
		"body":     body,
		"head": map[string]interface{}{
			"ref": headRef,
			"user": map[string]interface{}{
				"login": headOwner,
			},
			"repo": map[string]interface{}{
				// This is mostly to shake out anything that depends on this to be the same.
				"name": repo + "asdf",
			},
		},
		"base": map[string]interface{}{
			"ref": base,
			"user": map[string]interface{}{
				"login": owner,
			},
			"repo": map[string]interface{}{
				"name": repo,
			},
		},
	})
	if err != nil {
		api.errorer.Errorf("Failed to marshal API response: %v")
		http.Error(w, `{"message":"Server errror"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", fmt.Sprint(len(response)))
	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write(response); err != nil {
		api.errorer.Errorf("Writing response: %v", err)
	}
}

func parseContentType(s string) string {
	t, _, err := mime.ParseMediaType(s)
	if err != nil {
		return ""
	}
	return t
}

func jsonString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func jsonBool(v interface{}, def bool) bool {
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

type logger interface {
	Logf(string, ...interface{})
}

func TestInferPullRequestMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		messages []string
		title    string
		body     string
		err      bool
	}{
		{
			name:     "NoCommits",
			messages: nil,
			err:      true,
		},
		{
			name:     "OneCommitNoDescription",
			messages: []string{"Hello World"},
			title:    "Hello World",
		},
		{
			name:     "OneCommit",
			messages: []string{"Hello World\n\nThis is an extended description\nspanning many lines."},
			title:    "Hello World",
			body:     "This is an extended description\nspanning many lines.",
		},
		{
			name: "TwoCommitsNoFirstDescription",
			messages: []string{
				"Hello World",
				"Test 1 2",
			},
			title: "Hello World",
			body:  "* Test 1 2",
		},
		{
			name: "TwoCommits",
			messages: []string{
				"Hello World\n\nGoodbye",
				"Test 1 2",
			},
			title: "Hello World",
			body:  "Goodbye\n\n* Test 1 2",
		},
		{
			name: "ThreeCommitsNoFirstDescription",
			messages: []string{
				"Hello World",
				"Test 1 2",
				"Test 3",
			},
			title: "Hello World",
			body:  "* Test 1 2\n\n* Test 3",
		},
		{
			name: "ThreeCommits",
			messages: []string{
				"Hello World\n\nEggs and bacon",
				"Test 1 2",
				"Test 3",
			},
			title: "Hello World",
			body:  "Eggs and bacon\n\n* Test 1 2\n\n* Test 3",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
			defer env.cleanup()
			if err := env.initRepoWithHistory(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.git.Run(ctx, "branch", "--track", "feature", "master"); err != nil {
				t.Fatal(err)
			}
			if err := env.newFile("mainline.txt"); err != nil {
				t.Fatal(err)
			}
			if err := env.addFiles(ctx, "mainline.txt"); err != nil {
				t.Fatal(err)
			}
			if _, err := env.newCommit(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.git.Run(ctx, "checkout", "--quiet", "feature"); err != nil {
				t.Fatal(err)
			}
			for i, msg := range test.messages {
				name := fmt.Sprintf("file%d.txt", i)
				if err := env.newFile(name); err != nil {
					t.Fatal(err)
				}
				if err := env.addFiles(ctx, name); err != nil {
					t.Fatal(err)
				}
				if err := env.git.Run(ctx, "commit", "-m", msg); err != nil {
					t.Fatal(err)
				}
			}

			title, body, err := inferPullRequestMessage(ctx, env.git, "master", "HEAD")
			if err != nil {
				if !test.err {
					t.Errorf("inferPullRequestMessage(...) = _, _, %v; want _, _, <nil>", err)
				}
				return
			}
			if test.err {
				t.Fatal("inferPullRequestMessage(...) = _, _, <nil>; want error")
			}
			if title != test.title || body != test.body {
				t.Errorf("inferPullRequestMessage(...) = %q, %q, <nil>; want %q, %q, <nil>", title, body, test.title, test.body)
			}
		})
	}
}

func TestParseGitHubRemoteURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url   string
		owner string
		repo  string
	}{
		{url: ""},
		{url: "https://github.com//"},
		{url: "https://github.com/foo/"},
		{url: "https://github.com//foo"},
		{url: "https://github.com/foo/bar", owner: "foo", repo: "bar"},
		{url: "https://github.com/foo/bar.git", owner: "foo", repo: "bar"},
		{url: "https://github.com:443/foo/bar", owner: "foo", repo: "bar"},
		{url: "https://github.com:443/foo/bar.git", owner: "foo", repo: "bar"},
		{url: "https://example.com/foo/bar.git"},
		{url: "https://github.com/foo/bar/baz"},
		{url: "https://github.com/?baz=foo/bar"},
		{url: "git@github.com:foo/bar.git", owner: "foo", repo: "bar"},
		{url: "git@github.com:foo/bar/baz.git"},
		{url: "git@github.com:/foo/bar.git"},
		{url: "github.com:foo/bar.git", owner: "foo", repo: "bar"},
		{url: "github.com:foo/bar/baz.git"},
		{url: "github.com:/foo/bar.git"},
		{url: "example.com:foo/bar.git"},
		{url: "ssh://git@github.com/foo/bar", owner: "foo", repo: "bar"},
		{url: "ssh://git@github.com/foo/bar.git", owner: "foo", repo: "bar"},
		{url: "ssh://github.com/foo/bar", owner: "foo", repo: "bar"},
		{url: "ssh://github.com/foo/bar.git", owner: "foo", repo: "bar"},
		{url: "ssh://git@github.com/foo/bar/baz.git"},
		{url: "ssh://example.com/foo/bar.git"},
		{url: "ssh://git@example.com/foo/bar.git"},
	}
	for _, test := range tests {
		owner, repo := parseGitHubRemoteURL(test.url)
		if owner != test.owner || repo != test.repo {
			t.Errorf("parseGitHubRemoteURL(%q) = %q, %q; want %q, %q", test.url, owner, repo, test.owner, test.repo)
		}
	}
}
