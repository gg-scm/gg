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
	f := flag.NewFlagSet(true, "gg update [-m] [[-r] REV]", updateSynopsis+"\n\n"+
		"aliases: up, checkout, co")
	merge := f.Bool("m", false, "merge uncommitted changes")
	f.Alias("m", "merge")
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
		if !*merge {
			// Simple case: fast-forward merge.
			_, err := cc.git.Run(ctx, "merge", "--quiet", "--ff-only")
			return err
		}
		// Hard case: fast-forward merge with local changes.
		head, err := cc.git.Head(ctx)
		if err != nil {
			return err
		}
		if !head.Ref.IsBranch() {
			return errors.New("can't update to upstream with no branch checked out; run 'gg update BRANCH'")
		}
		if isAncestor, err := cc.git.IsAncestor(ctx, head.Commit.String(), "@{upstream}"); err != nil {
			return err
		} else if !isAncestor {
			return errors.New("upstream has diverged; run 'gg merge' or 'gg rebase'")
		}
		// Here's the trickiness: move the working copy to the given revision
		// while merging the local changes, then move the branch ref to match the
		// current revision. This is only really "safe" because of the ancestor
		// check before.
		if err := cc.git.CheckoutRev(ctx, "@{upstream}", git.CheckoutOptions{Merge: true}); err != nil {
			return err
		}
		if err := cc.git.NewBranch(ctx, head.Ref.Branch(), git.BranchOptions{Overwrite: true, Checkout: true}); err != nil {
			return err
		}
		return nil
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
			Merge: *merge,
		})
	}
	return cc.git.CheckoutRev(ctx, r.Commit.String(), git.CheckoutOptions{
		Merge: *merge,
	})
}
