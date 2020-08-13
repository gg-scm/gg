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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const requestPullSynopsis = "create a GitHub pull request"

func requestPull(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg requestpull [-n] [-e=0] [--title=MSG [--body=MSG]] [--draft] [-R user1[,user2]] [BRANCH]", requestPullSynopsis+`

aliases: pr

	Create a new GitHub pull request for the given branch (defaults to the
	one currently checked out). The source will be inferred from the
	branch's remote push information and the destination will be inferred
	from upstream fetch information. This command does not push any new
	commits; it just creates a pull request.

	Before sending the pull request, gg will open an editor with a summary
	of the commits it knows about. The first line will be the pull request
	title, and any subsequent lines will be used as the body. You can exit
	your editor without modifications to accept the default summary.

	For non-dry runs, you must create a [personal access token][] at
	https://github.com/settings/tokens/new and save it to
	`+"`$XDG_CONFIG_HOME/gg/github_token`"+` (or in any other directory
	in `+"`$XDG_CONFIG_DIRS`"+`). By default, this would be
	`+"`~/.config/gg/github_token`"+`. gg needs at least `+"`public_repo`"+` scope
	to be able to create pull requests, but you can grant `+"`repo`"+` scope to
	create pull requests in any repositories you have access to.

[personal access token]: https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/`)
	bodyFlag := f.String("body", "", "pull request `description` (requires --title)")
	draft := f.Bool("draft", false, "create a pull request as draft")
	edit := f.Bool("e", true, "invoke editor on pull request message (ignored if --title is specified)")
	f.Alias("e", "edit")
	dryRun := f.Bool("n", false, "prints the pull request instead of creating it")
	f.Alias("n", "dry-run")
	maintainerEdits := f.Bool("maintainer-edits", true, "allow maintainers to edit this branch")
	reviewers := f.MultiString("R", "GitHub `user`names of reviewers to add")
	f.Alias("R", "reviewer")
	titleFlag := f.String("title", "", "pull request title")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("only one branch allowed")
	}
	*titleFlag = strings.TrimSpace(*titleFlag)
	if *bodyFlag != "" && *titleFlag == "" {
		return usagef("cannot specify --body without specifying --title")
	}
	cfg, err := cc.git.ReadConfig(ctx)
	if err != nil {
		return err
	}
	var token []byte
	if !*dryRun {
		var err error
		token, err = cc.xdgDirs.readConfig("github_token")
		if os.IsNotExist(err) {
			fmt.Fprintln(cc.stderr, "Missing github_token config file. Generate a new GitHub personal access")
			fmt.Fprintln(cc.stderr, "token at https://github.com/settings/tokens/new?scopes=repo and save it to")
			fmt.Fprintln(cc.stderr, filepath.Join(cc.xdgDirs.configPaths()[0], "gg", "github_token")+".")
			return err
		}
		if err != nil {
			return err
		}
		token = bytes.TrimSpace(token)
	}

	// Find local branch name.
	var branch string
	if branchArg := f.Arg(0); branchArg == "" {
		branch = currentBranch(ctx, cc)
		if branch == "" {
			return errors.New("no branch currently checked out")
		}
	} else {
		rev, err := cc.git.ParseRev(ctx, branchArg)
		if err != nil {
			return err
		}
		branch = rev.Ref.Branch()
		if branch == "" {
			return fmt.Errorf("%s is not a branch", branchArg)
		}
	}

	// Find base repository and ref.
	baseRemote := cfg.Value("branch." + branch + ".remote")
	if baseRemote == "" {
		remotes := cfg.ListRemotes()
		if _, ok := remotes["origin"]; !ok {
			return errors.New("branch has no remote and no remote named \"origin\" found")
		}
		baseRemote = "origin"
	}
	baseURL := cfg.Value("remote." + baseRemote + ".url")
	baseOwner, baseRepo := parseGitHubRemoteURL(baseURL)
	if baseOwner == "" || baseRepo == "" {
		return fmt.Errorf("%s is not a GitHub repository", baseURL)
	}
	baseBranch := inferUpstream(cfg, branch).Branch()

	// Find head repository and ref.
	headRemote, err := inferPushRepo(cfg, branch)
	if err != nil {
		return err
	}
	headURL := cfg.Value("remote." + headRemote + ".pushurl")
	if headURL == "" {
		headURL = cfg.Value("remote." + headRemote + ".url")
	}
	headOwner, _ := parseGitHubRemoteURL(headURL)
	if headOwner == "" {
		return fmt.Errorf("%s is not a GitHub repository", headURL)
	}

	// Create pull request. Run message inference no matter what, since it
	// has the side effect of detecting no change.
	title, body, err := inferPullRequestMessage(ctx, cc.git, branch+"@{upstream}", branch)
	if err != nil {
		return err
	}
	if *titleFlag != "" {
		title, body = *titleFlag, *bodyFlag
	}
	if *dryRun {
		draftText := ""
		if *draft {
			draftText = "[DRAFT] "
		}
		_, err := fmt.Fprintf(cc.stdout, "%s%s/%s: %s\nMerge into %s:%s from %s:%s\n",
			draftText, baseOwner, baseRepo, title, baseOwner, baseBranch, headOwner, branch)
		if err != nil {
			return err
		}
		if body != "" {
			_, err = fmt.Fprintf(cc.stdout, "\n%s\n", body)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if *edit && *titleFlag == "" {
		editorInit := new(bytes.Buffer)
		editorInit.WriteString(title)
		if body != "" {
			editorInit.WriteString("\n\n")
			editorInit.WriteString(body)
		}
		editorInit.WriteString("\n# Please enter the pull request message. Lines starting with '#' will\n" +
			"# be ignored, and an empty message aborts the pull request. The first\n" +
			"# line will be used as the title and must not be empty.\n")
		fmt.Fprintf(editorInit, "# %s/%s: merge into %s:%s from %s:%s\n",
			baseOwner, baseRepo, baseOwner, baseBranch, headOwner, branch)
		newMsg, err := cc.editor.open(ctx, "PR_EDITMSG.md", editorInit.Bytes())
		if err != nil {
			return err
		}
		title, body, err = parseEditedPullRequestMessage(newMsg)
		if err != nil {
			return err
		}
	}
	prNum, prURL, err := createPullRequest(ctx, cc.httpClient, pullRequestParams{
		authToken:              string(token),
		baseOwner:              baseOwner,
		baseRepo:               baseRepo,
		baseBranch:             baseBranch,
		headOwner:              headOwner,
		headBranch:             branch,
		title:                  title,
		body:                   body,
		draft:                  *draft,
		disableMaintainerEdits: !*maintainerEdits,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cc.stdout, "Created pull request at %s\n", prURL)
	if err != nil {
		return err
	}
	if len(*reviewers) > 0 {
		var fullReviewers []string
		for _, r := range *reviewers {
			fullReviewers = append(fullReviewers, strings.Split(r, ",")...)
		}
		err := addPullRequestReviewers(ctx, cc.httpClient, pullRequestReviewParams{
			authToken: string(token),
			owner:     baseOwner,
			repo:      baseRepo,
			prNum:     prNum,
			users:     fullReviewers,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func inferPullRequestMessage(ctx context.Context, g *git.Git, base, head string) (title, body string, _ error) {
	// Read commit messages of divergent commits.
	commits, err := g.Log(ctx, git.LogOptions{
		Revs:        []string{base + ".." + head},
		Reverse:     true,
		MaxParents:  1,
		FirstParent: true,
	})
	if err != nil {
		return "", "", fmt.Errorf("infer PR message: %w", err)
	}
	bodyBuilder := new(strings.Builder)
	i := 0
	for ; commits.Next(); i++ {
		msg := commits.CommitInfo().Message
		if i == 0 {
			// First line of first commit message is the title.
			if j := strings.IndexByte(msg, '\n'); j != -1 {
				title = strings.TrimSpace(msg[:j])
				bodyBuilder.WriteString(strings.TrimSpace(msg[j+1:]))
			} else {
				title = string(strings.TrimSpace(msg))
			}
			continue
		}
		// Join rest of messages by bullets into body.
		bodyBuilder.WriteString("\n\n* ")
		bodyBuilder.WriteString(strings.TrimSpace(msg))
	}
	if err := commits.Close(); err != nil {
		return "", "", fmt.Errorf("infer PR message: %w", err)
	}
	if i == 0 {
		return "", "", errors.New("infer PR message: no divergent commits")
	}

	body = strings.TrimSpace(bodyBuilder.String())
	if template := readPullRequestTemplate(ctx, g); template != "" {
		body += "\n\n" + strings.TrimSpace(template)
	}
	return title, body, nil
}

func readPullRequestTemplate(ctx context.Context, g *git.Git) string {
	potential := []git.TopPath{
		"pull_request_template.md",
		"PULL_REQUEST_TEMPLATE/pull_request_template.md",
		"docs/pull_request_template.md",
		"docs/PULL_REQUEST_TEMPLATE/pull_request_template.md",
		".github/pull_request_template.md",
		".github/PULL_REQUEST_TEMPLATE/pull_request_template.md",
	}
	for _, p := range potential {
		rc, err := g.Cat(ctx, git.Head.String(), p)
		if err != nil {
			continue
		}
		content := new(strings.Builder)
		_, err = io.Copy(content, rc)
		rc.Close()
		if err != nil {
			continue
		}
		return content.String()
	}
	return ""
}

func parseEditedPullRequestMessage(b []byte) (title, body string, _ error) {
	// Split into lines.
	lines := bytes.Split(b, []byte{'\n'})
	// Strip comment lines.
	n := 0
	for i := range lines {
		if !bytes.HasPrefix(lines[i], []byte{'#'}) {
			lines[n] = lines[i]
			n++
		}
	}
	lines = lines[:n]
	// Abort on empty title.
	if len(lines) == 0 {
		return "", "", errors.New("pull request message is empty")
	}
	title = string(bytes.TrimSpace(lines[0]))
	if title == "" {
		return "", "", errors.New("pull request title is empty")
	}
	// Remove leading and trailing blank lines from body.
	lines = lines[1:]
	for len(lines) > 0 && len(bytes.TrimSpace(lines[0])) == 0 {
		lines = lines[1:]
	}
	for len(lines) > 0 && len(bytes.TrimSpace(lines[len(lines)-1])) == 0 {
		lines = lines[:len(lines)-1]
	}
	return title, string(bytes.Join(lines, []byte{'\n'})), nil
}

type pullRequestParams struct {
	authToken string

	baseOwner  string
	baseRepo   string
	baseBranch string

	headOwner  string
	headBranch string

	title string
	body  string

	draft                  bool
	disableMaintainerEdits bool
}

func createPullRequest(ctx context.Context, client *http.Client, params pullRequestParams) (prNum uint64, prURL string, _ error) {
	if params.authToken == "" {
		return 0, "", errors.New("create pull request: missing authentication token")
	}
	if params.baseOwner == "" || params.baseRepo == "" {
		return 0, "", errors.New("create pull request: missing base owner or repository name")
	}
	if params.baseBranch == "" {
		return 0, "", errors.New("create pull request: missing base branch")
	}
	if params.headOwner == "" || params.headBranch == "" {
		return 0, "", errors.New("create pull request: missing head branch or owner")
	}
	if params.title == "" {
		return 0, "", errors.New("create pull request: missing title")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls",
		url.PathEscape(params.baseOwner), url.PathEscape(params.baseRepo))
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return 0, "", fmt.Errorf("create pull request: %w", err)
	}
	req.Header.Set("User-Agent", userAgentString())
	req.Header.Set("Accept", draftPRAPIAccept)
	req.Header.Set("Authorization", "token "+params.authToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	reqBody := map[string]interface{}{
		"title":                 params.title,
		"base":                  params.baseBranch,
		"head":                  params.headOwner + ":" + params.headBranch,
		"maintainer_can_modify": !params.disableMaintainerEdits,
	}
	if params.body != "" {
		reqBody["body"] = params.body
	}
	if params.draft {
		reqBody["draft"] = true
	}
	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return 0, "", fmt.Errorf("create pull request: %w", err)
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(reqBodyJSON))

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return 0, "", fmt.Errorf("create pull request for %s/%s: %w", params.baseOwner, params.baseRepo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		err := parseGitHubErrorResponse(resp)
		return 0, "", fmt.Errorf("create pull request for %s/%s: %w", params.baseOwner, params.baseRepo, err)
	}
	var respDoc struct {
		Number  uint64
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respDoc); err != nil {
		return 0, "", fmt.Errorf("create pull request for %s/%s: parsing response: %w", params.baseOwner, params.baseRepo, err)
	}
	return respDoc.Number, respDoc.HTMLURL, nil
}

type pullRequestReviewParams struct {
	authToken string

	owner string
	repo  string
	prNum uint64
	users []string
}

func addPullRequestReviewers(ctx context.Context, client *http.Client, params pullRequestReviewParams) error {
	if params.authToken == "" {
		return errors.New("add pull request reviewers: missing authentication token")
	}
	if params.owner == "" || params.repo == "" {
		return errors.New("add pull request reviewers: missing repository owner or name")
	}
	if len(params.users) == 0 {
		return errors.New("add pull request reviewers: no reviewers to add")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/requested_reviewers",
		url.PathEscape(params.owner), url.PathEscape(params.repo), params.prNum)
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return fmt.Errorf("add pull request reviewers to %s/%s/pulls/%d: %w", params.owner, params.repo, params.prNum, err)
	}
	req.Header.Set("User-Agent", userAgentString())
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", "token "+params.authToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	reqBody := map[string]interface{}{
		"reviewers": params.users,
	}
	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("add pull request reviewers: %w", err)
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(reqBodyJSON))
	req.Header.Set("Content-Length", fmt.Sprint(len(reqBodyJSON)))

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("add pull request reviewers to %s/%s/pulls/%d: %w", params.owner, params.repo, params.prNum, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		err := parseGitHubErrorResponse(resp)
		return fmt.Errorf("add pull request reviewers to %s/%s/pulls/%d: %w", params.owner, params.repo, params.prNum, err)
	}
	return nil
}

// inferUpstream returns the default remote ref to pull from.
// localBranch may be empty.
func inferUpstream(cfg *git.Config, localBranch string) git.Ref {
	if localBranch == "" {
		return git.Head
	}
	merge := cfg.Value("branch." + localBranch + ".merge")
	if merge != "" {
		return git.Ref(merge)
	}
	return git.BranchRef(localBranch)
}

// draftPRAPIAccept is the media type that GitHub uses to enable the draft PR
// feature.
const draftPRAPIAccept = "application/vnd.github.shadow-cat-preview+json"

func parseGitHubErrorResponse(resp *http.Response) error {
	t, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || t != "application/json" {
		return fmt.Errorf("GitHub API HTTP %s", resp.Status)
	}
	var payload struct {
		Message string
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil || payload.Message == "" {
		return fmt.Errorf("GitHub API HTTP %s", resp.Status)
	}
	return fmt.Errorf("GitHub API HTTP %s: %s", resp.Status, payload.Message)
}

func parseGitHubRemoteURL(u string) (owner, repo string) {
	var path string
	switch {
	case strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "ssh://"):
		uu, err := url.Parse(u)
		if err != nil {
			return "", ""
		}
		if uu.Hostname() != "github.com" || uu.RawQuery != "" || uu.Fragment != "" {
			return "", ""
		}
		path = strings.TrimPrefix(uu.Path, "/")
	case strings.HasPrefix(u, "github.com:"):
		path = u[len("github.com:"):]
	case strings.HasPrefix(u, "git@github.com:"):
		path = u[len("git@github.com:"):]
	default:
		return "", ""
	}
	path = strings.TrimSuffix(path, ".git")
	i := strings.IndexByte(path, '/')
	if i == 0 || len(path)-i-1 == 0 {
		// One or part is empty.
		return "", ""
	}
	if i == -1 {
		return "", ""
	}
	if strings.Count(path[i+1:], "/") > 0 {
		return "", ""
	}
	return path[:i], path[i+1:]
}
