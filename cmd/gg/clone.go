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
	"context"
	"errors"
	"fmt"
	"strings"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/gitobj"
	"gg-scm.io/pkg/internal/gittool"
)

const cloneSynopsis = "make a copy of an existing repository"

func clone(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg clone [-b BRANCH] SOURCE [DEST]", cloneSynopsis)
	branch := f.String("b", gitobj.Head.String(), "`branch` to check out")
	f.Alias("b", "branch")
	gerrit := f.Bool("gerrit", false, "install Gerrit hook")
	gerritHookURL := f.String("gerrit-hook-url", commitMsgHookDefaultURL, "URL of hook script to download")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() == 0 {
		return usagef("must pass clone source")
	}
	if f.NArg() > 2 {
		return usagef("can't pass more than one destination")
	}
	src, dst := f.Arg(0), f.Arg(1)
	if dst == "" {
		dst = defaultCloneDest(src)
	}
	if *branch == gitobj.Head.String() {
		if err := cc.git.RunInteractive(ctx, "clone", "--", src, dst); err != nil {
			return err
		}
	} else {
		if err := cc.git.RunInteractive(ctx, "clone", "--branch="+*branch, "--", src, dst); err != nil {
			return err
		}
	}
	cc = cc.withDir(dst)
	refs, err := listRefs(ctx, cc.git)
	if err != nil {
		return err
	}
	branches := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		b := r.name.Branch()
		if b != "" {
			branches[b] = struct{}{}
		}
	}
	for _, r := range refs {
		// Guaranteed to be the mapping used by clone.
		const originPrefix = "refs/remotes/origin/"
		if !strings.HasPrefix(r.name.String(), originPrefix) {
			continue
		}
		name := string(r.name[len(originPrefix):])
		if name == gitobj.Head.String() {
			continue
		}
		if _, hasLocal := branches[string(name)]; !hasLocal {
			if err := cc.git.Run(ctx, "branch", "--track", "--", name, r.name.String()); err != nil {
				return fmt.Errorf("mirroring local branch %q: %v", name, err)
			}
		}
	}
	if *gerrit {
		if err := installGerritHook(ctx, cc, *gerritHookURL); err != nil {
			return err
		}
	}
	return nil
}

type refList []refListEntry

type refListEntry struct {
	name   gitobj.Ref
	commit gitobj.Hash
}

func listRefs(ctx context.Context, git *gittool.Tool) (refList, error) {
	p, err := git.Start(ctx, "show-ref")
	if err != nil {
		return nil, err
	}
	defer p.Wait()
	s := bufio.NewScanner(p)
	var refs refList
	for s.Scan() {
		line := s.Bytes()
		const spaceLoc = len(refListEntry{}.commit) * 2
		if spaceLoc >= len(line) || line[spaceLoc] != ' ' {
			return refs, errors.New("parse git show-ref: line must start with commit hash")
		}
		h, err := gitobj.ParseHash(string(line[:spaceLoc]))
		if err != nil {
			return refs, fmt.Errorf("parse git show-ref: %v", err)
		}
		refs = append(refs, refListEntry{
			name:   gitobj.Ref(line[spaceLoc+1:]),
			commit: h,
		})
	}
	if err := p.Wait(); err != nil {
		return refs, err
	}
	return refs, nil
}

func defaultCloneDest(url string) string {
	if strings.HasSuffix(url, "/.git") {
		url = url[:len(url)-5]
	} else if strings.HasSuffix(url, ".git") {
		url = url[:len(url)-4]
	}
	if i := strings.LastIndexByte(url, '/'); i != -1 {
		return url[i+1:]
	}
	if i := strings.LastIndexByte(url, '\\'); i != -1 {
		return url[i+1:]
	}
	return url
}
