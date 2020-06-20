// Copyright 2018 The gg Authors
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
	"strconv"
	"strings"
	"sync"
	"testing"

	"gg-scm.io/pkg/internal/escape"
	"gg-scm.io/pkg/internal/filesystem"
	"gg-scm.io/pkg/internal/git"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
		reviewers []string
		draft     bool
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
		{
			name:        "OneReviewer",
			branch:      "shared",
			upstreamURL: "https://github.com/example/foo.git",
			args:        []string{"--reviewer", "zombiezen"},

			headOwner: "example",
			headRef:   "shared",
			title:     "Commit title",
			body:      "Commit description",
			reviewers: []string{"zombiezen"},
		},
		{
			name:        "TwoReviewers",
			branch:      "shared",
			upstreamURL: "https://github.com/example/foo.git",
			args:        []string{"--reviewer", "zombiezen", "--reviewer", "octocat"},

			headOwner: "example",
			headRef:   "shared",
			title:     "Commit title",
			body:      "Commit description",
			reviewers: []string{"octocat", "zombiezen"},
		},
		{
			name:        "CommaSeparatedReviewers",
			branch:      "shared",
			upstreamURL: "https://github.com/example/foo.git",
			args:        []string{"--reviewer", "zombiezen,octocat"},

			headOwner: "example",
			headRef:   "shared",
			title:     "Commit title",
			body:      "Commit description",
			reviewers: []string{"octocat", "zombiezen"},
		},
		{
			name:        "Draft",
			branch:      "shared",
			upstreamURL: "https://github.com/example/foo.git",
			args:        []string{"--draft"},

			headOwner: "example",
			headRef:   "shared",
			title:     "Commit title",
			body:      "Commit description",
			draft:     true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			env, err := newTestEnv(ctx, t)
			if err != nil {
				t.Fatal(err)
			}
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
			localDir := env.root.FromSlash("local")
			localGit := env.git.WithDir(localDir)
			if err := localGit.NewBranch(ctx, "shared", git.BranchOptions{StartPoint: "origin/main", Track: true}); err != nil {
				t.Fatal(err)
			}
			if err := localGit.NewBranch(ctx, "myfork", git.BranchOptions{StartPoint: "origin/main", Track: true}); err != nil {
				t.Fatal(err)
			}
			if err := localGit.Run(ctx, "remote", "add", "forkremote", test.forkURL); err != nil {
				t.Fatal(err)
			}
			if err := localGit.Run(ctx, "remote", "set-url", "origin", test.upstreamURL); err != nil {
				t.Fatal(err)
			}
			if test.forkURL != "" {
				if err := localGit.Run(ctx, "config", "branch.myfork.pushRemote", "forkremote"); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := localGit.Run(ctx, "config", "--unset", "branch.myfork.pushRemote"); err != nil {
						t.Error(err)
					}
				}()
			}
			for _, b := range []string{"shared", "myfork"} {
				if err := localGit.CheckoutBranch(ctx, b, git.CheckoutOptions{}); err != nil {
					t.Fatal(err)
				}
				if err := env.root.Apply(filesystem.Write("local/blah.txt", dummyContent)); err != nil {
					t.Fatal(err)
				}
				if err := env.addFiles(ctx, "local/blah.txt"); err != nil {
					t.Fatal(err)
				}
				if err := localGit.Commit(ctx, "Commit title\n\nCommit description", git.CommitOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			if err := localGit.CheckoutBranch(ctx, test.branch, git.CheckoutOptions{}); err != nil {
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
			if got, want := prs[0].baseRef, "main"; got != want {
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
			if got, want := prs[0].draft, test.draft; got != want {
				t.Errorf("Draft = %t; want %t", got, want)
			}
			if got, want := prs[0].maintainerCanModify, true; got != want {
				t.Errorf("Maintainer can modify = %t; want %t", got, want)
			}
			sortStrings := cmpopts.SortSlices(func(s1, s2 string) bool {
				return s1 < s2
			})
			if got, want := prs[0].reviewers, test.reviewers; !cmp.Equal(got, want, sortStrings, cmpopts.EquateEmpty()) {
				t.Errorf("Reviewers list = %q; want %q", got, want)
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
	localDir := env.root.FromSlash("local")
	localGit := env.git.WithDir(localDir)
	if err := localGit.Run(ctx, "remote", "set-url", "origin", "https://github.com/example/foo.git"); err != nil {
		t.Fatal(err)
	}
	err = localGit.NewBranch(ctx, "feature", git.BranchOptions{
		StartPoint: "origin/main",
		Track:      true,
		Checkout:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("local/blah.txt", dummyContent)); err != nil {
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
	localDir := env.root.FromSlash("local")
	localGit := env.git.WithDir(localDir)
	if err := localGit.Run(ctx, "remote", "set-url", "origin", "https://github.com/example/foo.git"); err != nil {
		t.Fatal(err)
	}
	if err := localGit.NewBranch(ctx, "feature", git.BranchOptions{StartPoint: "origin/main", Track: true}); err != nil {
		t.Fatal(err)
	}
	if err := env.root.Apply(filesystem.Write("local/blah.txt", dummyContent)); err != nil {
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
			config := fmt.Sprintf("[core]\neditor = %s\n", escape.GitConfig(cmd))
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

	title     string
	body      string
	reviewers []string

	draft               bool
	maintainerCanModify bool
}

type fakeGitHubPullRequestAPI struct {
	logger         logger
	errorer        errorer
	permittedToken string

	mu  sync.Mutex
	prs []fakePullRequest
}

const testDraftAccept = "application/vnd.github.shadow-cat-preview+json"

func (api *fakeGitHubPullRequestAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Host == "api.github.com" {
		if got, want := r.Header.Get("Authorization"), "token "+api.permittedToken; got != want {
			api.errorer.Errorf("Authorization header = %q; want %q", got, want)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			http.Error(w, `{"message":"Bad auth token"}`, http.StatusUnauthorized)
			return
		}
		if got, want := r.Header.Get("Accept"), "application/vnd.github.v3+json"; got != want && got != draftPRAPIAccept {
			api.errorer.Errorf("Accept header = %q; want %q or %q", got, want, draftPRAPIAccept)
		}
		pathParts := strings.Split(strings.TrimPrefix(path.Clean(r.URL.Path), "/"), "/")
		switch {
		case r.Method == "POST" && len(pathParts) == 4 && pathParts[0] == "repos" && pathParts[3] == "pulls":
			api.createPullRequest(w, r, pathParts)
			return
		case r.Method == "POST" && len(pathParts) == 6 && pathParts[0] == "repos" && pathParts[3] == "pulls" && pathParts[5] == "requested_reviewers":
			api.createReviewRequest(w, r, pathParts)
			return
		}
	}
	api.logger.Logf("%s received unhandled API request %s %s", r.Host, r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	http.Error(w, `{"message":"Not implemented"}`, http.StatusNotFound)
}

func (api *fakeGitHubPullRequestAPI) createPullRequest(w http.ResponseWriter, r *http.Request, pathParts []string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if got, want := r.Header.Get("Content-Type"), "application/json"; parseContentType(got) != want {
		api.errorer.Errorf("Content-Type header = %q; want %q", got, want)
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
	draft := jsonBool(body["draft"], false)
	if got := r.Header.Get("Accept"); draft && got != draftPRAPIAccept {
		api.errorer.Errorf("draft unavailable with Accept = %q; want %q", got, draftPRAPIAccept)
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
		draft:               draft,
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
	w.Header().Set("Content-Length", fmt.Sprint(len(response)))
	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write(response); err != nil {
		api.errorer.Errorf("Writing response: %v", err)
	}
}

func (api *fakeGitHubPullRequestAPI) createReviewRequest(w http.ResponseWriter, r *http.Request, pathParts []string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if got, want := r.Header.Get("Content-Type"), "application/json"; parseContentType(got) != want {
		api.errorer.Errorf("Content-Type header = %q; want %q", got, want)
	}
	var body map[string]interface{} // Struct field matches are always case-insensitive.
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.errorer.Errorf("Decode body: %v", err)
		http.Error(w, `{"message":"Could not parse body"}`, http.StatusBadRequest)
		return
	}
	owner := pathParts[1]
	repo := pathParts[2]
	pathNum := pathParts[4]
	num, err := strconv.ParseUint(pathNum, 10, 64)
	if err != nil {
		api.errorer.Errorf("PR # = %q; error: %v", pathNum, err)
		http.Error(w, `{"message":"Invalid pull request #"}`, http.StatusNotFound)
		return
	}
	reviewers := jsonStringArray(body["reviewers"])
	api.mu.Lock()
	for i := range api.prs {
		pr := &api.prs[i]
		if pr.owner == owner && pr.repo == repo && uint64(pr.num) == num {
			pr.reviewers = append(pr.reviewers, reviewers...)
			break
		}
	}
	api.mu.Unlock()

	response, err := json.Marshal(map[string]interface{}{
		"number":   num,
		"url":      fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, num),
		"html_url": fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, num),
		"state":    "open",
	})
	if err != nil {
		api.errorer.Errorf("Failed to marshal API response: %v")
		http.Error(w, `{"message":"Server errror"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Length", fmt.Sprint(len(response)))
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

func jsonStringArray(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	slice := make([]string, len(arr))
	for i := range slice {
		slice[i] = jsonString(arr[i])
	}
	return slice
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
			if err := env.initRepoWithHistory(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.git.NewBranch(ctx, "feature", git.BranchOptions{Track: true, StartPoint: "main"}); err != nil {
				t.Fatal(err)
			}
			if err := env.root.Apply(filesystem.Write("mainline.txt", dummyContent)); err != nil {
				t.Fatal(err)
			}
			if err := env.addFiles(ctx, "mainline.txt"); err != nil {
				t.Fatal(err)
			}
			if _, err := env.newCommit(ctx, "."); err != nil {
				t.Fatal(err)
			}
			if err := env.git.CheckoutBranch(ctx, "feature", git.CheckoutOptions{}); err != nil {
				t.Fatal(err)
			}
			for i, msg := range test.messages {
				name := fmt.Sprintf("file%d.txt", i)
				if err := env.root.Apply(filesystem.Write(name, dummyContent)); err != nil {
					t.Fatal(err)
				}
				if err := env.addFiles(ctx, name); err != nil {
					t.Fatal(err)
				}
				if err := env.git.Commit(ctx, msg, git.CommitOptions{}); err != nil {
					t.Fatal(err)
				}
			}

			title, body, err := inferPullRequestMessage(ctx, env.git, "main", "HEAD")
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
