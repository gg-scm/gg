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

	// Guaranteed to be the mapping used by clone.
	const originPrefix = "refs/remotes/origin/"

	iter := cc.git.IterateRefs(ctx, git.IterateRefsOptions{})
	localBranches := make(map[string]struct{})
	var remoteBranchNames []string
	for iter.Next() {
		if b := iter.Ref().Branch(); b != "" {
			localBranches[b] = struct{}{}
		} else if name, isRemote := strings.CutPrefix(iter.Ref().String(), originPrefix); isRemote && name != git.Head.String() {
			remoteBranchNames = append(remoteBranchNames, name)
		}
	}
	if err := iter.Close(); err != nil {
		return err
	}
	for _, name := range remoteBranchNames {
		if _, hasLocal := localBranches[name]; !hasLocal {
			err := cc.git.NewBranch(ctx, name, git.BranchOptions{
				StartPoint: git.Ref(originPrefix + name).String(),
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
	url, trimmed := strings.CutSuffix(url, "/.git")
	if !trimmed {
		url = strings.TrimSuffix(url, ".git")
	}
	if i := strings.LastIndexByte(url, '/'); i != -1 {
		return url[i+1:]
	}
	if i := strings.LastIndexByte(url, '\\'); i != -1 {
		return url[i+1:]
	}
	return url
}
