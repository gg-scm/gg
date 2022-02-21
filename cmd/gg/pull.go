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

const pullSynopsis = "pull changes from the specified source"

func pull(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg pull [-u] [-r REV [...]] [SOURCE]", pullSynopsis+`

	If no source repository is given, the remote called `+"`origin`"+` is used.
	If the source repository is not a named remote, then the branches will be
	saved under `+"`refs/ggpull/`"+`.

	Local branches with the same name as a remote branch will be
	fast-forwarded if possible. The currently checked out branch will not be
	fast-forwarded unless `+"`-u`"+` is passed. If a branch is removed from
	all known remotes and the local branch points to the last-known commit for
	that branch, then it will be moved under `+"`refs/gg-old/`"+`.

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
		repo = "origin"
		if _, ok := remotes[repo]; !ok {
			return fmt.Errorf("no source given and no remote named %q found", repo)
		}
	}
	allLocalRefs, err := cc.git.ListRefsVerbatim(ctx)
	if err != nil {
		return err
	}
	allRemoteRefs, err := cc.git.ListRemoteRefs(ctx, repo)
	if err != nil {
		return err
	}

	remote := remotes[repo]
	gitArgs, ops, err := buildFetchArgs(repo, remotes, allLocalRefs, allRemoteRefs, *remoteRefArgs)
	if err != nil {
		return err
	}
	if remote == nil {
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

	if len(gitArgs) > 0 {
		err = cc.interactiveGit(ctx, gitArgs...)
		if err != nil {
			return err
		}
	}
	if err := ops.reconcile(ctx, cc.git, headBranch); err != nil {
		return err
	}
	if *update && headBranch != "" {
		var target git.Ref
		if remote != nil {
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

type deferredFetchOps struct {
	remoteName      string
	localRefs       map[git.Ref]git.Hash
	remoteRefs      map[git.Ref]git.Hash
	branches        []git.Ref
	deletedBranches map[git.Ref]git.Hash
}

// buildFetchArgs computes the set of branches and tags to fetch.
// If buildFetchArgs returns an empty list of arguments,
// then fetch should not be run.
func buildFetchArgs(repo string, remotes map[string]*git.Remote, localRefs, remoteRefs map[git.Ref]git.Hash, remoteRefArgs []string) (gitArgs []string, ops *deferredFetchOps, _ error) {
	ops = &deferredFetchOps{
		localRefs:       localRefs,
		remoteRefs:      remoteRefs,
		deletedBranches: make(map[git.Ref]git.Hash),
	}
	gitArgs = []string{"fetch"}
	var prevRemoteRefs map[git.Ref]git.Hash
	remote := remotes[repo]
	if remote != nil {
		ops.remoteName = repo
		prevRemoteRefs = reverseFetchMap(remote.Fetch, localRefs)
	} else {
		gitArgs = append(gitArgs, "--refmap=+refs/heads/*:refs/ggpull/*")
	}
	var tags []git.Ref
	if len(remoteRefArgs) == 0 {
		for ref := range remoteRefs {
			switch {
			case ref.IsBranch():
				ops.branches = append(ops.branches, ref)
			case ref.IsTag():
				tags = append(tags, ref)
			}
		}
		for ref := range prevRemoteRefs {
			if _, exists := remoteRefs[ref]; ref.IsBranch() && !exists && isRefOrphaned(remotes, localRefs, repo, ref) {
				ops.deletedBranches[ref] = localRefs[ref]
			}
		}
		if remote != nil {
			// Don't specify refs so remote-tracking branches can be pruned.
			gitArgs = append(gitArgs, "--prune", "--tags", "--", repo)
			return gitArgs, ops, nil
		}
	} else {
		// -r flag given. Filter remote refs by given set.
		for _, arg := range remoteRefArgs {
			switch ref := git.Ref(arg); {
			case ref.IsBranch():
				_, exists := remoteRefs[ref]
				_, prevExists := prevRemoteRefs[ref]
				if !exists && !prevExists {
					return nil, nil, fmt.Errorf("can't find ref %q on remote %q", arg, repo)
				}
				if exists {
					ops.branches = append(ops.branches, ref)
				} else if isRefOrphaned(remotes, localRefs, repo, ref) {
					ops.deletedBranches[ref] = localRefs[ref]
				}
				continue
			case ref.IsTag():
				_, exists := remoteRefs[ref]
				if !exists {
					return nil, nil, fmt.Errorf("can't find ref %q on remote %q", arg, repo)
				}
				tags = append(tags, ref)
				continue
			}
			branchRef := git.BranchRef(arg)
			if _, hasBranch := remoteRefs[branchRef]; hasBranch {
				ops.branches = append(ops.branches, branchRef)
				continue
			} else if _, hasPrevBranch := prevRemoteRefs[branchRef]; hasPrevBranch && isRefOrphaned(remotes, localRefs, repo, branchRef) {
				ops.deletedBranches[branchRef] = localRefs[branchRef]
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
	if len(ops.branches)+len(tags) == 0 {
		return nil, ops, nil
	}
	gitArgs = append(gitArgs, "--", repo)
	for _, branch := range ops.branches {
		gitArgs = append(gitArgs, branch.String()+":")
	}
	for _, tag := range tags {
		if localHash, hasTag := localRefs[tag]; hasTag {
			if remoteHash, remoteHasTag := remoteRefs[tag]; remoteHasTag && localHash != remoteHash {
				return nil, nil, fmt.Errorf("tag %q is %v on remote, does not match %v locally", tag.Tag(), remoteHash, localHash)
			}
			continue
		}
		gitArgs = append(gitArgs, "+"+tag.String()+":"+tag.String())
	}
	return gitArgs, ops, nil
}

func (ops *deferredFetchOps) reconcile(ctx context.Context, g *git.Git, headBranch string) error {
	for _, branchRef := range ops.branches {
		branchName := branchRef.Branch()
		if branchName == headBranch {
			continue
		}
		localCommit, existsLocally := ops.localRefs[branchRef]
		remoteCommit := ops.remoteRefs[branchRef]

		// If the branch doesn't exist yet, then create it.
		if !existsLocally {
			err := g.NewBranch(ctx, branchName, git.BranchOptions{
				StartPoint: remoteCommit.String(),
			})
			if err != nil {
				return err
			}
			// And set upstream, if necessary.
			if ops.remoteName != "" {
				if err := g.Run(ctx, "config", "branch."+branchName+".remote", ops.remoteName); err != nil {
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
	if len(ops.deletedBranches) > 0 {
		inserts := make(map[git.Ref]git.RefMutation)
		branchDeleteArgs := []string{"branch", "-D", "--"}
		for branchRef, expectHash := range ops.deletedBranches {
			branchName := branchRef.Branch()
			val := expectHash.String()
			inserts[git.Ref("refs/gg-old/"+branchName)] = git.SetRef(val)
			branchDeleteArgs = append(branchDeleteArgs, branchName)
		}
		if err := g.MutateRefs(ctx, inserts); err != nil {
			return err
		}
		if err := g.Run(ctx, branchDeleteArgs...); err != nil {
			return err
		}
	}
	return nil
}

// reverseFetchMap returns the refs last fetched from the remote.
func reverseFetchMap(fetch []git.FetchRefspec, localRefs map[git.Ref]git.Hash) map[git.Ref]git.Hash {
	result := make(map[git.Ref]git.Hash)
	for _, spec := range fetch {
		src, dst, _ := spec.Parse()
		srcPrefix, srcWildcard := src.Prefix()
		_, dstWildcard := dst.Prefix()
		if srcWildcard != dstWildcard {
			continue
		}
		if !dstWildcard {
			if hash, ok := localRefs[git.Ref(dst)]; ok {
				result[git.Ref(src)] = hash
			}
			continue
		}
		for ref, hash := range localRefs {
			if suffix, ok := dst.Match(ref); ok {
				result[git.Ref(srcPrefix+suffix)] = hash
			}
		}
	}
	return result
}

// isRefOrphaned reports whether the given remote
// is the last one to contain the given ref.
// It will always return false if the local ref
// does not match the remote ref.
func isRefOrphaned(remotes map[string]*git.Remote, localRefs map[git.Ref]git.Hash, currRemoteName string, ref git.Ref) bool {
	localHash, ok := localRefs[ref]
	if !ok {
		return false
	}
	currRemote := remotes[currRemoteName]
	if currRemote == nil {
		return false
	}
	trackingRef := currRemote.MapFetch(ref)
	if trackingRef == "" || localRefs[trackingRef] != localHash {
		return false
	}
	for remoteName, remote := range remotes {
		if remoteName == currRemoteName {
			continue
		}
		trackingRef := remote.MapFetch(ref)
		if trackingRef == "" {
			continue
		}
		if _, ok := localRefs[trackingRef]; ok {
			return false
		}
	}
	return true
}

func currentBranch(ctx context.Context, cc *cmdContext) string {
	ref, err := cc.git.HeadRef(ctx)
	if err != nil {
		return ""
	}
	return ref.Branch()
}
