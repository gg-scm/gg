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
	"errors"
	"fmt"
	"sort"
	"strings"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
	"gg-scm.io/tool/internal/sigterm"
)

const pushSynopsis = "push changes to the specified destination"

func push(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg push [-f] [-r REF [...]] [--new-branch] [DST]", pushSynopsis+`

	`+"`gg push`"+` pushes branches and tags to mirror the local repository in the
	destination repository. It does not permit diverging commits unless `+"`-f`"+`
	is passed. If the `+"`-r`"+` is not given, `+"`gg push`"+` will push all
	branches that exist in both the local and destination repository as well as
	all tags. The argument to `+"`-r`"+` must name a ref: it cannot be an
	arbitrary commit.

	When no destination repository is given, tries to use the remote specified by
	the configuration value of `+"`remote.pushDefault`"+` or the remoted called
	`+"`origin`"+` otherwise.

	By default, `+"`gg push`"+` will fail instead of creating a new ref in the
	destination repository. If this is desired (e.g. you are creating a new
	branch), then you can pass `+"`--new-branch`"+` to override this check.
	`+"`-f`"+` will also skip this check.`)
	create := f.Bool("new-branch", false, "allow pushing a new ref")
	force := f.Bool("f", false, "allow overwriting ref if it is not an ancestor, as long as it matches the remote-tracking branch")
	f.Alias("f", "force")
	refArgs := f.MultiString("r", "source `ref`s")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("can't pass multiple destinations")
	}
	refsImplicit := len(*refArgs) == 0
	if refsImplicit && (*force || *create) {
		return usagef("can't pass --force or --new-branch without specifying refs")
	}
	dstRepo := f.Arg(0)
	if dstRepo == "" {
		cfg, err := cc.git.ReadConfig(ctx)
		if err != nil {
			return err
		}
		dstRepo, err = inferPushRepo(cfg, "")
		if err != nil {
			return err
		}
	}
	var refsToPush []git.Ref
	if refsImplicit {
		localRefs, err := cc.git.ListRefs(ctx)
		if err != nil {
			return err
		}
		for ref := range localRefs {
			if ref.IsBranch() || ref.IsTag() {
				refsToPush = append(refsToPush, ref)
			}
		}
		sort.Slice(refsToPush, func(i, j int) bool { return refsToPush[i] < refsToPush[j] })
	} else {
		for _, arg := range *refArgs {
			resolved, err := cc.git.ParseRev(ctx, arg)
			if err != nil {
				return err
			}
			if !resolved.Ref.IsBranch() && !resolved.Ref.IsTag() {
				return fmt.Errorf("%q is not a branch or tag", arg)
			}
			refsToPush = append(refsToPush, resolved.Ref)
		}
	}

	if !*force && !*create {
		remoteRefs, err := cc.git.ListRemoteRefs(ctx, dstRepo)
		if err != nil {
			return err
		}
		n := 0
		conflicts := false
		for _, ref := range refsToPush {
			if _, exists := remoteRefs[ref]; exists || ref.IsTag() {
				refsToPush[n] = ref
				n++
				continue
			}
			if refsImplicit {
				// If the user ran `gg push`, ignore the ref.
				continue
			}
			conflicts = true
			fmt.Fprintf(cc.stderr, "gg: push: %q does not exist on remote\n", ref)
		}
		if conflicts {
			return errors.New("push would create refs (if this is what you want, run again with --new-branch)")
		}
		refsToPush = refsToPush[:n]
	}

	if len(refsToPush) == 0 {
		return errors.New("no refs to push")
	}

	var pushArgs []string
	pushArgs = append(pushArgs, "push")
	if *force {
		pushArgs = append(pushArgs, "--force-with-lease")
	}
	pushArgs = append(pushArgs, "--", dstRepo)
	for _, ref := range refsToPush {
		if tag := ref.Tag(); tag != "" {
			pushArgs = append(pushArgs, "tag", tag)
		} else {
			pushArgs = append(pushArgs, ref.String()+":"+ref.String())
		}
	}
	c := cc.git.Command(ctx, pushArgs...)
	c.Stdin = cc.stdin
	c.Stdout = cc.stdout
	c.Stderr = cc.stderr
	return sigterm.Run(ctx, c)
}

const mailSynopsis = "creates or updates a Gerrit change"

func mail(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg mail [options] [DST]", mailSynopsis)
	allowDirty := f.Bool("allow-dirty", false, "allow mailing when working copy has uncommitted changes")
	dstBranch := f.String("d", "", "destination `branch`")
	f.Alias("d", "dest", "for")
	rev := f.String("r", git.Head.String(), "source `rev`ision")
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
	src, err := cc.git.ParseRev(ctx, *rev)
	if err != nil {
		return err
	}
	srcBranch := src.Ref.Branch()
	if srcBranch == "" {
		possible, err := branchesContaining(ctx, cc.git, src.Commit.String())
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
	var cfg *git.Config
	if dstRepo == "" || *dstBranch == "" {
		var err error
		cfg, err = cc.git.ReadConfig(ctx)
		if err != nil {
			return err
		}
	}
	if dstRepo == "" {
		var err error
		dstRepo, err = inferPushRepo(cfg, srcBranch)
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
	c := cc.git.Command(ctx, "push", "--", dstRepo, src.Commit.String()+":"+ref.String())
	c.Stdin = cc.stdin
	c.Stdout = cc.stdout
	c.Stderr = cc.stderr
	return sigterm.Run(ctx, c)
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

func gerritPushRef(branch string, opts *gerritOptions) git.Ref {
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
	return git.Ref(sb.String())
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
			sb.WriteByte(hexDigit[b>>4])
			sb.WriteByte(hexDigit[b&0xf])
		}
	}
}

func inferPushRepo(cfg *git.Config, branch string) (string, error) {
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
	remotes := cfg.ListRemotes()
	if _, ok := remotes["origin"]; !ok {
		return "", errors.New("no destination given and no remote named \"origin\" found")
	}
	return "origin", nil
}

// isClean returns true iff all tracked files are unmodified in the
// working copy.  Untracked and ignored files are not considered.
func isClean(ctx context.Context, g *git.Git) (bool, error) {
	st, err := g.Status(ctx, git.StatusOptions{})
	if err != nil {
		return false, err
	}
	for _, ent := range st {
		if !ent.Code.IsUntracked() {
			return false, nil
		}
	}
	return true, nil
}

const hexDigit = "0123456789abcdef"
