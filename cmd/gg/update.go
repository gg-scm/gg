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

	"gg-scm.io/pkg/git"
	"gg-scm.io/tool/internal/flag"
)

const updateSynopsis = "update working directory (or switch revisions)"

func update(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg update [--clean] [[-r] REV]", updateSynopsis+`

aliases: up, checkout, co

	Update the working directory to the specified revision. If no
	revision is specified, update to the tip of the upstream branch if
	it has the same name as the current branch or the tip of the push
	branch otherwise.

	If the commit is not a descendant or ancestor of the HEAD commit,
	the update is aborted.`)
	rev := f.String("r", "", "`rev`ision")
	clean := f.Bool("clean", false, "discard uncommitted changes (no backup)")
	f.Alias("clean", "C")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	behavior := git.MergeLocal
	if *clean {
		behavior = git.DiscardLocal
	}
	var r *git.Rev
	switch {
	case f.NArg() == 0 && *rev == "":
		cfg, err := cc.git.ReadConfig(ctx)
		if err != nil {
			return err
		}
		ref, err := cc.git.HeadRef(ctx)
		if err != nil {
			return err
		}
		branch := ref.Branch()
		if branch == "" {
			return errors.New("can't update with no branch checked out; run 'gg update BRANCH'")
		}
		target := targetForUpdate(cfg, branch)
		return updateToBranch(ctx, cc.git, branch, target, behavior)
	case f.NArg() == 0 && *rev != "":
		var err error
		r, err = cc.git.ParseRev(ctx, *rev)
		if err != nil {
			return err
		}
	case f.NArg() == 1 && *rev == "":
		var err error
		r, err = cc.git.ParseRev(ctx, f.Arg(0))
		if err != nil {
			return err
		}
	default:
		return usagef("can pass only one revision")
	}
	b := r.Ref.Branch()
	if b == "" {
		return cc.git.CheckoutRev(ctx, r.Commit.String(), git.CheckoutOptions{
			ConflictBehavior: behavior,
		})
	}
	cfg, err := cc.git.ReadConfig(ctx)
	if err != nil {
		return err
	}
	target := targetForUpdate(cfg, b)
	return updateToBranch(ctx, cc.git, b, target, behavior)
}

// updateToBranch switches to another branch and fast-forwards it.
// If branch is the empty string, then updateToBranch does nothing.
// behavior must be one of MergeLocal or DiscardLocal or updateToBranch
// returns an error.
func updateToBranch(ctx context.Context, g *git.Git, branch string, target git.Ref, behavior git.CheckoutConflictBehavior) error {
	if behavior != git.MergeLocal && behavior != git.DiscardLocal {
		return fmt.Errorf("updateToBranch takes MergeLocal or DiscardLocal as behaviors (got %v)", behavior)
	}
	if branch == "" {
		return nil
	}
	if target == "" {
		// No fast-forward target, so just do a simple checkout.
		return g.CheckoutBranch(ctx, branch, git.CheckoutOptions{ConflictBehavior: behavior})
	}
	if _, err := g.ParseRev(ctx, target.String()); err != nil {
		// Remote-tracking branch does not exist, so just do a simple checkout.
		return g.CheckoutBranch(ctx, branch, git.CheckoutOptions{ConflictBehavior: behavior})
	}
	if isAheadOfTarget, err := g.IsAncestor(ctx, target.String(), git.BranchRef(branch).String()); err != nil {
		return err
	} else if isAheadOfTarget {
		return g.CheckoutBranch(ctx, branch, git.CheckoutOptions{ConflictBehavior: behavior})
	}

	// Check out and fast-forward.
	//
	// git merge --ff-only is insufficient, as it does not three-way merge
	// local modifications. We use some sneaky checkout invocations to get
	// around this.

	if isAncestor, err := g.IsAncestor(ctx, git.BranchRef(branch).String(), target.String()); err != nil {
		return err
	} else if !isAncestor {
		return errors.New("upstream has diverged; run 'gg merge' or 'gg rebase'")
	}
	// Here's the trickiness: move the working copy to the given revision
	// while merging the local changes, then move the branch ref to match the
	// current revision. This is only really "safe" because of the ancestor
	// check before.
	if err := g.CheckoutRev(ctx, target.String(), git.CheckoutOptions{ConflictBehavior: behavior}); err != nil {
		return err
	}
	if err := g.NewBranch(ctx, branch, git.BranchOptions{Overwrite: true, Checkout: true}); err != nil {
		return err
	}
	return nil
}

// targetForUpdate returns the revision to use for fast-forwarding a
// branch. If targetForUpdate returns an empty string, it means that no
// target could be found. The ref returned may not exist.
func targetForUpdate(cfg *git.Config, branch string) git.Ref {
	if branch == "" {
		return ""
	}
	remotes := cfg.ListRemotes()
	branchRef := git.BranchRef(branch)
	var remoteName string
	var remoteRef git.Ref
	if merge := git.Ref(cfg.Value("branch." + branch + ".merge")); merge == branchRef {
		// Upstream branch matches; use upstream remote-tracking branch.
		remoteName = cfg.Value("branch." + branch + ".remote")
		remoteRef = merge
	} else {
		// Default: use push remote-tracking branch.
		var err error
		remoteName, err = inferPushRepo(cfg, branch)
		if err != nil {
			return ""
		}
		remoteRef = branchRef
	}
	if remoteName == "" {
		return ""
	}
	remote := remotes[remoteName]
	if remote == nil {
		return ""
	}
	return remote.MapFetch(remoteRef)
}
