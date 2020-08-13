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
	"strings"

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
	"gg-scm.io/tool/internal/sigterm"
)

const pullSynopsis = "pull changes from the specified source"

func pull(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg pull [-u] [-r REV [...]] [SOURCE]", pullSynopsis+`

	If no source repository is given, the remote called `+"`origin`"+` is used.
	If the source repository is not a named remote, then the branches will be
	saved under `+"`refs/ggpull/`"+`.

	Local branches with the same name as a remote branch will be
	fast-forwarded if possible. The currently checked out branch will not be
	fast-forwarded unless `+"`-u`"+` is passed.

	If no revisions are specified, then all the remote's branches and tags
	will be fetched. If the source is a named remote, then its remote
	tracking branches will be pruned.`)
	remoteRefArgs := f.MultiString("r", "`ref`s to pull")
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
	cfg, err := cc.git.ReadConfig(ctx)
	if err != nil {
		return err
	}
	remotes := cfg.ListRemotes()
	headBranch := currentBranch(ctx, cc)
	repo := f.Arg(0)
	if repo == "" {
		if _, ok := remotes["origin"]; !ok {
			return errors.New("no source given and no remote named \"origin\" found")
		}
		repo = "origin"
	}
	allLocalRefs, err := cc.git.ListRefsVerbatim(ctx)
	if err != nil {
		return err
	}
	allRemoteRefs, err := cc.git.ListRemoteRefs(ctx, repo)
	if err != nil {
		return err
	}

	_, isNamedRemote := remotes[repo]
	gitArgs, branches, err := buildFetchArgs(repo, isNamedRemote, allLocalRefs, allRemoteRefs, *remoteRefArgs)
	if err != nil {
		return err
	}
	if !isNamedRemote {
		// Delete anything under refs/ggpull/...
		// (Need to do this before fetching, but after validating that this
		// invocation will be reasonable.)
		localMuts := make(map[git.Ref]git.RefMutation, len(allLocalRefs))
		for ref := range allLocalRefs {
			if strings.HasPrefix(ref.String(), "refs/ggpull/") {
				localMuts[ref] = git.DeleteRef()
			}
		}
		if err := cc.git.MutateRefs(ctx, localMuts); err != nil {
			return fmt.Errorf("clearing refs/ggpull: %w", err)
		}
	}

	c := cc.git.Command(ctx, gitArgs...)
	c.Stdin = cc.stdin
	c.Stdout = cc.stdout
	c.Stderr = cc.stderr
	if err := sigterm.Run(ctx, c); err != nil {
		return err
	}
	remoteName := ""
	if isNamedRemote {
		remoteName = repo
	}
	if err := reconcileBranches(ctx, cc.git, headBranch, remoteName, allLocalRefs, allRemoteRefs, branches); err != nil {
		return err
	}
	if *update && headBranch != "" {
		var target git.Ref
		if isNamedRemote {
			headRef := git.BranchRef(headBranch)
			for _, spec := range remotes[repo].Fetch {
				target = spec.Map(headRef)
				if target != "" {
					break
				}
			}
		} else {
			target = git.Ref("refs/ggpull/" + headBranch)
		}
		if err := updateToBranch(ctx, cc.git, headBranch, target, git.MergeLocal); err != nil {
			return err
		}
	}
	return nil
}

func buildFetchArgs(repo string, isNamedRemote bool, localRefs, remoteRefs map[git.Ref]git.Hash, remoteRefArgs []string) (gitArgs []string, branches []git.Ref, _ error) {
	gitArgs = []string{"fetch"}
	if !isNamedRemote {
		gitArgs = append(gitArgs, "--refmap=+refs/heads/*:refs/ggpull/*")
	}
	var tags []git.Ref
	if len(remoteRefArgs) == 0 {
		for ref := range remoteRefs {
			switch {
			case ref.IsBranch():
				branches = append(branches, ref)
			case ref.IsTag():
				tags = append(tags, ref)
			}
		}
		if isNamedRemote {
			// Don't specify refs so remote-tracking branches can be pruned.
			gitArgs = append(gitArgs, "--prune", "--tags", "--", repo)
			return gitArgs, branches, nil
		}
	} else {
		// -r flag given. Filter remote refs by given set.
		for _, arg := range remoteRefArgs {
			switch ref := git.Ref(arg); {
			case ref.IsBranch():
				if _, exists := remoteRefs[ref]; !exists {
					return nil, nil, fmt.Errorf("can't find ref %q on remote %q", arg, repo)
				}
				branches = append(branches, ref)
				continue
			case ref.IsTag():
				if _, exists := remoteRefs[ref]; !exists {
					return nil, nil, fmt.Errorf("can't find ref %q on remote %q", arg, repo)
				}
				tags = append(tags, ref)
				continue
			}
			branchRef := git.BranchRef(arg)
			if _, hasBranch := remoteRefs[branchRef]; hasBranch {
				branches = append(branches, branchRef)
				continue
			}
			tagRef := git.TagRef(arg)
			if _, hasTag := remoteRefs[tagRef]; hasTag {
				tags = append(tags, tagRef)
				continue
			}
			return nil, nil, fmt.Errorf("can't find ref %q on remote %q", arg, repo)
		}
	}
	gitArgs = append(gitArgs, "--", repo)
	for _, branch := range branches {
		gitArgs = append(gitArgs, branch.String()+":")
	}
	for _, tag := range tags {
		if localHash, hasTag := localRefs[tag]; hasTag {
			// (Already validated above that ref exists on remote.)
			if remoteHash := remoteRefs[tag]; localHash != remoteHash {
				return nil, nil, fmt.Errorf("tag %q is %v on remote, does not match %v locally", tag.Tag(), remoteHash, localHash)
			}
			continue
		}
		gitArgs = append(gitArgs, "+"+tag.String()+":"+tag.String())
	}
	return gitArgs, branches, nil
}

func reconcileBranches(ctx context.Context, g *git.Git, headBranch, remoteName string, localRefs, remoteRefs map[git.Ref]git.Hash, branches []git.Ref) error {
	for _, branchRef := range branches {
		branchName := branchRef.Branch()
		if branchName == headBranch {
			continue
		}
		localCommit, existsLocally := localRefs[branchRef]
		remoteCommit := remoteRefs[branchRef]

		// If the branch doesn't exist yet, then create it.
		if !existsLocally {
			err := g.NewBranch(ctx, branchName, git.BranchOptions{
				StartPoint: remoteCommit.String(),
			})
			if err != nil {
				return err
			}
			// And set upstream, if necessary.
			if remoteName != "" {
				if err := g.Run(ctx, "config", "branch."+branchName+".remote", remoteName); err != nil {
					return err
				}
				if err := g.Run(ctx, "config", "branch."+branchName+".merge", branchRef.String()); err != nil {
					return err
				}
			}
			continue
		}

		// If branch didn't change, move on.
		if localCommit == remoteCommit {
			continue
		}

		// If branch can be fast-forwarded, then do it.
		isOlder, err := g.IsAncestor(ctx, localCommit.String(), remoteCommit.String())
		if err != nil {
			return err
		}
		if isOlder {
			err := g.NewBranch(ctx, branchName, git.BranchOptions{
				StartPoint: remoteCommit.String(),
				Overwrite:  true,
			})
			if err != nil {
				return err
			}
			continue
		}
	}
	return nil
}

func currentBranch(ctx context.Context, cc *cmdContext) string {
	ref, err := cc.git.HeadRef(ctx)
	if err != nil {
		return ""
	}
	return ref.Branch()
}
