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

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
)

const pullSynopsis = "pull changes from the specified source"

func pull(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg pull [-u] [-r REF] [SOURCE]", pullSynopsis+`

	The fetched reference is written to FETCH_HEAD.

	If no source repository is given and a branch with a remote tracking
	branch is currently checked out, then that remote is used. Otherwise,
	the remote called "origin" is used.

	If no remote reference is given and a branch is currently checked out,
	then the branch's remote tracking branch is used or the branch with
	the same name if the branch has no remote tracking branch. Otherwise,
	"HEAD" is used.`)
	remoteRefArg := f.String("r", "", "remote `ref`erence intended to be pulled")
	tags := f.Bool("tags", true, "pull all tags from remote")
	update := f.Bool("u", false, "update to new head if new descendants were pulled")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	if f.NArg() > 1 {
		return usagef("can't pass multiple sources")
	}
	cfg, err := git.ReadConfig(ctx, cc.git)
	if err != nil {
		return err
	}
	branch := currentBranch(ctx, cc)
	repo := f.Arg(0)
	if repo == "" {
		if branch != "" {
			repo = cfg.Value("branch." + branch + ".remote")
		}
		if repo == "" {
			remotes, _ := listRemotes(ctx, cc.git)
			if _, ok := remotes["origin"]; !ok {
				return errors.New("no source given and no remote named \"origin\" found")
			}
			repo = "origin"
		}
	}
	var remoteRef git.Ref
	if *remoteRefArg == "" {
		remoteRef = inferUpstream(cfg, branch)
	} else {
		remoteRef = git.Ref(*remoteRefArg)
		if !remoteRef.IsValid() {
			return fmt.Errorf("invalid ref %q", *remoteRefArg)
		}
	}

	var gitArgs []string
	if *update {
		gitArgs = append(gitArgs, "pull", "--ff-only")
	} else {
		gitArgs = append(gitArgs, "fetch")
	}
	if *tags {
		gitArgs = append(gitArgs, "--tags")
	}
	gitArgs = append(gitArgs, "--", repo, remoteRef.String()+":")
	return cc.git.Run(ctx, gitArgs...)
}

func currentBranch(ctx context.Context, cc *cmdContext) string {
	r, err := git.ParseRev(ctx, cc.git, git.Head.String())
	if err != nil {
		return ""
	}
	return r.Ref().Branch()
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

func listRemotes(ctx context.Context, git *git.Git) (map[string]struct{}, error) {
	p, err := git.Start(ctx, "remote")
	if err != nil {
		return nil, err
	}
	calledWait := false
	defer func() {
		if !calledWait {
			p.Wait()
		}
	}()
	s := bufio.NewScanner(p)
	remotes := make(map[string]struct{})
	for s.Scan() {
		remotes[s.Text()] = struct{}{}
	}
	if s.Err() != nil {
		return remotes, s.Err()
	}
	calledWait = true
	err = p.Wait()
	return remotes, err
}
