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
	"context"
	"errors"

	"gg-scm.io/pkg/internal/flag"
	"gg-scm.io/pkg/internal/git"
)

const updateSynopsis = "update working directory (or switch revisions)"

func update(ctx context.Context, cc *cmdContext, args []string) error {
	f := flag.NewFlagSet(true, "gg update [[-r] REV]", updateSynopsis+`

aliases: up, checkout, co

	Update the working directory to the specified revision. If no
	revision is specified, update to the tip of the upstream branch if
	it has the same name as the current branch or the tip of the push
	branch otherwise.

	If the commit is not a descendant or ancestor of the HEAD commit,
	the update is aborted.`)
	rev := f.String("r", "", "`rev`ision")
	if err := f.Parse(args); flag.IsHelp(err) {
		f.Help(cc.stdout)
		return nil
	} else if err != nil {
		return usagef("%v", err)
	}
	var r *git.Rev
	switch {
	case f.NArg() == 0 && *rev == "":
		cfg, err := cc.git.ReadConfig(ctx)
		if err != nil {
			return err
		}
		return updateCurrentBranch(ctx, cc.git, cfg)
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
	if b := r.Ref.Branch(); b != "" {
		return cc.git.CheckoutBranch(ctx, b, git.CheckoutOptions{
			Merge: true,
		})
	}
	return cc.git.CheckoutRev(ctx, r.Commit.String(), git.CheckoutOptions{
		Merge: true,
	})
}

// updateCurrentBranch fast-forwards the current branch while preserving local changes.
func updateCurrentBranch(ctx context.Context, g *git.Git, cfg *git.Config) error {
	// git merge --ff-only is insufficient, as it does not three-way merge
	// local modifications. We use some sneaky checkout invocations to get
	// around this.

	head, err := g.Head(ctx)
	if err != nil {
		return err
	}
	branch := head.Ref.Branch()
	if branch == "" {
		return errors.New("can't update with no branch checked out; run 'gg update BRANCH'")
	}
	target := targetForUpdate(cfg, branch)
	if target == "" {
		// No-op: nothing to update.
		return nil
	}

	if isAncestor, err := g.IsAncestor(ctx, head.Commit.String(), target); err != nil {
		return err
	} else if !isAncestor {
		return errors.New("upstream has diverged; run 'gg merge' or 'gg rebase'")
	}
	// Here's the trickiness: move the working copy to the given revision
	// while merging the local changes, then move the branch ref to match the
	// current revision. This is only really "safe" because of the ancestor
	// check before.
	if err := g.CheckoutRev(ctx, target, git.CheckoutOptions{Merge: true}); err != nil {
		return err
	}
	if err := g.NewBranch(ctx, branch, git.BranchOptions{Overwrite: true, Checkout: true}); err != nil {
		return err
	}
	return nil
}

// targetForUpdate returns the revision to use for
// fast-forwarding a branch. If targetForUpdate returns an empty
// string, it means that no target could be found.
func targetForUpdate(cfg *git.Config, branch string) string {
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
	return remote.MapFetch(remoteRef).String()
}
