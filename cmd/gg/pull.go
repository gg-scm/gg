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
	"io"
	"regexp"
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
	var input pullInput
	f.MultiStringVar(&input.remoteRefArgs, "r", "`ref`s to pull")
	f.RegexpVar(&input.remoteRefPattern, "p", "`regexp` of branch or tag names to pull (can be specified multiple times)")
	f.Alias("p", "pattern")
	f.BoolVar(&input.forceTags, "force-tags", false, "update any tags pulled")
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
	input.remotes = cfg.ListRemotes()
	headBranch := currentBranch(ctx, cc)
	input.repo = f.Arg(0)
	if input.repo == "" {
		input.repo = "origin"
		if _, ok := input.remotes[input.repo]; !ok {
			return fmt.Errorf("no source given and no remote named %q found", input.repo)
		}
	}
	input.localRefs, err = refIteratorToMap(cc.git.IterateRefs(ctx, git.IterateRefsOptions{
		IncludeHead: true,
	}))
	if err != nil {
		return err
	}
	input.remoteRefs, err = refIteratorToMap(cc.git.IterateRemoteRefs(ctx, input.repo, git.IterateRemoteRefsOptions{
		LimitToBranches: true,
		LimitToTags:     true,
	}))
	if err != nil {
		return err
	}

	gitArgs, ops, err := input.buildFetchArgs()
	if err != nil {
		return err
	}
	remote := input.remotes[input.repo]
	if remote == nil {
		// Delete anything under refs/ggpull/...
		// (Need to do this before fetching, but after validating that this
		// invocation will be reasonable.)
		localMuts := make(map[git.Ref]git.RefMutation, len(input.localRefs))
		for ref := range input.localRefs {
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
	if err := ops.reconcile(ctx, cc.git, cc.stderr, headBranch); err != nil {
		return err
	}
	if *update && headBranch != "" {
		var target git.Ref
		if remote != nil {
			headRef := git.BranchRef(headBranch)
			for _, spec := range remote.Fetch {
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

type pullInput struct {
	remoteRefArgs    []string
	remoteRefPattern *regexp.Regexp
	forceTags        bool

	// repo is the name or URL of the remote to fetch from.
	repo string
	// remotes is the configured set of remotes.
	remotes map[string]*git.Remote
	// localRefs is the set of refs in the local repository.
	localRefs map[git.Ref]git.Hash
	// remoteRefs is the set of branches and tags in the repository being fetched.
	remoteRefs map[git.Ref]git.Hash
}

type deferredFetchOps struct {
	remote      *git.Remote
	localRefs   map[git.Ref]git.Hash
	remoteRefs  map[git.Ref]git.Hash
	branches    []git.Ref
	deletedRefs map[git.Ref]git.Hash
}

// buildFetchArgs computes the set of branches and tags to fetch.
// If buildFetchArgs returns an empty list of arguments,
// then fetch should not be run.
func (input *pullInput) buildFetchArgs() (gitArgs []string, ops *deferredFetchOps, _ error) {
	ops = &deferredFetchOps{
		remote:      input.remotes[input.repo],
		localRefs:   input.localRefs,
		remoteRefs:  input.remoteRefs,
		deletedRefs: make(map[git.Ref]git.Hash),
	}
	gitArgs = []string{"fetch"}
	var prevRemoteRefs map[git.Ref]git.Hash
	if ops.remote != nil {
		prevRemoteRefs = reverseFetchMap(ops.remote.Fetch, input.localRefs)
	} else {
		gitArgs = append(gitArgs, "--refmap=+refs/heads/*:refs/ggpull/*")
	}

	// Convert remoteRefArgs into a set of refs.
	var resolvedRefs []git.Ref
	if len(input.remoteRefArgs) == 0 && input.remoteRefPattern == nil {
		// Empty means everything.
		for ref := range input.remoteRefs {
			if ref.IsBranch() || ref.IsTag() {
				resolvedRefs = append(resolvedRefs, ref)
			}
		}
		for ref := range prevRemoteRefs {
			if _, exists := input.remoteRefs[ref]; ref.IsBranch() && !exists {
				resolvedRefs = append(resolvedRefs, ref)
			}
		}
	} else {
		// Add refs/heads/ or refs/tags/ as appropriate.
		for _, arg := range input.remoteRefArgs {
			if ref := git.Ref(arg); ref.IsBranch() || ref.IsTag() {
				resolvedRefs = append(resolvedRefs, ref)
				continue
			}
			branchRef := git.BranchRef(arg)
			_, hasBranch := input.remoteRefs[branchRef]
			_, hasPrevBranch := prevRemoteRefs[branchRef]
			if hasBranch || hasPrevBranch {
				resolvedRefs = append(resolvedRefs, branchRef)
				continue
			}
			tagRef := git.TagRef(arg)
			if _, hasTag := input.remoteRefs[tagRef]; hasTag {
				resolvedRefs = append(resolvedRefs, tagRef)
				continue
			}
			return nil, nil, fmt.Errorf("can't find ref %q on remote %q", arg, input.repo)
		}

		if input.remoteRefPattern != nil {
			for ref := range input.remoteRefs {
				s := ref.Branch()
				if s == "" {
					s = ref.Tag()
					if s == "" {
						continue
					}
				}
				if input.remoteRefPattern.MatchString(s) {
					resolvedRefs = append(resolvedRefs, ref)
				}
			}
			for ref := range prevRemoteRefs {
				branch := ref.Branch()
				if branch == "" {
					continue
				}
				_, exists := input.remoteRefs[ref]
				if !exists && input.remoteRefPattern.MatchString(ref.String()) {
					resolvedRefs = append(resolvedRefs, ref)
				}
			}
		}
	}

	// Build fetch arguments and validate ref selection.
	gitArgs = append(gitArgs, "--", input.repo)
	zeroFetchArgsLen := len(gitArgs)
	for _, ref := range resolvedRefs {
		switch {
		case ref.IsBranch():
			_, hasBranch := input.remoteRefs[ref]
			_, hasPrevBranch := prevRemoteRefs[ref]
			switch {
			case hasBranch:
				gitArgs = append(gitArgs, ref.String()+":")
				ops.branches = append(ops.branches, ref)
			case hasPrevBranch:
				trackingRef := ops.remote.MapFetch(ref)
				ops.deletedRefs[trackingRef] = input.localRefs[trackingRef]
				if isRefOrphaned(input.remotes, input.localRefs, input.repo, ref) {
					ops.deletedRefs[ref] = input.localRefs[ref]
				}
			default:
				return nil, nil, fmt.Errorf("can't find ref %q on remote %q", ref, input.repo)
			}
		case ref.IsTag():
			localHash, hasLocalTag := input.localRefs[ref]
			remoteHash, hasRemoteTag := input.remoteRefs[ref]
			refspec := ref.String() + ":" + ref.String()
			switch {
			case !hasRemoteTag:
				return nil, nil, fmt.Errorf("tag %q does not exist on remote", ref.Tag())
			case !hasLocalTag:
				gitArgs = append(gitArgs, refspec)
			case localHash != remoteHash && input.forceTags:
				gitArgs = append(gitArgs, "+"+refspec)
			case localHash != remoteHash && !input.forceTags:
				return nil, nil, fmt.Errorf("tag %q is %v on remote, does not match %v locally", ref.Tag(), remoteHash, localHash)
			}
		default:
			panic("unsupported ref " + ref.String())
		}
	}
	if len(gitArgs) == zeroFetchArgsLen {
		return nil, ops, nil
	}
	return gitArgs, ops, nil
}

func (ops *deferredFetchOps) reconcile(ctx context.Context, g *git.Git, stderr io.Writer, headBranch string) error {
	success := true
	report := func(err error) {
		success = false
		fmt.Fprintln(stderr, "gg:", err)
	}
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
				report(err)
				continue
			}
			// And set upstream, if necessary.
			if ops.remote != nil {
				if err := g.Run(ctx, "config", "branch."+branchName+".remote", ops.remote.Name); err != nil {
					report(err)
				}
				if err := g.Run(ctx, "config", "branch."+branchName+".merge", branchRef.String()); err != nil {
					report(err)
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
			report(err)
			continue
		}
		if isOlder {
			err := g.NewBranch(ctx, branchName, git.BranchOptions{
				StartPoint: remoteCommit.String(),
				Overwrite:  true,
			})
			if err != nil {
				report(err)
			}
			continue
		}
	}

	if len(ops.deletedRefs) > 0 {
		const oldNamespace = "refs/gg-old/"
		deleteOldOlds := make(map[git.Ref]git.RefMutation)
		for ref := range ops.localRefs {
			if strings.HasPrefix(string(ref), oldNamespace) {
				deleteOldOlds[ref] = git.DeleteRef()
			}
		}
		if err := g.MutateRefs(ctx, deleteOldOlds); err != nil {
			report(err)
		}

		mutations := make(map[git.Ref]git.RefMutation)
		var branchesToDelete []string
		for ref, expectHash := range ops.deletedRefs {
			val := expectHash.String()
			if branchName := ref.Branch(); branchName != "" {
				mutations[git.Ref(oldNamespace+branchName)] = git.SetRef(val)
				if branchName == headBranch {
					fmt.Fprintf(stderr, "gg: currently-checked out branch '%s' was deleted on remote\n", headBranch)
					if err := g.Run(ctx, "config", "--unset-all", "branch."+branchName+".remote"); err != nil {
						report(err)
					}
					if err := g.Run(ctx, "config", "--unset-all", "branch."+branchName+".merge"); err != nil {
						report(err)
					}
				} else {
					branchesToDelete = append(branchesToDelete, branchName)
				}
			} else {
				mutations[ref] = git.DeleteRefIfMatches(val)
			}
		}
		if err := g.MutateRefs(ctx, mutations); err != nil {
			report(err)
		}
		err := g.DeleteBranches(ctx, branchesToDelete, git.DeleteBranchOptions{
			Force: true,
		})
		if err != nil {
			report(err)
		}
	}

	if !success {
		return fmt.Errorf("did not update all local branches")
	}
	return nil
}

func refIteratorToMap(iter *git.RefIterator) (map[git.Ref]git.Hash, error) {
	defer iter.Close()

	refs := make(map[git.Ref]git.Hash)
	tags := make(map[git.Ref]struct{})
	for iter.Next() {
		if iter.IsDereference() {
			// Dereferenced tag. This takes precedence over the previous hash stored in the map.
			if _, used := tags[iter.Ref()]; used {
				return refs, fmt.Errorf("multiple hashes found for tag %v", iter.Ref())
			}
			tags[iter.Ref()] = struct{}{}
		} else if _, exists := refs[iter.Ref()]; exists {
			return refs, fmt.Errorf("multiple hashes found for %v", iter.Ref())
		}
		refs[iter.Ref()] = iter.ObjectSHA1()
	}
	return refs, iter.Close()
}

// reverseFetchMap returns the refs last fetched from the remote.
// `refs/remotes/<name>/HEAD` is ignored.
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
		const remoteHeadPrefix = "refs/remotes/"
		const remoteHeadSuffix = "/HEAD"
		for ref, hash := range localRefs {
			if strings.HasPrefix(string(ref), remoteHeadPrefix) &&
				strings.HasSuffix(string(ref), remoteHeadSuffix) &&
				len(ref) > len(remoteHeadPrefix+remoteHeadSuffix) {
				continue
			}
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
