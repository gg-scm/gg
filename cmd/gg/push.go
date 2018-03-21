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

	"zombiezen.com/go/gg/internal/flag"
	"zombiezen.com/go/gg/internal/gittool"
)

const pushSynopsis = "push changes to the specified destination"

func push(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg push [-r REV] [-d REF] [--create] [DST]", pushSynopsis+`

	When no destination repository is given, push uses the first non-
	empty configuration value of:

	1.  branch.*.pushRemote, if the source is a branch
	2.  remote.pushDefault
	3.  branch.*.remote, if the source is a branch
	4.  Otherwise, the remote called "origin" is used.

	(This is the same repository selection logic that git uses.)

	If -d is given and begins with "refs/", then it specifies the remote
	ref to update. If the argument passed to -d does not begin with
	"refs/", it is assumed to be a branch name ("refs/heads/<arg>").
	If -d is not given and the source is a ref, then the same ref name is
	used. Otherwise, push exits with a failure exit code. This differs
	from git, which will consult remote.*.push and push.default. You can
	imagine this being the most similar to push.default=current.

	By default, gg push will fail instead of creating a new ref on the
	remote. If this is desired (e.g. you are creating a new branch), then
	you can pass -create to override this check.`)
	create := f.Bool("create", false, "allow pushing a new ref")
	dstRef := f.String("d", "", "destination `ref`")
	f.Alias("d", "dest")
	rev := f.String("r", "HEAD", "source `rev`ision")
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
	dstRepo := f.Arg(0)
	if dstRepo == "" {
		var err error
		dstRepo, err = inferPushRepo(ctx, cc.git, src.Branch())
		if err != nil {
			return err
		}
	}
	if *dstRef == "" {
		if src.RefName() == "" {
			return errors.New("cannot infer destination (source is not a ref). Use -d to specify destination ref.")
		}
		*dstRef = src.RefName()
	} else if !strings.HasPrefix(*dstRef, "refs/") {
		*dstRef = "refs/heads/" + *dstRef
	}
	if !*create {
		if err := verifyRemoteRef(ctx, cc.git, dstRepo, *dstRef); err != nil {
			return err
		}
	}
	return cc.git.RunInteractive(ctx, "push", "--", dstRepo, src.CommitHex()+":"+*dstRef)
}

const mailSynopsis = "creates or updates a Gerrit change"

func mail(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg mail [options] [DST]", mailSynopsis)
	dstBranch := f.String("d", "", "destination `branch`")
	f.Alias("d", "dest", "for")
	rev := f.String("r", "HEAD", "source `rev`ision")
	gopts := new(gerritOptions)
	f.MultiStringVar(&gopts.reviewers, "R", "reviewer `email`")
	f.Alias("R", "reviewer")
	f.MultiStringVar(&gopts.cc, "CC", "`email`s to CC")
	f.Alias("CC", "cc")
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
	src, err := gittool.ParseRev(ctx, cc.git, *rev)
	if err != nil {
		return err
	}
	dstRepo := f.Arg(0)
	if dstRepo == "" {
		var err error
		dstRepo, err = inferPushRepo(ctx, cc.git, src.Branch())
		if err != nil {
			return err
		}
	}
	if *dstBranch == "" {
		branch := src.Branch()
		if branch == "" {
			return errors.New("cannot infer destination (source is not a branch). Use -d to specify destination branch.")
		}
		up, err := inferUpstream(ctx, cc.git, branch)
		if err != nil {
			return err
		}
		const wantPrefix = "refs/heads/"
		if !strings.HasPrefix(up, wantPrefix) {
			return fmt.Errorf("cannot infer destination (upstream %s is not a branch). Use -d to specify destination branch.", up)
		}
		*dstBranch = up[len(wantPrefix):]
	} else {
		*dstBranch = strings.TrimPrefix(*dstBranch, "refs/for/")
	}
	ref := gerritPushRef(*dstBranch, gopts)
	return cc.git.RunInteractive(ctx, "push", "--", dstRepo, src.CommitHex()+":"+ref)
}

type gerritOptions struct {
	reviewers       []string // unflattened (may contain comma-separated elements)
	cc              []string // unflattened (may contain comma-separated elements)
	publishComments bool
	message         string
}

func gerritPushRef(branch string, opts *gerritOptions) string {
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
	}
	return sb.String()
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

func verifyRemoteRef(ctx context.Context, git *gittool.Tool, remote string, ref string) error {
	p, err := git.Start(ctx, "ls-remote", "--quiet", remote, ref)
	if err != nil {
		return fmt.Errorf("verify remote ref %s: %v", ref, err)
	}
	defer p.Wait()
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
	if err := p.Wait(); err != nil {
		return fmt.Errorf("verify remote ref %s: %v", ref, err)
	}
	return fmt.Errorf("remote %s does not have ref %s", remote, ref)
}

func inferPushRepo(ctx context.Context, git *gittool.Tool, branch string) (string, error) {
	if branch != "" {
		r, err := gittool.Config(ctx, git, "branch."+branch+".pushRemote")
		if err != nil {
			return "", err
		}
		if r != "" {
			return r, nil
		}
	}
	r, err := gittool.Config(ctx, git, "remote.pushDefault")
	if err != nil {
		return "", err
	}
	if r != "" {
		return r, nil
	}
	if branch != "" {
		r, err := gittool.Config(ctx, git, "branch."+branch+".remote")
		if err != nil {
			return "", err
		}
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
