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
	"fmt"
	"strings"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const cloneSynopsis = "make a copy of an existing repository"

func clone(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg clone [-b BRANCH] SOURCE [DEST]", cloneSynopsis)
	branch := f.String("b", git.Head.String(), "`branch` to check out")
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
	if *branch == git.Head.String() {
		err := cc.interactiveGit(ctx, "clone", "--", src, dst)
		if err != nil {
			return err
		}
	} else {
		err := cc.interactiveGit(ctx, "clone", "--branch="+*branch, "--", src, dst)
		if err != nil {
			return err
		}
	}
	cc = cc.withDir(dst)
	refs, err := cc.git.ListRefs(ctx)
	if err != nil {
		return err
	}
	branches := make(map[string]struct{}, len(refs))
	for r := range refs {
		if b := r.Branch(); b != "" {
			branches[b] = struct{}{}
		}
	}
	for r := range refs {
		// Guaranteed to be the mapping used by clone.
		const originPrefix = "refs/remotes/origin/"
		if !strings.HasPrefix(r.String(), originPrefix) {
			continue
		}
		name := string(r[len(originPrefix):])
		if name == git.Head.String() {
			continue
		}
		if _, hasLocal := branches[string(name)]; !hasLocal {
			err := cc.git.NewBranch(ctx, name, git.BranchOptions{
				StartPoint: r.String(),
				Track:      true,
			})
			if err != nil {
				return fmt.Errorf("mirroring local branch %q: %v", name, err)
			}
		}
	}
	if *gerrit {
		if err := installGerritHook(ctx, cc, *gerritHookURL, false); err != nil {
			return err
		}
	}
	return nil
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
