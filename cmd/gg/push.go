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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/gitobj"
	"gg-scm.io/pkg/internal/gittool"
	"gg-scm.io/pkg/internal/singleclose"
)

const pushSynopsis = "push changes to the specified destination"

func push(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg push [-f] [-n] [-r REV] [-d REF] [--create] [DST]", pushSynopsis+`

	When no destination repository is given, push uses the first non-
	empty configuration value of:

	1.  `+"`branch.*.pushRemote`"+`, if the source is a branch or is part of only
	    one branch.
	2.  `+"`remote.pushDefault`"+`.
	3.  `+"`branch.*.remote`"+`, if the source is a branch or is part of only one
	    branch.
	4.  Otherwise, the remote called `+"`origin`"+` is used.

	If `+"`-d`"+` is given and begins with `+"`refs/`"+`, then it specifies the remote
	ref to update. If the argument passed to `+"`-d`"+` does not begin with
	`+"`refs/`"+`, it is assumed to be a branch name (`+"`refs/heads/<arg>`"+`).
	If `+"`-d`"+` is not given and the source is a ref or part of only one local
	branch, then the same ref name is used. Otherwise, push exits with a
	failure exit code. This differs from git, which will consult
	`+"`remote.*.push`"+` and `+"`push.default`"+`. You can imagine this being the most
	similar to `+"`push.default=current`"+`.

	By default, `+"`gg push`"+` will fail instead of creating a new ref on the
	remote. If this is desired (e.g. you are creating a new branch), then
	you can pass `+"`--create`"+` to override this check.`)
	create := f.Bool("create", false, "allow pushing a new ref")
	dstRefArg := f.String("d", "", "destination `ref`")
	f.Alias("d", "dest")
	force := f.Bool("f", false, "allow overwriting ref if it is not an ancestor, as long as it matches the remote-tracking branch")
	dryRun := f.Bool("n", false, "do everything except send the changes")
	f.Alias("n", "dry-run")
	rev := f.String("r", gitobj.Head.String(), "source `rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("can't pass multiple destinations")
	}
	src, err := gittool.ParseRev(ctx, cc.git, *rev)
	if err != nil {
		return err
	}
	srcRef := src.Ref()
	if srcRef == "" {
		possible, err := branchesContaining(ctx, cc.git, src.Commit().String())
		if err == nil && len(possible) == 1 {
			srcRef = possible[0]
		}
	}
	dstRepo := f.Arg(0)
	if dstRepo == "" {
		cfg, err := gittool.ReadConfig(ctx, cc.git)
		if err != nil {
			return err
		}
		dstRepo, err = inferPushRepo(ctx, cc.git, cfg, srcRef.Branch())
		if err != nil {
			return err
		}
	}
	var dstRef gitobj.Ref
	switch {
	case *dstRefArg == "":
		if !srcRef.IsValid() {
			return errors.New("cannot infer destination (source is not a ref). Use -d to specify destination ref.")
		}
		dstRef = srcRef
	case strings.HasPrefix(*dstRefArg, "refs/"):
		dstRef = gitobj.Ref(*dstRefArg)
	default:
		dstRef = gitobj.BranchRef(*dstRefArg)
	}
	if !*create {
		if err := verifyPushRemoteRef(ctx, cc.git, dstRepo, dstRef); err != nil {
			return err
		}
	}
	var pushArgs []string
	pushArgs = append(pushArgs, "push")
	if *force {
		pushArgs = append(pushArgs, "--force-with-lease")
	}
	if *dryRun {
		pushArgs = append(pushArgs, "--dry-run")
	}
	pushArgs = append(pushArgs, "--", dstRepo, src.Commit().String()+":"+dstRef.String())
	return cc.git.RunInteractive(ctx, pushArgs...)
}

const mailSynopsis = "creates or updates a Gerrit change"

func mail(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg mail [options] [DST]", mailSynopsis)
	allowDirty := f.Bool("allow-dirty", false, "allow mailing when working copy has uncommitted changes")
	dstBranch := f.String("d", "", "destination `branch`")
	f.Alias("d", "dest", "for")
	rev := f.String("r", gitobj.Head.String(), "source `rev`ision")
	gopts := new(gerritOptions)
	f.MultiStringVar(&gopts.reviewers, "R", "reviewer `email`")
	f.Alias("R", "reviewer")
	f.MultiStringVar(&gopts.cc, "CC", "`email`s to CC")
	f.Alias("CC", "cc")
	f.StringVar(&gopts.notify, "notify", "", `who to send email notifications to; one of "none", "owner", "owner_reviewers", or "all"`)
	f.MultiStringVar(&gopts.notifyTo, "notify-to", "`email` to send notification")
	f.MultiStringVar(&gopts.notifyCC, "notify-cc", "`email` to CC notification")
	f.MultiStringVar(&gopts.notifyBCC, "notify-bcc", "`email` to BCC notification")
	f.StringVar(&gopts.message, "m", "", "use text as comment `message`")
	f.BoolVar(&gopts.publishComments, "p", false, "publish draft comments")
	f.Alias("p", "publish-comments")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("can't pass multiple destinations")
	}
	if strings.HasPrefix(*dstBranch, "refs/") && !strings.HasPrefix(*dstBranch, "refs/for/") || strings.Contains(*dstBranch, "%") {
		return usagef("-d argument must be a branch")
	}
	gopts.notify = strings.ToUpper(gopts.notify)
	if gopts.notify != "" && gopts.notify != "NONE" && gopts.notify != "OWNER" && gopts.notify != "OWNER_REVIEWERS" && gopts.notify != "ALL" {
		return usagef(`--notify must be one of "none", "owner", "owner_reviewers", or "all"`)
	}
	src, err := gittool.ParseRev(ctx, cc.git, *rev)
	if err != nil {
		return err
	}
	srcBranch := src.Ref().Branch()
	if srcBranch == "" {
		possible, err := branchesContaining(ctx, cc.git, src.Commit().String())
		if err == nil && len(possible) == 1 {
			srcBranch = possible[0].Branch()
		}
	}
	if !*allowDirty {
		clean, err := isClean(ctx, cc.git)
		if err != nil {
			return err
		}
		if !clean {
			return errors.New("working copy has uncommitted changes. " +
				"Either commit them, stash them, or use gg mail --allow-dirty if this is intentional.")
		}
	}
	dstRepo := f.Arg(0)
	var cfg *gittool.Config
	if dstRepo == "" || *dstBranch == "" {
		var err error
		cfg, err = gittool.ReadConfig(ctx, cc.git)
		if err != nil {
			return err
		}
	}
	if dstRepo == "" {
		var err error
		dstRepo, err = inferPushRepo(ctx, cc.git, cfg, srcBranch)
		if err != nil {
			return err
		}
	}
	if *dstBranch == "" {
		branch := srcBranch
		if branch == "" {
			return errors.New("cannot infer destination (source is not a branch). Use -d to specify destination branch.")
		}
		up := inferUpstream(cfg, branch)
		*dstBranch = up.Branch()
		if *dstBranch == "" {
			return fmt.Errorf("cannot infer destination (upstream %s is not a branch). Use -d to specify destination branch.", up)
		}
	} else {
		*dstBranch = strings.TrimPrefix(*dstBranch, "refs/for/")
	}
	ref := gerritPushRef(*dstBranch, gopts)
	return cc.git.RunInteractive(ctx, "push", "--", dstRepo, src.Commit().String()+":"+ref.String())
}

type gerritOptions struct {
	reviewers       []string // unflattened (may contain comma-separated elements)
	cc              []string // unflattened (may contain comma-separated elements)
	publishComments bool
	message         string

	notify    string // one of "", "NONE", "OWNER", "OWNER_REVIEWERS", or "ALL"
	notifyTo  []string
	notifyCC  []string
	notifyBCC []string
}

func gerritPushRef(branch string, opts *gerritOptions) gitobj.Ref {
	sb := new(strings.Builder)
	sb.WriteString("refs/for/")
	sb.WriteString(branch)
	sb.WriteByte('%')
	if opts != nil && opts.publishComments {
		sb.WriteString("publish-comments")
	} else {
		sb.WriteString("no-publish-comments")
	}
	if opts != nil {
		for _, r := range opts.reviewers {
			for _, r := range strings.Split(r, ",") {
				sb.WriteString(",r=")
				sb.WriteString(r)
			}
		}
		for _, cc := range opts.cc {
			for _, cc := range strings.Split(cc, ",") {
				sb.WriteString(",cc=")
				sb.WriteString(cc)
			}
		}
		if opts.message != "" {
			sb.WriteString(",m=")
			escapeGerritMessage(sb, opts.message)
		}
		if opts.notify != "" {
			sb.WriteString(",notify=")
			sb.WriteString(opts.notify)
		}
		for _, to := range opts.notifyTo {
			sb.WriteString(",notify-to=")
			sb.WriteString(to)
		}
		for _, cc := range opts.notifyCC {
			sb.WriteString(",notify-cc=")
			sb.WriteString(cc)
		}
		for _, bcc := range opts.notifyBCC {
			sb.WriteString(",notify-bcc=")
			sb.WriteString(bcc)
		}
	}
	return gitobj.Ref(sb.String())
}

func escapeGerritMessage(sb *strings.Builder, msg string) {
	sb.Grow(len(msg))
	for i := 0; i < len(msg); i++ {
		b := msg[i]
		switch {
		case b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z':
			sb.WriteByte(b)
		case b == ' ':
			sb.WriteByte('+')
		default:
			sb.WriteByte('%')
			sb.WriteByte(hexDigit(b >> 4))
			sb.WriteByte(hexDigit(b & 0xf))
		}
	}
}

// verifyPushRemoteRef returns nil if the given ref exists in the
// remote. remote may either be a URL or the name of a remote, in
// which case the remote's push URL will be queried.
func verifyPushRemoteRef(ctx context.Context, git *gittool.Tool, remote string, ref gitobj.Ref) error {
	remotes, _ := listRemotes(ctx, git)
	if _, isRemote := remotes[remote]; isRemote {
		pushURL, err := git.RunOneLiner(ctx, '\n', "remote", "get-url", "--push", "--", remote)
		if err != nil {
			return err
		}
		remote = string(pushURL)
	}
	p, err := git.Start(ctx, "ls-remote", "--quiet", remote, ref.String())
	if err != nil {
		return fmt.Errorf("verify remote ref %s: %v", ref, err)
	}
	calledWait := false
	defer func() {
		if !calledWait {
			p.Wait()
		}
	}()
	refBytes := []byte(ref)
	s := bufio.NewScanner(p)
	for s.Scan() {
		const tabLoc = 40
		line := s.Bytes()
		if tabLoc >= len(line) || line[tabLoc] != '\t' || !isHex(line[:tabLoc]) {
			return errors.New("parse git ls-remote: line must start with SHA1")
		}
		remoteRef := line[tabLoc+1:]
		if bytes.Equal(remoteRef, refBytes) {
			return nil
		}
	}
	if s.Err() != nil {
		return fmt.Errorf("verify remote ref %s: %v", ref, err)
	}
	calledWait = true
	if err := p.Wait(); err != nil {
		return fmt.Errorf("verify remote ref %s: %v", ref, err)
	}
	return fmt.Errorf("remote %s does not have ref %s", remote, ref)
}

func inferPushRepo(ctx context.Context, git *gittool.Tool, cfg *gittool.Config, branch string) (string, error) {
	if branch != "" {
		r := cfg.Value("branch." + branch + ".pushRemote")
		if r != "" {
			return r, nil
		}
	}
	r := cfg.Value("remote.pushDefault")
	if r != "" {
		return r, nil
	}
	if branch != "" {
		r := cfg.Value("branch." + branch + ".remote")
		if r != "" {
			return r, nil
		}
	}
	remotes, _ := listRemotes(ctx, git)
	if _, ok := remotes["origin"]; !ok {
		return "", errors.New("no destination given and no remote named \"origin\" found")
	}
	return "origin", nil
}

// isClean returns true iff all tracked files are unmodified in the
// working copy.  Untracked and ignored files are not considered.
func isClean(ctx context.Context, git *gittool.Tool) (bool, error) {
	st, err := gittool.Status(ctx, git, gittool.StatusOptions{})
	if err != nil {
		return false, err
	}
	stClose := singleclose.For(st)
	defer stClose.Close()
	for st.Scan() {
		if !st.Entry().Code().IsUntracked() {
			return false, nil
		}
	}
	if err := st.Err(); err != nil {
		return false, err
	}
	if err := stClose.Close(); err != nil {
		return false, err
	}
	return true, nil
}

func isHex(b []byte) bool {
	for _, bb := range b {
		if !(bb >= '0' && bb <= '9') && !(bb >= 'a' && bb <= 'f') && !(bb >= 'A' && bb <= 'F') {
			return false
		}
	}
	return true
}

func hexDigit(n byte) byte {
	switch {
	case n < 0xa:
		return '0' + n
	case n < 0xf:
		return 'a' + (n - 0xa)
	default:
		panic("argument too large")
	}
}
